package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shuichirohayafuji/spendable-today/server/internal/evidenceflow"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/restoreevidence"
)

// evidenceJournal is an unconnected adapter; runtime rights remain revoked.
// It only records receipts and grants no execution right.
type evidenceJournal struct {
	repo *Repository
}

func intentFromEntry(e restoreevidence.Entry) evidenceIntent {
	return evidenceIntent{OwnerID: e.OwnerID, OperationID: e.OperationID, LedgerID: e.LedgerID, Kind: e.Kind, PolicyVersion: e.PolicyVersion, RelatedOperationID: e.RelatedOperationID}
}
func (j evidenceJournal) Load(ctx context.Context, owner, operation string) (evidenceflow.Intent, error) {
	user := identity.UserID(ctx)
	if j.repo == nil || user <= 0 || !evidenceIDPattern.MatchString(owner) || !evidenceIDPattern.MatchString(operation) {
		return evidenceflow.Intent{}, errEvidenceIntentUnknown
	}
	var e restoreevidence.Entry
	var related sql.NullString
	var state string
	var gen, attemptGen sql.NullInt64
	var digest, attemptPrevious, attemptDigest sql.NullString
	err := j.repo.db.DB.QueryRowContext(ctx, `SELECT ledger_id,owner_id,operation_id,kind,policy_version,related_operation_id,state,event_generation,event_digest,attempt_generation,attempt_previous,attempt_digest FROM evidence_intents WHERE user_id=$1 AND owner_id=$2 AND operation_id=$3`, user, owner, operation).Scan(&e.LedgerID, &e.OwnerID, &e.OperationID, &e.Kind, &e.PolicyVersion, &related, &state, &gen, &digest, &attemptGen, &attemptPrevious, &attemptDigest)
	if err != nil {
		return evidenceflow.Intent{}, errEvidenceIntentUnknown
	}
	if related.Valid {
		e.RelatedOperationID = related.String
	}
	result := evidenceflow.Intent{Expected: e}
	if state == "witnessed" {
		if !gen.Valid || gen.Int64 <= 0 || !digest.Valid {
			return evidenceflow.Intent{}, errEvidenceIntentUnknown
		}
		result.Witnessed = true
		result.Generation = uint64(gen.Int64)
		result.Digest = digest.String
	} else if state != "prepared" {
		return evidenceflow.Intent{}, errEvidenceIntentUnknown
	}
	if attemptGen.Valid || attemptPrevious.Valid || attemptDigest.Valid {
		if !attemptGen.Valid || attemptGen.Int64 <= 0 || !attemptPrevious.Valid || !attemptDigest.Valid {
			return evidenceflow.Intent{}, errEvidenceIntentUnknown
		}
		attempt := e
		attempt.Generation = uint64(attemptGen.Int64)
		attempt.Previous = attemptPrevious.String
		attempt.Digest = attemptDigest.String
		if restoreevidence.Digest(attempt) != attempt.Digest {
			return evidenceflow.Intent{}, errEvidenceIntentUnknown
		}
		result.Attempt = &attempt
	}
	if result.Witnessed && (result.Attempt == nil || result.Generation != result.Attempt.Generation || result.Digest != result.Attempt.Digest) {
		return evidenceflow.Intent{}, errEvidenceIntentUnknown
	}
	return result, nil
}
func (j evidenceJournal) BindAttempt(ctx context.Context, in evidenceflow.Intent, event restoreevidence.Entry) error {
	if j.repo == nil {
		return errEvidenceIntentUnknown
	}
	return j.repo.bindEvidenceAttempt(ctx, intentFromEntry(in.Expected), event)
}

type matchingConfirmer struct {
	inner    evidenceConfirmer
	expected restoreevidence.Entry
}

func (c matchingConfirmer) ConfirmOperation(ctx context.Context, e restoreevidence.Entry) (restoreevidence.Entry, error) {
	if c.inner == nil {
		return restoreevidence.Entry{}, errEvidenceIntentUnknown
	}
	got, err := c.inner.ConfirmOperation(ctx, e)
	if err != nil || got != c.expected {
		return restoreevidence.Entry{}, errEvidenceIntentUnknown
	}
	return got, nil
}
func (j evidenceJournal) Witness(ctx context.Context, lease evidenceflow.Lease, in evidenceflow.Intent, event restoreevidence.Entry) error {
	if j.repo == nil || lease == nil {
		return errors.New("evidence journal unavailable")
	}
	return j.repo.witnessEvidenceIntent(ctx, intentFromEntry(in.Expected), matchingConfirmer{inner: lease, expected: event})
}
