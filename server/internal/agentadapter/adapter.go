// Package agentadapter はアプリのドメイン型と独立した Agent の型を相互変換する。
// 金融判定はドメインに残し、Agent には説明生成に必要な情報だけを渡す。
package agentadapter

import (
	"context"
	"slices"

	"github.com/shuichiro-hayafuji/arran_agent"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)

type adapter struct{ agent agent.Agent }

func New(primary, fallback agent.Model, source string) *adapter {
	return &adapter{agent: agent.New(primary, fallback, source)}
}

// Run はカテゴリを確定してドメインで支出を判定し、その事実を Agent に説明させる。
func (a *adapter) Run(
	ctx context.Context,
	input domain.ConsultationContext,
	message string,
	plannedAmount *int64,
) (domain.AdviceDraft, string, error) {
	category, err := a.agent.InferCategory(ctx, message, domain.Categories)
	if err != nil || !slices.Contains(domain.Categories, category) {
		category = "その他"
	}
	input.AdviceFacts = domain.EvaluateSpending(domain.SpendingEvaluationInput{
		Profile: input.Profile, Dashboard: input.Dashboard, Memories: input.Memories,
		Message: message, PlannedAmount: plannedAmount, Category: category,
	})
	result, err := a.agent.Consult(ctx, consultationInput(input, message, plannedAmount))
	if err != nil {
		return domain.AdviceDraft{}, "", err
	}
	return adviceDraft(result.Draft), result.Source, nil
}

func (a *adapter) Review(
	ctx context.Context,
	profile domain.Profile,
	dashboard domain.Dashboard,
	transactions []domain.Transaction,
	consultations []domain.Consultation,
	local []domain.ReviewCandidate,
) ([]domain.ReviewCandidate, error) {
	items, err := a.agent.Review(ctx, agent.ReviewInput{
		Profile: profileValue(profile), Dashboard: dashboardValue(dashboard),
		Transactions: transactionValues(transactions), Consultations: consultationSummaries(consultations),
		LocalCandidates: reviewCandidates(local),
	})
	return domainReviewCandidates(items), err
}

func (a *adapter) ExtractMemory(ctx context.Context, item domain.Consultation) ([]domain.MemoryItem, error) {
	items, err := a.agent.ExtractMemory(ctx, agent.MemoryInput{
		Category: item.InferredCategory, UserDecisionReason: item.UserDecisionReason,
		SatisfactionScore: item.SatisfactionScore, RegretScore: item.RegretScore,
	})
	return domainMemories(items), err
}

func consultationInput(input domain.ConsultationContext, message string, plannedAmount *int64) agent.ConsultationInput {
	return agent.ConsultationInput{
		Now: input.Now, Profile: profileValue(input.Profile), Dashboard: dashboardValue(input.Dashboard),
		RecentConsultations: consultationSummaries(input.RecentConsultations), Memories: memoryValues(input.Memories),
		RelevantTransactions: transactionValues(input.RelevantTransactions), AdviceFacts: adviceFacts(input.AdviceFacts),
		Categories: domain.Categories, Message: message, PlannedAmount: plannedAmount,
	}
}
func profileValue(v domain.Profile) agent.Profile {
	return agent.Profile{MonthlyIncome: v.MonthlyIncome, CurrentBalance: v.CurrentBalance, MonthlyFixedCosts: v.MonthlyFixedCosts, MonthlyFreeBudget: v.MonthlyFreeBudget, MonthlySavingsGoal: v.MonthlySavingsGoal, ReduceCategories: v.ReduceCategories, AllowedCategories: v.AllowedCategories, LongTermGoal: v.LongTermGoal, AdviceStrictness: v.AdviceStrictness}
}
func dashboardValue(v domain.Dashboard) agent.Dashboard {
	return agent.Dashboard{Month: v.Month, ExpenseTotal: v.ExpenseTotal, FixedExpenseEstimate: v.FixedExpenseEstimate, FreeExpenseTotal: v.FreeExpenseTotal, MonthlyFreeBudget: v.MonthlyFreeBudget, FreeBudgetRemaining: v.FreeBudgetRemaining, DrinkingTotal: v.DrinkingTotal, SubscriptionTotal: v.SubscriptionTotal, ByCategory: categoryAmounts(v.ByCategory)}
}
func categoryAmounts(items []domain.CategoryAmount) []agent.CategoryAmount {
	result := make([]agent.CategoryAmount, len(items))
	for i, item := range items {
		result[i] = agent.CategoryAmount{Category: item.Category, Amount: item.Amount}
	}
	return result
}

