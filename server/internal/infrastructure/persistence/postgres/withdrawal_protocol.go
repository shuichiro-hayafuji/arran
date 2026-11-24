package postgres

import (
	"context"
	"errors"
	"fmt"
)

// These interfaces describe a safety protocol, not existing production
// integrations. A real caller must hold one exclusive operator/ledger lease for
// the entire operation, prove that all API and background writers have stopped,
// and authenticate an independent current checkpoint for every ledger read.
// The current application has no such caller. In particular, a process-local
// mutex, a backup on the same failure domain, an unkeyed hash, or a stale
// checkpoint cannot establish these preconditions.
type withdrawalQuiescence interface {
	check(context.Context) error
}
type withdrawalLedgerLease interface {
	// read returns a non-nil, fully authenticated, current and complete ledger
	// and its independently authenticated head under the held exclusive lease.
	read(context.Context) ([]withdrawalLedgerEntry, *withdrawalLedgerCheckpoint, error)
	// appendDurable must compare the current head to expected, persist the entry
	// independently of the database, and report errors including unknown results.
	appendDurable(context.Context, withdrawalLedgerCheckpoint, withdrawalLedgerEntry) error
}
type withdrawalCommitter interface {
	// preflight runs the exact safety checks against a quiescent DB and rolls
	// back without changing it. It is not a substitute for rechecking inside
	// deleteAndCommit, nor for holding the external stop/lease throughout.
	preflight(context.Context, withdrawalIdentity) error
	// deleteAndCommit executes the owner-isolated deletion in one transaction.
	// Any error, including an ambiguous commit outcome, is treated as unknown.
	deleteAndCommit(context.Context, withdrawalIdentity) error
}

var errWithdrawalProtocolUnsafe = errors.New("withdrawal protocol cannot establish safety")

// executeWithdrawalProtocol records preparation durably before touching the DB,
// reads it back, commits deletion, then durably records and reads back completion.
// It refuses unresolved preparations and never infers success from DB absence.
// Any failure after preparation requires offline investigation and recovery;
// this helper does not clear pending evidence or resume an ambiguous commit.
// It is intentionally not connected to an HTTP or operator entry point.
func executeWithdrawalProtocol(ctx context.Context, guard withdrawalQuiescence, ledger withdrawalLedgerLease, db withdrawalCommitter, id withdrawalIdentity) error {
	if guard == nil || ledger == nil || db == nil || !validWithdrawalIdentity(id) {
		return errWithdrawalProtocolUnsafe
	}
	if err := guard.check(ctx); err != nil {
		return fmt.Errorf("check stopped writers: %w", err)
	}
	entries, checkpoint, err := ledger.read(ctx)
	if err != nil {
		return fmt.Errorf("read independent ledger: %w", err)
	}
	if _, err := planWithdrawalReplay(entries, checkpoint, []withdrawalRestoreUser{}); err != nil {
		return errWithdrawalProtocolUnsafe
	}
	for _, entry := range entries {
		if entry.identity.userID == id.userID {
			return errWithdrawalProtocolUnsafe
		}
	}
	if err := db.preflight(ctx, id); err != nil {
		return fmt.Errorf("preflight deletion: %w", err)
	}
	if err := guard.check(ctx); err != nil {
		return fmt.Errorf("recheck stopped writers before preparation: %w", err)
	}
	prepared := withdrawalLedgerEntry{sequence: checkpoint.sequence + 1, previous: checkpoint.digest, state: "prepared", identity: id}
	preparedHead := withdrawalLedgerCheckpoint{sequence: prepared.sequence, digest: ledgerEntryDigest(prepared)}
	if err := ledger.appendDurable(ctx, *checkpoint, prepared); err != nil {
		return fmt.Errorf("persist preparation: %w", err)
	}
	if err := verifyWithdrawalAppend(ctx, ledger, prepared, preparedHead); err != nil {
		return err
	}
	if err := guard.check(ctx); err != nil {
		return fmt.Errorf("recheck stopped writers: %w", err)
	}
	if err := db.deleteAndCommit(ctx, id); err != nil {
		return fmt.Errorf("delete and commit: %w", err)
	}
	completed := withdrawalLedgerEntry{sequence: preparedHead.sequence + 1, previous: preparedHead.digest, state: "completed", identity: id}
	completedHead := withdrawalLedgerCheckpoint{sequence: completed.sequence, digest: ledgerEntryDigest(completed)}
	if err := ledger.appendDurable(ctx, preparedHead, completed); err != nil {
		return fmt.Errorf("persist completion: %w", err)
	}
	if err := verifyWithdrawalAppend(ctx, ledger, completed, completedHead); err != nil {
		return err
	}
	return nil
}

func verifyWithdrawalAppend(ctx context.Context, ledger withdrawalLedgerLease, expected withdrawalLedgerEntry, head withdrawalLedgerCheckpoint) error {
	entries, observed, err := ledger.read(ctx)
	if err != nil {
		return fmt.Errorf("verify independent ledger: %w", err)
	}
	if entries == nil || observed == nil || *observed != head || len(entries) != int(head.sequence) || len(entries) == 0 {
		return errWithdrawalProtocolUnsafe
	}
	// Authenticate the whole chain again, including all earlier entries. A
	// prepared final entry is expected here, but planWithdrawalReplay rejects it.
	var previous [32]byte
	for i, entry := range entries {
		if entry.sequence != uint64(i+1) || entry.previous != previous || !validWithdrawalIdentity(entry.identity) {
			return errWithdrawalProtocolUnsafe
		}
		previous = ledgerEntryDigest(entry)
	}
	if previous != head.digest || ledgerEntryDigest(entries[len(entries)-1]) != ledgerEntryDigest(expected) {
		return errWithdrawalProtocolUnsafe
	}
	return nil
}
