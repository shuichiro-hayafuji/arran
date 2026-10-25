package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestImportPreviewStoragePreservesInternalFields(t *testing.T) {
	item := domain.Transaction{ID: 71, Revision: 4, TransactionDate: "2035-01-01", Description: "PRIVATE", NormalizedMerchant: "PRIVATE", Amount: 2500, TransactionType: "expense", Category: "未分類", Source: "bank", SourceAccountName: "account", Fingerprint: "v2-key", ImportedAt: "2035-01-01T00:00:00Z", RawData: map[string]any{"date": "2035-01-01", "amount": "2500"}, IdentityVersion: 2, ProviderTransactionID: "bank-id", ImportFileKey: "file-key", ImportRow: 3, Occurrence: 2, LegacyFingerprint: "legacy", IdentityAction: "same", IdentityCandidateSnapshot: "snapshot", IdentityTransactionID: 81, ImportMetadata: &domain.ImportMetadata{CommitKey: "preview", ConfirmReactivation: true, FileName: "private.csv", Encoding: "CP932", Mapping: domain.CSVMapping{DateColumn: "日付", DescriptionColumn: "摘要", AmountColumn: "金額", TransactionIDColumn: "ID"}}, Reconciliation: &domain.ReconciliationDecision{Row: 3, Action: "same", PurchaseID: "purchase", Snapshot: "purchase-snapshot", StateVersion: 42}}
	encoded, err := encodeImportPreview([]domain.Transaction{item})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeImportPreview(encoded, "preview")
	if err != nil || !reflect.DeepEqual(decoded, []domain.Transaction{item}) {
		t.Fatalf("internal metadata lost: %#v %v", decoded, err)
	}
	for _, invalid := range []string{`{"version":99,"rows":[]}`, `{"rows":[]}`, `{"version":1}`, `{`} {
		if _, err := decodeImportPreview([]byte(invalid), "preview"); err == nil {
			t.Fatal("accepted invalid storage version/payload")
		}
	}
	if _, err := decodeImportPreview(encoded, "other-preview"); err == nil {
		t.Fatal("accepted wrong commit key")
	}
	// Persistence metadata remains private when returning the normal HTTP DTO.
	wire, _ := json.Marshal(decoded[0])
	for _, secret := range []string{"bank-id", "file-key", "legacy", "snapshot", "private.csv", "purchase"} {
		if strings.Contains(string(wire), secret) {
			t.Fatalf("internal field leaked: %s", secret)
		}
	}
}
func previewItems(t *testing.T, id string) []domain.Transaction {
	t.Helper()
	data := "date,description,amount\n"
	for i := 1; i <= 12; i++ {
		data += fmt.Sprintf("2035-01-%02d,PRIVATE STORE %d,%d\n", i, i, 100+i)
	}
	parsed, err := csvimport.Parse([]byte(data), csvimport.Options{Source: "bank", SourceAccountName: "account"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range parsed.Transactions {
		parsed.Transactions[i].ImportMetadata = &domain.ImportMetadata{CommitKey: id, FileName: "twelve.csv", Encoding: parsed.Encoding, Mapping: parsed.Mapping}
	}
	return parsed.Transactions
}
func verifyDurablePreviewRetryAndExpiry(t *testing.T, r *Repository) {
	owner := identity.WithUser(context.Background(), 701)
	other := identity.WithUser(context.Background(), 702)
	now := time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC)
	id := "durable-preview"
	items := previewItems(t, id)
	if err := r.SaveImportPreview(owner, id, items, now, now.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	// A second Repository has no shared application memory.
	var schema string
	if err := r.db.QueryRow("SELECT current_schema()").Scan(&schema); err != nil {
		t.Fatal(err)
	}
	address, err := url.Parse(os.Getenv("ARRAN_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	params := address.Query()
	params.Set("search_path", schema)
	address.RawQuery = params.Encode()
	alternate, err := Open(owner, address.String())
	if err != nil {
		t.Fatal(err)
	}
	defer alternate.Close()
	if _, err := alternate.CommitImportPreview(other, id, now); err == nil {
		t.Fatal("other owner committed")
	}
	rows, err := r.ListTransactions(owner, "", 100)
	if err != nil || len(rows) != 0 {
		t.Fatal("preview changed ledger", len(rows), err)
	}
	var wg sync.WaitGroup
	results := make(chan domain.ImportCommitResult, 2)
	errs := make(chan error, 2)
	for _, repository := range []*Repository{r, alternate} {
		wg.Add(1)
		go func(repo *Repository) {
			defer wg.Done()
			result, err := repo.CommitImportPreview(owner, id, now)
			results <- result
			errs <- err
		}(repository)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first domain.ImportCommitResult
	for result := range results {
		if first.ImportID == "" {
			first = result
		}
		if result != first {
			t.Fatal("parallel receipts differ", first, result)
		}
	}
	if first.ImportedCount != 12 || first.DuplicateCount != 0 || first.ImportID == "" {
		t.Fatal(first)
	}
	saved, err := alternate.CommitImportPreview(owner, id, now.Add(time.Hour))
	if err != nil || saved != first {
		t.Fatal("expired response-loss retry", saved, err)
	}
	if _, err := alternate.CommitImportPreview(other, id, now.Add(time.Hour)); err == nil {
		t.Fatal("other owner read receipt")
	}
	var n int
	if err = r.db.QueryRowContext(owner, userQuery(owner, "SELECT count(*) FROM csv_import_previews WHERE user_id=:user_id AND id=?"), id).Scan(&n); err != nil || n != 0 {
		t.Fatal("completed payload retained", n, err)
	}
	rows, err = r.ListTransactions(owner, "", 100)
	if err != nil || len(rows) != 12 {
		t.Fatal("duplicate import", len(rows), err)
	}
	details, err := r.ImportDetails(owner, first.ImportID)
	if err != nil || len(details.Rows) != 12 || details.Metadata.FileName != "twelve.csv" {
		t.Fatal("full preview lost", details, err)
	}
	expiryID := "expired-preview"
	if err = r.SaveImportPreview(owner, expiryID, previewItems(t, expiryID), now, now.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommitImportPreview(owner, expiryID, now.Add(30*time.Minute)); err == nil {
		t.Fatal("accepted expiry boundary")
	}
	if err = r.SaveImportPreview(owner, "cleanup", nil, now.Add(time.Hour), now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = r.db.QueryRowContext(owner, userQuery(owner, "SELECT count(*) FROM csv_import_previews WHERE user_id=:user_id AND id=?"), expiryID).Scan(&n); err != nil || n != 0 {
		t.Fatal("expired payload not cleaned", n, err)
	}
	empty, err := r.CommitImportPreview(owner, "cleanup", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	queried, err := r.ImportCommit(owner, "cleanup")
	if err != nil || queried != empty {
		t.Fatal("empty result lookup", queried, err)
	}
	retry, err := r.CommitImportPreview(owner, "cleanup", now.Add(3*time.Hour))
	if err != nil || retry != empty {
		t.Fatal("empty receipt retry", retry, err)
	}
}
func verifyDurablePreviewLateFailure(t *testing.T, r *Repository) {
	owner := identity.WithUser(context.Background(), 711)
	now := time.Now()
	id := "rollback-preview"
	if err := r.SaveImportPreview(owner, id, previewItems(t, id), now, now.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	seed, err := r.LoadChatState(owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec("CREATE FUNCTION reject_preview_delete() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected late failure'; END $$; CREATE TRIGGER reject_preview_delete BEFORE DELETE ON csv_import_previews FOR EACH ROW EXECUTE FUNCTION reject_preview_delete()"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommitImportPreview(owner, id, now); err == nil {
		t.Fatal("late failure not injected")
	}
	for _, table := range []string{"transactions", "csv_import_batches", "csv_import_evidence", "csv_identity_file_rows", "csv_identity_aliases"} {
		var n int
		if err = r.db.QueryRowContext(owner, userQuery(owner, "SELECT count(*) FROM "+table+" WHERE user_id=:user_id")).Scan(&n); err != nil || n != 0 {
			t.Fatal("partial state", table, n, err)
		}
	}
	after, err := r.LoadChatState(owner)
	if err != nil || after.Version != seed.Version {
		t.Fatal("chat state partially saved", after.Version, err)
	}
	if _, err = r.db.Exec("DROP TRIGGER reject_preview_delete ON csv_import_previews; DROP FUNCTION reject_preview_delete()"); err != nil {
		t.Fatal(err)
	}
	result, err := r.CommitImportPreview(owner, id, now)
	if err != nil || result.ImportedCount != 12 {
		t.Fatal("retry after rollback", result, err)
	}
}
func verifyPreviewMigration(t *testing.T, r *Repository) {
	ctx := identity.WithUser(context.Background(), 712)
	original, err := r.ImportTransactions(ctx, previewItems(t, "before-migration"))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"DROP TABLE csv_import_previews", "DELETE FROM schema_migrations WHERE version=15", "CREATE INDEX idx_csv_preview_expiry ON transactions(user_id)"} {
		if _, err = r.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.Migrate(ctx); err == nil {
		t.Fatal("late DDL failure not injected")
	}
	var n int
	if err = r.db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=15").Scan(&n); err != nil || n != 0 {
		t.Fatal("partial migration", n, err)
	}
	if _, err = r.db.Exec("DROP INDEX idx_csv_preview_expiry"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- r.Migrate(ctx) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = r.Migrate(ctx); err != nil {
		t.Fatal("restart migration", err)
	}
	receipt, err := r.ImportCommit(ctx, "before-migration")
	if err != nil || receipt != original {
		t.Fatal("old receipt lost", receipt, err)
	}
	rows, err := r.ListTransactions(ctx, "", 100)
	if err != nil || len(rows) != 12 {
		t.Fatal("old ledger lost", len(rows), err)
	}
}

func verifyPreviewFinishRequiresExistingRow(t *testing.T, r *Repository) {
	owner := identity.WithUser(context.Background(), 701)
	tx, err := r.db.DB.BeginTx(owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for _, items := range [][]domain.Transaction{nil, {{Amount: 1}}} {
		if err = finishImportPreview(owner, tx, "missing-preview", items, domain.ImportCommitResult{}); err == nil {
			t.Fatal("missing preview reported successful")
		}
	}
}

func TestDurablePreviewCleanupRace(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("dedicated PostgreSQL not configured")
	}
	verifyDurablePreviewCleanupRace(t, openReconciliationRepository(t))
}
func verifyDurablePreviewCleanupRace(t *testing.T, r *Repository) {
	owner := identity.WithUser(context.Background(), 701)
	ctx, cancel := context.WithTimeout(owner, 5*time.Second)
	defer cancel()
	now := time.Now()
	if err := r.SaveImportPreview(ctx, "cleanup-race", nil, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	items, saved, err := loadImportPreview(ctx, tx, "cleanup-race", now)
	if err != nil || saved != nil || len(items) != 0 {
		t.Fatal(items, saved, err)
	}
	// Simulate the owner's next preview arriving after expiry while this already
	// accepted empty import is still inside its transaction.
	done := make(chan error, 1)
	go func() {
		done <- r.SaveImportPreview(ctx, "next-preview", nil, now.Add(2*time.Minute), now.Add(32*time.Minute))
	}()
	select {
	case err := <-done:
		t.Fatalf("expiry cleanup passed the in-flight preview row lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	result := domain.ImportCommitResult{}
	if err = finishImportPreview(ctx, tx, "cleanup-race", items, result); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	receipt, err := r.ImportCommit(ctx, "cleanup-race")
	if err != nil || receipt != result {
		t.Fatal("empty receipt deleted by cleanup", receipt, err)
	}
	receipt, err = r.CommitImportPreview(ctx, "cleanup-race", now.Add(time.Hour))
	if err != nil || receipt != result {
		t.Fatal("lost-response replay after cleanup", receipt, err)
	}
}