// transactionValues は日付・金額・カテゴリだけを明示的に転記する。
// ドメイン型に項目が増えても、加盟店名やCSV原文を意図せず外部へ送らないための境界。
func transactionValues(items []domain.Transaction) []agent.Transaction {
	result := make([]agent.Transaction, len(items))
	for i, item := range items {
		result[i] = agent.Transaction{TransactionDate: item.TransactionDate, Amount: item.Amount, Category: item.Category}
	}
	return result
}
func consultationSummaries(items []domain.Consultation) []agent.ConsultationSummary {
	result := make([]agent.ConsultationSummary, len(items))
	for i, item := range items {
		result[i] = agent.ConsultationSummary{CreatedAt: item.CreatedAt, PlannedAmount: item.PlannedAmount, Category: item.InferredCategory, Status: item.Status, ActualAmount: item.ActualAmount, SatisfactionScore: item.SatisfactionScore, RegretScore: item.RegretScore}
	}
	return result
}
func memoryValues(items []domain.MemoryItem) []agent.MemoryItem {
	result := make([]agent.MemoryItem, len(items))
	for i, item := range items {
		result[i] = agent.MemoryItem{Type: item.Type, Content: item.Content, Evidence: item.Evidence, Confidence: item.Confidence}
	}
	return result
}
func adviceFacts(v domain.AdviceFacts) agent.AdviceFacts {
	return agent.AdviceFacts{Category: v.Category, PlannedAmount: v.PlannedAmount, FreeBudgetRemaining: v.FreeBudgetRemaining, RemainingAfterPlanned: v.RemainingAfterPlanned, CategoryTotal: v.CategoryTotal, CategoryIsReduced: v.CategoryIsReduced, CategoryIsAllowed: v.CategoryIsAllowed, BudgetExceeded: v.BudgetExceeded, RegretPattern: v.RegretPattern, Verdict: v.Verdict}
}
func adviceDraft(v agent.AdviceDraft) domain.AdviceDraft {
	return domain.AdviceDraft{Recommendation: v.Recommendation, Verdict: v.Verdict, ReasoningSummary: v.ReasoningSummary, CurrentSituation: v.CurrentSituation, Alternative: v.Alternative, FinalQuestion: v.FinalQuestion, InferredCategory: v.InferredCategory, NeedsFollowUp: v.NeedsFollowUp, FollowUpQuestion: v.FollowUpQuestion}
}
func reviewCandidates(items []domain.ReviewCandidate) []agent.ReviewCandidate {
	result := make([]agent.ReviewCandidate, len(items))
	for i, item := range items {
		result[i] = agent.ReviewCandidate{Label: item.Label, Judgement: item.Judgement, TotalAmount: item.TotalAmount, Count: item.Count, Reason: item.Reason, AnnualizedAmount: item.AnnualizedAmount, Question: item.Question}
	}
	return result
}
func domainReviewCandidates(items []agent.ReviewCandidate) []domain.ReviewCandidate {
	result := make([]domain.ReviewCandidate, len(items))
	for i, item := range items {
		result[i] = domain.ReviewCandidate{Label: item.Label, Judgement: item.Judgement, TotalAmount: item.TotalAmount, Count: item.Count, Reason: item.Reason, AnnualizedAmount: item.AnnualizedAmount, Question: item.Question}
	}
	return result
}
func domainMemories(items []agent.MemoryItem) []domain.MemoryItem {
	result := make([]domain.MemoryItem, len(items))
	for i, item := range items {
		result[i] = domain.MemoryItem{Type: item.Type, Content: item.Content, Evidence: item.Evidence, Confidence: item.Confidence}
	}
	return result
}
