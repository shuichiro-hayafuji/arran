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
	_, err = db.ExecContext(ctx, string(sqlBytes))
	return err
}

func ValidatePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("database path is empty")
	}
	return nil
}
