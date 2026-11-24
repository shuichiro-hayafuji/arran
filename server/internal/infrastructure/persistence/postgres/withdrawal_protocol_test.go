package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type protocolGuard struct {
	calls int
	fail  int
}

func (g *protocolGuard) check(context.Context) error {
	g.calls++
	if g.calls == g.fail {
		return errors.New("writers not stopped")
	}
	return nil
}

type protocolLedger struct {
	entries              []withdrawalLedgerEntry
	head                 withdrawalLedgerCheckpoint
	appendCalls          int
	failAppend           int
	failRead             int
	readCalls            int
	staleRead            int
	persistBeforeFailure bool
}

func (l *protocolLedger) read(context.Context) ([]withdrawalLedgerEntry, *withdrawalLedgerCheckpoint, error) {
	l.readCalls++
	if l.readCalls == l.failRead {
		return nil, nil, errors.New("read failure")
	}
	if l.readCalls == l.staleRead {
		return []withdrawalLedgerEntry{}, &withdrawalLedgerCheckpoint{}, nil
	}
	entries := make([]withdrawalLedgerEntry, len(l.entries))
	copy(entries, l.entries)
	head := l.head
	return entries, &head, nil
}
func (l *protocolLedger) appendDurable(_ context.Context, expected withdrawalLedgerCheckpoint, entry withdrawalLedgerEntry) error {
	l.appendCalls++
	if expected != l.head {
		return errors.New("head changed")
	}
	if l.appendCalls == l.failAppend {
		if l.persistBeforeFailure {
			l.entries = append(l.entries, entry)
			l.head = withdrawalLedgerCheckpoint{sequence: entry.sequence, digest: ledgerEntryDigest(entry)}
		}
		return errors.New("unknown append result")
	}
	l.entries = append(l.entries, entry)
	l.head = withdrawalLedgerCheckpoint{sequence: entry.sequence, digest: ledgerEntryDigest(entry)}
	return nil
}

type protocolDB struct {
	calls         int
	preflightErr  error
	preflightSeen int
	err           error
	id            withdrawalIdentity
}

func (d *protocolDB) preflight(_ context.Context, _ withdrawalIdentity) error {
	d.preflightSeen++
	return d.preflightErr
}

func (d *protocolDB) deleteAndCommit(_ context.Context, id withdrawalIdentity) error {
	d.calls++
	d.id = id
	return d.err
}

func TestWithdrawalProtocol(t *testing.T) {
	ctx := context.Background()
	id := withdrawalIdentity{userID: 7, username: "test-user", createdAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), requestedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}
	newLedger := func() *protocolLedger { return &protocolLedger{entries: []withdrawalLedgerEntry{}} }
	t.Run("prepare before database and durable completion", func(t *testing.T) {
		g, l, d := &protocolGuard{}, newLedger(), &protocolDB{}
		if err := executeWithdrawalProtocol(ctx, g, l, d, id); err != nil {
			t.Fatal(err)
		}
		if g.calls != 3 || d.preflightSeen != 1 || d.calls != 1 || !sameWithdrawalIdentity(d.id, id) || len(l.entries) != 2 || l.entries[0].state != "prepared" || l.entries[1].state != "completed" {
			t.Fatalf("unsafe sequence: %#v %#v %#v", g, l, d)
		}
		if _, err := planWithdrawalReplay(l.entries, &l.head, []withdrawalRestoreUser{}); err != nil {
			t.Fatal(err)
		}
		if err := executeWithdrawalProtocol(ctx, g, l, d, id); !errors.Is(err, errWithdrawalProtocolUnsafe) || d.calls != 1 {
			t.Fatalf("duplicate operation: %v", err)
		}
	})
	for _, tc := range []struct {
		name                                       string
		guardFail, appendFail, readFail, staleRead int
		dbErr                                      bool
		calls                                      int
		states                                     []string
	}{
		{"writer active", 1, 0, 0, 0, false, 0, nil},
		{"initial read error", 0, 0, 1, 0, false, 0, nil},
		{"preparation unknown", 0, 1, 0, 0, false, 0, nil},
		{"preparation readback error", 0, 0, 2, 0, false, 0, []string{"prepared"}},
		{"preparation stale readback", 0, 0, 0, 2, false, 0, []string{"prepared"}},
		{"writers restarted before preparation", 2, 0, 0, 0, false, 0, nil},
		{"writers restarted before deletion", 3, 0, 0, 0, false, 0, []string{"prepared"}},
		{"database commit unknown", 0, 0, 0, 0, true, 1, []string{"prepared"}},
		{"completion unknown", 0, 2, 0, 0, false, 1, []string{"prepared"}},
		{"completion readback error", 0, 0, 3, 0, false, 1, []string{"prepared", "completed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, l, d := &protocolGuard{fail: tc.guardFail}, newLedger(), &protocolDB{}
			l.failAppend, l.failRead, l.staleRead = tc.appendFail, tc.readFail, tc.staleRead
			if tc.dbErr {
				d.err = errors.New("commit lost")
			}
			if err := executeWithdrawalProtocol(ctx, g, l, d, id); err == nil {
				t.Fatal("unexpected success")
			}
			if d.calls != tc.calls {
				t.Fatalf("database called %d times", d.calls)
			}
			var states []string
			for _, e := range l.entries {
				states = append(states, e.state)
			}
			if !reflect.DeepEqual(states, tc.states) {
				t.Fatalf("unexpected states: %#v", states)
			}
		})
	}
	t.Run("definite preflight failure records nothing and can be retried", func(t *testing.T) {
		l, d := newLedger(), &protocolDB{preflightErr: errors.New("unsafe schema")}
		if err := executeWithdrawalProtocol(ctx, &protocolGuard{}, l, d, id); err == nil || d.calls != 0 || len(l.entries) != 0 {
			t.Fatalf("failed preflight recorded preparation: %v", err)
		}
		d.preflightErr = nil
		if err := executeWithdrawalProtocol(ctx, &protocolGuard{}, l, d, id); err != nil || d.calls != 1 {
			t.Fatalf("retry after failed preflight: %v", err)
		}
	})
	t.Run("unknown preparation already durable blocks retries", func(t *testing.T) {
		l, d := newLedger(), &protocolDB{}
		l.failAppend, l.persistBeforeFailure = 1, true
		if err := executeWithdrawalProtocol(ctx, &protocolGuard{}, l, d, id); err == nil || d.calls != 0 {
			t.Fatalf("unknown append was accepted: %v", err)
		}
		l.failAppend = 0
		if err := executeWithdrawalProtocol(ctx, &protocolGuard{}, l, d, id); !errors.Is(err, errWithdrawalProtocolUnsafe) || d.calls != 0 {
			t.Fatalf("pending retry was accepted: %v", err)
		}
	})
	t.Run("pending operation blocks other deletion", func(t *testing.T) {
		l := newLedger()
		e := withdrawalLedgerEntry{sequence: 1, state: "prepared", identity: id}
		l.entries = append(l.entries, e)
		l.head = withdrawalLedgerCheckpoint{sequence: 1, digest: ledgerEntryDigest(e)}
		d := &protocolDB{}
		other := id
		other.userID++
		if err := executeWithdrawalProtocol(ctx, &protocolGuard{}, l, d, other); !errors.Is(err, errWithdrawalProtocolUnsafe) || d.calls != 0 {
			t.Fatalf("pending accepted: %v", err)
		}
	})
}
