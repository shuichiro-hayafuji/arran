package postgres

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// These types implement an offline reconciliation plan only. They do not
// access storage, authenticate a checkpoint, enforce freshness or stop an API.
// No production path may consume this plan until an independent, authenticated
// checkpoint and the ledger's current head have been verified by its caller.
type withdrawalLedgerEntry struct {
	sequence uint64
	previous [32]byte
	state    string // prepared or completed
	identity withdrawalIdentity
}
type withdrawalLedgerCheckpoint struct {
	sequence uint64
	digest   [32]byte
}
type withdrawalRestoreUser struct {
	userID    int64
	username  string
	createdAt time.Time
}
type withdrawalReplayPlan struct{ deleteIdentities []withdrawalIdentity }

var errWithdrawalLedgerUnsafe = errors.New("withdrawal ledger cannot be reconciled safely")

// ledgerEntryDigest uses domain separation and length-delimited UTF-8 fields;
// timestamps are canonical UTC instants. An external trusted checkpoint must
// authenticate the full chain head; this unkeyed hash is not authentication.
func ledgerEntryDigest(e withdrawalLedgerEntry) [32]byte {
	h := sha256.New()
	field := func(v string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(v)))
		_, _ = h.Write(n[:])
		_, _ = h.Write([]byte(v))
	}
	field("arran-withdrawal-ledger-v1")
	field(fmt.Sprint(e.sequence))
	field(hex.EncodeToString(e.previous[:]))
	field(e.state)
	field(fmt.Sprint(e.identity.userID))
	field(e.identity.username)
	field(e.identity.createdAt.UTC().Format(time.RFC3339Nano))
	field(e.identity.requestedAt.UTC().Format(time.RFC3339Nano))
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func validWithdrawalIdentity(id withdrawalIdentity) bool {
	return id.userID > 0 && id.username != "" && !id.createdAt.IsZero() && !id.requestedAt.IsZero()
}
func sameWithdrawalIdentity(a, b withdrawalIdentity) bool {
	return a.userID == b.userID && a.username == b.username && a.createdAt.Equal(b.createdAt) && a.requestedAt.Equal(b.requestedAt)
}

// planWithdrawalReplay accepts an already authenticated, independent and fresh
// checkpoint from the future operator integration; this function cannot verify
// those claims. An empty ledger still requires an independently authenticated
// sequence-zero checkpoint; absence/unavailability must never be passed as one.
// The database user list must come from one quiescent consistent snapshot.
// A prepared operation always blocks, even if the restored user is absent.
func planWithdrawalReplay(entries []withdrawalLedgerEntry, checkpoint *withdrawalLedgerCheckpoint, users []withdrawalRestoreUser) (withdrawalReplayPlan, error) {
	var empty withdrawalReplayPlan
	if entries == nil || checkpoint == nil || users == nil {
		return empty, errWithdrawalLedgerUnsafe
	}
	var previous [32]byte
	type state struct {
		identity  withdrawalIdentity
		completed bool
	}
	states := map[int64]state{}
	for i, e := range entries {
		if e.sequence != uint64(i+1) || e.previous != previous || !validWithdrawalIdentity(e.identity) {
			return empty, errWithdrawalLedgerUnsafe
		}
		prior, exists := states[e.identity.userID]
		switch e.state {
		case "prepared":
			if exists {
				return empty, errWithdrawalLedgerUnsafe
			}
			states[e.identity.userID] = state{identity: e.identity}
		case "completed":
			if !exists || prior.completed || !sameWithdrawalIdentity(prior.identity, e.identity) {
				return empty, errWithdrawalLedgerUnsafe
			}
			states[e.identity.userID] = state{identity: e.identity, completed: true}
		default:
			return empty, errWithdrawalLedgerUnsafe
		}
		previous = ledgerEntryDigest(e)
	}
	if checkpoint.sequence != uint64(len(entries)) || checkpoint.digest != previous {
		return empty, errWithdrawalLedgerUnsafe
	}
	byID := map[int64]withdrawalRestoreUser{}
	byName := map[string]withdrawalRestoreUser{}
	for _, u := range users {
		if u.userID <= 0 || u.username == "" || u.createdAt.IsZero() {
			return empty, errWithdrawalLedgerUnsafe
		}
		if _, ok := byID[u.userID]; ok {
			return empty, errWithdrawalLedgerUnsafe
		}
		if _, ok := byName[u.username]; ok {
			return empty, errWithdrawalLedgerUnsafe
		}
		byID[u.userID] = u
		byName[u.username] = u
	}
	plan := withdrawalReplayPlan{}
	// Entries define stable ordering for deterministic recovery and tests.
	for _, e := range entries {
		if e.state != "prepared" {
			continue
		}
		st := states[e.identity.userID]
		if !st.completed {
			return empty, errWithdrawalLedgerUnsafe
		}
		current, found := byID[e.identity.userID]
		if !found {
			if reused, exists := byName[e.identity.username]; exists {
				// A later account may legitimately reuse a username. It is safe
				// only when the restored account matches a different completed
				// ledger identity exactly; unknown reuse requires investigation.
				other, recorded := states[reused.userID]
				if !recorded || !other.completed || other.identity.username != reused.username || !other.identity.createdAt.Equal(reused.createdAt) {
					return empty, errWithdrawalLedgerUnsafe
				}
			}
			continue
		}
		if current.username != e.identity.username || !current.createdAt.Equal(e.identity.createdAt) {
			return empty, errWithdrawalLedgerUnsafe
		}
		plan.deleteIdentities = append(plan.deleteIdentities, e.identity)
	}
	return plan, nil
}
