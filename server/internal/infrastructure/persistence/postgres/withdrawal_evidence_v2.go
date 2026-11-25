package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

// withdrawalV2Payload is an unwitnessed, disconnected DB payload. Saving it
// is not acceptance, an authority receipt, or permission to replay a restore.
type withdrawalV2Payload struct {
	OwnerID, LedgerID, OperationID, Kind, RequestedAt, RequestOperationID string
}

func validWithdrawalV2Payload(p withdrawalV2Payload) bool {
	if !evidenceIDPattern.MatchString(p.OwnerID) || !evidenceIDPattern.MatchString(p.LedgerID) || !evidenceIDPattern.MatchString(p.OperationID) {
		return false
	}
	at, err := time.Parse(time.RFC3339Nano, p.RequestedAt)
	if err != nil || at.IsZero() || at.Location() != time.UTC || at.Format(time.RFC3339Nano) != p.RequestedAt {
		return false
	}
	switch p.Kind {
	case "withdrawal_requested":
		return p.RequestOperationID == ""
	case "withdrawal_completed":
		return evidenceIDPattern.MatchString(p.RequestOperationID) && p.RequestOperationID != p.OperationID
	}
	return false
}
func (r *Repository) prepareWithdrawalV2(ctx context.Context, p withdrawalV2Payload) error {
	user := identity.UserID(ctx)
	if user <= 0 || !validWithdrawalV2Payload(p) {
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
	var ref any
	if p.Kind == "withdrawal_completed" {
		var refOwner, refLedger, refTime, refKind string
		if err = tx.QueryRowContext(ctx, `SELECT owner_id,ledger_id,request_time_utc,kind FROM evidence_intents WHERE user_id=$1 AND operation_id=$2 AND format_version=2 FOR SHARE`, user, p.RequestOperationID).Scan(&refOwner, &refLedger, &refTime, &refKind); err != nil || refOwner != owner || refLedger != p.LedgerID || refTime != p.RequestedAt || refKind != "withdrawal_requested" {
			return errEvidenceIntentUnknown
		}
		ref = p.RequestOperationID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO evidence_intents(user_id,owner_id,ledger_id,operation_id,kind,policy_version,format_version,request_time_utc,request_operation_id) VALUES($1,$2,$3,$4,$5,'',2,$6,$7) ON CONFLICT DO NOTHING`, user, p.OwnerID, p.LedgerID, p.OperationID, p.Kind, p.RequestedAt, ref)
	if err != nil {
		return errEvidenceIntentUnknown
	}
	got, err := readWithdrawalV2(ctx, tx, user, p.OperationID, p.OwnerID)
	if err != nil || got != p {
		return errEvidenceIntentUnknown
	}
	if err = tx.Commit(); err != nil {
		return errEvidenceIntentUnknown
	}
	return nil
}

type v2Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readWithdrawalV2(ctx context.Context, q v2Queryer, user int64, operation, owner string) (withdrawalV2Payload, error) {
	var p withdrawalV2Payload
	var ref sql.NullString
	err := q.QueryRowContext(ctx, `SELECT i.owner_id,i.ledger_id,i.operation_id,i.kind,i.request_time_utc,i.request_operation_id FROM evidence_intents i LEFT JOIN evidence_intents r ON r.user_id=i.user_id AND r.operation_id=i.request_operation_id AND r.format_version=2 AND r.state='prepared' AND r.kind='withdrawal_requested' AND r.owner_id=i.owner_id AND r.ledger_id=i.ledger_id AND r.request_time_utc=i.request_time_utc WHERE i.user_id=$1 AND i.operation_id=$2 AND i.owner_id=$3 AND i.format_version=2 AND i.state='prepared' AND (i.kind='withdrawal_requested' OR (i.kind='withdrawal_completed' AND r.operation_id IS NOT NULL))`, user, operation, owner).Scan(&p.OwnerID, &p.LedgerID, &p.OperationID, &p.Kind, &p.RequestedAt, &ref)
	if ref.Valid {
		p.RequestOperationID = ref.String
	}
	if err != nil || !validWithdrawalV2Payload(p) {
		return withdrawalV2Payload{}, errEvidenceIntentUnknown
	}
	return p, nil
}
func (r *Repository) readWithdrawalV2(ctx context.Context, owner, operation string) (withdrawalV2Payload, error) {
	user := identity.UserID(ctx)
	if user <= 0 || !evidenceIDPattern.MatchString(owner) || !evidenceIDPattern.MatchString(operation) {
		return withdrawalV2Payload{}, errEvidenceIntentUnknown
	}
	return readWithdrawalV2(ctx, r.db.DB, user, operation, owner)
}
