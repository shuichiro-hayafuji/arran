// Package application は HTTP に依存せず、家計管理のユースケースを組み立てる。
package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

// IsNotFound is an application-level error mapping for HTTP transport.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }

type Application struct {
	repository        Repository
	consultationAgent ConsultationAgent
	fallbackAgent     ConsultationAgent
	reviewAgent       ReviewAgent
	fallbackReview    ReviewAgent
	memoryAgent       MemoryAgent
	fallbackMemory    MemoryAgent
	adminNotifier     AdminNotifier
	environment       string
	now               func() time.Time

	previewMu sync.Mutex
	previews  map[string]cachedPreview
}

type cachedPreview struct {
	userID       int64
	transactions []domain.Transaction
	expiresAt    time.Time
}

// ConsultationAgent はアプリケーションのドメイン型で相談を受け付ける境界。
// 独立した Agent モジュールの型への変換は agentadapter が担う。
type ConsultationAgent interface {
	Run(context.Context, domain.ConsultationContext, string, *int64) (domain.AdviceDraft, string, error)
}
type ReviewAgent interface {
	Review(context.Context, domain.Profile, domain.Dashboard, []domain.Transaction, []domain.Consultation, []domain.ReviewCandidate) ([]domain.ReviewCandidate, error)
}
type MemoryAgent interface {
	ExtractMemory(context.Context, domain.Consultation) ([]domain.MemoryItem, error)
}
type AdminNotifier interface {
	NotifyMonthlyLimit(context.Context, domain.MonthlyLimitEvent) error
}

// ProfileRepository はプロフィールを扱うユースケースが必要とする永続化境界。
type ProfileRepository interface {
	GetProfile(context.Context) (domain.Profile, error)
	PutProfile(context.Context, domain.Profile) error
}

// TransactionRepository は取引取込・一覧・集計に必要な永続化境界。
type TransactionRepository interface {
	MerchantRules(context.Context) (map[string]string, error)
	ImportTransactions(context.Context, []domain.Transaction) (domain.ImportCommitResult, error)
	ListTransactions(context.Context, string, int) ([]domain.Transaction, error)
	UpdateTransactionCategory(context.Context, int64, string, bool) (domain.Transaction, error)
	MonthlyDashboard(context.Context, string, domain.Profile) (domain.Dashboard, error)
}

// ConsultationRepository は相談と会話履歴に必要な永続化境界。
type ConsultationRepository interface {
	MonthlyConsultationLimit(context.Context) (int, error)
	MonthlyConsultationUsage(context.Context, string) (domain.ConsultationUsage, error)
	ReserveMonthlyConsultation(context.Context, string) (domain.ConsultationUsage, error)
	ReleaseMonthlyConsultation(context.Context, string) error
	ClaimMonthlyLimitNotification(context.Context, domain.MonthlyLimitEvent) (bool, error)
	CompleteMonthlyLimitNotification(context.Context, domain.MonthlyLimitEvent, error) error
	CreateConsultation(context.Context, domain.Consultation) (domain.Consultation, error)
	UpdateConsultationAdvice(context.Context, domain.Consultation) (domain.Consultation, error)
	ListConsultations(context.Context, int) ([]domain.Consultation, error)
	GetConsultation(context.Context, int64) (domain.Consultation, error)
	UpdateConsultationResult(context.Context, int64, domain.ConsultationResultUpdate) (domain.Consultation, error)
	AddMessage(context.Context, int64, string, string) error
}

// MemoryRepository は相談から得た記憶を扱う永続化境界。
type MemoryRepository interface {
	ListMemories(context.Context) ([]domain.MemoryItem, error)
	SaveMemory(context.Context, domain.MemoryItem) (domain.MemoryItem, error)
	DeleteMemory(context.Context, int64) error
}

// ReviewRepository は月次レビューを扱う永続化境界。
type ReviewRepository interface {
	SaveReview(context.Context, domain.MonthlyReview) (domain.MonthlyReview, error)
	LatestReview(context.Context) (domain.MonthlyReview, error)
}

// Repository は Application が利用する用途別の永続化境界を束ねる。
// PostgreSQL とテスト用 SQLite の実装は infrastructure 配下に置く。
type Repository interface {
	ProfileRepository
	TransactionRepository
	ConsultationRepository
	MemoryRepository
	ReviewRepository
}

