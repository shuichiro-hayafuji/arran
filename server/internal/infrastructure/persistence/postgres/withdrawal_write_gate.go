package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

// withWithdrawalWriteTx is an unconnected prototype for a write performed
// entirely on the transaction that holds the user's row lock. A separate
// connection holding an advisory lease cannot enforce this guarantee: losing
// that connection releases the lease while another connection can still commit.
//
// Callers must not use a different connection, perform an external side effect,
// start a goroutine, or report success before this function returns. It is not
// a general HTTP drain, nor a proof about legacy or third-party writers. This
// prototype does not authorize enabling withdrawal intake.
type withdrawalWriteSession interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// A wrapper, rather than *sql.Tx itself, keeps Commit and Rollback outside the
// callback's interface and prevents type-asserting the interface back to *sql.Tx.
// It cannot prevent a closure from using a different connection or owner ID.
type withdrawalWriteQueries struct{ tx *sql.Tx }

func (q withdrawalWriteQueries) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return q.tx.ExecContext(ctx, query, args...)
}
func (q withdrawalWriteQueries) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.tx.QueryContext(ctx, query, args...)
}
func (q withdrawalWriteQueries) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return q.tx.QueryRowContext(ctx, query, args...)
}

func (r *Repository) withWithdrawalWriteTx(ctx context.Context, userID int64, write func(context.Context, withdrawalWriteSession) error) error {
	if userID <= 0 || identity.UserID(ctx) != userID || write == nil {
		return errors.New("authenticated owner and transaction callback required")
	}
	tx, err := r.db.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockActiveWithdrawalUser(ctx, tx, userID); err != nil {
		return err
	}
	if err := write(ctx, withdrawalWriteQueries{tx: tx}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return tx.Commit()
}
