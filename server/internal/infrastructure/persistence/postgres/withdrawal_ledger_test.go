package postgres

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestWithdrawalLedgerPlan(t *testing.T) {
	id := withdrawalIdentity{userID: 7, username: "test-user", createdAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), requestedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}
	user := withdrawalRestoreUser{userID: id.userID, username: id.username, createdAt: id.createdAt}
	prepare := withdrawalLedgerEntry{sequence: 1, state: "prepared", identity: id}
	complete := withdrawalLedgerEntry{sequence: 2, previous: ledgerEntryDigest(prepare), state: "completed", identity: id}
	valid := []withdrawalLedgerEntry{prepare, complete}
	head := withdrawalLedgerCheckpoint{sequence: 2, digest: ledgerEntryDigest(complete)}
	plan, err := planWithdrawalReplay(valid, &head, []withdrawalRestoreUser{user})
	if err != nil || len(plan.deleteIdentities) != 1 || !sameWithdrawalIdentity(plan.deleteIdentities[0], id) {
		t.Fatalf("valid: %#v %v", plan, err)
	}
	again, err := planWithdrawalReplay(valid, &head, []withdrawalRestoreUser{user})
	if err != nil || !reflect.DeepEqual(plan, again) {
		t.Fatalf("non deterministic replay: %#v %v", again, err)
	}
	// A completed deletion whose identity does not exist in this snapshot needs
	// no database deletion; proof of the snapshot and checkpoint is external.
	plan, err = planWithdrawalReplay(valid, &head, []withdrawalRestoreUser{})
	if err != nil || len(plan.deleteIdentities) != 0 {
		t.Fatalf("absent: %#v %v", plan, err)
	}
	if _, err := planWithdrawalReplay(valid, &head, nil); !errors.Is(err, errWithdrawalLedgerUnsafe) {
		t.Fatalf("missing user snapshot accepted: %v", err)
	}
	emptyHead := withdrawalLedgerCheckpoint{}
	if _, err := planWithdrawalReplay(nil, &emptyHead, []withdrawalRestoreUser{}); !errors.Is(err, errWithdrawalLedgerUnsafe) {
		t.Fatalf("unread ledger accepted: %v", err)
	}
	if p, err := planWithdrawalReplay([]withdrawalLedgerEntry{}, &emptyHead, []withdrawalRestoreUser{}); err != nil || len(p.deleteIdentities) != 0 {
		t.Fatalf("verified empty snapshot: %v", err)
	}
	if _, err := planWithdrawalReplay(nil, nil, nil); !errors.Is(err, errWithdrawalLedgerUnsafe) {
		t.Fatalf("missing checkpoint accepted: %v", err)
	}
	cases := []struct {
		name    string
		entries []withdrawalLedgerEntry
		head    withdrawalLedgerCheckpoint
		users   []withdrawalRestoreUser
	}{
		{"missing ledger with nonzero checkpoint", []withdrawalLedgerEntry{}, head, []withdrawalRestoreUser{}},
		{"truncated", valid[:1], head, []withdrawalRestoreUser{user}},
		{"incomplete", valid[:1], withdrawalLedgerCheckpoint{sequence: 1, digest: ledgerEntryDigest(prepare)}, []withdrawalRestoreUser{user}},
		{"incomplete even when user absent", valid[:1], withdrawalLedgerCheckpoint{sequence: 1, digest: ledgerEntryDigest(prepare)}, []withdrawalRestoreUser{}},
		{"out of order", []withdrawalLedgerEntry{complete, prepare}, head, []withdrawalRestoreUser{user}},
		{"duplicate", []withdrawalLedgerEntry{prepare, prepare}, head, []withdrawalRestoreUser{user}},
		{"stale head", valid, withdrawalLedgerCheckpoint{sequence: 1, digest: ledgerEntryDigest(prepare)}, []withdrawalRestoreUser{user}},
		{"wrong digest", valid, withdrawalLedgerCheckpoint{sequence: 2}, []withdrawalRestoreUser{user}},
		{"wrong username", valid, head, []withdrawalRestoreUser{{userID: id.userID, username: "other", createdAt: id.createdAt}}},
		{"reused id", valid, head, []withdrawalRestoreUser{{userID: id.userID, username: id.username, createdAt: id.createdAt.Add(time.Hour)}}},
		{"reused username", valid, head, []withdrawalRestoreUser{{userID: 8, username: id.username, createdAt: id.createdAt}}},
		{"duplicate database identity", valid, head, []withdrawalRestoreUser{user, user}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := planWithdrawalReplay(tc.entries, &tc.head, tc.users)
			if !errors.Is(err, errWithdrawalLedgerUnsafe) {
				t.Fatalf("expected failure: %v", err)
			}
		})
	}
	// Even a matching externally supplied head cannot make a bad internal
	// previous link acceptable.
	badPrevious := complete
	badPrevious.previous = [32]byte{}
	badPreviousHead := withdrawalLedgerCheckpoint{sequence: 2, digest: ledgerEntryDigest(badPrevious)}
	if _, err := planWithdrawalReplay([]withdrawalLedgerEntry{prepare, badPrevious}, &badPreviousHead, []withdrawalRestoreUser{user}); !errors.Is(err, errWithdrawalLedgerUnsafe) {
		t.Fatalf("bad previous link accepted: %v", err)
	}
	// Different users may reuse the same username at different times. The old
	// absent identity must not cause the matching later identity to be deleted
	// under the old identity; each record retains its own ID and creation time.
	id2 := id
	id2.userID = 8
	id2.createdAt = id.createdAt.Add(time.Hour)
	id2.requestedAt = id.requestedAt.Add(time.Hour)
	p2 := withdrawalLedgerEntry{sequence: 2, previous: ledgerEntryDigest(prepare), state: "prepared", identity: id2}
	c1 := withdrawalLedgerEntry{sequence: 3, previous: ledgerEntryDigest(p2), state: "completed", identity: id}
	c2 := withdrawalLedgerEntry{sequence: 4, previous: ledgerEntryDigest(c1), state: "completed", identity: id2}
	interleaved := []withdrawalLedgerEntry{prepare, p2, c1, c2}
	interHead := withdrawalLedgerCheckpoint{sequence: 4, digest: ledgerEntryDigest(c2)}
	newUser := withdrawalRestoreUser{userID: id2.userID, username: id2.username, createdAt: id2.createdAt}
	p, err := planWithdrawalReplay(interleaved, &interHead, []withdrawalRestoreUser{newUser})
	if err != nil || len(p.deleteIdentities) != 1 || !sameWithdrawalIdentity(p.deleteIdentities[0], id2) {
		t.Fatalf("reused username: %#v %v", p, err)
	}
	partialHead := withdrawalLedgerCheckpoint{sequence: 3, digest: ledgerEntryDigest(c1)}
	if _, err := planWithdrawalReplay(interleaved[:3], &partialHead, []withdrawalRestoreUser{newUser}); !errors.Is(err, errWithdrawalLedgerUnsafe) {
		t.Fatalf("interleaved pending accepted: %v", err)
	}
	// The first completed user is present and would be a deletion candidate,
	// but a different user's pending operation must return an empty plan.
	id3 := id2
	id3.userID = 9
	id3.username = "different-user"
	p3 := withdrawalLedgerEntry{sequence: 2, previous: ledgerEntryDigest(prepare), state: "prepared", identity: id3}
	c3 := withdrawalLedgerEntry{sequence: 3, previous: ledgerEntryDigest(p3), state: "completed", identity: id}
	partial := []withdrawalLedgerEntry{prepare, p3, c3}
	partialVerified := withdrawalLedgerCheckpoint{sequence: 3, digest: ledgerEntryDigest(c3)}
	partialPlan, partialErr := planWithdrawalReplay(partial, &partialVerified, []withdrawalRestoreUser{user})
	if !errors.Is(partialErr, errWithdrawalLedgerUnsafe) || len(partialPlan.deleteIdentities) != 0 {
		t.Fatalf("partial plan escaped: %#v %v", partialPlan, partialErr)
	}
	tampered := append([]withdrawalLedgerEntry(nil), valid...)
	tampered[1].identity.username = "other"
	_, err = planWithdrawalReplay(tampered, &head, []withdrawalRestoreUser{user})
	if !errors.Is(err, errWithdrawalLedgerUnsafe) {
		t.Fatalf("tampered entry accepted: %v", err)
	}
}
