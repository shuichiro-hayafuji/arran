package sqlite

import (
	"context"
	"database/sql"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/migrations"
	"path/filepath"
	"sync"
	"testing"
)

func TestCSVSchemaRollbackOwnersAndRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open(sqliteDriverName, path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	for _, file := range []string{"001_init.sql", "003_chat.sql"} {
		data, _ := migrations.Files.ReadFile(file)
		if _, err = db.ExecContext(ctx, string(data)); err != nil {
			t.Fatal(err)
		}
	}
	_, err = db.ExecContext(ctx, "INSERT INTO transactions(id,transaction_date,description,normalized_merchant,amount,transaction_type,category,source,source_account_name,fingerprint,imported_at,raw_data_json) VALUES(41,'2032-01-01','店','店',2500,'expense','学習','bank','account','old-key','old-time','{}')")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "INSERT INTO chat_transaction_owners VALUES(41,301)"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "CREATE TABLE transactions_v2(blocker TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err == nil {
		t.Fatal("migration failure not injected")
	}
	var category string
	if err = db.QueryRow("SELECT category FROM transactions WHERE id=41").Scan(&category); err != nil || category != "学習" {
		t.Fatal("failed migration damaged legacy row")
	}
	var owner int
	if err = db.QueryRow("SELECT user_id FROM chat_transaction_owners WHERE transaction_id=41").Scan(&owner); err != nil || owner != 301 {
		t.Fatal("failed migration damaged ownership")
	}
	var applied int
	db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=6").Scan(&applied)
	if applied != 0 {
		t.Fatal("failed migration recorded as applied")
	}
	db.Exec("DROP TABLE transactions_v2")
	if _, err = db.Exec("CREATE TABLE csv_identity_aliases(blocker TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err == nil {
		t.Fatal("late migration failure not injected")
	}
	if err = db.QueryRow("SELECT category FROM transactions WHERE id=41").Scan(&category); err != nil || category != "学習" {
		t.Fatal("late rollback damaged legacy row")
	}
	db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='csv_identity_file_rows'").Scan(&applied)
	if applied != 0 {
		t.Fatal("late rollback retained file bindings")
	}
	db.Exec("DROP TABLE csv_identity_aliases")
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT user_id FROM transactions WHERE id=41").Scan(&owner); err != nil || owner != 301 {
		t.Fatal("new owner assignment lost")
	}
	var foreignKeys int
	db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
	if foreignKeys != 1 {
		t.Fatal("foreign keys not restored")
	}
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		rows.Close()
		t.Fatal("ownership mapping FK damaged")
	}
	rows.Close()
	r := &Repository{db: db}
	data := []byte("日付,摘要,金額\n2032-01-02,新しい店,2500\n2032-01-02,新しい店,2500\n")
	parsed, err := csvimport.Parse(data, csvimport.Options{Source: "bank", SourceAccountName: "account"})
	if err != nil {
		t.Fatal(err)
	}
	ca := identity.WithUser(ctx, 301)
	cb := identity.WithUser(ctx, 302)
	first, err := r.ImportTransactions(ca, parsed.Transactions)
	if err != nil || first.ImportedCount != 2 {
		t.Fatal(err)
	}
	if _, err = r.ImportTransactions(cb, parsed.Transactions); err != nil {
		t.Fatal("SQLite global fingerprint still applies", err)
	}
	if _, err = r.GetTransaction(cb, 41); err == nil {
		t.Fatal("old row exposed to another owner")
	}
	if _, err = r.UpdateTransactionCategory(cb, 41, "健康", false); err == nil {
		t.Fatal("old row modified by another owner")
	}
	db.Close()
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	result, err := reopened.ImportTransactions(ca, parsed.Transactions)
	if err != nil || result.ImportedCount != 0 || result.DuplicateCount != 2 {
		t.Fatalf("restart identity lost %+v %v", result, err)
	}
	if row, err := reopened.GetTransaction(ca, 41); err != nil || row.Category != "学習" || row.ImportedAt != "old-time" {
		t.Fatal("legacy row metadata lost")
	}
	// Concurrent migrations on the same file are serialized before version check.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, e := sql.Open(sqliteDriverName, path)
			if e == nil {
				conn.SetMaxOpenConns(1)
				e = Migrate(ctx, conn)
				conn.Close()
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
}
