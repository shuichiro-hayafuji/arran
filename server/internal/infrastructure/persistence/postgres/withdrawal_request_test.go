package postgres

import (
	"context"
	"os"
	"sync"
	"testing"
)

func TestWithdrawalRequestDurabilityAndOwnerIsolation(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("ARRAN_TEST_DATABASE_URL is not set")
	}
	verifyWithdrawalRequestDurabilityAndOwnerIsolation(t)
}

func verifyWithdrawalRequestDurabilityAndOwnerIsolation(t *testing.T) {
	r := openReconciliationRepository(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- r.RequestWithdrawal(ctx, 701) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		id   int64
		want bool
	}{{701, true}, {702, false}} {
		got, err := r.WithdrawalRequested(ctx, tc.id)
		if err != nil || got != tc.want {
			t.Fatalf("user %d: %v %v", tc.id, got, err)
		}
	}
	var count int
	if err := r.db.DB.QueryRowContext(ctx, "SELECT count(*) FROM withdrawal_requests WHERE user_id=701").Scan(&count); err != nil || count != 1 {
		t.Fatalf("count %d %v", count, err)
	}
	if err := r.RequestWithdrawal(ctx, 0); err == nil {
		t.Fatal("missing owner accepted")
	}
	// A privileged operator can record the first review without treating it as
	// deletion. Retrying the review must preserve its original timestamp and
	// never mark another owner's request as reviewed.
	if err := r.RequestWithdrawal(ctx, 702); err != nil {
		t.Fatal(err)
	}
	var first, retried string
	q := `UPDATE withdrawal_requests SET reviewed_at=COALESCE(reviewed_at,CURRENT_TIMESTAMP) WHERE user_id=$1 RETURNING reviewed_at::text`
	if err := r.db.DB.QueryRowContext(ctx, q, 701).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := r.db.DB.QueryRowContext(ctx, q, 701).Scan(&retried); err != nil || first != retried {
		t.Fatalf("review retry changed timestamp: %q %q %v", first, retried, err)
	}
	var otherReviewed bool
	if err := r.db.DB.QueryRowContext(ctx, `SELECT reviewed_at IS NOT NULL FROM withdrawal_requests WHERE user_id=702`).Scan(&otherReviewed); err != nil || otherReviewed {
		t.Fatalf("other owner marked reviewed: %v %v", otherReviewed, err)
	}
}
