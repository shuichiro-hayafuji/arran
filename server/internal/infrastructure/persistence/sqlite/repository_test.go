package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)

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
