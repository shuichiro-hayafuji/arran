package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

// ReserveSyncConsultation binds one quota increment to a durable operation ID.
// Existing IDs never grant a second execution, including after an unknown commit.
func (r *Repository) ReserveSyncConsultation(ctx context.Context, id, month string) (domain.ConsultationUsage, error) {
	user := identity.UserID(ctx)
	if user <= 0 || id == "" || month == "" {
		return domain.ConsultationUsage{}, errors.New("invalid reservation identity")
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.ConsultationUsage{}, err
	}
	defer tx.Rollback()
	if err = lockActiveWithdrawalUser(ctx, tx, user); err != nil {
		return domain.ConsultationUsage{}, err
	}
	usage := domain.ConsultationUsage{Month: month}
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sync_consultation_reservations WHERE id=$1 AND user_id=$2)`, id, user).Scan(&exists); err != nil {
		return domain.ConsultationUsage{}, err
	}
	if exists {
		return domain.ConsultationUsage{}, errors.New("reservation already exists; execution not granted")
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO monthly_consultation_usage(user_id,month,consultation_count) VALUES($1,$2,1) ON CONFLICT(user_id,month) DO UPDATE SET consultation_count=monthly_consultation_usage.consultation_count+1,updated_at=CURRENT_TIMESTAMP RETURNING consultation_count`, user, month).Scan(&usage.Count)
	if err != nil {
		return domain.ConsultationUsage{}, err
	}
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(user_limits.monthly_limit,settings.integer_value) FROM service_settings AS settings LEFT JOIN user_consultation_limits AS user_limits ON user_limits.user_id=$1 WHERE settings.key='default_monthly_consultation_limit'`, user).Scan(&usage.Limit)
	if err != nil {
		return domain.ConsultationUsage{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sync_consultation_reservations(id,user_id,month,consultation_count,consultation_limit,state) VALUES($1,$2,$3,$4,$5,'active')`, id, user, month, usage.Count, usage.Limit)
	if err != nil {
		return domain.ConsultationUsage{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.ConsultationUsage{}, err
	}
	return usage, nil
}

// BeginSyncContinuation creates a durable fence without consuming quota.
func (r *Repository) BeginSyncContinuation(ctx context.Context, id string) error {
	user := identity.UserID(ctx)
	if user <= 0 || id == "" {
		return errors.New("invalid operation identity")
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockActiveWithdrawalUser(ctx, tx, user); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sync_consultation_reservations(id,user_id,month,consultation_count,consultation_limit,quota_reserved,state) VALUES($1,$2,'',0,0,FALSE,'active')`, id, user)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// FinishSyncConsultation is idempotent for the same outcome. Release and its
// decrement commit together so a retry cannot consume another request's slot.
func (r *Repository) FinishSyncConsultation(ctx context.Context, id string, release bool) error {
	user := identity.UserID(ctx)
	if user <= 0 || id == "" {
		return errors.New("invalid reservation identity")
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var month, state string
	var quotaReserved bool
	err = tx.QueryRowContext(ctx, `SELECT month,state,quota_reserved FROM sync_consultation_reservations WHERE id=$1 AND user_id=$2 FOR UPDATE`, id, user).Scan(&month, &state, &quotaReserved)
	if err != nil {
		return err
	}
	if release && !quotaReserved {
		return errors.New("continuation has no quota reservation")
	}
	desired := "completed"
	if release {
		desired = "released"
	}
	if state == desired {
		return tx.Commit()
	}
	if state != "active" {
		return errors.New("reservation outcome conflict")
	}
	if release {
		result, err := tx.ExecContext(ctx, `UPDATE monthly_consultation_usage SET consultation_count=consultation_count-1,updated_at=CURRENT_TIMESTAMP WHERE user_id=$1 AND month=$2 AND consultation_count>0`, user, month)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("reservation quota unavailable")
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE sync_consultation_reservations SET state=$1 WHERE id=$2 AND user_id=$3`, desired, id, user); err != nil {
		return err
	}
	return tx.Commit()
}
