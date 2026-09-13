package application

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuichiro-hayafuji/arran_agent"
	"github.com/shuichirohayafuji/spendable-today/server/internal/agentadapter"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/sqlite"
)

func TestBuildConsultationContextUsesSummariesAndStructuredMemory(t *testing.T) {
	ctx := context.Background()
	service, repo := testService(t)
	profile := testProfile()
	if _, err := service.PutProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	importSample(t, ctx, repo)
	if _, err := service.SaveMemory(ctx, domain.MemoryItem{
		Type:       "value",
		Content:    "技術書は学習投資として許容する",
		Evidence:   "本人が設定",
		Confidence: 0.9,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := service.BuildConsultationContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Dashboard.ExpenseTotal != 18090 {
		t.Fatalf("expense total = %d", result.Dashboard.ExpenseTotal)
	}
	if len(result.Memories) != 1 {
		t.Fatalf("memories = %d", len(result.Memories))
	}
	for _, transaction := range result.RelevantTransactions {
		if transaction.RawData != nil {
			t.Fatal("raw CSV data must not enter consultation context")
		}
	}
}

func TestConsultationResultCreatesReusableMemory(t *testing.T) {
	ctx := context.Background()
	service, _ := testService(t)
	if _, err := service.PutProfile(ctx, testProfile()); err != nil {
		t.Fatal(err)
	}
	amount := int64(6000)
	consultation, err := service.StartConsultation(ctx, "今から飲みに行っていい？", &amount)
	if err != nil {
		t.Fatal(err)
	}
	score := 5
	consultation, err = service.UpdateConsultationResult(
		ctx,
		consultation.ID,
		domain.ConsultationResultUpdate{
			Status:             "spent",
			ActualAmount:       &amount,
			UserDecisionReason: "なんとなく参加して翌日に後悔した",
			RegretScore:        &score,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	memories, err := service.ListMemories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 ||
		!strings.Contains(memories[0].Content, "酒・飲み会") {
		t.Fatalf("memories = %#v", memories)
	}
}

func TestDetectSubscriptionCandidates(t *testing.T) {
	transactions := []domain.Transaction{
		{
			TransactionDate:    "2026-05-03",
			NormalizedMerchant: "netflix",
			Amount:             1490,
			TransactionType:    "expense",
		},
		{
			TransactionDate:    "2026-06-03",
			NormalizedMerchant: "netflix",
			Amount:             1490,
			TransactionType:    "expense",
		},
		{
			TransactionDate:    "2026-07-03",
			NormalizedMerchant: "netflix",
			Amount:             1490,
			TransactionType:    "expense",
		},
	}
	got := DetectSubscriptionCandidates(transactions)
	if len(got) != 1 {
		t.Fatalf("candidates = %#v", got)
	}
	if got[0].AnnualizedAmount != 17880 {
		t.Fatalf("annualized = %d", got[0].AnnualizedAmount)
	}
}

func TestReviewCandidateSanitizationRemovesMerchant(t *testing.T) {
	result := sanitizeReviewCandidates([]domain.ReviewCandidate{
		{Label: "secret merchant"},
		{Label: "酒・飲み会"},
	})
	if result[0].Label != "同一加盟店の支出パターン" {
		t.Fatalf("merchant label = %q", result[0].Label)
	}
	if result[1].Label != "酒・飲み会" {
		t.Fatalf("category label = %q", result[1].Label)
	}
}

func testService(t *testing.T) (*Application, *sqlite.Repository) {
	t.Helper()
	repo, err := sqlite.Open(
		context.Background(),
		filepath.Join(t.TempDir(), "test.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, jst)
	fallback := agent.MockClient{}
	agents := agentadapter.New(fallback, fallback, "mock")
	return New(Config{
		Repository: repo, ConsultationAgent: agents, ReviewAgent: agents, MemoryAgent: agents,
		Now: func() time.Time { return now },
	}), repo
}

func importSample(
	t *testing.T,
	ctx context.Context,
	repo *sqlite.Repository,
) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "sample_data", "utf8_general.csv"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := csvimport.Parse(data, csvimport.Options{
		Source: "card",
		Now:    time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ImportTransactions(ctx, parsed.Transactions); err != nil {
		t.Fatal(err)
	}
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
	}
}

func TestAdviceUsesBoundedRevisionLoop(t *testing.T) {
	ctx := context.Background()
	repo, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, jst)
	client := &scriptedAdviceClient{}
	service := New(Config{
		Repository:        repo,
		ConsultationAgent: agentadapter.New(client, agent.MockClient{}, "scripted"),
		ReviewAgent:       agentadapter.New(client, agent.MockClient{}, "scripted"),
		MemoryAgent:       agentadapter.New(client, agent.MockClient{}, "scripted"),
		Now:               func() time.Time { return now },
	})
	if _, err := service.PutProfile(ctx, testProfile()); err != nil {
		t.Fatal(err)
	}

	amount := int64(6000)
	consultation, err := service.StartConsultation(ctx, "今から飲みに行っていい？", &amount)
	if err != nil {
		t.Fatal(err)
	}
	if client.revisionCalls != 1 {
		t.Fatalf("revision calls = %d, want 1", client.revisionCalls)
	}
	if consultation.ResponseSource != "scripted" {
		t.Fatalf("response source = %q", consultation.ResponseSource)
	}
	if consultation.AIRecommendation == "" || consultation.InferredCategory != "酒・飲み会" {
		t.Fatalf("consultation = %#v", consultation)
	}
}

type scriptedAdviceClient struct {
	revisionCalls int
}

func (c *scriptedAdviceClient) GenerateAdvice(
	context.Context,
	agent.ConsultationInput,
) (agent.AdviceDraft, error) {
	return agent.AdviceDraft{}, nil
}

func (c *scriptedAdviceClient) GenerateAdviceRevision(
	ctx context.Context,
	input agent.AdviceRevisionInput,
) (agent.AdviceDraft, error) {
	c.revisionCalls++
	return (agent.MockClient{}).GenerateAdvice(ctx, input.Input)
}

func (c *scriptedAdviceClient) InferCategory(
	ctx context.Context,
	description string,
	categories []string,
) (string, error) {
	return (agent.MockClient{}).InferCategory(ctx, description, categories)
}

func (c *scriptedAdviceClient) GenerateReview(
	ctx context.Context,
	input agent.ReviewInput,
) ([]agent.ReviewCandidate, error) {
	return (agent.MockClient{}).GenerateReview(ctx, input)
}

func (c *scriptedAdviceClient) ExtractMemoryCandidates(
	ctx context.Context,
	consultation agent.MemoryInput,
) ([]agent.MemoryItem, error) {
	return (agent.MockClient{}).ExtractMemoryCandidates(ctx, consultation)
}

// An unauthorized commit must not consume the rightful owner's preview.
func TestPreviewOwnership(t *testing.T) {
	service, _ := testService(t)
	owner := identity.WithUser(context.Background(), 1)
	other := identity.WithUser(context.Background(), 2)
	service.previews["owned-preview"] = cachedPreview{userID: 1, expiresAt: time.Now().Add(time.Hour)}
	if _, err := service.CommitImport(other, "owned-preview"); err == nil {
		t.Fatal("cross-user preview accepted")
	}
	if _, err := service.CommitImport(owner, "owned-preview"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CommitImport(owner, "owned-preview"); err == nil {
		t.Fatal("preview reused")
	}
}
