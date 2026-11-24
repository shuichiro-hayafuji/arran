package postgres

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func TestUsageOperationReceipt(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	a := identity.WithUser(ctx, 701)
	b := identity.WithUser(ctx, 702)
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.beginUsageOperation(a, "op"); err != nil {
		t.Fatal(err)
	}
	if err := r.beginUsageOperation(a, "op"); err == nil {
		t.Fatal("duplicate operation admitted")
	}
	if err := r.beginUsageOperation(b, "op"); err != nil {
		t.Fatalf("other owner: %v", err)
	}
	if err := r.RequestWithdrawal(a, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("pending operation accepted: %v", err)
	}
	usage := domain.LLMUsage{UserID: 701, Operation: "test", Model: "model", InputTokens: 4, OutputTokens: 2, TotalTokens: 6, OccurredAt: time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)}
	if err := r.markUsageSending(a, "op"); err != nil {
		t.Fatal(err)
	}
	if err := r.markUsageSending(a, "op"); err == nil {
		t.Fatal("duplicate send admission")
	}
	if err := r.completeUsageWithoutRecord(a, "op", "not_sent"); err == nil {
		t.Fatal("sending operation cleared as not sent")
	}
	badFirst := usage
	badFirst.TotalTokens = -1
	if err := r.stageUsageOperation(a, "op", badFirst); err == nil {
		t.Fatal("invalid initial staging accepted")
	}
	if _, err := r.loadStagedUsage(a, "op"); err == nil {
		t.Fatal("invalid payload was staged")
	}
	if _, err := r.db.DB.ExecContext(ctx, `UPDATE ai_usage_operations SET no_usage_reason='not_sent',staged_usage='{}'::jsonb WHERE user_id=701 AND operation_id='op'`); err == nil {
		t.Fatal("contradictory no-usage staging accepted")
	}
	if err := r.stageUsageOperation(a, "op", usage); err != nil {
		t.Fatal(err)
	}
	if err := r.stageUsageOperation(a, "op", usage); err != nil {
		t.Fatal(err)
	}
	staged, err := r.loadStagedUsage(a, "op")
	if err != nil || staged.UserID != 701 || staged.TotalTokens != 6 || !staged.OccurredAt.Equal(usage.OccurredAt) {
		t.Fatalf("restart payload: %+v %v", staged, err)
	}
	if _, err := r.loadStagedUsage(identity.WithUser(ctx, 711), "op"); err == nil {
		t.Fatal("cross-owner recovery")
	}
	changed := usage
	changed.TotalTokens++
	if err := r.stageUsageOperation(a, "op", changed); err == nil {
		t.Fatal("changed staged payload accepted")
	}
	cancelled, cancel := context.WithCancel(a)
	cancel()
	if err := r.completeUsageOperation(cancelled, "op", usage); err == nil {
		t.Fatal("cancelled write succeeded")
	}
	if err := r.RequestWithdrawal(a, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("cancelled write cleared marker: %v", err)
	}
	otherOwner := usage
	otherOwner.UserID = 702
	if err := r.completeUsageOperation(a, "op", otherOwner); err == nil {
		t.Fatal("foreign usage owner accepted")
	}
	invalid := usage
	invalid.TotalTokens = -1
	if err := r.completeUsageOperation(a, "op", invalid); err == nil {
		t.Fatal("invalid usage accepted")
	}
	if err := r.RequestWithdrawal(a, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("failed insert cleared marker: %v", err)
	}

	if _, err := r.db.DB.ExecContext(ctx, `CREATE FUNCTION ignore_usage_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$; CREATE TRIGGER test_ignore_usage BEFORE UPDATE ON ai_usage_operations FOR EACH ROW EXECUTE FUNCTION ignore_usage_receipt()`); err != nil {
		t.Fatal(err)
	}
	if err := r.completeUsageOperation(a, "op", usage); err == nil {
		t.Fatal("zero-row receipt update accepted")
	}
	var priorCount int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM llm_usage WHERE user_id=701`).Scan(&priorCount); err != nil || priorCount != 0 {
		t.Fatalf("unreceipted usage persisted: %d %v", priorCount, err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `DROP TRIGGER test_ignore_usage ON ai_usage_operations; DROP FUNCTION ignore_usage_receipt()`); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { <-start; results <- r.completeUsageOperation(a, "op", usage) }()
	}
	close(start)
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent completion timed out")
		}
	}
	if err := r.completeUsageOperation(a, "op", usage); err != nil {
		t.Fatal(err)
	}
	different := usage
	different.TotalTokens++
	if err := r.completeUsageOperation(a, "op", different); err == nil {
		t.Fatal("mismatched receipt accepted")
	}
	var count int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM llm_usage WHERE user_id=701`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("usage count %d %v", count, err)
	}
	if err := r.RequestWithdrawal(a, 701); err != nil {
		t.Fatal(err)
	}
	if err := r.RequestWithdrawal(b, 702); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("other owner marker missing: %v", err)
	}
}

func TestUsageOperationNoUsage(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	owner := identity.WithUser(ctx, 701)
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.beginUsageOperation(owner, "prepared"); err != nil {
		t.Fatal(err)
	}
	if err := r.completeUsageWithoutRecord(owner, "prepared", "not_sent"); err != nil {
		t.Fatal(err)
	}
	if err := r.completeUsageWithoutRecord(owner, "prepared", "not_sent"); err != nil {
		t.Fatal(err)
	}
	if err := r.beginUsageOperation(owner, "sent"); err != nil {
		t.Fatal(err)
	}
	if err := r.markUsageSending(owner, "sent"); err != nil {
		t.Fatal(err)
	}
	if err := r.completeUsageWithoutRecord(owner, "sent", "not_sent"); err == nil {
		t.Fatal("sending marker cleared after restart")
	}
	if err := r.completeUsageWithoutRecord(owner, "sent", "no_usage"); err == nil {
		t.Fatal("unproven no-usage marker cleared")
	}
	if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("sending marker not blocking: %v", err)
	}

}

