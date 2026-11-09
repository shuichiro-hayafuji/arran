package postgres

import (
	"context"
	"database/sql"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"sync"
	"testing"
)

func verifyReconciliationMigrationRollbackAndConcurrentRestart(t *testing.T) {
	ctx := context.Background()
	// Build the actual version-12 schema, not a version-14 schema with a hole in its ledger.
	r := openReconciliationSchema(t, func(ctx context.Context, db *sql.DB) error {
		for _, migration := range migrations[:12] {
			contents, err := migrationFiles.ReadFile(migration.file)
			if err != nil {
				return err
			}
			if _, err := db.ExecContext(ctx, string(contents)); err != nil {
				return err
			}
		}
		return nil
	})
	if _, err := r.db.Exec("CREATE TABLE purchase_csv_receipts(blocker TEXT)"); err != nil {
		t.Fatal(err)
	}
	var pgErr *pgconn.PgError
	if err := Migrate(ctx, r.db.DB); !errors.As(err, &pgErr) || pgErr.Code != "42P07" {
		t.Fatal("did not encounter injected duplicate table", err)
	}
	var n int
	if err := r.db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=13").Scan(&n); err != nil || n != 0 {
		t.Fatal("failed migration recorded", n, err)
	}
	if err := r.db.QueryRow("SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='purchase_csv_decisions'").Scan(&n); err != nil || n != 0 {
		t.Fatal("partial tables survived", n, err)
	}
	if _, err := r.db.Exec("DROP TABLE purchase_csv_receipts"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- Migrate(ctx, r.db.DB) }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, r.db.DB); err != nil {
		t.Fatal("restart", err)
	}
	if err := r.db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=13").Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}
