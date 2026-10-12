package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

func verifyCSVMigration(t *testing.T, root *Repository) {
	t.Helper()
	ctx := context.Background()
	schema := fmt.Sprintf("csv_migration_%d", time.Now().UnixNano())
	if _, err := root.db.DB.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.db.DB.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE") })
	address, err := url.Parse(os.Getenv("ARRAN_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	params := address.Query()
	params.Set("search_path", schema)
	address.RawQuery = params.Encode()
	db, err := sql.Open("pgx", address.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, file := range []string{"001_init.sql", "002_auth.sql", "003_consultation_limits.sql", "004_consultation_usage.sql", "005_admin_notifications.sql", "006_free_user_limit.sql", "007_llm_usage.sql", "008_monthly_costs.sql", "009_chat.sql", "010_purchase_schedule.sql", "011_chat_requests.sql"} {
		data, e := migrationFiles.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.ExecContext(ctx, string(data)); e != nil {
			t.Fatal(e)
		}
	}
	if _, err = db.Exec("INSERT INTO users(id,username,password_hash) VALUES(303,'csv-owner','synthetic'),(404,'csv-other','synthetic')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO transactions(id,user_id,transaction_date,description,normalized_merchant,amount,transaction_type,category,source,source_account_name,fingerprint,imported_at,raw_data_json) VALUES(41,303,'2032-01-01','店','店',2500,'income','学習','bank','account','old-key','old-time','{}')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("CREATE TABLE csv_identity_aliases(blocker TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err == nil {
		t.Fatal("migration fault not injected")
	}
	var count int
	db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=12").Scan(&count)
	if count != 0 {
		t.Fatal("failed migration marked applied")
	}
	db.QueryRow("SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='transactions' AND column_name='identity_version'").Scan(&count)
	if count != 0 {
		t.Fatal("failed migration did not roll back ALTERs")
	}
	db.QueryRow("SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='csv_identity_file_rows'").Scan(&count)
	if count != 0 {
		t.Fatal("failed migration retained file bindings")
	}
	var kind, category, fp string
	if err = db.QueryRow("SELECT transaction_type,category,fingerprint FROM transactions WHERE id=41 AND user_id=303").Scan(&kind, &category, &fp); err != nil || kind != "income" || category != "学習" || fp != "old-key" {
		t.Fatal("legacy record damaged")
	}
	db.Exec("DROP TABLE csv_identity_aliases")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- Migrate(ctx, db) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	r := &Repository{db: &database{DB: db}}
	owner := identity.WithUser(ctx, 303)
	row, err := r.GetTransaction(owner, 41)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := csvimport.Parse([]byte("日付,摘要,金額\n2032-01-01,店,-2500\n"), csvimport.Options{Source: "bank", SourceAccountName: "account"})
	if err != nil {
		t.Fatal(err)
	}
	item := parsed.Transactions[0]
	item.IdentityAction = "same"
	item.IdentityTransactionID = 41
	item.IdentityCandidateSnapshot = domain.ImportIdentitySnapshot(row)
	result, err := r.ImportTransactions(owner, []domain.Transaction{item})
	if err != nil || result.ImportedCount != 0 || result.DuplicateCount != 1 {
		t.Fatalf("legacy confirmation %+v %v", result, err)
	}
	if _, err = db.Exec("INSERT INTO csv_identity_file_rows(user_id,file_key,source_scope,account_scope,transaction_type,row_number,transaction_id) VALUES(404,'cross','bank','account','income',2,41)"); err == nil {
		t.Fatal("cross-owner file binding FK accepted")
	}
	db.Close()
	reopened, err := OpenDatabase(ctx, address.String())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	r = &Repository{db: reopened}
	result, err = r.ImportTransactions(owner, parsed.Transactions)
	if err != nil || result.ImportedCount != 0 || result.DuplicateCount != 1 {
		t.Fatalf("restart alias %+v %v", result, err)
	}
	row, err = r.GetTransaction(owner, 41)
	if err != nil || row.Category != "学習" || row.ImportedAt != "old-time" || row.TransactionType != "income" {
		t.Fatal("restart changed legacy metadata")
	}
}
