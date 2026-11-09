package postgres

import (
	"context"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"sync"
	"testing"
)

func verifyImportManagementMigrationPreservesProvenAndUnknownLegacyAndRollsBack(t *testing.T, r *Repository) {
	ctx := identity.WithUser(context.Background(), 3440)
	parsed, _ := csvimport.Parse([]byte("date,description,amount\n2035-01-01,KNOWN LEGACY,2500\n"), csvimport.Options{Source: "bank", SourceAccountName: "account"})
	if _, err := r.ImportTransactions(ctx, parsed.Transactions); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.ExecContext(ctx, userQuery(ctx, "INSERT INTO transactions(user_id,transaction_date,description,normalized_merchant,amount,transaction_type,category,source,source_account_name,fingerprint,imported_at,raw_data_json) VALUES(:user_id,'2035-01-02','UNKNOWN LEGACY','UNKNOWN LEGACY',3000,'expense','未分類','unknown','unknown','migration-unknown','2035-01-01T00:00:00Z','{}')")); err != nil {
		t.Fatal(err)
	}
	// Emulate the prior schema; failure is injected after ALTERs and early CREATEs.
	for _, q := range []string{"DROP VIEW effective_transactions", "DROP TABLE csv_financial_items", "DROP TABLE csv_financial_receipts", "DROP TABLE transaction_change_events", "DROP TABLE csv_mutations", "DROP TABLE csv_import_evidence", "DROP TABLE csv_import_batches", "ALTER TABLE transactions DROP COLUMN effective_date", "ALTER TABLE transactions DROP COLUMN effective_amount", "ALTER TABLE transactions DROP COLUMN effective_category", "ALTER TABLE transactions DROP COLUMN revision", "ALTER TABLE transactions DROP COLUMN legacy_retained", "DELETE FROM schema_migrations WHERE version=14", "CREATE TABLE csv_financial_receipts(blocker TEXT)"} {
		if _, err := r.db.ExecContext(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	if err := Migrate(ctx, r.db.DB); err == nil {
		t.Fatal("late failure was not injected")
	}
	var n int
	if err := r.db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=14").Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if err := r.db.QueryRow("SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='csv_import_batches'").Scan(&n); err != nil || n != 0 {
		t.Fatal("partial tables survived", n, err)
	}
	if _, err := r.db.Exec("DROP TABLE csv_financial_receipts"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- Migrate(ctx, r.db.DB) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, r.db.DB); err != nil {
		t.Fatal("restart", err)
	}
	history, err := r.ImportHistory(ctx, "", 50)
	if err != nil || len(history) != 1 || history[0].RecordedAt != "" || !history[0].Legacy {
		t.Fatal(history, err)
	}
	batch, err := r.ImportDetails(ctx, history[0].ID)
	if err != nil || len(batch.Rows) != 1 || batch.Rows[0].Original.Amount != 2500 {
		t.Fatal(batch, err)
	}
	var retained bool
	if err = r.db.QueryRowContext(ctx, userQuery(ctx, "SELECT legacy_retained FROM transactions WHERE user_id=:user_id AND description='UNKNOWN LEGACY'")).Scan(&retained); err != nil || !retained {
		t.Fatal("unproven unit not protected", retained, err)
	}
	if rows, e := r.ListTransactions(ctx, "", 100); e != nil || len(rows) != 2 {
		t.Fatal(rows, e)
	}
	// Unknown legacy evidence stays active and independently correctable, rather
	// than being grouped into the known import by an invented timestamp.
	if _, err = r.MutateCSV(ctx, batch.ID, domain.MutationInput{ID: "cancel-migrated", Action: "cancel", Version: batch.Version, StateVersion: batch.StateVersion, Snapshot: batch.Snapshot}); err != nil {
		t.Fatal(err)
	}
	if rows, e := r.ListTransactions(ctx, "", 100); e != nil || len(rows) != 1 || rows[0].Description != "UNKNOWN LEGACY" {
		t.Fatal(rows, e)
	}
}
