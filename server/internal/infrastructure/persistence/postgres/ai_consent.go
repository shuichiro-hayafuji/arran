package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

var _ identity.AIConsentStore = (*Repository)(nil)

func consentOwner(ctx context.Context, userID int64) error {
	if userID <= 0 || identity.UserID(ctx) != userID {
		return identity.ErrInvalidUser
	}
	return ctx.Err()
}

// lockConsentPolicy prevents a policy update from overtaking admission.
func lockConsentPolicy(ctx context.Context, tx *sql.Tx, version string) error {
	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT singleton FROM ai_consent_policy_lock WHERE singleton=TRUE FOR SHARE`).Scan(&locked); err != nil {
		return err
	}
	var current sql.NullString
	var description sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT version,description FROM ai_consent_policy WHERE singleton=TRUE`).Scan(&current, &description); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return identity.ErrConsentRequired
		}
		return err
	}
	if !current.Valid || current.String == "" || current.String != version || !description.Valid || description.String == "" {
		return identity.ErrConsentRequired
	}
	return nil
}

func (r *Repository) ConsentStatus(ctx context.Context, userID int64, version string) (identity.AIConsentStatus, error) {
	if err := consentOwner(ctx, userID); err != nil {
		return identity.AIConsentStatus{}, err
	}
	var status identity.AIConsentStatus
	err := r.db.DB.QueryRowContext(ctx, `SELECT p.version,p.description,
		COALESCE(c.granted_version=p.version AND NOT c.withdrawal_pending,FALSE),
		COALESCE(c.withdrawal_pending,FALSE)
		FROM ai_consent_policy p LEFT JOIN ai_user_consents c ON c.user_id=$1
		WHERE p.singleton=TRUE AND p.version=$2 AND p.description IS NOT NULL`, userID, version).Scan(
		&status.Version, &status.Description, &status.Granted, &status.WithdrawalPending)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.AIConsentStatus{}, identity.ErrConsentRequired
	}
	return status, err
}

// lockConsentUser serializes the first grant, admission and withdrawal even
// before a consent row exists. Missing users never acquire a consent record.
func lockConsentUser(ctx context.Context, tx *sql.Tx, userID int64) error {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.ErrInvalidUser
	}
	return err
}

func (r *Repository) GrantConsent(ctx context.Context, userID int64, version string) error {
	if err := consentOwner(ctx, userID); err != nil {
		return err
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockConsentPolicy(ctx, tx, version); err != nil {
		return err
	}
	if err = lockConsentUser(ctx, tx, userID); err != nil {
		return err
	}
	if err = lockActiveWithdrawalUser(ctx, tx, userID); err != nil {
		return err
	}
	var pending bool
	err = tx.QueryRowContext(ctx, `SELECT withdrawal_pending FROM ai_user_consents WHERE user_id=$1`, userID).Scan(&pending)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if pending {
		return identity.ErrConsentRequired
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ai_user_consents(user_id,granted_version) VALUES($1,$2)
		ON CONFLICT(user_id) DO UPDATE SET granted_version=EXCLUDED.granted_version,updated_at=CURRENT_TIMESTAMP`, userID, version)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) WithConsent(ctx context.Context, userID int64, version string, fn func(context.Context) error) error {
	if err := consentOwner(ctx, userID); err != nil {
		return err
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	id := hex.EncodeToString(random[:])
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockConsentPolicy(ctx, tx, version); err != nil {
		return err
	}
	if err = lockConsentUser(ctx, tx, userID); err != nil {
		return err
	}
	var grant sql.NullString
	if err = lockActiveWithdrawalUser(ctx, tx, userID); err != nil {
		return err
	}
	var pending bool
	err = tx.QueryRowContext(ctx, `SELECT granted_version,withdrawal_pending FROM ai_user_consents WHERE user_id=$1`, userID).Scan(&grant, &pending)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.ErrConsentRequired
	}
	if err != nil {
		return err
	}
	if pending || !grant.Valid || grant.String != version {
		return identity.ErrConsentRequired
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO ai_consent_inflight(id,user_id,version) VALUES($1,$2,$3)`, id, userID, version); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// On any uncertain outcome the marker remains, including callback errors,
	// process termination and failures while clearing it.
	if err = fn(ctx); err != nil {
		return err
	}
	cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := r.db.DB.ExecContext(cleanup, `DELETE FROM ai_consent_inflight WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("AI send completion marker missing")
	}
	return nil
}

// beginConsentWithdrawal records withdrawal before waiting. Calls that overlap
// while this state is pending share one withdrawal generation.
func (r *Repository) beginConsentWithdrawal(ctx context.Context, userID int64) error {
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockConsentUser(ctx, tx, userID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ai_user_consents(user_id,granted_version,withdrawal_pending) VALUES($1,NULL,TRUE)
		ON CONFLICT(user_id) DO UPDATE SET granted_version=NULL,withdrawal_pending=TRUE,updated_at=CURRENT_TIMESTAMP`, userID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// finishConsentWithdrawal clears the current pending state once no sends remain.
// A later withdrawal may set pending again before an older caller observes it;
// the older caller then waits for the later withdrawal as well.
func (r *Repository) finishConsentWithdrawal(ctx context.Context, userID int64) (bool, error) {
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err = lockConsentUser(ctx, tx, userID); err != nil {
		return false, err
	}
	var pending bool
	if err = tx.QueryRowContext(ctx, `SELECT withdrawal_pending FROM ai_user_consents WHERE user_id=$1`, userID).Scan(&pending); err != nil {
		return false, err
	}
	if !pending {
		return true, nil
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM ai_consent_inflight WHERE user_id=$1`, userID).Scan(&count); err != nil {
		return false, err
	}
	if count != 0 {
		return false, nil
	}
	_, err = tx.ExecContext(ctx, `UPDATE ai_user_consents SET withdrawal_pending=FALSE,updated_at=CURRENT_TIMESTAMP WHERE user_id=$1`, userID)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (r *Repository) WithdrawConsent(ctx context.Context, userID int64) error {
	if err := consentOwner(ctx, userID); err != nil {
		return err
	}
	if err := r.beginConsentWithdrawal(ctx, userID); err != nil {
		return err
	}
	for {
		done, err := r.finishConsentWithdrawal(ctx, userID)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
