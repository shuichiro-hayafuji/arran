package postgres

import (
	"context"
	"database/sql"
	"errors"
	"regexp"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/restoreevidence"
)

type evidenceConfirmer interface {
	ConfirmOperation(context.Context, restoreevidence.Entry) (restoreevidence.Entry, error)
}

// witnessEvidenceIntent records a strong-read receipt for a previously prepared
// intent. This bookkeeping does not grant execution ownership or authorize an
// account mutation. The caller must supply an authenticated store bound to the
// correct authority; runtime write permissions are intentionally absent.
func (r *Repository) witnessEvidenceIntent(ctx context.Context, expected evidenceIntent, store evidenceConfirmer) error {
	user := identity.UserID(ctx)
	if user <= 0 || store == nil || !evidenceIDPattern.MatchString(expected.LedgerID) {
		return errEvidenceIntentUnknown
	}
	if _, err := r.readEvidenceIntent(ctx, expected); err != nil {
		return errEvidenceIntentUnknown
	}
	want := restoreevidence.Entry{LedgerID: expected.LedgerID, OwnerID: expected.OwnerID, OperationID: expected.OperationID, RelatedOperationID: expected.RelatedOperationID, Kind: expected.Kind, PolicyVersion: expected.PolicyVersion}
	event, err := store.ConfirmOperation(ctx, want)
	if err != nil || event.LedgerID != want.LedgerID || event.OwnerID != want.OwnerID || event.OperationID != want.OperationID || event.RelatedOperationID != want.RelatedOperationID || event.Kind != want.Kind || event.PolicyVersion != want.PolicyVersion || event.Generation == 0 || event.Digest != restoreevidence.Digest(event) {
		return errEvidenceIntentUnknown
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errEvidenceIntentUnknown
	}
	defer tx.Rollback()
	var currentOwner string
	if err = tx.QueryRowContext(ctx, `SELECT evidence_owner_id FROM users WHERE id=$1 FOR NO KEY UPDATE`, user).Scan(&currentOwner); err != nil || currentOwner != expected.OwnerID {
		return errEvidenceIntentUnknown
	}
	var state string
	var gen sql.NullInt64
	var digest sql.NullString
	var attemptGen sql.NullInt64
	var attemptPrev, attemptDigest sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT state,event_generation,event_digest,attempt_generation,attempt_previous,attempt_digest FROM evidence_intents WHERE user_id=$1 AND operation_id=$2 AND owner_id=$3 AND ledger_id=$4 AND kind=$5 AND policy_version=$6 AND COALESCE(related_operation_id,'')=$7 FOR UPDATE`, user, expected.OperationID, expected.OwnerID, expected.LedgerID, expected.Kind, expected.PolicyVersion, expected.RelatedOperationID).Scan(&state, &gen, &digest, &attemptGen, &attemptPrev, &attemptDigest)
	if err != nil {
		return errEvidenceIntentUnknown
	}
	if !attemptGen.Valid || attemptGen.Int64 <= 0 || uint64(attemptGen.Int64) != event.Generation || !attemptPrev.Valid || attemptPrev.String != event.Previous || !attemptDigest.Valid || attemptDigest.String != event.Digest {
		return errEvidenceIntentUnknown
	}
	if state == "witnessed" {
		if !gen.Valid || gen.Int64 <= 0 || uint64(gen.Int64) != event.Generation || !digest.Valid || digest.String != event.Digest {
			return errEvidenceIntentUnknown
		}
		if err := tx.Commit(); err != nil {
			return errors.Join(errEvidenceIntentUnknown, errors.New("commit outcome unknown"))
		}
		return nil
	}
	if state != "prepared" || event.Generation > uint64(^uint64(0)>>1) {
		return errEvidenceIntentUnknown
	}
	result, err := tx.ExecContext(ctx, `UPDATE evidence_intents SET state='witnessed',event_generation=$1,event_digest=$2 WHERE user_id=$3 AND operation_id=$4 AND state='prepared'`, int64(event.Generation), event.Digest, user, expected.OperationID)
	if err != nil {
		return errEvidenceIntentUnknown
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errEvidenceIntentUnknown
	}
	if err = tx.Commit(); err != nil {
		return errors.Join(errEvidenceIntentUnknown, errors.New("commit outcome unknown"))
	}
	return nil
}

// bindEvidenceAttempt fixes the exact external event before any append. A
// retry may only observe the same attempt; it never grants send ownership.
func (r *Repository) bindEvidenceAttempt(ctx context.Context, expected evidenceIntent, event restoreevidence.Entry) error {
	user := identity.UserID(ctx)
	if user <= 0 || event.LedgerID != expected.LedgerID || event.OwnerID != expected.OwnerID || event.OperationID != expected.OperationID || event.RelatedOperationID != expected.RelatedOperationID || event.Kind != expected.Kind || event.PolicyVersion != expected.PolicyVersion || event.Generation == 0 || event.Generation > uint64(^uint64(0)>>1) || event.Digest != restoreevidence.Digest(event) || (event.Generation == 1 && event.Previous != "") || (event.Generation > 1 && !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(event.Previous)) {
		return errEvidenceIntentUnknown
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errEvidenceIntentUnknown
	}
	defer tx.Rollback()
	var owner string
	if err = tx.QueryRowContext(ctx, `SELECT evidence_owner_id FROM users WHERE id=$1 FOR NO KEY UPDATE`, user).Scan(&owner); err != nil || owner != expected.OwnerID {
		return errEvidenceIntentUnknown
	}
	var generation sql.NullInt64
	var previous, digest sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT attempt_generation,attempt_previous,attempt_digest FROM evidence_intents WHERE user_id=$1 AND operation_id=$2 AND owner_id=$3 AND ledger_id=$4 AND kind=$5 AND policy_version=$6 AND COALESCE(related_operation_id,'')=$7 FOR UPDATE`, user, expected.OperationID, expected.OwnerID, expected.LedgerID, expected.Kind, expected.PolicyVersion, expected.RelatedOperationID).Scan(&generation, &previous, &digest)
	if err != nil {
		return errEvidenceIntentUnknown
	}
	if generation.Valid || previous.Valid || digest.Valid {
		if !generation.Valid || !previous.Valid || !digest.Valid || generation.Int64 != int64(event.Generation) || previous.String != event.Previous || digest.String != event.Digest {
			return errEvidenceIntentUnknown
		}
	} else {
		result, err := tx.ExecContext(ctx, `UPDATE evidence_intents SET attempt_generation=$1,attempt_previous=$2,attempt_digest=$3 WHERE user_id=$4 AND operation_id=$5 AND state='prepared' AND attempt_generation IS NULL`, int64(event.Generation), event.Previous, event.Digest, user, expected.OperationID)
		if err != nil {
			return errEvidenceIntentUnknown
		}
		n, err := result.RowsAffected()
		if err != nil || n != 1 {
			return errEvidenceIntentUnknown
		}
	}
	if err := tx.Commit(); err != nil {
		return errors.Join(errEvidenceIntentUnknown, errors.New("bind commit outcome unknown"))
	}
	return nil
}
