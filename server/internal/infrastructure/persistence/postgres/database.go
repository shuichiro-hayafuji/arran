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

func OpenDatabase(ctx context.Context, url string) (*database, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("database URL is empty")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &database{DB: db}, nil
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
	contents, err := migrationFiles.ReadFile("001_init.sql")
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, string(contents)); err != nil {
		return err
	}
	for _, migration := range []struct {
		version int
		file    string
	}{
		{version: 2, file: "002_auth.sql"},
		{version: 3, file: "003_consultation_limits.sql"},
		{version: 4, file: "004_consultation_usage.sql"},
		{version: 5, file: "005_admin_notifications.sql"},
		{version: 6, file: "006_free_user_limit.sql"},
		{version: 7, file: "007_llm_usage.sql"},
		{version: 8, file: "008_monthly_costs.sql"},
	} {
		var applied bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", migration.version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		contents, err = migrationFiles.ReadFile(migration.file)
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