func TestUsageOperationSendAdmissionRace(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	owner := identity.WithUser(context.Background(), 701)
	if err := r.beginUsageOperation(owner, "finished"); err != nil {
		t.Fatal(err)
	}
	if err := r.completeUsageWithoutRecord(owner, "finished", "not_sent"); err != nil {
		t.Fatal(err)
	}
	if err := r.markUsageSending(owner, "finished"); err == nil {
		t.Fatal("finished operation allowed send")
	}
	if err := r.beginUsageOperation(owner, "race"); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; results <- r.markUsageSending(owner, "race") }()
	go func() { <-start; results <- r.completeUsageWithoutRecord(owner, "race", "not_sent") }()
	close(start)
	successes := 0
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err == nil {
				successes++
			}
		case <-time.After(5 * time.Second):
			t.Fatal("send-admission race timed out")
		}
	}
	if successes != 1 {
		t.Fatalf("simultaneous send/close successes: %d", successes)
	}
}

func TestUsageSendAdapter(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	r := openReconciliationRepository(t)
	ctx := context.Background()
	owner := identity.WithUser(ctx, 701)
	if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
		t.Fatal(err)
	}
	usage := domain.LLMUsage{UserID: 701, Operation: "test", Model: "model", InputTokens: 2, TotalTokens: 2, OccurredAt: time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)}
	if _, err := r.db.DB.ExecContext(ctx, `CREATE FUNCTION fail_send_mark() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.send_state='sending' THEN RAISE EXCEPTION 'test failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER test_send_mark BEFORE UPDATE ON ai_usage_operations FOR EACH ROW EXECUTE FUNCTION fail_send_mark()`); err != nil {
		t.Fatal(err)
	}
	invoked := false
	sendErr, _ := r.runUsageSend(owner, "mark-failure", func(context.Context) (*domain.LLMUsage, error) { invoked = true; return &usage, nil })
	if sendErr == nil || invoked {
		t.Fatalf("mark failure allowed send: %v %v", sendErr, invoked)
	}
	var reason sql.NullString
	if err := r.db.DB.QueryRowContext(ctx, `SELECT no_usage_reason FROM ai_usage_operations WHERE user_id=701 AND operation_id='mark-failure'`).Scan(&reason); err != nil || !reason.Valid || reason.String != "not_sent" {
		t.Fatalf("prepared cleanup: %v %v", reason, err)
	}
	if _, err := r.db.DB.ExecContext(ctx, `DROP TRIGGER test_send_mark ON ai_usage_operations; DROP FUNCTION fail_send_mark()`); err != nil {
		t.Fatal(err)
	}
	calls := 0
	send := func(context.Context) (*domain.LLMUsage, error) { calls++; return &usage, nil }
	sendErr, accountErr := r.runUsageSend(owner, "success", send)
	if sendErr != nil || accountErr != nil || calls != 1 {
		t.Fatalf("success: %v %v %d", sendErr, accountErr, calls)
	}
	sendErr, _ = r.runUsageSend(owner, "success", send)
	if sendErr == nil || calls != 1 {
		t.Fatalf("duplicate sent: %v %d", sendErr, calls)
	}
	canceled, cancel := context.WithCancel(owner)
	sendErr, accountErr = r.runUsageSend(canceled, "cancel", func(context.Context) (*domain.LLMUsage, error) { cancel(); return &usage, nil })
	if sendErr != nil || accountErr != nil {
		t.Fatalf("post-response cancellation: %v %v", sendErr, accountErr)
	}
	providerErr := errors.New("terminal provider error")
	sendErr, accountErr = r.runUsageSend(owner, "provider-error", func(context.Context) (*domain.LLMUsage, error) { return &usage, providerErr })
	if !errors.Is(sendErr, providerErr) || accountErr != nil {
		t.Fatalf("terminal error accounting: %v %v", sendErr, accountErr)
	}
	var recorded int
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM ai_usage_operations o JOIN llm_usage u ON u.id=o.usage_id AND u.user_id=o.user_id WHERE o.user_id=701 AND o.operation_id='provider-error' AND u.total_tokens=2`).Scan(&recorded); err != nil || recorded != 1 {
		t.Fatalf("provider-error receipt count %d: %v", recorded, err)
	}
	if err := r.completeUsageOperation(owner, "provider-error", usage); err != nil {
		t.Fatal(err)
	}
	if err := r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM llm_usage WHERE user_id=701`).Scan(&recorded); err != nil || recorded != 3 {
		t.Fatalf("duplicate usage after retry: %d %v", recorded, err)
	}

	bad := usage
	bad.TotalTokens = -1
	sendErr, accountErr = r.runUsageSend(owner, "accounting-failure", func(context.Context) (*domain.LLMUsage, error) { return &bad, nil })
	if sendErr != nil || accountErr == nil {
		t.Fatalf("answer lost or accounting failure missed: %v %v", sendErr, accountErr)
	}
	if err := r.RequestWithdrawal(owner, 701); !errors.Is(err, identity.ErrWithdrawalBlocked) {
		t.Fatalf("unconfirmed accounting accepted: %v", err)
	}
	if sendErr, _ = r.runUsageSend(owner, "accounting-failure", send); sendErr == nil || calls != 1 {
		t.Fatalf("failed operation resent: %v %d", sendErr, calls)
	}
}
