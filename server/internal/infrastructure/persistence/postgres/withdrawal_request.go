package postgres

import (
	"context"
	"errors"
)

func (r *Repository) WithdrawalRequested(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, errors.New("authenticated user required")
	}
	var requested bool
	err := r.db.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM withdrawal_requests WHERE user_id=$1)", userID).Scan(&requested)
	return requested, err
}
func (r *Repository) RequestWithdrawal(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return errors.New("authenticated user required")
	}
	_, err := r.db.DB.ExecContext(ctx, "INSERT INTO withdrawal_requests(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING", userID)
	return err
}