type Config struct {
	Repository        Repository
	ConsultationAgent ConsultationAgent
	FallbackAgent     ConsultationAgent
	ReviewAgent       ReviewAgent
	FallbackReview    ReviewAgent
	MemoryAgent       MemoryAgent
	FallbackMemory    MemoryAgent
	AdminNotifier     AdminNotifier
	Environment       string
	Now               func() time.Time
}

func New(config Config) *Application {
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &Application{
		repository:        config.Repository,
		consultationAgent: config.ConsultationAgent,
		fallbackAgent:     config.FallbackAgent,
		reviewAgent:       config.ReviewAgent,
		fallbackReview:    config.FallbackReview,
		memoryAgent:       config.MemoryAgent,
		fallbackMemory:    config.FallbackMemory,
		adminNotifier:     config.AdminNotifier,
		environment:       config.Environment,
		now:               now,
		previews:          map[string]cachedPreview{},
	}
}

func (s *Application) GetProfile(ctx context.Context) (domain.Profile, error) {
	return s.repository.GetProfile(ctx)
}

func (s *Application) PutProfile(
	ctx context.Context,
	profile domain.Profile,
) (domain.Profile, error) {
	if err := validateProfile(profile); err != nil {
		return domain.Profile{}, err
	}
	profile.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if err := s.repository.PutProfile(ctx, profile); err != nil {
		return domain.Profile{}, err
	}
	return profile, nil
}

// PreviewImport は解析した全明細を利用者に紐づけてプロセス内に30分保持する。
// 応答の Rows は先頭10件に限定し、DBへの登録は CommitImport まで行わない。
func (s *Application) PreviewImport(
	ctx context.Context,
	data []byte,
	mapping domain.CSVMapping,
	source string,
	account string,
) (domain.ImportPreview, error) {
	rules, err := s.repository.MerchantRules(ctx)
	if err != nil {
		return domain.ImportPreview{}, err
	}
	parsed, err := csvimport.Parse(data, csvimport.Options{
		Mapping:           mapping,
		Source:            source,
		SourceAccountName: account,
		MerchantRules:     rules,
		Now:               s.now(),
	})
	if err != nil {
		return domain.ImportPreview{}, err
	}
	preview := domain.ImportPreview{
		DetectedEncoding: parsed.Encoding,
		Headers:          parsed.Headers,
		Mapping:          parsed.Mapping,
		NeedsMapping:     parsed.NeedsMapping,
		ReadCount:        len(parsed.Transactions),
		Warnings:         parsed.Warnings,
		Rows:             parsed.Transactions,
	}
	if len(preview.Rows) > 10 {
		preview.Rows = preview.Rows[:10]
	}
	for _, transaction := range parsed.Transactions {
		if preview.PeriodStart == "" || transaction.TransactionDate < preview.PeriodStart {
			preview.PeriodStart = transaction.TransactionDate
		}
		if transaction.TransactionDate > preview.PeriodEnd {
			preview.PeriodEnd = transaction.TransactionDate
		}
		if transaction.TransactionType == "expense" {
			preview.ExpenseTotal += transaction.Amount
		}
	}
	if !parsed.NeedsMapping {
		preview.PreviewID = randomID()
		s.previewMu.Lock()
		s.cleanupPreviewsLocked()
		s.previews[preview.PreviewID] = cachedPreview{
			userID:       identity.UserID(ctx),
			transactions: parsed.Transactions,
			expiresAt:    s.now().Add(30 * time.Minute),
		}
		s.previewMu.Unlock()
	}
	return preview, nil
}

// CommitImport は所有者と有効期限を確認して、保持中の明細を登録する。
// プレビューはDB書き込み前に消費するため、書き込み失敗後も再プレビューが必要。
func (s *Application) CommitImport(
	ctx context.Context,
	previewID string,
) (domain.ImportCommitResult, error) {
	s.previewMu.Lock()
	cached, ok := s.previews[previewID]
	ok = ok && cached.userID == identity.UserID(ctx)
	if ok {
		delete(s.previews, previewID)
	}
	s.previewMu.Unlock()
	if !ok || s.now().After(cached.expiresAt) {
		return domain.ImportCommitResult{}, fmt.Errorf("プレビューの有効期限が切れています")
	}
	return s.repository.ImportTransactions(ctx, cached.transactions)
}

func (s *Application) ListTransactions(
	ctx context.Context,
	month string,
	limit int,
) ([]domain.Transaction, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	return s.repository.ListTransactions(ctx, month, limit)
}

