package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
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
	alternate := &Repository{db: r.db}
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
	if err = r.db.QueryRowContext(owner, chatQuery(owner, "SELECT count(*) FROM csv_import_previews WHERE user_id=:user_id AND id=?"), id).Scan(&n); err != nil || n != 0 {
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
	if err = r.db.QueryRowContext(owner, chatQuery(owner, "SELECT count(*) FROM csv_import_previews WHERE user_id=:user_id AND id=?"), expiryID).Scan(&n); err != nil || n != 0 {
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
	if _, err = r.db.Exec("CREATE TRIGGER reject_preview_delete BEFORE DELETE ON csv_import_previews BEGIN SELECT RAISE(ABORT, 'injected late failure'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommitImportPreview(owner, id, now); err == nil {
		t.Fatal("late failure not injected")
	}
	for _, table := range []string{"transactions", "csv_import_batches", "csv_import_evidence", "csv_identity_file_rows", "csv_identity_aliases"} {
		var n int
		if err = r.db.QueryRowContext(owner, chatQuery(owner, "SELECT count(*) FROM "+table+" WHERE user_id=:user_id")).Scan(&n); err != nil || n != 0 {
			t.Fatal("partial state", table, n, err)
		}
	}
	after, err := r.LoadChatState(owner)
	if err != nil || after.Version != seed.Version {
		t.Fatal("chat state partially saved", after.Version, err)
	}
	if _, err = r.db.Exec("DROP TRIGGER reject_preview_delete"); err != nil {
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
	for _, q := range []string{"DROP TABLE csv_import_previews", "DELETE FROM schema_migrations WHERE version=9", "CREATE INDEX idx_csv_preview_expiry ON transactions(user_id)"} {
		if _, err = r.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.Migrate(ctx); err == nil {
		t.Fatal("late DDL failure not injected")
	}
	var n int
	if err = r.db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=9").Scan(&n); err != nil || n != 0 {
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

func TestDurablePreviewRetryAndExpiry(t *testing.T) {
	verifyDurablePreviewRetryAndExpiry(t, openTestRepository(t))
}
func TestDurablePreviewLateFailure(t *testing.T) {
	verifyDurablePreviewLateFailure(t, openTestRepository(t))
}
func TestPreviewMigration(t *testing.T) { verifyPreviewMigration(t, openTestRepository(t)) }

func verifyPreviewFinishRequiresExistingRow(t *testing.T, r *Repository) {
	owner := identity.WithUser(context.Background(), 701)
	tx, err := r.db.BeginTx(owner, nil)
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
func TestPreviewFinishRequiresExistingRow(t *testing.T) {
	verifyPreviewFinishRequiresExistingRow(t, openTestRepository(t))
}
