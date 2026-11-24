package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type fakeRestoreTx struct {
	usersValue   []withdrawalRestoreUser
	fail         string
	failReplayAt int
	replayCalls  int
	events       *[]string
	replayed     []withdrawalIdentity
}

func (t *fakeRestoreTx) validate(_ context.Context, candidate string) error {
	*t.events = append(*t.events, "validate:"+candidate)
	if t.fail == "validate" {
		return errors.New("wrong candidate")
	}
	return nil
}
func (t *fakeRestoreTx) users(context.Context) ([]withdrawalRestoreUser, error) {
	*t.events = append(*t.events, "users")
	if t.fail == "users" {
		return nil, errors.New("snapshot failed")
	}
	return t.usersValue, nil
}
func (t *fakeRestoreTx) replay(_ context.Context, _ withdrawalReplayPlan, id withdrawalIdentity) error {
	*t.events = append(*t.events, "replay")
	t.replayCalls++
	if t.fail == "replay" || t.replayCalls == t.failReplayAt {
		return errors.New("delete failed")
	}
	t.replayed = append(t.replayed, id)
	return nil
}
func (t *fakeRestoreTx) commit() error {
	*t.events = append(*t.events, "commit")
	if t.fail == "commit" {
		return errors.New("commit unknown")
	}
	return nil
}
func (t *fakeRestoreTx) rollback() error { *t.events = append(*t.events, "rollback"); return nil }

type fakeRestoreDB struct {
	id   string
	tx   *fakeRestoreTx
	fail bool
}

func (d *fakeRestoreDB) candidateID() string { return d.id }
func (d *fakeRestoreDB) begin(context.Context) (withdrawalRestoreTx, error) {
	*d.tx.events = append(*d.tx.events, "begin")
	if d.fail {
		return nil, errors.New("begin failure")
	}
	return d.tx, nil
}

type fakeRestoreCutover struct {
	events      *[]string
	safetyCalls int
	failSafety  int
	failSwitch  bool
	candidate   string
}

// Simulates a real client that reuses the same checkpoint memory on each
// network read, then observes another completed deletion between reads.
type aliasRestoreLedger struct {
	first, next []withdrawalLedgerEntry
	head        withdrawalLedgerCheckpoint
	reads       int
}

func (l *aliasRestoreLedger) read(context.Context) ([]withdrawalLedgerEntry, *withdrawalLedgerCheckpoint, error) {
	l.reads++
	if l.reads == 1 {
		return l.first, &l.head, nil
	}
	last := l.next[len(l.next)-1]
	l.head = withdrawalLedgerCheckpoint{sequence: last.sequence, digest: ledgerEntryDigest(last)}
	return l.next, &l.head, nil
}

func (*aliasRestoreLedger) appendDurable(context.Context, withdrawalLedgerCheckpoint, withdrawalLedgerEntry) error {
	return errors.New("unexpected append")
}

func (c *fakeRestoreCutover) checkOtherSafety(_ context.Context, candidate string) error {
	*c.events = append(*c.events, "safety:"+candidate)
	c.safetyCalls++
	if candidate != c.candidate || c.safetyCalls == c.failSafety {
		return errors.New("safety unavailable")
	}
	return nil
}
func (c *fakeRestoreCutover) switchWithoutPublishing(_ context.Context, candidate string) error {
	*c.events = append(*c.events, "switch:"+candidate)
	if candidate != c.candidate || c.failSwitch {
		return errors.New("cutover unknown")
	}
	return nil
}

