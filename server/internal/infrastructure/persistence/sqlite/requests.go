package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"strings"
	"time"
)

func (r *Repository) ClaimChatRequest(ctx context.Context, in consultation.RequestReceipt) (consultation.RequestReceipt, bool, error) {
	if existing, err := r.GetChatRequest(ctx, in.ID); err == nil {
		if existing.PayloadHash != in.PayloadHash {
			return existing, false, consultation.ErrRequestPayload
		}
		return existing, false, nil
	} else if !errors.Is(err, consultation.ErrNotFound) {
		return consultation.RequestReceipt{}, false, err
	}
	rows, err := r.db.QueryContext(ctx, chatQuery(ctx, "SELECT request_id FROM chat_requests WHERE user_id=:user_id AND status='processing' AND expires_at<=?"), time.Now().UnixMilli())
	if err != nil {
		return consultation.RequestReceipt{}, false, err
	}
	var expired []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return consultation.RequestReceipt{}, false, err
		}
		expired = append(expired, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return consultation.RequestReceipt{}, false, err
	}
	for _, id := range expired {
		if err = r.FailChatRequest(ctx, id, "deadline_exceeded"); err != nil {
			return consultation.RequestReceipt{}, false, err
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return consultation.RequestReceipt{}, false, err
	}
	defer tx.Rollback()
	query := func(q string) string { return chatQuery(ctx, q) }
	result, err := tx.ExecContext(ctx, query("INSERT INTO chat_requests(user_id,request_id,payload_hash,month,new_consultation,quota_reserved,status,expires_at,cancel_requested,error_code) VALUES(:user_id,?,?,?,?,?,'processing',?,FALSE,'') ON CONFLICT(user_id,request_id) DO NOTHING"), in.ID, in.PayloadHash, in.Month, in.NewConsultation, in.NewConsultation, in.ExpiresAt.UnixMilli())
	if err != nil {
		if strings.Contains(err.Error(), "idx_chat_requests_active_owner") || strings.Contains(err.Error(), "chat_requests.user_id") {
			return consultation.RequestReceipt{}, false, consultation.ErrRequestBusy
		}
		return consultation.RequestReceipt{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return consultation.RequestReceipt{}, false, err
	}
	if affected == 1 && in.NewConsultation {
		if _, err = tx.ExecContext(ctx, query("INSERT INTO monthly_consultation_usage(user_id,month,consultation_count,updated_at) VALUES(:user_id,?,1,CURRENT_TIMESTAMP) ON CONFLICT(user_id,month) DO UPDATE SET consultation_count=monthly_consultation_usage.consultation_count+1,updated_at=excluded.updated_at"), in.Month); err != nil {
			return consultation.RequestReceipt{}, false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return consultation.RequestReceipt{}, false, err
	}
	// The successful commit is the acceptance boundary. Do not reread using
	// the wire context: it may be cancelled immediately after acceptance.
	if affected == 1 {
		in.Status = "processing"
		in.ExpiresAt = time.UnixMilli(in.ExpiresAt.UnixMilli())
		in.CancelRequested = false
		in.ErrorCode = ""
		in.Result = nil
		return in, true, nil
	}
	receipt, err := r.GetChatRequest(ctx, in.ID)
	if err != nil {
		return receipt, false, err
	}
	if receipt.PayloadHash != in.PayloadHash {
		return receipt, false, consultation.ErrRequestPayload
	}
	return receipt, false, nil
}
func (r *Repository) GetChatRequest(ctx context.Context, id string) (consultation.RequestReceipt, error) {
	var receipt consultation.RequestReceipt
	var expires int64
	var payload sql.NullString
	err := r.db.QueryRowContext(ctx, chatQuery(ctx, "SELECT request_id,payload_hash,month,new_consultation,status,expires_at,cancel_requested,error_code,result_json FROM chat_requests WHERE user_id=:user_id AND request_id=?"), id).Scan(&receipt.ID, &receipt.PayloadHash, &receipt.Month, &receipt.NewConsultation, &receipt.Status, &expires, &receipt.CancelRequested, &receipt.ErrorCode, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return receipt, consultation.ErrNotFound
	}
	if err != nil {
		return receipt, err
	}
	receipt.ExpiresAt = time.UnixMilli(expires)
	if receipt.Status == "processing" && expires <= time.Now().UnixMilli() {
		if err = r.FailChatRequest(ctx, id, "deadline_exceeded"); err != nil {
			return receipt, err
		}
		return r.GetChatRequest(ctx, id)
	}
	if payload.Valid {
		var thread consultation.Thread
		if err = json.Unmarshal([]byte(payload.String), &thread); err != nil {
			return receipt, err
		}
		receipt.Result = &thread
	}
	return receipt, nil
}
func (r *Repository) CancelChatRequest(ctx context.Context, id string) (consultation.RequestReceipt, error) {
	if _, err := r.db.ExecContext(ctx, chatQuery(ctx, "INSERT INTO chat_requests(user_id,request_id,payload_hash,month,new_consultation,quota_reserved,status,expires_at,cancel_requested,error_code) VALUES(:user_id,?,'','',FALSE,FALSE,'cancelled',?,TRUE,'cancelled_before_accept') ON CONFLICT(user_id,request_id) DO NOTHING"), id, time.Now().UnixMilli()); err != nil {
		return consultation.RequestReceipt{}, err
	}
	if _, err := r.db.ExecContext(ctx, chatQuery(ctx, "UPDATE chat_requests SET cancel_requested=TRUE WHERE user_id=:user_id AND request_id=? AND status='processing'"), id); err != nil {
		return consultation.RequestReceipt{}, err
	}
	return r.GetChatRequest(ctx, id)
}
func (r *Repository) FailChatRequest(ctx context.Context, id, code string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	query := func(q string) string { return chatQuery(ctx, q) }
	var status, month string
	var reserved bool
	err = tx.QueryRowContext(ctx, query("SELECT status,month,quota_reserved FROM chat_requests WHERE user_id=:user_id AND request_id=?"), id).Scan(&status, &month, &reserved)
	if err != nil {
		return err
	}
	if status != "processing" && status != "failed" {
		return nil
	}
	if reserved {
		if _, err = tx.ExecContext(ctx, query("UPDATE monthly_consultation_usage SET consultation_count=consultation_count-1,updated_at=CURRENT_TIMESTAMP WHERE user_id=:user_id AND month=? AND consultation_count>0"), month); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, query("UPDATE chat_requests SET status='failed',error_code=?,quota_reserved=FALSE WHERE user_id=:user_id AND request_id=?"), code, id); err != nil {
		return err
	}
	return tx.Commit()
}