func (s *Application) UpdateTransactionCategory(
	ctx context.Context,
	id int64,
	category string,
	remember bool,
) (domain.Transaction, error) {
	if !slices.Contains(domain.Categories, category) {
		return domain.Transaction{}, fmt.Errorf("未対応のカテゴリです")
	}
	return s.repository.UpdateTransactionCategory(ctx, id, category, remember)
}

func (s *Application) Dashboard(
	ctx context.Context,
	month string,
) (domain.Dashboard, error) {
	if month == "" {
		month = s.now().In(jst).Format("2006-01")
	}
	profile, err := s.repository.GetProfile(ctx)
	if err != nil {
		return domain.Dashboard{}, err
	}
	return s.repository.MonthlyDashboard(ctx, month, profile)
}

func (s *Application) StartConsultation(
	ctx context.Context,
	message string,
	plannedAmount *int64,
) (domain.Consultation, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return domain.Consultation{}, fmt.Errorf("相談内容を入力してください")
	}
	if plannedAmount != nil && *plannedAmount < 0 {
		return domain.Consultation{}, fmt.Errorf("予定額は0円以上にしてください")
	}
	month := s.now().In(jst).Format("2006-01")
	usage, err := s.repository.ReserveMonthlyConsultation(ctx, month)
	if err != nil {
		return domain.Consultation{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.repository.ReleaseMonthlyConsultation(ctx, month)
		}
	}()
	consultationContext, err := s.BuildConsultationContext(ctx)
	if err != nil {
		return domain.Consultation{}, err
	}
	consultationAgent := s.consultationAgent
	if !usage.UseExternalModel() {
		consultationAgent = s.fallbackAgent
	}
	draft, source, err := consultationAgent.Run(ctx, consultationContext, message, plannedAmount)
	if err != nil {
		return domain.Consultation{}, err
	}
	consultation := draftToConsultation(
		draft,
		message,
		plannedAmount,
		consultationContext.Dashboard,
		s.now(),
		source,
	)
	created, err := s.repository.CreateConsultation(ctx, consultation)
	if err != nil {
		return domain.Consultation{}, err
	}
	committed = true
	s.notifyMonthlyLimit(ctx, usage)
	_ = s.repository.AddMessage(ctx, created.ID, "user", message)
	_ = s.repository.AddMessage(ctx, created.ID, "assistant", draft.Recommendation)
	return created, nil
}

func (s *Application) notifyMonthlyLimit(ctx context.Context, usage domain.ConsultationUsage) {
	if s.adminNotifier == nil || usage.Count < usage.Limit || usage.Count == 0 {
		return
	}
	event := domain.MonthlyLimitEvent{
		Environment: s.environment,
		UserID:      identity.UserID(ctx),
		Month:       usage.Month,
		Count:       usage.Count,
		Limit:       usage.Limit,
		OccurredAt:  s.now().UTC(),
	}
	claimed, err := s.repository.ClaimMonthlyLimitNotification(ctx, event)
	if err != nil || !claimed {
		return
	}
	notifyErr := s.adminNotifier.NotifyMonthlyLimit(ctx, event)
	_ = s.repository.CompleteMonthlyLimitNotification(ctx, event, notifyErr)
}

// ContinueConsultation は元の相談文と今回の追加情報で助言を更新する。
// 予定額が未指定なら前回値を引き継ぎ、相談IDと作成日時は維持する。
func (s *Application) ContinueConsultation(
	ctx context.Context,
	id int64,
	message string,
	plannedAmount *int64,
) (domain.Consultation, error) {
	consultation, err := s.repository.GetConsultation(ctx, id)
	if err != nil {
		return domain.Consultation{}, err
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return domain.Consultation{}, fmt.Errorf("メッセージを入力してください")
	}
	if plannedAmount == nil {
		plannedAmount = consultation.PlannedAmount
	}
	consultationContext, err := s.BuildConsultationContext(ctx)
	if err != nil {
		return domain.Consultation{}, err
	}
	usage, err := s.repository.MonthlyConsultationUsage(ctx, s.now().In(jst).Format("2006-01"))
	if err != nil {
		return domain.Consultation{}, err
	}
	consultationAgent := s.consultationAgent
	if usage.Count >= usage.Limit {
		consultationAgent = s.fallbackAgent
	}
	draft, source, err := consultationAgent.Run(ctx, consultationContext, consultation.UserMessage+"\n追加情報: "+message, plannedAmount)
	if err != nil {
		return domain.Consultation{}, err
	}
	updated := draftToConsultation(
		draft,
		consultation.UserMessage,
		plannedAmount,
		consultationContext.Dashboard,
		s.now(),
		source,
	)
	updated.ID = id
	updated.CreatedAt = consultation.CreatedAt
	created, err := s.repository.UpdateConsultationAdvice(ctx, updated)
	if err != nil {
		return domain.Consultation{}, err
	}
	_ = s.repository.AddMessage(ctx, id, "user", message)
	_ = s.repository.AddMessage(ctx, id, "assistant", draft.Recommendation)
	return created, nil
}

