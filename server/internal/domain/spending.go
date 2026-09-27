package domain

import (
	"slices"
	"strings"
)

// SpendingEvaluationInput は通信・DB・モデルに依存しない支出判定用の業務情報。
type SpendingEvaluationInput struct {
	Profile       Profile
	Dashboard     Dashboard
	Memories      []MemoryItem
	Message       string
	PlannedAmount *int64
	Category      string
}

// EvaluateSpending は金額・カテゴリ・verdict の最終的な判定規則。
// LLM はこの結果を説明するだけで、判定を変更しない。予定額 nil は情報不足と扱う。
func EvaluateSpending(input SpendingEvaluationInput) AdviceFacts {
	category := input.Category
	if !slices.Contains(Categories, category) {
		category = "その他"
	}
	facts := AdviceFacts{
		Category:            category,
		PlannedAmount:       input.PlannedAmount,
		FreeBudgetRemaining: input.Dashboard.FreeBudgetRemaining,
		CategoryTotal:       categoryTotal(input.Dashboard, category),
		CategoryIsReduced:   slices.Contains(input.Profile.ReduceCategories, category),
		CategoryIsAllowed:   slices.Contains(input.Profile.AllowedCategories, category),
	}
	if input.PlannedAmount == nil {
		facts.Verdict = "insufficient_data"
		return facts
	}
	facts.RemainingAfterPlanned = input.Dashboard.FreeBudgetRemaining - *input.PlannedAmount
	facts.BudgetExceeded = *input.PlannedAmount > input.Dashboard.FreeBudgetRemaining
	for _, memory := range input.Memories {
		if memory.Type == "regret_pattern" &&
			(strings.Contains(memory.Content, category) || strings.Contains(input.Message, "飲")) {
			facts.RegretPattern = memory.Content
			break
		}
	}
	switch {
	case facts.BudgetExceeded || facts.RegretPattern != "":
		facts.Verdict = "avoid"
	case facts.CategoryIsReduced && *input.PlannedAmount > max(input.Dashboard.FreeBudgetRemaining/5, 3000):
		facts.Verdict = "avoid"
	case input.Profile.AdviceStrictness == "厳しい" && *input.PlannedAmount > input.Dashboard.FreeBudgetRemaining/4:
		facts.Verdict = "caution"
	default:
		facts.Verdict = "safe"
	}
	return facts
}

func categoryTotal(dashboard Dashboard, category string) int64 {
	for _, item := range dashboard.ByCategory {
		if item.Category == category {
			return item.Amount
		}
	}
	return 0
}

func max(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
