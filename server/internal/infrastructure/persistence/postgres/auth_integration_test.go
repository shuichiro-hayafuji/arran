package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/auth"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

// Each run uses a fresh schema, never the configured application's public tables.
func integrationRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := os.Getenv("ARRAN_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ARRAN_TEST_DATABASE_URL is not set; PostgreSQL integration test not executed")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	schema := fmt.Sprintf("auth_test_%d", time.Now().UnixNano())
	if _, err = db.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE") })
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	scoped, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = scoped.Close() })
	legacy, _ := migrationFiles.ReadFile("001_init.sql")
	if _, err = scoped.ExecContext(ctx, string(legacy)); err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.ExecContext(ctx, `INSERT INTO profiles VALUES(1,100,100,1,1,1,'[]','[]','','', '2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, scoped); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, scoped); err != nil {
		t.Fatalf("migration not repeatable: %v", err)
	}
	var count int
	if err = scoped.QueryRowContext(ctx, "SELECT count(*) FROM profiles WHERE user_id IS NULL").Scan(&count); err != nil || count != 1 {
		t.Fatalf("legacy record not retained: %v %d", err, count)
	}
	return &Repository{db: &database{DB: scoped}}
}

func TestPostgresOwnershipAndSessions(t *testing.T) {
	r := integrationRepository(t)
	ctx := context.Background()
	hash, err := auth.HashPassword("integration password")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alice", "bob"} {
		if _, err = r.db.DB.ExecContext(ctx, `INSERT INTO users(username,password_hash) VALUES($1,$2)`, name, hash); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = r.db.DB.ExecContext(ctx, `
		UPDATE service_settings SET integer_value=2 WHERE key='free_user_limit'
	`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.DB.ExecContext(ctx, `
		INSERT INTO users(username,password_hash) VALUES($1,$2)
	`, "carol", hash); err == nil {
		t.Fatal("free user limit accepted a third active user")
	}
	a, _ := r.FindUser(ctx, "alice")
	b, _ := r.FindUser(ctx, "bob")
	ca := identity.WithUser(ctx, a.ID)
	cb := identity.WithUser(ctx, b.ID)
	if limit, limitErr := r.MonthlyConsultationLimit(ca); limitErr != nil || limit != 30 {
		t.Fatalf("default consultation limit = %d, err = %v", limit, limitErr)
	}
	if _, err = r.db.DB.ExecContext(ctx, `
		INSERT INTO user_consultation_limits(user_id, monthly_limit)
		VALUES($1, 12)
	`, a.ID); err != nil {
		t.Fatal(err)
	}
	if limit, limitErr := r.MonthlyConsultationLimit(ca); limitErr != nil || limit != 12 {
		t.Fatalf("user consultation limit = %d, err = %v", limit, limitErr)
	}
	if limit, limitErr := r.MonthlyConsultationLimit(cb); limitErr != nil || limit != 30 {
		t.Fatalf("other user consultation limit = %d, err = %v", limit, limitErr)
	}
	usage, err := r.ReserveMonthlyConsultation(ca, "2026-09")
	if err != nil || usage.Count != 1 || usage.Limit != 12 {
		t.Fatalf("reserved consultation usage = %#v, err = %v", usage, err)
	}
	if err = r.ReleaseMonthlyConsultation(ca, "2026-09"); err != nil {
		t.Fatal(err)
	}
	usage, err = r.MonthlyConsultationUsage(ca, "2026-09")
	if err != nil || usage.Count != 0 {
		t.Fatalf("released consultation usage = %#v, err = %v", usage, err)
	}
	var wait sync.WaitGroup
	errorsFromReservations := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, reserveErr := r.ReserveMonthlyConsultation(ca, "2026-10")
			errorsFromReservations <- reserveErr
		}()
	}
	wait.Wait()
	close(errorsFromReservations)
	for reserveErr := range errorsFromReservations {
		if reserveErr != nil {
			t.Fatal(reserveErr)
		}
	}
	usage, err = r.MonthlyConsultationUsage(ca, "2026-10")
	if err != nil || usage.Count != 20 {
		t.Fatalf("concurrent consultation usage = %#v, err = %v", usage, err)
	}
	event := domain.MonthlyLimitEvent{
		Environment: "test", UserID: a.ID, Month: "2026-09", Count: 12, Limit: 12,
		OccurredAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
	}
	claimed, err := r.ClaimMonthlyLimitNotification(ca, event)
	if err != nil || !claimed {
		t.Fatalf("notification claim = %v, err = %v", claimed, err)
	}
	if err = r.CompleteMonthlyLimitNotification(ca, event, nil); err != nil {
		t.Fatal(err)
	}
	claimed, err = r.ClaimMonthlyLimitNotification(ca, event)
	if err != nil || claimed {
		t.Fatalf("duplicate notification claim = %v, err = %v", claimed, err)
	}
	if err = r.RecordLLMUsage(ca, domain.LLMUsage{
		Operation: "spending_advice", Model: "gpt-5.6-terra",
		InputTokens: 100, CachedInputTokens: 20, OutputTokens: 40,
		ReasoningTokens: 10, TotalTokens: 140, OccurredAt: event.OccurredAt,
	}); err != nil {
		t.Fatal(err)
	}
	var usageRows int
	if err = r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM llm_usage WHERE user_id=$1`, a.ID).Scan(&usageRows); err != nil || usageRows != 1 {
		t.Fatalf("llm usage rows = %d, err = %v", usageRows, err)
	}
	if _, err = r.db.DB.ExecContext(ctx, `
		INSERT INTO monthly_operating_costs(month, infrastructure_cost_usd, support_case_count, support_minutes)
		VALUES('2026-09', 10, 2, 30)
	`); err != nil {
		t.Fatal(err)
	}
	var apiCost float64
	if err = r.db.DB.QueryRowContext(ctx, `
		SELECT (
		  (usage.input_tokens - usage.cached_input_tokens) * price.input_usd_per_million
		  + usage.cached_input_tokens * price.cached_input_usd_per_million
		  + usage.output_tokens * price.output_usd_per_million
		) / 1000000
		FROM llm_usage AS usage
		JOIN LATERAL (
		  SELECT * FROM llm_model_prices
		  WHERE model=usage.model AND effective_from <= usage.occurred_at::date
		  ORDER BY effective_from DESC LIMIT 1
		) AS price ON true
		WHERE usage.user_id=$1
	`, a.ID).Scan(&apiCost); err != nil || math.Abs(apiCost-0.000644) > 0.0000001 {
		t.Fatalf("api cost = %.6f, err = %v", apiCost, err)
	}
	if _, err = r.GetProfile(ca); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("legacy data exposed: %v", err)
	}
	for _, c := range []context.Context{ca, cb} {
		if err = r.PutProfile(c, domain.Profile{MonthlyIncome: identity.UserID(c)}); err != nil {
			t.Fatal(err)
		}
	}
	if p, err := r.GetProfile(ca); err != nil || p.MonthlyIncome != a.ID {
		t.Fatal("profile crossed owners", err)
	}
	if _, err = r.GetProfile(ctx); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("missing identity exposed data")
	}
	if err = r.PutProfile(ctx, domain.Profile{}); err == nil {
		t.Fatal("missing identity wrote data")
	}
	tr := domain.Transaction{TransactionDate: "2026-09-01", Amount: 10, TransactionType: "expense", Category: "学習", Fingerprint: "same-fingerprint"}
	for _, c := range []context.Context{ca, cb} {
		result, err := r.ImportTransactions(c, []domain.Transaction{tr})
		if err != nil || result.ImportedCount != 1 {
			t.Fatalf("cross-user dedup: %+v %v", result, err)
		}
	}
	duplicate, err := r.ImportTransactions(ca, []domain.Transaction{tr})
	if err != nil || duplicate.DuplicateCount != 1 {
		t.Fatal("same-user dedup failed", err)
	}
	rows, err := r.ListTransactions(ca, "2026-09", 100)
	if err != nil || len(rows) != 1 {
		t.Fatal("transaction list", err)
	}
	id := rows[0].ID
	if _, err = r.GetTransaction(cb, id); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-user transaction read", err)
	}
	if _, err = r.UpdateTransactionCategory(cb, id, "食費", true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-user transaction write", err)
	}
	if _, err = r.UpdateTransactionCategory(ca, id, "学習", true); err != nil {
		t.Fatal(err)
	}
	rules, err := r.MerchantRules(cb)
	if err != nil || len(rules) != 0 {
		t.Fatal("merchant rules leaked", err)
	}
	dash, err := r.MonthlyDashboard(ca, "2026-09", domain.Profile{})
	if err != nil || dash.ExpenseTotal != 10 {
		t.Fatal("dashboard owners mixed", err)
	}
	consultation, err := r.CreateConsultation(ca, domain.Consultation{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.GetConsultation(cb, consultation.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("consultation leaked", err)
	}
	if _, err = r.UpdateConsultationAdvice(cb, consultation); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("consultation updated by other user", err)
	}
	if _, err = r.UpdateConsultationResult(cb, consultation.ID, domain.ConsultationResultUpdate{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("result updated by other user", err)
	}
	if err = r.AddMessage(cb, consultation.ID, "user", "wrong owner"); err == nil {
		t.Fatal("cross-user message accepted")
	}
	if err = r.AddMessage(ca, consultation.ID, "user", "owner"); err != nil {
		t.Fatal(err)
	}
	list, err := r.ListConsultations(cb, 10)
	if err != nil || len(list) != 0 {
		t.Fatal("consultations leaked", err)
	}
	memory, err := r.SaveMemory(ca, domain.MemoryItem{Content: "private"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.SaveMemory(cb, memory); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("memory updated by other user", err)
	}
	if err = r.DeleteMemory(cb, memory.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("memory deleted by other user", err)
	}
	memories, err := r.ListMemories(cb)
	if err != nil || len(memories) != 0 {
		t.Fatal("memories leaked", err)
	}
	if _, err = r.SaveReview(ca, domain.MonthlyReview{Month: "2026-09"}); err != nil {
		t.Fatal(err)
	}
	if _, err = r.LatestReview(cb); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("review leaked", err)
	}
	if err = r.CreateSession(ctx, "test-token-hash", a, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ResolveSession(ctx, "test-token-hash"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.DB.ExecContext(ctx, `UPDATE users SET is_active=FALSE WHERE id=$1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ResolveSession(ctx, "test-token-hash"); !errors.Is(err, auth.ErrInvalid) {
		t.Fatal("disabled user accepted", err)
	}
	if _, err = r.db.DB.ExecContext(ctx, `UPDATE users SET is_active=TRUE,password_hash='changed' WHERE id=$1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ResolveSession(ctx, "test-token-hash"); !errors.Is(err, auth.ErrInvalid) {
		t.Fatal("reset password left session usable", err)
	}
	if err = r.CreateSession(ctx, "expired", b, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ResolveSession(ctx, "expired"); !errors.Is(err, auth.ErrInvalid) {
		t.Fatal("expired token accepted", err)
	}
	for i := 0; i < 6; i++ {
		allowed, err := r.AllowLogin(ctx, "ip-key", "username-key")
		if err != nil || allowed != (i < 5) {
			t.Fatalf("rate limit %d: %v %v", i, allowed, err)
		}
	}
}