func TestWithdrawalRestoreProtocol(t *testing.T) {
	ctx := context.Background()
	id := withdrawalIdentity{userID: 7, username: "test", createdAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), requestedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}
	p := withdrawalLedgerEntry{sequence: 1, state: "prepared", identity: id}
	c := withdrawalLedgerEntry{sequence: 2, previous: ledgerEntryDigest(p), state: "completed", identity: id}
	newLedger := func() *protocolLedger {
		return &protocolLedger{entries: []withdrawalLedgerEntry{p, c}, head: withdrawalLedgerCheckpoint{sequence: 2, digest: ledgerEntryDigest(c)}}
	}
	makeTest := func() (*[]string, *fakeRestoreDB, *fakeRestoreCutover) {
		events := []string{}
		tx := &fakeRestoreTx{usersValue: []withdrawalRestoreUser{{userID: id.userID, username: id.username, createdAt: id.createdAt}}, events: &events}
		return &events, &fakeRestoreDB{id: "arran_restore_test", tx: tx}, &fakeRestoreCutover{events: &events, candidate: "arran_restore_test"}
	}
	t.Run("reconcile before nonpublic switch", func(t *testing.T) {
		events, db, cutover := makeTest()
		if err := reconcileAndSwitchWithdrawalRestore(ctx, &protocolGuard{}, newLedger(), db, cutover); err != nil {
			t.Fatal(err)
		}
		want := []string{"safety:arran_restore_test", "begin", "validate:arran_restore_test", "users", "replay", "commit", "safety:arran_restore_test", "switch:arran_restore_test", "rollback"}
		if !reflect.DeepEqual(*events, want) || len(db.tx.replayed) != 1 || !sameWithdrawalIdentity(db.tx.replayed[0], id) {
			t.Fatalf("events=%#v replay=%#v", *events, db.tx.replayed)
		}
	})
	for _, tc := range []struct {
		name                                       string
		txFail                                     string
		readFail, staleRead, guardFail, safetyFail int
		beginFail, missingUsers, missingCandidate  bool
	}{
		{"no candidate", "", 0, 0, 0, 0, false, false, true},
		{"writers active", "", 0, 0, 1, 0, false, false, false},
		{"consent proof unavailable", "", 0, 0, 0, 1, false, false, false},
		{"ledger unavailable", "", 1, 0, 0, 0, false, false, false},
		{"begin failed", "", 0, 0, 0, 0, true, false, false},
		{"candidate mismatch", "validate", 0, 0, 0, 0, false, false, false},
		{"snapshot failed", "users", 0, 0, 0, 0, false, false, false},
		{"snapshot missing", "", 0, 0, 0, 0, false, true, false},
		{"delete failed", "replay", 0, 0, 0, 0, false, false, false},
		{"head changed before commit", "", 0, 2, 0, 0, false, false, false},
		{"writers restarted before commit", "", 0, 0, 2, 0, false, false, false},
		{"commit unknown", "commit", 0, 0, 0, 0, false, false, false},
		{"head changed after commit", "", 0, 3, 0, 0, false, false, false},
		{"writers restarted after commit", "", 0, 0, 3, 0, false, false, false},
		{"consent proof expired", "", 0, 0, 0, 2, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events, db, cutover := makeTest()
			l := newLedger()
			l.failRead, l.staleRead = tc.readFail, tc.staleRead
			db.tx.fail, db.fail = tc.txFail, tc.beginFail
			if tc.missingUsers {
				db.tx.usersValue = nil
			}
			if tc.missingCandidate {
				db.id = ""
			}
			cutover.failSafety = tc.safetyFail
			if err := reconcileAndSwitchWithdrawalRestore(ctx, &protocolGuard{fail: tc.guardFail}, l, db, cutover); err == nil {
				t.Fatal("unexpected success")
			}
			for _, event := range *events {
				if event == "switch:arran_restore_test" {
					t.Fatalf("unsafe switch: %#v", *events)
				}
			}
		})
	}
	t.Run("pending ledger never switches", func(t *testing.T) {
		events, db, cutover := makeTest()
		l := newLedger()
		l.entries = l.entries[:1]
		l.head = withdrawalLedgerCheckpoint{sequence: 1, digest: ledgerEntryDigest(p)}
		if err := reconcileAndSwitchWithdrawalRestore(ctx, &protocolGuard{}, l, db, cutover); err == nil {
			t.Fatal("pending accepted")
		}
		for _, e := range *events {
			if e == "commit" || e == "switch:arran_restore_test" {
				t.Fatalf("unsafe event: %#v", *events)
			}
		}
	})
	t.Run("shared checkpoint updated on reread cannot change the initial head", func(t *testing.T) {
		events, db, cutover := makeTest()
		other := id
		other.userID, other.username = 8, "another"
		p2 := withdrawalLedgerEntry{sequence: 3, previous: ledgerEntryDigest(c), state: "prepared", identity: other}
		c2 := withdrawalLedgerEntry{sequence: 4, previous: ledgerEntryDigest(p2), state: "completed", identity: other}
		l := &aliasRestoreLedger{first: []withdrawalLedgerEntry{p, c}, next: []withdrawalLedgerEntry{p, c, p2, c2}, head: withdrawalLedgerCheckpoint{sequence: 2, digest: ledgerEntryDigest(c)}}
		db.tx.usersValue = append(db.tx.usersValue, withdrawalRestoreUser{userID: other.userID, username: other.username, createdAt: other.createdAt})
		if err := reconcileAndSwitchWithdrawalRestore(ctx, &protocolGuard{}, l, db, cutover); !errors.Is(err, errWithdrawalLedgerUnsafe) {
			t.Fatalf("changed aliased head accepted: %v", err)
		}
		if db.tx.replayCalls != 1 || (*events)[len(*events)-1] != "rollback" {
			t.Fatalf("reconciliation not rolled back: %#v", *events)
		}
		for _, e := range *events {
			if e == "commit" || e == "switch:arran_restore_test" {
				t.Fatalf("changed head escaped: %#v", *events)
			}
		}
	})
	t.Run("second deletion failure rolls back first and never switches", func(t *testing.T) {
		events, db, cutover := makeTest()
		other := id
		other.userID = 8
		other.username = "another"
		p2 := withdrawalLedgerEntry{sequence: 3, previous: ledgerEntryDigest(c), state: "prepared", identity: other}
		c2 := withdrawalLedgerEntry{sequence: 4, previous: ledgerEntryDigest(p2), state: "completed", identity: other}
		l := &protocolLedger{entries: []withdrawalLedgerEntry{p, c, p2, c2}, head: withdrawalLedgerCheckpoint{sequence: 4, digest: ledgerEntryDigest(c2)}}
		db.tx.usersValue = append(db.tx.usersValue, withdrawalRestoreUser{userID: other.userID, username: other.username, createdAt: other.createdAt})
		db.tx.failReplayAt = 2
		if err := reconcileAndSwitchWithdrawalRestore(ctx, &protocolGuard{}, l, db, cutover); err == nil {
			t.Fatal("partial deletion accepted")
		}
		if db.tx.replayCalls != 2 || len(db.tx.replayed) != 1 || (*events)[len(*events)-1] != "rollback" {
			t.Fatalf("missing rollback: %#v", *events)
		}
		for _, e := range *events {
			if e == "commit" || e == "switch:arran_restore_test" {
				t.Fatalf("partial deletion escaped: %#v", *events)
			}
		}
	})
	t.Run("unknown cutover outcome is never reported as success", func(t *testing.T) {
		_, db, cutover := makeTest()
		cutover.failSwitch = true
		if err := reconcileAndSwitchWithdrawalRestore(ctx, &protocolGuard{}, newLedger(), db, cutover); err == nil {
			t.Fatal("unknown cutover accepted")
		}
	})
}
