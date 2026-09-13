package domain

type CategoryAmount struct {
	Category string `json:"category"`
	Amount   int64  `json:"amount"`
}

type Dashboard struct {
	Month                string           `json:"month"`
	ExpenseTotal         int64            `json:"expense_total"`
	FixedExpenseEstimate int64            `json:"fixed_expense_estimate"`
	FreeExpenseTotal     int64            `json:"free_expense_total"`
	MonthlyFreeBudget    int64            `json:"monthly_free_budget"`
	FreeBudgetRemaining  int64            `json:"free_budget_remaining"`
	DrinkingTotal        int64            `json:"drinking_total"`
	SubscriptionTotal    int64            `json:"subscription_total"`
	ByCategory           []CategoryAmount `json:"by_category"`
	RecentTransactions   []Transaction    `json:"recent_transactions"`
}

type BudgetSnapshot struct {
	Month               string           `json:"month"`
	ExpenseTotal        int64            `json:"expense_total"`
	FreeBudgetRemaining int64            `json:"free_budget_remaining"`
	DrinkingTotal       int64            `json:"drinking_total"`
	SubscriptionTotal   int64            `json:"subscription_total"`
	ByCategory          []CategoryAmount `json:"by_category"`
}
