package postgres

import (
	"context"
	"sync"
	"testing"
)

func verifyReconciliationMigrationRollbackAndConcurrentRestart(t *testing.T, r *Repository) {
	ctx := context.Background()
	for _, q := range []string{"DROP TABLE purchase_csv_receipts", "DROP TABLE purchase_csv_decisions", "DELETE FROM schema_migrations WHERE version=13", "CREATE TABLE purchase_csv_receipts(blocker TEXT)"} {
		if _, err := r.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, r.db.DB); err == nil {
		t.Fatal("late migration failure not injected")
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
