package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func (r *Repository) WithdrawalRequested(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 || identity.UserID(ctx) != userID {
		return false, errors.New("authenticated user required")
	}
	var requested bool
	err := r.db.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM withdrawal_requests WHERE user_id=$1)", userID).Scan(&requested)
	return requested, err
}
func (r *Repository) RequestWithdrawal(ctx context.Context, userID int64) error {
	if userID <= 0 || identity.UserID(ctx) != userID {
		return errors.New("authenticated user required")
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize the request's commit boundary with the midnight snapshot. The
	// DB default uses clock_timestamp() after acquiring this lock, so a request
	// admitted after a completed batch is scheduled for the next midnight.
	if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout='5s'`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock_shared(1634890337,1)`); err != nil {
		return err
	}
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=$1 FOR NO KEY UPDATE`, userID).Scan(&active); err != nil {
		return err
	}
	if !active {
		return errors.New("inactive user")
	}
	var completed sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT completed_at FROM withdrawal_requests WHERE user_id=$1`, userID).Scan(&completed)
	if err == nil {
		if completed.Valid {
			return identity.ErrWithdrawalBlocked
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err := withdrawalBatchReady(ctx, tx); err != nil {
		return err
	}
	var busy bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ai_consent_inflight WHERE user_id=$1) OR EXISTS(SELECT 1 FROM chat_requests WHERE user_id=$1 AND (status='processing' OR (status='failed' AND quota_reserved))) OR EXISTS(SELECT 1 FROM admin_notifications WHERE user_id=$1 AND (status = 'sending' OR withdrawal_uncertain)) OR EXISTS(SELECT 1 FROM sync_consultation_reservations WHERE user_id=$1 AND state='active') OR EXISTS(SELECT 1 FROM ai_usage_operations WHERE user_id=$1 AND usage_id IS NULL AND no_usage_reason IS NULL) OR EXISTS(SELECT 1 FROM evidence_intents started WHERE started.user_id=$1 AND started.kind='send_started' AND NOT EXISTS (SELECT 1 FROM evidence_intents finished WHERE finished.user_id=started.user_id AND finished.ledger_id=started.ledger_id AND finished.owner_id=started.owner_id AND finished.policy_version=started.policy_version AND finished.related_operation_id=started.operation_id AND finished.kind='send_completed' AND finished.state='witnessed'))`, userID).Scan(&busy); err != nil {
		return err
	}
	if busy {
		return identity.ErrWithdrawalBlocked
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO withdrawal_requests(user_id) VALUES($1)`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func withdrawalBatchReady(ctx context.Context, tx *sql.Tx) error {
	var ready bool
	err := tx.QueryRowContext(ctx, `SELECT
		COALESCE((SELECT checked_at > CURRENT_TIMESTAMP - INTERVAL '3 minutes' AND checked_at <= CURRENT_TIMESTAMP FROM withdrawal_batch_health WHERE singleton=TRUE),FALSE)
		AND has_table_privilege(current_user,'users','SELECT')
		AND has_table_privilege(current_user,'users','UPDATE')
		AND has_table_privilege(current_user,'auth_sessions','SELECT')
		AND has_table_privilege(current_user,'auth_sessions','DELETE')
		AND has_any_column_privilege(current_user,'auth_sessions','UPDATE')
		AND has_table_privilege(current_user,'withdrawal_requests','SELECT')
		AND has_column_privilege(current_user,'withdrawal_requests','completed_at','UPDATE')
		AND has_table_privilege(current_user,'chat_requests','SELECT')
		AND has_table_privilege(current_user,'admin_notifications','SELECT')
		AND has_table_privilege(current_user,'sync_consultation_reservations','SELECT')
		AND has_table_privilege(current_user,'ai_usage_operations','SELECT')
		AND has_table_privilege(current_user,'evidence_intents','SELECT')
		AND has_table_privilege(current_user,'ai_consent_inflight','SELECT')`).Scan(&ready)
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("withdrawal scheduler is not recently verified")
	}
	return nil
}

// RefreshWithdrawalBatchHealth is invoked only by the operator's health check
// after it verifies the scheduler. It is evidence of a recent check, not a
// guarantee that a later batch can run.
func (r *Repository) RefreshWithdrawalBatchHealth(ctx context.Context) error {
	result, err := r.db.DB.ExecContext(ctx, `UPDATE withdrawal_batch_health SET checked_at=CURRENT_TIMESTAMP WHERE singleton=TRUE`)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("withdrawal health row unavailable")
	}
	return nil
}

