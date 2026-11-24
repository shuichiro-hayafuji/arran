package postgres

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func TestWithdrawalCSVGate(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	t.Run("receipts remain readable for pending owner", func(t *testing.T) {
		r := openReconciliationRepository(t)
		ctx := context.Background()
		a := identity.WithUser(ctx, 701)
		b := identity.WithUser(ctx, 702)
		now := time.Now()
		for _, id := range []string{"saved-preview", "empty-preview", "unsaved-preview"} {
			var items []domain.Transaction
			if id != "empty-preview" {
				items = previewItems(t, id)
			}
			if err := r.SaveImportPreview(a, id, items, now, now.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
		}
		saved, err := r.CommitImportPreview(a, "saved-preview", now)
		if err != nil {
			t.Fatal(err)
		}
		empty, err := r.CommitImportPreview(a, "empty-preview", now)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
			t.Fatal(err)
		}
		if err := r.RequestWithdrawal(a, 701); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			id   string
			want domain.ImportCommitResult
		}{{"saved-preview", saved}, {"empty-preview", empty}} {
			got, err := r.CommitImportPreview(a, tc.id, now)
			if err != nil || got != tc.want {
				t.Fatalf("pending receipt %s: %#v %v", tc.id, got, err)
			}
			if _, err := r.CommitImportPreview(b, tc.id, now); err == nil {
				t.Fatalf("other owner read %s", tc.id)
			}
		}
		if _, err := r.CommitImportPreview(a, "unsaved-preview", now); err == nil {
			t.Fatal("pending user committed an uncommitted preview")
		}
		if err := r.SaveImportPreview(a, "another-preview", nil, now, now.Add(time.Hour)); err == nil {
			t.Fatal("pending user saved a preview")
		}
		if _, err := r.MutateCSV(a, "1", domain.MutationInput{ID: "blocked-mutation", Action: "correct"}); err == nil {
			t.Fatal("pending user mutated CSV")
		}
		if err := r.ResolvePurchaseCSV(a, 1, domain.ReconciliationDecision{}); err == nil {
			t.Fatal("pending user changed reconciliation")
		}
		if _, err := r.db.DB.ExecContext(ctx, `UPDATE users SET is_active=FALSE WHERE id=701`); err != nil {
			t.Fatal(err)
		}
		if _, err := r.CommitImportPreview(a, "saved-preview", now); err == nil {
			t.Fatal("inactive user accessed a receipt")
		}
	})
	t.Run("read committed even when server default is repeatable read", func(t *testing.T) {
		r := openReconciliationRepository(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		owner := identity.WithUser(ctx, 701)
		var schema string
		if err := r.db.DB.QueryRowContext(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(os.Getenv("ARRAN_TEST_DATABASE_URL"))
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		peer, err := sql.Open("pgx", u.String())
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()
		r.db.DB.SetMaxOpenConns(1)
		if _, err := r.db.DB.ExecContext(ctx, `SET default_transaction_isolation='repeatable read'`); err != nil {
			t.Fatal(err)
		}
		blocker, err := peer.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer blocker.Rollback()
		var pid int
		if err := blocker.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
			t.Fatal(err)
		}
		if _, err := blocker.ExecContext(ctx, `SELECT id FROM users WHERE id=701 FOR NO KEY UPDATE`); err != nil {
			t.Fatal(err)
		}
		if _, err := blocker.ExecContext(ctx, `INSERT INTO withdrawal_requests(user_id) VALUES(701)`); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			_, e := r.ImportTransactions(owner, []domain.Transaction{{TransactionDate: "2026-09-01", Amount: 10, TransactionType: "expense", Category: "test", Fingerprint: "rr-pending"}})
			done <- e
		}()
		deadline := time.Now().Add(4 * time.Second)
		for {
			var waiting bool
			if err := peer.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND query LIKE 'SELECT is_active FROM users%FOR NO KEY UPDATE%')`, pid).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("import did not wait for admission lock")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := blocker.Commit(); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("import admitted using stale snapshot")
			}
		case <-ctx.Done():
			t.Fatal("import did not finish")
		}
		var count int
		if err := peer.QueryRowContext(ctx, `SELECT count(*) FROM transactions WHERE user_id=701 AND fingerprint='rr-pending'`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("stale snapshot imported %d: %v", count, err)
		}
	})
}
