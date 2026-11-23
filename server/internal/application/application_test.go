package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuichiro-hayafuji/arran_agent"
	"github.com/shuichirohayafuji/spendable-today/server/internal/agentadapter"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/openai"
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

type consentBlockedMemory struct{ err error }

func (m consentBlockedMemory) ExtractMemory(context.Context, domain.Consultation) ([]domain.MemoryItem, error) {
	return nil, m.err
}

func TestResultSaveReportsBlockedMemoryWithoutLosingResult(t *testing.T) {
	ctx := context.Background()
	service, repo := testService(t)
	if _, err := service.PutProfile(ctx, testProfile()); err != nil {
		t.Fatal(err)
	}
	created, err := service.StartConsultation(ctx, "相談", nil)
	if err != nil {
		t.Fatal(err)
	}
	service.memoryAgent = consentBlockedMemory{err: consultation.ErrConsentRequired}
	updated, err := service.UpdateConsultationResult(ctx, created.ID, domain.ConsultationResultUpdate{Status: "skipped"})
	if err != nil || updated.MemoryExtractionStatus != "consent_required" || updated.Status != "skipped" {
		t.Fatalf("result %+v %v", updated, err)
	}
	stored, err := repo.GetConsultation(ctx, created.ID)
	if err != nil || stored.Status != "skipped" {
		t.Fatalf("saved %+v %v", stored, err)
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

func TestMonthlyConsultationLimitUsesFallbackWithoutCountingFollowUp(t *testing.T) {
	ctx := context.Background()
	repo, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	now := time.Date(2026, 7, 31, 23, 59, 0, 0, jst)
	primary := &recordingConsultationAgent{source: "openai"}
	fallback := &recordingConsultationAgent{source: "quota_fallback"}
	primaryAux := &recordingAuxAgent{}
	fallbackAux := &recordingAuxAgent{}
	notifier := &recordingAdminNotifier{}
	service := New(Config{
		Repository: repo, ConsultationAgent: primary, FallbackAgent: fallback,
		ReviewAgent: primaryAux, FallbackReview: fallbackAux,
		MemoryAgent: primaryAux, FallbackMemory: fallbackAux,
		AdminNotifier: notifier, Environment: "test",
		Now: func() time.Time { return now },
	})
	if _, err = service.PutProfile(ctx, testProfile()); err != nil {
		t.Fatal(err)
	}

	var thirtieth domain.Consultation
	for i := 0; i < 30; i++ {
		thirtieth, err = service.StartConsultation(ctx, "相談", nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	if primary.calls != 30 || fallback.calls != 0 || thirtieth.ResponseSource != "openai" {
		t.Fatalf("at limit primary=%d fallback=%d source=%q", primary.calls, fallback.calls, thirtieth.ResponseSource)
	}
	if len(notifier.events) != 1 || notifier.events[0].Count != 30 || notifier.events[0].UserID != 0 {
		t.Fatalf("limit notifications = %#v", notifier.events)
	}
	if _, err = service.UpdateConsultationResult(ctx, thirtieth.ID, domain.ConsultationResultUpdate{Status: "skipped"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateMonthlyReview(ctx, "2026-07"); err != nil {
		t.Fatal(err)
	}
	if primaryAux.memoryCalls != 0 || primaryAux.reviewCalls != 0 ||
		fallbackAux.memoryCalls != 1 || fallbackAux.reviewCalls != 1 {
		t.Fatalf("auxiliary calls primary=%#v fallback=%#v", primaryAux, fallbackAux)
	}

	overLimit, err := service.StartConsultation(ctx, "31回目の相談", nil)
	if err != nil {
		t.Fatal(err)
	}
	if primary.calls != 30 || fallback.calls != 1 || overLimit.ResponseSource != "quota_fallback" {
		t.Fatalf("over limit primary=%d fallback=%d source=%q", primary.calls, fallback.calls, overLimit.ResponseSource)
	}
	if len(notifier.events) != 1 {
		t.Fatalf("duplicate limit notifications = %#v", notifier.events)
	}
	if _, err = service.ContinueConsultation(ctx, overLimit.ID, "追加情報", nil); err != nil {
		t.Fatal(err)
	}
	usage, err := repo.MonthlyConsultationUsage(ctx, "2026-07")
	if err != nil || usage.Count != 31 || fallback.calls != 2 {
		t.Fatalf("follow-up usage=%#v fallback=%d err=%v", usage, fallback.calls, err)
	}

	now = time.Date(2026, 8, 1, 0, 0, 0, 0, jst)
	newMonth, err := service.StartConsultation(ctx, "翌月の相談", nil)
	if err != nil {
		t.Fatal(err)
	}
	if primary.calls != 31 || newMonth.ResponseSource != "openai" {
		t.Fatalf("new month primary=%d source=%q", primary.calls, newMonth.ResponseSource)
	}
}

func TestFailedConsultationReleasesMonthlyReservation(t *testing.T) {
	ctx := context.Background()
	repo, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	primary := &recordingConsultationAgent{source: "openai", err: errors.New("model unavailable")}
	service := New(Config{
		Repository: repo, ConsultationAgent: primary,
		Now: func() time.Time { return time.Date(2026, 7, 31, 12, 0, 0, 0, jst) },
	})
	if _, err = service.PutProfile(ctx, testProfile()); err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartConsultation(ctx, "失敗する相談", nil); err == nil {
		t.Fatal("consultation unexpectedly succeeded")
	}
	usage, err := repo.MonthlyConsultationUsage(ctx, "2026-07")
	if err != nil || usage.Count != 0 {
		t.Fatalf("failed consultation usage = %#v, err = %v", usage, err)
	}
}

type recordingConsultationAgent struct {
	calls  int
	source string
	err    error
}

type recordingAdminNotifier struct {
	events []domain.MonthlyLimitEvent
	err    error
}

func (n *recordingAdminNotifier) NotifyMonthlyLimit(_ context.Context, event domain.MonthlyLimitEvent) error {
	n.events = append(n.events, event)
	return n.err
}

type recordingAuxAgent struct {
	reviewCalls int
	memoryCalls int
}

func (a *recordingAuxAgent) Review(
	context.Context,
	domain.Profile,
	domain.Dashboard,
	[]domain.Transaction,
	[]domain.Consultation,
	[]domain.ReviewCandidate,
) ([]domain.ReviewCandidate, error) {
	a.reviewCalls++
	return nil, nil
}

func (a *recordingAuxAgent) ExtractMemory(context.Context, domain.Consultation) ([]domain.MemoryItem, error) {
	a.memoryCalls++
	return nil, nil
}

func (a *recordingConsultationAgent) Run(
	context.Context,
	domain.ConsultationContext,
	string,
	*int64,
) (domain.AdviceDraft, string, error) {
	a.calls++
	if a.err != nil {
		return domain.AdviceDraft{}, "", a.err
	}
	return domain.AdviceDraft{
		Recommendation:   "今回は見送りましょう",
		Verdict:          "skip",
		ReasoningSummary: "予算を守るためです",
		CurrentSituation: "予算を確認しました",
		Alternative:      "無料の選択肢を検討してください",
		FinalQuestion:    "この方針でよいですか？",
		InferredCategory: "その他",
	}, a.source, nil
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
	preview, err := service.PreviewImport(owner, []byte("date,description,amount\n2035-01-01,coffee,500\n"), domain.CSVMapping{}, "bank", "account")
	if err != nil || preview.PreviewID == "" {
		t.Fatal(preview, err)
	}
	if _, err := service.CommitImport(other, preview.PreviewID); err == nil {
		t.Fatal("cross-user preview accepted")
	}
	first, err := service.CommitImport(owner, preview.PreviewID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CommitImport(owner, preview.PreviewID)
	if err != nil || second != first {
		t.Fatal("retry must return saved receipt", second, err)
	}
	if _, err := service.CommitImport(other, preview.PreviewID); err == nil {
		t.Fatal("cross-user receipt accepted")
	}
}

type reviewBoundaryRepository struct {
	Repository
	transactions []domain.Transaction
	quota        bool
}

func (r reviewBoundaryRepository) ListTransactions(context.Context, string, int) ([]domain.Transaction, error) {
	return r.transactions, nil
}
func (r reviewBoundaryRepository) GetProfile(context.Context) (domain.Profile, error) {
	return domain.Profile{ReduceCategories: []string{"食費"}}, nil
}
func (r reviewBoundaryRepository) MonthlyDashboard(context.Context, string, domain.Profile) (domain.Dashboard, error) {
	return domain.Dashboard{}, nil
}
func (r reviewBoundaryRepository) ListConsultations(context.Context, int) ([]domain.Consultation, error) {
	return nil, nil
}
func (r reviewBoundaryRepository) MonthlyConsultationUsage(context.Context, string) (domain.ConsultationUsage, error) {
	if r.quota {
		return domain.ConsultationUsage{Count: 1, Limit: 1}, nil
	}
	return domain.ConsultationUsage{Limit: 1}, nil
}
func (r reviewBoundaryRepository) SaveReview(_ context.Context, review domain.MonthlyReview) (domain.MonthlyReview, error) {
	return review, nil
}

type capturingReviewModel struct {
	agent.MockClient
	input agent.ReviewInput
}

func (m *capturingReviewModel) GenerateReview(_ context.Context, input agent.ReviewInput) ([]agent.ReviewCandidate, error) {
	m.input = input
	return input.LocalCandidates, nil
}

func TestReviewAgentReceivesOnlyNumberedCandidateCopies(t *testing.T) {
	for _, merchant := range []string{"酒・飲み会", "秘密（高後悔）", "普通の加盟店"} {
		t.Run(merchant, func(t *testing.T) {
			s, _ := testService(t)
			txs := []domain.Transaction{
				{TransactionDate: "2026-06-01", TransactionType: "expense", NormalizedMerchant: merchant, Amount: 9000, Category: "食費"},
				{TransactionDate: "2026-07-01", TransactionType: "expense", NormalizedMerchant: merchant, Amount: 9000, Category: "食費"},
				{TransactionDate: "2026-07-02", TransactionType: "expense", NormalizedMerchant: merchant + "最大額", Amount: 15000, Category: "食費"},
			}
			repo := reviewBoundaryRepository{Repository: s.repository, transactions: txs}
			s.repository = repo
			model := &capturingReviewModel{}
			s.reviewAgent = agentadapter.New(model, agent.MockClient{}, "test")
			if _, err := s.CreateMonthlyReview(context.Background(), "2026-07"); err != nil {
				t.Fatal(err)
			}
			if len(model.input.LocalCandidates) != 3 {
				t.Fatalf("candidates: %#v", model.input.LocalCandidates)
			}
			for i, candidate := range model.input.LocalCandidates {
				if candidate.Label != fmt.Sprintf("見直し候補%d", i+1) {
					t.Fatalf("unsafe label: %#v", candidate)
				}
			}
			if txs[0].NormalizedMerchant != merchant {
				t.Fatal("source mutated")
			}
			repo.quota = true
			s.repository = repo
			s.fallbackReview = agentadapter.New(agent.MockClient{}, agent.MockClient{}, "fallback")
			review, err := s.CreateMonthlyReview(context.Background(), "2026-07")
			if err != nil {
				t.Fatal(err)
			}
			labels := make(map[string]bool)
			for _, candidate := range review.Candidates {
				labels[candidate.Label] = true
			}
			if !labels[merchant] || !labels[merchant+"最大額"] || !labels["食費"] {
				t.Fatalf("local fallback lost labels: %#v", review.Candidates)
			}
		})
	}
}

type reviewOutcomeModel struct {
	agent.MockClient
	err error
}

func (m reviewOutcomeModel) GenerateReview(_ context.Context, _ agent.ReviewInput) ([]agent.ReviewCandidate, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []agent.ReviewCandidate{{Label: "外部回答"}}, nil
}

func TestReviewRestoresLocalOnlyForExplicitFallback(t *testing.T) {
	cases := []struct {
		name   string
		source string
		model  agent.Model
		err    error
		local  bool
	}{
		{"mock", "mock", agent.MockClient{}, nil, true},
		{"missing key", "openai", openai.NewOpenAIClient("", "model", "medium"), nil, true},
		{"ordinary error", "openai", reviewOutcomeModel{err: errors.New("ordinary")}, nil, true},
		{"external success", "openai", reviewOutcomeModel{}, nil, false},
		{"consent required", "openai", reviewOutcomeModel{err: consultation.ErrConsentRequired}, consultation.ErrConsentRequired, false},
		{"consent unavailable", "openai", reviewOutcomeModel{err: consultation.ErrConsentUnavailable}, consultation.ErrConsentUnavailable, false},
		{"unknown", "openai", reviewOutcomeModel{err: consultation.ErrAIOutcomeUnknown}, consultation.ErrAIOutcomeUnknown, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := testService(t)
			s.repository = reviewBoundaryRepository{Repository: s.repository, transactions: []domain.Transaction{{TransactionDate: "2026-07-02", TransactionType: "expense", NormalizedMerchant: "元の加盟店", Amount: 15000, Category: "食費"}}}
			check := func(err error) bool {
				return errors.Is(err, consultation.ErrConsentRequired) || errors.Is(err, consultation.ErrConsentUnavailable) || errors.Is(err, consultation.ErrAIOutcomeUnknown)
			}
			s.reviewAgent = agentadapter.NewWithFailureCheck(tc.model, agent.MockClient{}, tc.source, check)
			review, err := s.CreateMonthlyReview(context.Background(), "2026-07")
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			labels := map[string]bool{}
			for _, item := range review.Candidates {
				labels[item.Label] = true
			}
			if tc.local && !labels["元の加盟店"] {
				t.Fatalf("local labels missing: %#v", review.Candidates)
			}
			if !tc.local && (!labels["外部回答"] || labels["元の加盟店"]) {
				t.Fatalf("external response changed: %#v", review.Candidates)
			}
		})
	}
}