// lockActiveWithdrawalUser serializes work admission with withdrawal admission
// and the midnight batch. Callers must hold the lock until admission commits.
func lockActiveWithdrawalUser(ctx context.Context, tx *sql.Tx, userID int64) error {
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=$1 FOR NO KEY UPDATE`, userID).Scan(&active); err != nil {
		return err
	}
	if !active {
		return errors.New("inactive user")
	}
	var pending bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM withdrawal_requests WHERE user_id=$1 AND completed_at IS NULL)`, userID).Scan(&pending); err != nil {
		return err
	}
	if pending {
		return identity.ErrWithdrawalPending
	}
	return nil
}

// RunWithdrawalBatch processes requests received before the most recent
// midnight in Japan. Each account is disabled and its sessions revoked in one
// transaction; a busy or failed account remains pending for a later retry.
func (r *Repository) RunWithdrawalBatch(ctx context.Context, now time.Time) error {
	// Keep the exclusive gate until every eligible request has been attempted.
	// Failures are retried by the service, including an unavailable gate.
	gate, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer gate.Rollback()
	if _, err := gate.ExecContext(ctx, `SET LOCAL lock_timeout='5s'`); err != nil {
		return err
	}
	if _, err := gate.ExecContext(ctx, `SELECT pg_advisory_xact_lock(1634890337,1)`); err != nil {
		return err
	}
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	local := now.In(jst)
	cutoff := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, jst)
	rows, err := gate.QueryContext(ctx, `SELECT user_id FROM withdrawal_requests WHERE completed_at IS NULL AND requested_at < $1 ORDER BY user_id`, cutoff)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var failures []error
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.completeWithdrawal(ctx, id, cutoff); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) != 0 {
		return errors.Join(failures...)
	}
	return gate.Commit()
}

func (r *Repository) completeWithdrawal(ctx context.Context, userID int64, cutoff time.Time) error {
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout='5s'`); err != nil {
		return err
	}
	var active bool
	// A locked account remains pending for retry; it must not consume the
	// batch's entire deadline and starve later accounts.
	if err := tx.QueryRowContext(ctx, `SELECT is_active FROM users WHERE id=$1 FOR NO KEY UPDATE SKIP LOCKED`, userID).Scan(&active); err != nil {
		return err
	}
	var requested time.Time
	var completed sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT requested_at,completed_at FROM withdrawal_requests WHERE user_id=$1 FOR UPDATE SKIP LOCKED`, userID).Scan(&requested, &completed); err != nil {
		return err
	}
	if completed.Valid || !requested.Before(cutoff) {
		return tx.Commit()
	}
	var busy bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM ai_consent_inflight WHERE user_id=$1) OR EXISTS(SELECT 1 FROM chat_requests WHERE user_id=$1 AND (status='processing' OR (status='failed' AND quota_reserved))) OR EXISTS(SELECT 1 FROM admin_notifications WHERE user_id=$1 AND (status = 'sending' OR withdrawal_uncertain)) OR EXISTS(SELECT 1 FROM sync_consultation_reservations WHERE user_id=$1 AND state='active') OR EXISTS(SELECT 1 FROM ai_usage_operations WHERE user_id=$1 AND usage_id IS NULL AND no_usage_reason IS NULL) OR EXISTS(SELECT 1 FROM evidence_intents started WHERE started.user_id=$1 AND started.kind='send_started' AND NOT EXISTS (SELECT 1 FROM evidence_intents finished WHERE finished.user_id=started.user_id AND finished.ledger_id=started.ledger_id AND finished.owner_id=started.owner_id AND finished.policy_version=started.policy_version AND finished.related_operation_id=started.operation_id AND finished.kind='send_completed' AND finished.state='witnessed'))`, userID).Scan(&busy); err != nil {
		return err
	}
	if busy {
		return errors.New("withdrawal remains pending: unresolved work")
	}
	// Do not wait per session: many independently locked sessions must not
	// starve later owners. Any contention rolls back this owner for retry.
	sessions, err := tx.QueryContext(ctx, `SELECT token_hash FROM auth_sessions WHERE user_id=$1 FOR UPDATE NOWAIT`, userID)
	if err != nil {
		return err
	}
	for sessions.Next() {
		var hash string
		if err := sessions.Scan(&hash); err != nil {
			sessions.Close()
			return err
		}
	}
	if err := sessions.Err(); err != nil {
		sessions.Close()
		return err
	}
	if err := sessions.Close(); err != nil {
		return err
	}
	if active {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET is_active=FALSE WHERE id=$1`, userID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE withdrawal_requests SET completed_at=CURRENT_TIMESTAMP WHERE user_id=$1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}