// UpdateConsultationResult は結果の保存後、根拠があり信頼度0.7以上の記憶候補を保存する。
// 記憶の抽出・保存は補助処理として扱い、その失敗では結果の保存を取り消さない。
func (s *Application) UpdateConsultationResult(
	ctx context.Context,
	id int64,
	update domain.ConsultationResultUpdate,
) (domain.Consultation, error) {
	allowedStatuses := []string{"spent", "skipped", "reduced", "pending"}
	if !slices.Contains(allowedStatuses, update.Status) {
		return domain.Consultation{}, fmt.Errorf("結果の選択肢が不正です")
	}
	if err := validateScore(update.SatisfactionScore); err != nil {
		return domain.Consultation{}, err
	}
	if err := validateScore(update.RegretScore); err != nil {
		return domain.Consultation{}, err
	}
	consultation, err := s.repository.UpdateConsultationResult(ctx, id, update)
	if err != nil {
		return domain.Consultation{}, err
	}
	memoryAgent := s.memoryAgent
	if !s.externalModelAllowed(ctx) {
		memoryAgent = s.fallbackMemory
	}
	candidates, _ := memoryAgent.ExtractMemory(ctx, consultation)
	for _, candidate := range candidates {
		if candidate.Confidence < 0.7 ||
			strings.TrimSpace(candidate.Content) == "" ||
			strings.TrimSpace(candidate.Evidence) == "" {
			continue
		}
		_, _ = s.repository.SaveMemory(ctx, candidate)
	}
	return consultation, nil
}

func (s *Application) externalModelAllowed(ctx context.Context) bool {
	usage, err := s.repository.MonthlyConsultationUsage(ctx, s.now().In(jst).Format("2006-01"))
	return err == nil && usage.Count < usage.Limit
}

func (s *Application) ListConsultations(
	ctx context.Context,
	limit int,
) ([]domain.Consultation, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repository.ListConsultations(ctx, limit)
}

func (s *Application) GetConsultation(
	ctx context.Context,
	id int64,
) (domain.Consultation, error) {
	return s.repository.GetConsultation(ctx, id)
}

// BuildConsultationContext は日本時間の当月集計と件数を絞った履歴を相談用に集める。
// CSV原文はここで除去し、加盟店名などの除去は Agent 境界での型変換が担う。
func (s *Application) BuildConsultationContext(
	ctx context.Context,
) (domain.ConsultationContext, error) {
	profile, err := s.repository.GetProfile(ctx)
	if err != nil {
		return domain.ConsultationContext{}, err
	}
	now := s.now().In(jst)
	dashboard, err := s.repository.MonthlyDashboard(
		ctx,
		now.Format("2006-01"),
		profile,
	)
	if err != nil {
		return domain.ConsultationContext{}, err
	}
	consultations, err := s.repository.ListConsultations(ctx, 5)
	if err != nil {
		return domain.ConsultationContext{}, err
	}
	memories, err := s.repository.ListMemories(ctx)
	if err != nil {
		return domain.ConsultationContext{}, err
	}
	if len(memories) > 20 {
		memories = memories[:20]
	}
	transactions, err := s.repository.ListTransactions(ctx, now.Format("2006-01"), 10)
	if err != nil {
		return domain.ConsultationContext{}, err
	}
	for index := range transactions {
		transactions[index].RawData = nil
	}
	return domain.ConsultationContext{
		Now:                  now,
		Profile:              profile,
		Dashboard:            dashboard,
		RecentConsultations:  consultations,
		Memories:             memories,
		RelevantTransactions: transactions,
	}, nil
}

func (s *Application) ListMemories(ctx context.Context) ([]domain.MemoryItem, error) {
	return s.repository.ListMemories(ctx)
}

