package domain

type Profile struct {
	MonthlyIncome      int64    `json:"monthly_income"`
	CurrentBalance     int64    `json:"current_balance"`
	MonthlyFixedCosts  int64    `json:"monthly_fixed_costs"`
	MonthlyFreeBudget  int64    `json:"monthly_free_budget"`
	MonthlySavingsGoal int64    `json:"monthly_savings_goal"`
	ReduceCategories   []string `json:"reduce_categories"`
	AllowedCategories  []string `json:"allowed_categories"`
	LongTermGoal       string   `json:"long_term_goal"`
	AdviceStrictness   string   `json:"advice_strictness"`
	UpdatedAt          string   `json:"updated_at"`
}
