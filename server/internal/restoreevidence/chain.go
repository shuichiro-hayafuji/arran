// Package restoreevidence validates an untrusted history against a separately
// obtained authoritative head. It does not connect to DynamoDB, authenticate
// callers, reconcile a database, or authorize publication.
package restoreevidence

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

type Head struct {
	LedgerID   string
	Generation uint64
	Digest     string
}
type Entry struct {
	LedgerID           string
	Generation         uint64
	OwnerID            string
	OperationID        string
	RelatedOperationID string
	PolicyVersion      string
	Kind               string
	Previous           string
	Digest             string
}

// Digest uses length prefixes to avoid ambiguous concatenations. LedgerID and
// OwnerID must be assigned independently of PostgreSQL serial user IDs.
func Digest(e Entry) string {
	h := sha256.New()
	for _, v := range []string{e.LedgerID, e.OwnerID, e.OperationID, e.RelatedOperationID, e.PolicyVersion, e.Kind, e.Previous} {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(v)))
		h.Write(n[:])
		h.Write([]byte(v))
	}
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], e.Generation)
	h.Write(n[:])
	return hex.EncodeToString(h.Sum(nil))
}
func validID(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func validDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// ValidateChain assumes authoritative was fetched using a strongly consistent
// read from the configured, independently controlled table. A caller-provided
// head or a locally stored checkpoint alone cannot establish freshness. The
// caller must also freeze writers until reconciliation and cutover are done.
// This function does not provide that guarantee and must not enable runtime.
func ValidateChain(authoritative Head, entries []Entry) error {
	if !validID(authoritative.LedgerID) || authoritative.Generation == 0 || !validDigest(authoritative.Digest) {
		return errors.New("authoritative head missing or invalid")
	}
	if uint64(len(entries)) != authoritative.Generation {
		return errors.New("incomplete history")
	}
	previous := ""
	operations := make(map[string]bool, len(entries))
	type sendStart struct{ owner, policy string }
	starts := make(map[string]sendStart)
	finished := make(map[string]bool)
	for i, e := range entries {
		if e.LedgerID != authoritative.LedgerID || e.Generation != uint64(i)+1 || !validID(e.OwnerID) || !validID(e.OperationID) || e.Previous != previous {
			return fmt.Errorf("history identity or sequence invalid at %d", i+1)
		}
		if operations[e.OperationID] {
			return errors.New("duplicate operation identity")
		}
		operations[e.OperationID] = true
		switch e.Kind {
		case "consent_granted", "consent_withdrawn":
			if e.PolicyVersion == "" {
				return errors.New("consent version missing")
			}
		case "send_started":
			if e.RelatedOperationID != "" || e.PolicyVersion == "" {
				return errors.New("send start has a related operation")
			}
			starts[e.OperationID] = sendStart{e.OwnerID, e.PolicyVersion}
		case "send_completed":
			if !validID(e.RelatedOperationID) || e.PolicyVersion == "" || starts[e.RelatedOperationID] != (sendStart{e.OwnerID, e.PolicyVersion}) || finished[e.RelatedOperationID] {
				return errors.New("send completion has no unique owner start")
			}
			finished[e.RelatedOperationID] = true
		case "withdrawal_requested", "withdrawal_completed":
		default:
			return errors.New("unknown event kind")
		}
		if e.Kind != "send_completed" && e.RelatedOperationID != "" {
			return errors.New("unexpected related operation")
		}
		if !validDigest(e.Digest) || Digest(e) != e.Digest {
			return fmt.Errorf("history digest invalid at %d", i+1)
		}
		previous = e.Digest
	}
	if previous != authoritative.Digest {
		return errors.New("history head mismatch")
	}
	return nil
}
