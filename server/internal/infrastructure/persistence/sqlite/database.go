// Package sqlite is the SQLite infrastructure adapter. It owns database
// opening, migrations, and the application repository implementation.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shuichirohayafuji/spendable-today/server/migrations"
)

// Open opens a database using a driver registered by the infrastructure
// package and applies the embedded schema before returning it.
func OpenDatabase(ctx context.Context, path string) (*sql.DB, error) {
	if err := ValidatePath(path); err != nil {
		return nil, err
	}
	if path != ":memory:" {
		path = filepath.Clean(path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	sqlBytes, err := migrations.Files.ReadFile("001_init.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(sqlBytes)); err != nil {
		return err
	}
	sqlBytes, err = migrations.Files.ReadFile("003_chat.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(sqlBytes)); err != nil {
		return err
	}
	sqlBytes, err = migrations.Files.ReadFile("004_purchase_schedule.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(sqlBytes)); err != nil {
		return err
	}
	sqlBytes, err = migrations.Files.ReadFile("005_chat_requests.sql")
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, string(sqlBytes)); err != nil {
		return err
	}
	return migrateCSVIdentity(ctx, db)
}

func ValidatePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("database path is empty")
	}
	return nil
}

func migrateCSVIdentity(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return err
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), "PRAGMA foreign_keys=ON")
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
	var count int
	if err = conn.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE version=6").Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		contents, e := migrations.Files.ReadFile("006_csv_identity.sql")
		if e != nil {
			return e
		}
		if _, err = conn.ExecContext(ctx, string(contents)); err != nil {
			return err
		}
	}
	if _, err = conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	return nil
}
