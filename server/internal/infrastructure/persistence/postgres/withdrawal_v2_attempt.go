package postgres

import (
	"context"
	"database/sql"
	"regexp"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/restoreevidence"
)

var lowerHexDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

// These methods fix an attempted event only. They do not write or verify an
// external receipt, witness an event, or authorize an account transition.
func validV2Attempt(p withdrawalV2Payload, e restoreevidence.WithdrawalV2) bool {
	if !validWithdrawalV2Payload(p) || e.Version != 2 || e.OwnerID != p.OwnerID || e.LedgerID != p.LedgerID || e.OperationID != p.OperationID || e.Kind != p.Kind || e.RequestedAt != p.RequestedAt || e.RequestOperationID != p.RequestOperationID || e.Generation == 0 || e.Generation > uint64(^uint64(0)>>1) || e.Digest != restoreevidence.WithdrawalV2Digest(e) {
		return false
	}
	if e.Generation == 1 {
		return e.Previous == ""
	}
	return len(e.Previous) == 64 && lowerHexDigest.MatchString(e.Previous)
}

func (r *Repository) bindWithdrawalV2Attempt(ctx context.Context, p withdrawalV2Payload, e restoreevidence.WithdrawalV2) error {
	user := identity.UserID(ctx)
	if user <= 0 || !validV2Attempt(p, e) {
		return errEvidenceIntentUnknown
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errEvidenceIntentUnknown
	}
	defer tx.Rollback()
	var owner string
	if err = tx.QueryRowContext(ctx, `SELECT evidence_owner_id FROM users WHERE id=$1 FOR NO KEY UPDATE`, user).Scan(&owner); err != nil || owner != p.OwnerID {
		return errEvidenceIntentUnknown
	}
	if p.Kind == "withdrawal_completed" {
		_, requestAttempt, readErr := readWithdrawalV2Attempt(ctx, tx, user, p.OwnerID, p.RequestOperationID, true)
		if readErr != nil || requestAttempt == nil || !v2CompletionAfterRequest(e, *requestAttempt) {
			return errEvidenceIntentUnknown
		}
	}
	got, old, err := readWithdrawalV2Attempt(ctx, tx, user, p.OwnerID, p.OperationID, true)
	if err != nil || got != p {
		return errEvidenceIntentUnknown
	}
	if old != nil {
		if *old != e {
			return errEvidenceIntentUnknown
		}
	} else {
		result, err := tx.ExecContext(ctx, `UPDATE evidence_intents SET attempt_generation=$1,attempt_previous=$2,attempt_digest=$3 WHERE user_id=$4 AND operation_id=$5 AND format_version=2 AND state='prepared' AND attempt_generation IS NULL`, int64(e.Generation), e.Previous, e.Digest, user, p.OperationID)
		if err != nil {
			return errEvidenceIntentUnknown
		}
		n, err := result.RowsAffected()
		if err != nil || n != 1 {
			return errEvidenceIntentUnknown
		}
	}
	if tx.Commit() != nil {
		return errEvidenceIntentUnknown
	}
	return nil
}

func v2CompletionAfterRequest(completion, request restoreevidence.WithdrawalV2) bool {
	if completion.Generation <= request.Generation {
		return false
	}
	return completion.Generation != request.Generation+1 || completion.Previous == request.Digest
}

// readWithdrawalV2Attempt obtains the payload and fixed attempt in one
// snapshot. lock=true is used only within the binding transaction.
func readWithdrawalV2Attempt(ctx context.Context, q v2Queryer, user int64, owner, operation string, lock bool) (withdrawalV2Payload, *restoreevidence.WithdrawalV2, error) {
	var p withdrawalV2Payload
	var ref, previous, digest, reqPrevious, reqDigest sql.NullString
	var generation, reqGeneration sql.NullInt64
	query := `SELECT i.owner_id,i.ledger_id,i.operation_id,i.kind,i.request_time_utc,i.request_operation_id,i.attempt_generation,i.attempt_previous,i.attempt_digest,r.attempt_generation,r.attempt_previous,r.attempt_digest FROM evidence_intents i LEFT JOIN evidence_intents r ON r.user_id=i.user_id AND r.operation_id=i.request_operation_id AND r.format_version=2 AND r.state='prepared' AND r.kind='withdrawal_requested' AND r.owner_id=i.owner_id AND r.ledger_id=i.ledger_id AND r.request_time_utc=i.request_time_utc WHERE i.user_id=$1 AND i.operation_id=$2 AND i.owner_id=$3 AND i.format_version=2 AND i.state='prepared' AND (i.kind='withdrawal_requested' OR (i.kind='withdrawal_completed' AND r.operation_id IS NOT NULL))`
	if lock {
		query += ` FOR UPDATE OF i`
	}
	err := q.QueryRowContext(ctx, query, user, operation, owner).Scan(&p.OwnerID, &p.LedgerID, &p.OperationID, &p.Kind, &p.RequestedAt, &ref, &generation, &previous, &digest, &reqGeneration, &reqPrevious, &reqDigest)
	if ref.Valid {
		p.RequestOperationID = ref.String
	}
	if err != nil || !validWithdrawalV2Payload(p) {
		return withdrawalV2Payload{}, nil, errEvidenceIntentUnknown
	}
	if !generation.Valid && !previous.Valid && !digest.Valid {
		return p, nil, nil
	}
	if !generation.Valid || generation.Int64 <= 0 || !previous.Valid || !digest.Valid {
		return withdrawalV2Payload{}, nil, errEvidenceIntentUnknown
	}
	e := restoreevidence.WithdrawalV2{Version: 2, OwnerID: p.OwnerID, LedgerID: p.LedgerID, OperationID: p.OperationID, Kind: p.Kind, RequestedAt: p.RequestedAt, RequestOperationID: p.RequestOperationID, Generation: uint64(generation.Int64), Previous: previous.String, Digest: digest.String}
	if !validV2Attempt(p, e) {
		return withdrawalV2Payload{}, nil, errEvidenceIntentUnknown
	}
	if p.Kind == "withdrawal_completed" {
		if !reqGeneration.Valid || reqGeneration.Int64 <= 0 || !reqPrevious.Valid || !reqDigest.Valid {
			return withdrawalV2Payload{}, nil, errEvidenceIntentUnknown
		}
		requestPayload := withdrawalV2Payload{OwnerID: p.OwnerID, LedgerID: p.LedgerID, OperationID: p.RequestOperationID, Kind: "withdrawal_requested", RequestedAt: p.RequestedAt}
		request := restoreevidence.WithdrawalV2{Version: 2, OwnerID: p.OwnerID, LedgerID: p.LedgerID, OperationID: p.RequestOperationID, Kind: requestPayload.Kind, RequestedAt: p.RequestedAt, Generation: uint64(reqGeneration.Int64), Previous: reqPrevious.String, Digest: reqDigest.String}
		if !validV2Attempt(requestPayload, request) || !v2CompletionAfterRequest(e, request) {
			return withdrawalV2Payload{}, nil, errEvidenceIntentUnknown
		}
	}
	return p, &e, nil
}
