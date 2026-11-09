package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

//go:embed *.sql
var migrationFiles embed.FS

type database struct{ *sql.DB }

var migrations = []struct {
	version int
	file    string
}{
	{version: 1, file: "001_init.sql"},
	{version: 2, file: "002_auth.sql"},
	{version: 3, file: "003_consultation_limits.sql"},
	{version: 4, file: "004_consultation_usage.sql"},
	{version: 5, file: "005_admin_notifications.sql"},
	{version: 6, file: "006_free_user_limit.sql"},
	{version: 7, file: "007_llm_usage.sql"},
	{version: 8, file: "008_monthly_costs.sql"},
	{version: 9, file: "009_chat.sql"},
	{version: 10, file: "010_purchase_schedule.sql"},
	{version: 11, file: "011_chat_requests.sql"},
	{version: 12, file: "012_csv_identity.sql"},
	{version: 13, file: "013_purchase_csv_reconciliation.sql"},
	{version: 14, file: "014_import_management.sql"},
}

func openDatabase(ctx context.Context, url string, prepare func(context.Context, *sql.DB) error) (*database, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("database URL is empty")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		// Driver parse errors can include parts of the supplied connection string.
		return nil, fmt.Errorf("invalid PostgreSQL connection settings")
	}
	db.SetMaxOpenConns(10)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("unable to connect to PostgreSQL; check host, TLS and credentials")
	}
	if err := prepare(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &database{DB: db}, nil
}

// MigrateURL uses the credentials of the one-off migration process.
func MigrateURL(ctx context.Context, url string) error {
	db, err := openDatabase(ctx, url, Migrate)
	if err != nil {
		return err
	}
	return db.Close()
}

// appliedPrefix rejects holes and versions newer than the release. A transaction
// can use it under the migration lock; an API connection only needs SELECT.
func appliedPrefix(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) (int, error) {
	rows, err := q.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return 0, fmt.Errorf("read schema_migrations (run migrate before starting the API): %w", err)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return 0, err
		}
		if i == len(migrations) || migrations[i].version != version {
			return 0, fmt.Errorf("schema history does not match this release at version %d", version)
		}
		i++
	}
	return i, rows.Err()
}

func checkSchema(ctx context.Context, db *sql.DB) error {
	i, err := appliedPrefix(ctx, db)
	if err != nil {
		return err
	}
	if i != len(migrations) {
		return fmt.Errorf("schema migration %d is missing; run migrate before starting the API", migrations[i].version)
	}
	return nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 同時起動したサーバーが同じ移行を重ねて実行しないよう、バージョン確認前にロックする。
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(81729421)"); err != nil {
		return err
	}
	var hasLedger bool
	if err = tx.QueryRowContext(ctx, "SELECT to_regclass('schema_migrations') IS NOT NULL").Scan(&hasLedger); err != nil {
		return err
	}
	applied := 0
	if hasLedger {
		applied, err = appliedPrefix(ctx, tx)
		if err != nil {
			return err
		}
	}
	for _, migration := range migrations[applied:] {
		contents, err := migrationFiles.ReadFile(migration.file)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(contents)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *database) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.DB.ExecContext(ctx, userQuery(ctx, query), args...)
}
func (db *database) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, userQuery(ctx, query), args...)
}
func (db *database) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return db.DB.QueryRowContext(ctx, userQuery(ctx, query), args...)
}

func bind(query string) string {
	var out strings.Builder
	index := 1
	for _, character := range query {
		if character == '?' {
			fmt.Fprintf(&out, "$%d", index)
			index++
			continue
		}
		out.WriteRune(character)
	}
	return out.String()
}

// userQuery はSQLに明示した :user_id だけを認証済みIDに置換する。
// 認証情報なしでは0となる。所有者条件を自動追加する処理ではないため、SQL側の指定が必要。
func userQuery(ctx context.Context, query string) string {
	return bind(strings.ReplaceAll(query, ":user_id", strconv.FormatInt(identity.UserID(ctx), 10)))
}