func (s *Application) SaveMemory(
	ctx context.Context,
	item domain.MemoryItem,
) (domain.MemoryItem, error) {
	if strings.TrimSpace(item.Type) == "" || strings.TrimSpace(item.Content) == "" {
		return domain.MemoryItem{}, fmt.Errorf("typeとcontentは必須です")
	}
	if item.Confidence < 0 || item.Confidence > 1 {
		return domain.MemoryItem{}, fmt.Errorf("confidenceは0から1で指定してください")
	}
	return s.repository.SaveMemory(ctx, item)
}

func (s *Application) DeleteMemory(ctx context.Context, id int64) error {
	return s.repository.DeleteMemory(ctx, id)
}

func (s *Application) CreateMonthlyReview(
	ctx context.Context,
	month string,
) (domain.MonthlyReview, error) {
	if month == "" {
		month = s.now().In(jst).Format("2006-01")
	}
	profile, err := s.repository.GetProfile(ctx)
	if err != nil {
		return domain.MonthlyReview{}, err
	}
	dashboard, err := s.repository.MonthlyDashboard(ctx, month, profile)
	if err != nil {
		return domain.MonthlyReview{}, err
	}
	transactions, err := s.repository.ListTransactions(ctx, "", 1000)
	if err != nil {
		return domain.MonthlyReview{}, err
	}
	consultations, err := s.repository.ListConsultations(ctx, 100)
	if err != nil {
		return domain.MonthlyReview{}, err
	}
	local := BuildReviewCandidates(
		month,
		s.now().In(jst),
		profile,
		dashboard,
		transactions,
		consultations,
	)
	reviewAgent := s.reviewAgent
	if !s.externalModelAllowed(ctx) {
		reviewAgent = s.fallbackReview
	}
	candidates, err := reviewAgent.Review(ctx, profile, dashboard, sanitizeForLLM(transactions), consultations, sanitizeReviewCandidates(local))
	if err != nil {
		candidates = local
	}
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	review := domain.MonthlyReview{
		Month:      month,
		CreatedAt:  s.now().UTC().Format(time.RFC3339),
		Summary:    fmt.Sprintf("今月の支出%sから、%d件の見直し候補を抽出しました。", yen(dashboard.ExpenseTotal), len(candidates)),
		Candidates: candidates,
	}
	return s.repository.SaveReview(ctx, review)
}

func (s *Application) LatestReview(ctx context.Context) (domain.MonthlyReview, error) {
	return s.repository.LatestReview(ctx)
}

