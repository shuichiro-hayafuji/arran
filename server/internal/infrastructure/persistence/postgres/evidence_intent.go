package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

var evidenceIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var errEvidenceIntentUnknown = errors.New("evidence intent unavailable or outcome unconfirmed")

type evidenceIntent struct {
	OperationID, OwnerID, LedgerID, Kind, PolicyVersion string
	RelatedOperationID                                  string
}

// prepareEvidenceIntent only creates or reads a local prepared record. It is
// deliberately not an external receipt and does not authorize any side effect
// or grant exclusive execution ownership, even after a successful retry.
// Runtime write permission is withheld until the full protocol is connected.
func (r *Repository) prepareEvidenceIntent(ctx context.Context, in evidenceIntent) (evidenceIntent, error) {
	user := identity.UserID(ctx)
	if user <= 0 || !evidenceIDPattern.MatchString(in.OperationID) || !evidenceIDPattern.MatchString(in.OwnerID) || !evidenceIDPattern.MatchString(in.LedgerID) {
		return evidenceIntent{}, errEvidenceIntentUnknown
	}
	switch in.Kind {
	case "withdrawal_requested":
		if in.RelatedOperationID != "" {
			return evidenceIntent{}, errEvidenceIntentUnknown
		}
	case "consent_granted", "consent_withdrawn", "send_started":
		if in.PolicyVersion == "" || in.RelatedOperationID != "" {
			return evidenceIntent{}, errEvidenceIntentUnknown
		}
	case "send_completed":
		if in.PolicyVersion == "" || !evidenceIDPattern.MatchString(in.RelatedOperationID) || in.RelatedOperationID == in.OperationID {
			return evidenceIntent{}, errEvidenceIntentUnknown
		}
	default:
		// Completion events require an independently confirmed start and are not
		// yet supported; never infer completion from a prepared local row.
		return evidenceIntent{}, errEvidenceIntentUnknown
	}
	var result evidenceIntent
	err := r.withWithdrawalWriteTx(ctx, user, func(ctx context.Context, tx withdrawalWriteSession) error {
		var owner string
		if err := tx.QueryRowContext(ctx, `SELECT evidence_owner_id FROM users WHERE id=$1`, user).Scan(&owner); err != nil {
			return err
		}
		if owner != in.OwnerID {
			return errEvidenceIntentUnknown
		}
		if in.Kind == "send_completed" {
			var kind, policy, ledger, state string
			if err := tx.QueryRowContext(ctx, `SELECT kind,policy_version,ledger_id,state FROM evidence_intents WHERE user_id=$1 AND operation_id=$2 AND owner_id=$3 FOR SHARE`, user, in.RelatedOperationID, owner).Scan(&kind, &policy, &ledger, &state); err != nil || kind != "send_started" || policy != in.PolicyVersion || ledger != in.LedgerID || state != "witnessed" {
				return errEvidenceIntentUnknown
			}
		}
		var relatedValue any
		if in.RelatedOperationID != "" {
			relatedValue = in.RelatedOperationID
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO evidence_intents(user_id,operation_id,owner_id,ledger_id,kind,policy_version,related_operation_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, user, in.OperationID, owner, in.LedgerID, in.Kind, in.PolicyVersion, relatedValue)
		if err != nil {
			return err
		}
		var related sql.NullString
		var state string
		err = tx.QueryRowContext(ctx, `SELECT operation_id,owner_id,ledger_id,kind,policy_version,related_operation_id,state FROM evidence_intents WHERE user_id=$1 AND operation_id=$2`, user, in.OperationID).Scan(&result.OperationID, &result.OwnerID, &result.LedgerID, &result.Kind, &result.PolicyVersion, &related, &state)
		if err != nil {
			return err
		}
		if related.Valid {
			result.RelatedOperationID = related.String
		}
		if state != "prepared" || result.OwnerID != owner || result.LedgerID != in.LedgerID || result.Kind != in.Kind || result.PolicyVersion != in.PolicyVersion || result.RelatedOperationID != in.RelatedOperationID {
			return errEvidenceIntentUnknown
		}
		return nil
	})
	if err != nil {
		return evidenceIntent{}, fmt.Errorf("%w: prepare failed", errEvidenceIntentUnknown)
	}
	return result, nil
}

// activeEvidenceOwner must be called and its result retained with the operation
// payload before prepare. It is not proof of identity after database restore;
// no caller may substitute a fresh lookup for independently retained evidence.
func (r *Repository) activeEvidenceOwner(ctx context.Context) (string, error) {
	user := identity.UserID(ctx)
	if user <= 0 {
		return "", errEvidenceIntentUnknown
	}
	var owner string
	err := r.db.DB.QueryRowContext(ctx, `SELECT evidence_owner_id FROM users WHERE id=$1 AND is_active AND NOT EXISTS(SELECT 1 FROM withdrawal_requests WHERE user_id=$1)`, user).Scan(&owner)
	if err != nil || !evidenceIDPattern.MatchString(owner) {
		return "", errEvidenceIntentUnknown
	}
	return owner, nil
}

// readEvidenceIntent only reconciles an existing local row. It does not
// require an active account, create a row, or authorize external execution.
func (r *Repository) readEvidenceIntent(ctx context.Context, expected evidenceIntent) (evidenceIntent, error) {
	user := identity.UserID(ctx)
	if user <= 0 || !evidenceIDPattern.MatchString(expected.OperationID) || !evidenceIDPattern.MatchString(expected.OwnerID) || !evidenceIDPattern.MatchString(expected.LedgerID) {
		return evidenceIntent{}, errEvidenceIntentUnknown
	}
	var result evidenceIntent
	var related sql.NullString
	err := r.db.DB.QueryRowContext(ctx, `SELECT operation_id,owner_id,ledger_id,kind,policy_version,related_operation_id FROM evidence_intents WHERE user_id=$1 AND operation_id=$2 AND owner_id=$3`, user, expected.OperationID, expected.OwnerID).Scan(&result.OperationID, &result.OwnerID, &result.LedgerID, &result.Kind, &result.PolicyVersion, &related)
	if related.Valid {
		result.RelatedOperationID = related.String
	}
	if err != nil || result != expected {
		return evidenceIntent{}, errEvidenceIntentUnknown
	}
	return result, nil
}
