package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func TestTrackedConsentUsage(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	owner := identity.WithUser(ctx, 701)
	if _, err := r.db.DB.ExecContext(ctx, `UPDATE ai_consent_policy SET version='v1',description='policy' WHERE singleton=TRUE`); err != nil {
		t.Fatal(err)
	}
	if err := r.GrantConsent(owner, 701, "v1"); err != nil {
		t.Fatal(err)
	}
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	tracked := identity.RequireUsageTracking(owner)
	var op string
	if err := r.WithConsent(tracked, 701, "v1", func(call context.Context) error {
		op = identity.UsageOperation(call)
		if op == "" || identity.UserID(call) != 701 {
			t.Fatal("accounting identity absent")
		}
		if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
			t.Fatalf("inflight accepted: %v", err)
		}
		usage := domain.LLMUsage{UserID: 701, Operation: "test", Model: "model", InputTokens: 2, OutputTokens: 3, TotalTokens: 5, OccurredAt: time.Now().UTC()}
		return r.RecordGuardedLLMUsage(call, usage)
	}); err != nil {
		t.Fatal(err)
	}
	var receipts, pending int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE usage_id IS NOT NULL),count(*) FILTER (WHERE usage_id IS NULL AND no_usage_reason IS NULL) FROM ai_usage_operations WHERE user_id=701 AND operation_id=$1`, op).Scan(&receipts, &pending); err != nil || receipts != 1 || pending != 0 {
		t.Fatalf("accounting: receipt=%d pending=%d err=%v", receipts, pending, err)
	}
	// Another terminal response with unavailable accounting leaves the owner
	// blocked after the consent marker has been cleared.
	if err := r.WithConsent(tracked, 701, "v1", func(call context.Context) error {
		if err := r.RecordGuardedLLMUsage(call, domain.LLMUsage{UserID: 702, TotalTokens: 1}); err == nil {
			t.Error("other owner accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("unrecorded accounting accepted: %v", err)
	}
	var inflight int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM ai_consent_inflight WHERE user_id=701`).Scan(&inflight); err != nil || inflight != 0 {
		t.Fatalf("consent inflight=%d err=%v", inflight, err)
	}
}

func TestTrackedConsentAdmissionSuppressedInsert(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	for _, table := range []string{"ai_consent_inflight", "ai_usage_operations"} {
		t.Run(table, func(t *testing.T) {
			r := openReconciliationRepository(t)
			ctx := context.Background()
			owner := identity.RequireUsageTracking(identity.WithUser(ctx, 701))
			if _, err := r.db.DB.ExecContext(ctx, `UPDATE ai_consent_policy SET version='v1',description='policy' WHERE singleton=TRUE`); err != nil {
				t.Fatal(err)
			}
			if err := r.GrantConsent(owner, 701, "v1"); err != nil {
				t.Fatal(err)
			}
			if _, err := r.db.DB.ExecContext(ctx, `CREATE FUNCTION suppress_admission() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$; CREATE TRIGGER suppress_admission BEFORE INSERT ON `+table+` FOR EACH ROW EXECUTE FUNCTION suppress_admission()`); err != nil {
				t.Fatal(err)
			}
			called := false
			if err := r.WithConsent(owner, 701, "v1", func(context.Context) error { called = true; return nil }); err == nil || called {
				t.Fatalf("suppressed %s admission: called=%v err=%v", table, called, err)
			}
			var consentCount, usageCount int
			if err := r.db.DB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM ai_consent_inflight WHERE user_id=701),(SELECT count(*) FROM ai_usage_operations WHERE user_id=701)`).Scan(&consentCount, &usageCount); err != nil || consentCount != 0 || usageCount != 0 {
				t.Fatalf("partial admission: consent=%d usage=%d err=%v", consentCount, usageCount, err)
			}
		})
	}
}