func BuildReviewCandidates(
	month string,
	now time.Time,
	profile domain.Profile,
	dashboard domain.Dashboard,
	transactions []domain.Transaction,
	consultations []domain.Consultation,
) []domain.ReviewCandidate {
	var candidates []domain.ReviewCandidate
	add := func(candidate domain.ReviewCandidate) {
		for _, existing := range candidates {
			if existing.Label == candidate.Label {
				return
			}
		}
		candidates = append(candidates, candidate)
	}

	for _, category := range profile.ReduceCategories {
		total, count := categoryStats(transactions, month, category)
		if total > 0 {
			add(domain.ReviewCandidate{
				Label:            category,
				Judgement:        "見直し候補",
				TotalAmount:      total,
				Count:            count,
				Reason:           "プロフィールで減らしたいカテゴリに指定されています。",
				AnnualizedAmount: total * 12,
				Question:         "このカテゴリのうち、満足度が低かった支出はどれですか？",
			})
		}
	}
	if dashboard.DrinkingTotal > 0 {
		_, count := categoryStats(transactions, month, "酒・飲み会")
		add(domain.ReviewCandidate{
			Label:            "酒・飲み会",
			Judgement:        "見直し候補",
			TotalAmount:      dashboard.DrinkingTotal,
			Count:            count,
			Reason:           "今月の飲酒関連支出をまとめました。",
			AnnualizedAmount: dashboard.DrinkingTotal * 12,
			Question:         "人間関係など、価値を感じた会だけを残せていますか？",
		})
	}
	if dashboard.SubscriptionTotal > 0 {
		_, count := categoryStats(transactions, month, "サブスクリプション")
		add(domain.ReviewCandidate{
			Label:            "サブスクリプション",
			Judgement:        "見直し候補",
			TotalAmount:      dashboard.SubscriptionTotal,
			Count:            count,
			Reason:           "定期支出は使っていなくても継続しやすいためです。",
			AnnualizedAmount: dashboard.SubscriptionTotal * 12,
			Question:         "先月から一度も使っていないサービスはありますか？",
		})
	}
	for _, candidate := range DetectSubscriptionCandidates(transactions) {
		add(candidate)
	}
	if dashboard.MonthlyFreeBudget > 0 &&
		now.Format("2006-01") == month &&
		now.Day() <= 15 &&
		dashboard.FreeExpenseTotal*2 > dashboard.MonthlyFreeBudget {
		add(domain.ReviewCandidate{
			Label:            "月前半の支出ペース",
			Judgement:        "不要の可能性が高い",
			TotalAmount:      dashboard.FreeExpenseTotal,
			Count:            countExpenses(transactions, month),
			Reason:           "月前半の時点で自由予算の半分を超えています。",
			AnnualizedAmount: dashboard.FreeExpenseTotal * 12,
			Question:         "月後半に優先したい予定の予算を残せていますか？",
		})
	}
	if largest := largestExpense(transactions, month); largest != nil {
		add(domain.ReviewCandidate{
			Label:            largest.NormalizedMerchant,
			Judgement:        "判断に追加情報が必要",
			TotalAmount:      largest.Amount,
			Count:            1,
			Reason:           "今月の中で金額が大きい支出です。",
			AnnualizedAmount: largest.Amount * 12,
			Question:         "この支出から、金額に見合う価値を得られましたか？",
		})
	}
	for _, consultation := range consultations {
		if consultation.RegretScore != nil && *consultation.RegretScore >= 4 {
			add(domain.ReviewCandidate{
				Label:            consultation.InferredCategory + "（高後悔）",
				Judgement:        "不要の可能性が高い",
				TotalAmount:      pointerValue(consultation.ActualAmount),
				Count:            1,
				Reason:           "過去の相談結果で後悔度が高く記録されています。",
				AnnualizedAmount: pointerValue(consultation.ActualAmount) * 12,
				Question:         "次回はどの条件なら見送れそうですか？",
			})
		}
	}
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return candidates
}

// DetectSubscriptionCandidates は同一加盟店で25〜35日間隔の同額支出を候補化する。
// 契約の確定情報ではなく、利用者に継続利用を確認するための推定結果を返す。
func DetectSubscriptionCandidates(
	transactions []domain.Transaction,
) []domain.ReviewCandidate {
	byMerchant := map[string][]domain.Transaction{}
	for _, transaction := range transactions {
		if transaction.TransactionType != "expense" {
			continue
		}
		byMerchant[transaction.NormalizedMerchant] = append(
			byMerchant[transaction.NormalizedMerchant],
			transaction,
		)
	}
	var candidates []domain.ReviewCandidate
	for merchant, items := range byMerchant {
		if len(items) < 2 {
			continue
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].TransactionDate < items[j].TransactionDate
		})
		matched := false
		for index := 1; index < len(items); index++ {
			before, errorBefore := time.Parse("2006-01-02", items[index-1].TransactionDate)
			after, errorAfter := time.Parse("2006-01-02", items[index].TransactionDate)
			if errorBefore != nil || errorAfter != nil {
				continue
			}
			days := int(after.Sub(before).Hours() / 24)
			if days >= 25 && days <= 35 &&
				items[index].Amount == items[index-1].Amount {
				matched = true
				break
			}
		}
		if matched {
			latest := items[len(items)-1]
			candidates = append(candidates, domain.ReviewCandidate{
				Label:            merchant,
				Judgement:        "見直し候補",
				TotalAmount:      latest.Amount,
				Count:            len(items),
				Reason:           "約1か月間隔で同額の支出があり、サブスクの可能性があります。",
				AnnualizedAmount: latest.Amount * 12,
				Question:         "現在も毎月利用していますか？",
			})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].AnnualizedAmount > candidates[j].AnnualizedAmount
	})
	return candidates
}

