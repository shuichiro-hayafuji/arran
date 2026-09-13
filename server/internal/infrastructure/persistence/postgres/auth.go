package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/auth"
)

func (r *Repository) FindUser(ctx context.Context, username string) (auth.User, error) {
	var user auth.User
	err := r.db.DB.QueryRowContext(ctx, `SELECT id,username,password_hash,is_active FROM users WHERE username=$1`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Active)
	if errors.Is(err, sql.ErrNoRows) {
		err = auth.ErrInvalid
	}
	return user, err
}
func (r *Repository) AllowLogin(ctx context.Context, ip, username string) (bool, error) {
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM auth_attempts WHERE window_end <= CURRENT_TIMESTAMP`); err != nil {
		return false, err
	}
	allowed := true
	for i, key := range []string{ip, username} {
		var count int
		err = tx.QueryRowContext(ctx, `INSERT INTO auth_attempts(key,attempts,window_end) VALUES($1,1,CURRENT_TIMESTAMP+INTERVAL '1 minute') ON CONFLICT(key) DO UPDATE SET attempts=auth_attempts.attempts+1 RETURNING attempts`, key).Scan(&count)
		if err != nil {
			return false, err
		}
		limit := 20
		if i == 1 {
			limit = 5
		}
		allowed = allowed && count <= limit
	}
	return allowed, tx.Commit()
}
func (r *Repository) CreateSession(ctx context.Context, hash string, user auth.User, expires time.Time) error {
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM auth_sessions WHERE expires_at <= CURRENT_TIMESTAMP`); err != nil {
		return err
	}
	// Compare the credential snapshot in the INSERT to handle concurrent disable/reset.
	result, err := tx.ExecContext(ctx, `INSERT INTO auth_sessions(token_hash,user_id,password_hash,expires_at) SELECT $1,id,password_hash,$4 FROM users WHERE id=$2 AND password_hash=$3 AND is_active`, hash, user.ID, user.PasswordHash, expires)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return auth.ErrInvalid
	}
	return tx.Commit()
}
func (r *Repository) ResolveSession(ctx context.Context, hash string) (auth.User, error) {
	var user auth.User
	err := r.db.DB.QueryRowContext(ctx, `SELECT u.id,u.username FROM auth_sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>CURRENT_TIMESTAMP AND u.is_active AND s.password_hash=u.password_hash`, hash).Scan(&user.ID, &user.Username)
	if errors.Is(err, sql.ErrNoRows) {
		err = auth.ErrInvalid
	}
	return user, err
}
func (r *Repository) DeleteSession(ctx context.Context, hash string) error {
	_, err := r.db.DB.ExecContext(ctx, `DELETE FROM auth_sessions WHERE token_hash=$1`, hash)
	return err
}
