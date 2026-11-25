package restoreevidence

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// WithdrawalV2 is an offline proposal, unsupported by the DB or DynamoDB
// adapters. RequestedAt is canonical UTC RFC3339Nano; its origin is unverified.
type WithdrawalV2 struct {
	Version                                                                       uint32
	LedgerID                                                                      string
	Generation                                                                    uint64
	OwnerID, OperationID, RequestOperationID, Kind, RequestedAt, Previous, Digest string
}

// WithdrawalV2Digest uses a distinct domain and preserves the Entry digest.
func WithdrawalV2Digest(e WithdrawalV2) string {
	h := sha256.New()
	for _, value := range []string{"arran-withdrawal-evidence-v2", e.LedgerID, e.OwnerID, e.OperationID, e.RequestOperationID, e.Kind, e.RequestedAt, e.Previous} {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(value)))
		h.Write(n[:])
		h.Write([]byte(value))
	}
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(e.Version))
	h.Write(n[:])
	binary.BigEndian.PutUint64(n[:], e.Generation)
	h.Write(n[:])
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateWithdrawalV2 checks a complete v2-only withdrawal history against a
// supplied head. It does not establish authority, DB identity, or permission to
// replay or publish. Mixed histories need an explicit migration contract.
func ValidateWithdrawalV2(head Head, entries []WithdrawalV2) error {
	if !validID(head.LedgerID) || head.Generation == 0 || !validDigest(head.Digest) || uint64(len(entries)) != head.Generation {
		return errors.New("invalid or incomplete v2 head")
	}
	type request struct{ owner, requestedAt string }
	requests := map[string]request{}
	ownerRequests := map[string]bool{}
	completed := map[string]bool{}
	operations := map[string]bool{}
	previous := ""
	for i, e := range entries {
		if e.Version != 2 || e.LedgerID != head.LedgerID || e.Generation != uint64(i)+1 || !validID(e.OwnerID) || !validID(e.OperationID) || e.Previous != previous || operations[e.OperationID] {
			return fmt.Errorf("invalid v2 identity or sequence at %d", i+1)
		}
		operations[e.OperationID] = true
		timestamp, err := time.Parse(time.RFC3339Nano, e.RequestedAt)
		if err != nil || timestamp.IsZero() || timestamp.Location() != time.UTC || timestamp.Format(time.RFC3339Nano) != e.RequestedAt {
			return fmt.Errorf("invalid canonical UTC request time at %d", i+1)
		}
		switch e.Kind {
		case "withdrawal_requested":
			if e.RequestOperationID != "" || ownerRequests[e.OwnerID] {
				return errors.New("request identity or owner already used")
			}
			ownerRequests[e.OwnerID] = true
			requests[e.OperationID] = request{e.OwnerID, e.RequestedAt}
		case "withdrawal_completed":
			if !validID(e.RequestOperationID) || e.RequestOperationID == e.OperationID || requests[e.RequestOperationID] != (request{e.OwnerID, e.RequestedAt}) || completed[e.RequestOperationID] {
				return errors.New("completion has no matching unique request")
			}
			completed[e.RequestOperationID] = true
		default:
			return errors.New("unknown v2 withdrawal event")
		}
		if !validDigest(e.Digest) || WithdrawalV2Digest(e) != e.Digest {
			return fmt.Errorf("invalid v2 digest at %d", i+1)
		}
		previous = e.Digest
	}
	if previous != head.Digest {
		return errors.New("v2 head mismatch")
	}
	return nil
}
