package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// withdrawalIdentity must come from an independently verified durable deletion
// ledger. An integer ID alone is insufficient when a backup is restored.
type withdrawalIdentity struct {
	userID      int64
	username    string
	createdAt   time.Time
	requestedAt time.Time
}

var errWithdrawalUnsafe = errors.New("withdrawal deletion cannot be confirmed safe")

// deleteWithdrawalRows performs only the database part of a withdrawal. It is
// deliberately not exposed by the API or by an operator command: callers must
// first stop/drain every writer and durably record the verified identity in an
// independent ledger, and must prevent publishing a restored DB until the ledger
// has been reconciled. This function does not commit or establish those guards.
// The privileged caller owns tx, MUST rollback on any error, and can commit only
// after its external safety protocol has completed its pre-commit steps.
func deleteWithdrawalRows(ctx context.Context, tx *sql.Tx, identity withdrawalIdentity) error {
	return deleteWithdrawalRowsMode(ctx, tx, identity, false)
}

// replayWithdrawalRows is only for an isolated, quiescent restored DB. The
// plan must be derived from a complete independently authenticated and fresh
// ledger; planWithdrawalReplay does not itself authenticate that ledger. It is
// deliberately not exposed to an API or operational command.
func replayWithdrawalRows(ctx context.Context, tx *sql.Tx, plan withdrawalReplayPlan, identity withdrawalIdentity) error {
	matches := 0
	for _, candidate := range plan.deleteIdentities {
		if sameWithdrawalIdentity(candidate, identity) {
			matches++
		}
	}
	if matches != 1 {
		return errWithdrawalUnsafe
	}
	return deleteWithdrawalRowsMode(ctx, tx, identity, true)
}

func deleteWithdrawalRowsMode(ctx context.Context, tx *sql.Tx, identity withdrawalIdentity, replay bool) error {
	if identity.userID <= 0 || identity.username == "" || identity.createdAt.IsZero() || identity.requestedAt.IsZero() {
		return errWithdrawalUnsafe
	}
	// Locking the complete list prevents another transaction from writing owned
	// rows after safety checks. Keep the migration version fixed so a future table
	// cannot silently be omitted from the deletion plan.
	if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '3s'`); err != nil {
		return err
	}
	const lock = `LOCK TABLE users, withdrawal_requests, schema_migrations, auth_sessions, auth_attempts,
  user_consultation_limits, monthly_consultation_usage, admin_notifications, llm_usage,
  profiles, transactions, merchant_rules, consultations, consultation_messages, memories,
  monthly_reviews, chat_states, chat_threads, chat_purchase_schedule, chat_requests,
  csv_identity_file_rows, csv_identity_aliases, purchase_csv_decisions, purchase_csv_receipts,
  csv_import_batches, csv_import_evidence, csv_mutations, transaction_change_events,
  csv_financial_receipts, csv_financial_items, csv_import_previews,
  ai_user_consents, ai_consent_inflight IN ACCESS EXCLUSIVE MODE`
	if _, err := tx.ExecContext(ctx, lock); err != nil {
		return fmt.Errorf("lock withdrawal tables: %w", err)
	}
	version, err := appliedPrefix(ctx, tx)
	if err != nil {
		return err
	}
	if version != 17 || len(migrations) != 17 {
		return errWithdrawalUnsafe
	}
	if err := verifyWithdrawalSchema(ctx, tx); err != nil {
		return err
	}
	if replay {
		// An older backup can predate the withdrawal request. Only the exact
		// independently recorded identity may reconstruct that request, in the
		// same transaction as deletion. An existing matching request may not
		// yet have been reviewed at the backup point; the completed ledger
		// proves that this later review occurred. A conflicting request is never
		// overwritten; the match check below refuses it.
		var userMatches bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id=$1 AND username=$2 AND created_at=$3)`, identity.userID, identity.username, identity.createdAt).Scan(&userMatches); err != nil {
			return err
		}
		if !userMatches {
			return errWithdrawalUnsafe
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO withdrawal_requests(user_id,requested_at,reviewed_at) VALUES($1,$2,CURRENT_TIMESTAMP) ON CONFLICT(user_id) DO UPDATE SET reviewed_at=COALESCE(withdrawal_requests.reviewed_at,CURRENT_TIMESTAMP) WHERE withdrawal_requests.requested_at=EXCLUDED.requested_at`, identity.userID, identity.requestedAt); err != nil {
			return fmt.Errorf("reconstruct withdrawal request: %w", err)
		}
	}
	var matches bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (
  SELECT 1 FROM users u JOIN withdrawal_requests w ON w.user_id=u.id
  WHERE u.id=$1 AND u.username=$2 AND u.created_at=$3 AND w.requested_at=$4 AND w.reviewed_at IS NOT NULL
 )`, identity.userID, identity.username, identity.createdAt, identity.requestedAt).Scan(&matches)
	if err != nil {
		return err
	}
	if !matches {
		return errWithdrawalUnsafe
	}
	var unsafe bool
	err = tx.QueryRowContext(ctx, `SELECT
  EXISTS(SELECT 1 FROM ai_consent_inflight WHERE user_id=$1) OR
  EXISTS(SELECT 1 FROM chat_requests WHERE user_id=$1 AND status='processing') OR
  EXISTS(SELECT 1 FROM admin_notifications WHERE user_id=$1 AND status='sending') OR
  EXISTS(SELECT 1 FROM consultation_messages m JOIN consultations c ON c.id=m.consultation_id
         WHERE c.user_id=$1 AND m.user_id IS DISTINCT FROM c.user_id)`, identity.userID).Scan(&unsafe)
	if err != nil {
		return err
	}
	if unsafe {
		return errWithdrawalUnsafe
	}
	// Every direct deletion is scoped by user_id. Dependent FK cascades are
	// owner-scoped except legacy consultation_messages, checked above.
	tables := []string{
		"csv_financial_items", "csv_financial_receipts", "transaction_change_events", "csv_mutations",
		"csv_import_evidence", "csv_import_batches", "csv_identity_aliases", "csv_identity_file_rows",
		"purchase_csv_receipts", "purchase_csv_decisions", "csv_import_previews", "chat_purchase_schedule",
		"chat_threads", "chat_requests", "chat_states", "consultation_messages", "consultations",
		"transactions", "merchant_rules", "memories", "monthly_reviews", "profiles",
		"auth_sessions", "user_consultation_limits", "monthly_consultation_usage", "admin_notifications",
		"llm_usage", "ai_user_consents", "withdrawal_requests",
	}
	for _, table := range tables {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE user_id=$1", identity.userID); err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
	}
	// The username-specific login limiter is stored as a SHA-256 digest; the
	// separately keyed IP limiter may be shared and must not be removed.
	usernameDigest := sha256.Sum256([]byte("user:" + identity.username))
	if _, err = tx.ExecContext(ctx, `DELETE FROM auth_attempts WHERE key=$1`, hex.EncodeToString(usernameDigest[:])); err != nil {
		return fmt.Errorf("delete username login limit: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id=$1 AND username=$2 AND created_at=$3`, identity.userID, identity.username, identity.createdAt)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errWithdrawalUnsafe
	}
	return nil
}
