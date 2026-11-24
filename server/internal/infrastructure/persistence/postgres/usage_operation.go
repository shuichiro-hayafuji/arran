package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

// beginUsageOperation is an unconnected prototype. Callers must establish the
// marker before any external work and not treat an uncertain commit as success.
func (r *Repository) beginUsageOperation(ctx context.Context, id string) error {
	user := identity.UserID(ctx)
	if user <= 0 || id == "" {
		return errors.New("invalid usage operation")
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockActiveWithdrawalUser(ctx, tx, user); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO ai_usage_operations(user_id,operation_id) VALUES($1,$2)`, user, id); err != nil {
		return err
	}
	return tx.Commit()
}

// markUsageSending must commit before any network call. A sending marker is
// never automatically cleared on restart or a lost response.
func (r *Repository) markUsageSending(ctx context.Context, id string) error {
	user := identity.UserID(ctx)
	if user <= 0 || id == "" {
		return errors.New("invalid usage operation")
	}
	result, err := r.db.DB.ExecContext(ctx, `UPDATE ai_usage_operations SET send_state='sending' WHERE user_id=$1 AND operation_id=$2 AND send_state='prepared' AND usage_id IS NULL AND no_usage_reason IS NULL AND staged_usage IS NULL`, user, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("usage send admission unavailable")
	}
	return nil
}

// stageUsageOperation persists the exact provider accounting payload for
// restart recovery. Repeating an ID with different data is always rejected.
func (r *Repository) stageUsageOperation(ctx context.Context, id string, usage domain.LLMUsage) error {
	user := identity.UserID(ctx)
	if user <= 0 || usage.UserID != user || id == "" {
		return errors.New("invalid usage operation")
	}
	if usage.InputTokens < 0 || usage.CachedInputTokens < 0 || usage.OutputTokens < 0 || usage.ReasoningTokens < 0 || usage.TotalTokens < 0 {
		return errors.New("invalid usage token count")
	}
	data, err := json.Marshal(usage)
	if err != nil {
		return err
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var staged []byte
	var reason sql.NullString
	var sendState string
	if err = tx.QueryRowContext(ctx, `SELECT staged_usage,no_usage_reason,send_state FROM ai_usage_operations WHERE user_id=$1 AND operation_id=$2 FOR UPDATE`, user, id).Scan(&staged, &reason, &sendState); err != nil {
		return err
	}
	if reason.Valid || sendState != "sending" {
		return errors.New("usage operation is not sending")
	}
	if staged != nil {
		var equal bool
		if err = tx.QueryRowContext(ctx, `SELECT staged_usage=$3::jsonb FROM ai_usage_operations WHERE user_id=$1 AND operation_id=$2`, user, id, string(data)).Scan(&equal); err != nil {
			return err
		}
		if !equal {
			return errors.New("usage staging payload conflict")
		}
		return tx.Commit()
	}
	result, err := tx.ExecContext(ctx, `UPDATE ai_usage_operations SET staged_usage=$3::jsonb WHERE user_id=$1 AND operation_id=$2`, user, id, string(data))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("usage staging unavailable")
	}
	return tx.Commit()
}

// loadStagedUsage is owner-scoped and can recover the original payload after a
// process restart. Missing or malformed state is never interpreted as success.
func (r *Repository) loadStagedUsage(ctx context.Context, id string) (domain.LLMUsage, error) {
	user := identity.UserID(ctx)
	var data []byte
	var usage domain.LLMUsage
	if user <= 0 || id == "" {
		return usage, errors.New("invalid usage operation")
	}
	if err := r.db.DB.QueryRowContext(ctx, `SELECT staged_usage FROM ai_usage_operations WHERE user_id=$1 AND operation_id=$2`, user, id).Scan(&data); err != nil {
		return usage, err
	}
	if len(data) == 0 {
		return usage, errors.New("usage payload unavailable")
	}
	if err := json.Unmarshal(data, &usage); err != nil {
		return usage, err
	}
	if usage.UserID != user {
		return usage, errors.New("usage payload owner mismatch")
	}
	return usage, nil
}

// completeUsageWithoutRecord requires authoritative evidence from the caller:
// no request was sent, as proven by the durable prepared state.
// This depends on a future adapter enforcing markUsageSending before any
// network call. A sending marker cannot be closed by this method, and there
// is no terminal no-usage recovery without independently verified evidence.
func (r *Repository) completeUsageWithoutRecord(ctx context.Context, id, reason string) error {
	user := identity.UserID(ctx)
	if user <= 0 || id == "" || reason != "not_sent" {
		return errors.New("invalid no-usage outcome")
	}
	result, err := r.db.DB.ExecContext(ctx, `UPDATE ai_usage_operations SET no_usage_reason=$3 WHERE user_id=$1 AND operation_id=$2 AND usage_id IS NULL AND staged_usage IS NULL AND send_state='prepared' AND (no_usage_reason IS NULL OR no_usage_reason=$3)`, user, id, reason)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("no-usage outcome unavailable")
	}
	return nil
}

// completeUsageOperation commits both the usage row and its receipt. A retry
// after an unknown commit checks exact payload equality instead of inserting again.
func (r *Repository) completeUsageOperation(ctx context.Context, id string, usage domain.LLMUsage) error {
	user := identity.UserID(ctx)
	if user <= 0 || id == "" || usage.UserID != user {
		return errors.New("invalid usage operation")
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var receipt sql.NullInt64
	var staged []byte
	var reason sql.NullString
	if err = tx.QueryRowContext(ctx, `SELECT usage_id,staged_usage,no_usage_reason FROM ai_usage_operations WHERE user_id=$1 AND operation_id=$2 FOR UPDATE`, user, id).Scan(&receipt, &staged, &reason); err != nil {
		return err
	}
	if reason.Valid || len(staged) == 0 {
		return errors.New("usage operation is not staged")
	}
	data, err := json.Marshal(usage)
	if err != nil {
		return err
	}
	var stagedEqual bool
	if err = tx.QueryRowContext(ctx, `SELECT staged_usage=$3::jsonb FROM ai_usage_operations WHERE user_id=$1 AND operation_id=$2`, user, id, string(data)).Scan(&stagedEqual); err != nil {
		return err
	}
	if !stagedEqual {
		return errors.New("usage staging payload conflict")
	}
	if receipt.Valid {
		var equal bool
		err = tx.QueryRowContext(ctx, `SELECT operation=$1 AND model=$2 AND input_tokens=$3 AND cached_input_tokens=$4 AND output_tokens=$5 AND reasoning_tokens=$6 AND total_tokens=$7 AND occurred_at=$8 FROM llm_usage WHERE id=$9 AND user_id=$10`, usage.Operation, usage.Model, usage.InputTokens, usage.CachedInputTokens, usage.OutputTokens, usage.ReasoningTokens, usage.TotalTokens, usage.OccurredAt, receipt.Int64, user).Scan(&equal)
		if err != nil {
			return err
		}
		if !equal {
			return errors.New("usage receipt payload conflict")
		}
		return tx.Commit()
	}
	var usageID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO llm_usage(user_id,operation,model,input_tokens,cached_input_tokens,output_tokens,reasoning_tokens,total_tokens,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, user, usage.Operation, usage.Model, usage.InputTokens, usage.CachedInputTokens, usage.OutputTokens, usage.ReasoningTokens, usage.TotalTokens, usage.OccurredAt).Scan(&usageID)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE ai_usage_operations SET usage_id=$1 WHERE user_id=$2 AND operation_id=$3`, usageID, user, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("usage receipt update unavailable")
	}
	return tx.Commit()
}

// runUsageSend is an unwired adapter prototype. It separates the provider
// outcome from accounting so accounting failures never discard an answer.
// Sending is invoked only after durable admission; unknown failures remain
// pending. A missing usage payload remains pending until verifiable evidence
// and a safe recovery path exist.
func (r *Repository) runUsageSend(ctx context.Context, id string, send func(context.Context) (*domain.LLMUsage, error)) (sendErr, accountingErr error) {
	if send == nil {
		return errors.New("usage send callback required"), nil
	}
	if err := r.beginUsageOperation(ctx, id); err != nil {
		return err, nil
	}
	if err := r.markUsageSending(ctx, id); err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.completeUsageWithoutRecord(cleanup, id, "not_sent")
		return err, nil
	}
	usage, sendErr := send(ctx)
	if usage == nil {
		return sendErr, errors.New("usage outcome unconfirmed")
	}
	accountingCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := r.stageUsageOperation(accountingCtx, id, *usage); err != nil {
		return sendErr, err
	}
	if err := r.completeUsageOperation(accountingCtx, id, *usage); err != nil {
		return sendErr, err
	}
	return sendErr, nil
}