func draftToConsultation(
	draft domain.AdviceDraft,
	message string,
	amount *int64,
	dashboard domain.Dashboard,
	now time.Time,
	source string,
) domain.Consultation {
	status := "awaiting_result"
	if draft.NeedsFollowUp {
		status = "awaiting_information"
	}
	return domain.Consultation{
		CreatedAt:          now.UTC().Format(time.RFC3339),
		UserMessage:        message,
		PlannedAmount:      amount,
		InferredCategory:   draft.InferredCategory,
		AIRecommendation:   draft.Recommendation,
		AIReasoningSummary: draft.ReasoningSummary,
		CurrentSituation:   draft.CurrentSituation,
		Alternative:        draft.Alternative,
		FinalQuestion:      draft.FinalQuestion,
		BudgetSnapshot: domain.BudgetSnapshot{
			Month:               dashboard.Month,
			ExpenseTotal:        dashboard.ExpenseTotal,
			FreeBudgetRemaining: dashboard.FreeBudgetRemaining,
			DrinkingTotal:       dashboard.DrinkingTotal,
			SubscriptionTotal:   dashboard.SubscriptionTotal,
			ByCategory:          dashboard.ByCategory,
		},
		Status:           status,
		ResponseSource:   source,
		NeedsFollowUp:    draft.NeedsFollowUp,
		FollowUpQuestion: draft.FollowUpQuestion,
	}
}

func validateProfile(profile domain.Profile) error {
	values := []int64{
		profile.MonthlyIncome,
		profile.CurrentBalance,
		profile.MonthlyFixedCosts,
		profile.MonthlyFreeBudget,
		profile.MonthlySavingsGoal,
	}
	for _, value := range values {
		if value < 0 {
			return fmt.Errorf("金額は0円以上にしてください")
		}
	}
	if profile.MonthlyFreeBudget == 0 {
		return fmt.Errorf("月間自由支出予算は必須です")
	}
	if !slices.Contains([]string{"やさしい", "バランス型", "厳しい"}, profile.AdviceStrictness) {
		return fmt.Errorf("AIの厳しさを選択してください")
	}
	return nil
}

func validateScore(score *int) error {
	if score != nil && (*score < 1 || *score > 5) {
		return fmt.Errorf("評価は1から5で指定してください")
	}
	return nil
}

func (s *Application) cleanupPreviewsLocked() {
	now := s.now()
	for id, preview := range s.previews {
		if now.After(preview.expiresAt) {
			delete(s.previews, id)
		}
	}
}

func randomID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(data[:])
}

// sanitizeForLLM は最大50件をコピーし、原文・加盟店・口座・取込元の情報を除去する。
// コピー先だけを書き換え、ローカルの表示や判定に使う元データは保持する。
func sanitizeForLLM(items []domain.Transaction) []domain.Transaction {
	if len(items) > 50 {
		items = items[:50]
	}
	result := make([]domain.Transaction, len(items))
	copy(result, items)
	for index := range result {
		result[index].RawData = nil
		result[index].Description = ""
		result[index].NormalizedMerchant = ""
		result[index].SourceAccountName = ""
		result[index].Source = ""
		result[index].Fingerprint = ""
	}
	return result
}

func sanitizeReviewCandidates(
	items []domain.ReviewCandidate,
) []domain.ReviewCandidate {
	result := make([]domain.ReviewCandidate, len(items))
	copy(result, items)
	for index := range result {
		if slices.Contains(domain.Categories, result[index].Label) ||
			strings.Contains(result[index].Label, "高後悔") ||
			result[index].Label == "月前半の支出ペース" {
			continue
		}
		result[index].Label = "同一加盟店の支出パターン"
	}
	return result
}

func categoryStats(
	transactions []domain.Transaction,
	month string,
	category string,
) (int64, int) {
	var total int64
	var count int
	for _, transaction := range transactions {
		if strings.HasPrefix(transaction.TransactionDate, month) &&
			transaction.TransactionType == "expense" &&
			transaction.Category == category {
			total += transaction.Amount
			count++
		}
	}
	return total, count
}

func countExpenses(transactions []domain.Transaction, month string) int {
	count := 0
	for _, transaction := range transactions {
		if strings.HasPrefix(transaction.TransactionDate, month) &&
			transaction.TransactionType == "expense" {
			count++
		}
	}
	return count
}

func largestExpense(
	transactions []domain.Transaction,
	month string,
) *domain.Transaction {
	var largest *domain.Transaction
	for index := range transactions {
		transaction := &transactions[index]
		if !strings.HasPrefix(transaction.TransactionDate, month) ||
			transaction.TransactionType != "expense" {
			continue
		}
		if largest == nil || transaction.Amount > largest.Amount {
			copyValue := *transaction
			largest = &copyValue
		}
	}
	return largest
}

func pointerValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func yen(value int64) string {
	return fmt.Sprintf("%d円", value)
}
