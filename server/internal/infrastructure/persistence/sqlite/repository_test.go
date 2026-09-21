package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func TestMonthlyConsultationLimitUsesDefaultAndUserOverride(t *testing.T) {
	repo := openTestRepository(t)
	ctx := context.Background()

	limit, err := repo.MonthlyConsultationLimit(ctx)
	if err != nil || limit != 30 {
		t.Fatalf("default limit = %d, err = %v", limit, err)
	}

	if _, err = repo.db.ExecContext(ctx, `
		INSERT INTO user_consultation_limits(user_id, monthly_limit, updated_at)
		VALUES(42, 12, '2026-09-21T00:00:00Z')
	`); err != nil {
		t.Fatal(err)
	}
	limit, err = repo.MonthlyConsultationLimit(identity.WithUser(ctx, 42))
	if err != nil || limit != 12 {
		t.Fatalf("user limit = %d, err = %v", limit, err)
	}

	if _, err = repo.db.ExecContext(ctx, `
		UPDATE service_settings SET integer_value = 20
		WHERE key = 'default_monthly_consultation_limit'
	`); err != nil {
		t.Fatal(err)
	}
	limit, err = repo.MonthlyConsultationLimit(identity.WithUser(ctx, 7))
	if err != nil || limit != 20 {
		t.Fatalf("changed default limit = %d, err = %v", limit, err)
	}
}

func TestMonthlyConsultationUsageReservesAndReleasesByMonth(t *testing.T) {
	repo := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 42)

	first, err := repo.ReserveMonthlyConsultation(ctx, "2026-07")
	if err != nil || first.Count != 1 || first.Limit != 30 || !first.UseExternalModel() {
		t.Fatalf("first usage = %#v, err = %v", first, err)
	}
	second, err := repo.ReserveMonthlyConsultation(ctx, "2026-07")
	if err != nil || second.Count != 2 {
		t.Fatalf("second usage = %#v, err = %v", second, err)
	}
	august, err := repo.MonthlyConsultationUsage(ctx, "2026-08")
	if err != nil || august.Count != 0 || august.Limit != 30 {
		t.Fatalf("august usage = %#v, err = %v", august, err)
	}
	if err = repo.ReleaseMonthlyConsultation(ctx, "2026-07"); err != nil {
		t.Fatal(err)
	}
	afterRelease, err := repo.MonthlyConsultationUsage(ctx, "2026-07")
	if err != nil || afterRelease.Count != 1 {
		t.Fatalf("released usage = %#v, err = %v", afterRelease, err)
	}
}

func TestMonthlyLimitNotificationIsDeduplicatedAndRetriesFailure(t *testing.T) {
	repo := openTestRepository(t)
	ctx := identity.WithUser(context.Background(), 42)
	event := domain.MonthlyLimitEvent{
		Environment: "test", UserID: 42, Month: "2026-07", Count: 30, Limit: 30,
		OccurredAt: time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC),
	}
	claimed, err := repo.ClaimMonthlyLimitNotification(ctx, event)
	if err != nil || !claimed {
		t.Fatalf("first claim = %v, err = %v", claimed, err)
	}
	if err = repo.CompleteMonthlyLimitNotification(ctx, event, errors.New("delivery failed")); err != nil {
		t.Fatal(err)
	}
	claimed, err = repo.ClaimMonthlyLimitNotification(ctx, event)
	if err != nil || !claimed {
		t.Fatalf("retry claim = %v, err = %v", claimed, err)
	}
	if err = repo.CompleteMonthlyLimitNotification(ctx, event, nil); err != nil {
		t.Fatal(err)
	}
	claimed, err = repo.ClaimMonthlyLimitNotification(ctx, event)
	if err != nil || claimed {
		t.Fatalf("delivered claim = %v, err = %v", claimed, err)
	}
	var status string
	var attempts int
	if err = repo.db.QueryRowContext(ctx, `
		SELECT status, attempts FROM admin_notifications
		WHERE user_id = 42 AND month = '2026-07'
	`).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "delivered" || attempts != 2 {
		t.Fatalf("notification status=%q attempts=%d", status, attempts)
	}
}

func TestImportDeduplicationAndMonthlyAggregates(t *testing.T) {
	ctx := context.Background()
	repo := openTestRepository(t)
	profile := testProfile()
	if err := repo.PutProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "sample_data", "utf8_general.csv"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := csvimport.Parse(data, csvimport.Options{
		Source:            "card",
		SourceAccountName: "main",
		Now:               time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := repo.ImportTransactions(ctx, parsed.Transactions)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.ImportTransactions(ctx, parsed.Transactions)
	if err != nil {
		t.Fatal(err)
	}
	if first.ImportedCount != 6 || first.DuplicateCount != 0 {
		t.Fatalf("first import = %#v", first)
	}
	if second.ImportedCount != 0 || second.DuplicateCount != 6 {
		t.Fatalf("second import = %#v", second)
	}

	dashboard, err := repo.MonthlyDashboard(ctx, "2026-07", profile)
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.ExpenseTotal != 18090 {
		t.Fatalf("expense_total = %d, want 18090", dashboard.ExpenseTotal)
	}
	if dashboard.DrinkingTotal != 8000 {
		t.Fatalf("drinking_total = %d, want 8000", dashboard.DrinkingTotal)
	}
	if dashboard.SubscriptionTotal != 1490 {
		t.Fatalf("subscription_total = %d, want 1490", dashboard.SubscriptionTotal)
	}
	if dashboard.FreeBudgetRemaining != 41910 {
		t.Fatalf("free_budget_remaining = %d, want 41910", dashboard.FreeBudgetRemaining)
	}
}

func openTestRepository(t *testing.T) *Repository {
	t.Helper()
	repo, err := Open(
		context.Background(),
		filepath.Join(t.TempDir(), "test.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func testProfile() domain.Profile {
	return domain.Profile{
		MonthlyIncome:      400000,
		CurrentBalance:     1200000,
		MonthlyFixedCosts:  180000,
		MonthlyFreeBudget:  60000,
		MonthlySavingsGoal: 80000,
		ReduceCategories:   []string{"酒・飲み会"},
		AllowedCategories:  []string{"学習", "健康"},
		LongTermGoal:       "1年で100万円貯める",
		AdviceStrictness:   "バランス型",
		UpdatedAt:          time.Now().UTC().Format(time.RFC3339),
	}
}
