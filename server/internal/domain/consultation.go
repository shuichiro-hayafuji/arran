package domain

import (
	"fmt"
	"time"
)

type PendingPurchase struct {
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	RecordedAt  string `json:"recorded_at"`
	Status      string `json:"status"`
}

type PurchaseEvent struct {
	BalanceAppliedAmount    *int64 `json:"balance_applied_amount"`
	FreeBudgetAppliedAmount *int64 `json:"free_budget_applied_amount"`
	BalanceBasis            string `json:"balance_basis"`
	Outcome                 string `json:"outcome"`
	Amount                  *int64 `json:"amount"`
	Classification          string `json:"classification"`
	RecordedAt              string `json:"recorded_at"`
	Source                  string `json:"source"`
}
type Purchase struct {
	ActualAmountBasis        string          `json:"actual_amount_basis"`
	PurchaseDate             string          `json:"purchase_date"`
	CSVReconciliationPending bool            `json:"csv_reconciliation_pending"`
	BalanceAppliedAmount     *int64          `json:"balance_applied_amount"`
	FreeBudgetAppliedAmount  *int64          `json:"free_budget_applied_amount"`
	ID                       string          `json:"id"`
	ConsultationID           int64           `json:"consultation_id"`
	Description              string          `json:"description"`
	Amount                   int64           `json:"amount"`
	RecordedAt               string          `json:"recorded_at"`
	Status                   string          `json:"status"`
	FirstAskedAt             string          `json:"first_asked_at"`
	SecondAskedAt            string          `json:"second_asked_at"`
	ActualAmount             *int64          `json:"actual_amount"`
	Classification           string          `json:"classification"`
	Events                   []PurchaseEvent `json:"events"`
}
type ChatMessage struct {
	Reason      string `json:"reason,omitempty"`
	Assumptions string `json:"assumptions,omitempty"`
	Role        string `json:"role"`
	Content     string `json:"content"`
	CreatedAt   string `json:"created_at"`
}

type Consultation struct {
	Purchases          []Purchase       `json:"purchases"`
	PendingPurchase    *PendingPurchase `json:"pending_purchase,omitempty"`
	Messages           []ChatMessage    `json:"messages"`
	ID                 int64            `json:"id"`
	CreatedAt          string           `json:"created_at"`
	UserMessage        string           `json:"user_message"`
	PlannedAmount      *int64           `json:"planned_amount"`
	InferredCategory   string           `json:"inferred_category"`
	AIRecommendation   string           `json:"ai_recommendation"`
	AIReasoningSummary string           `json:"ai_reasoning_summary"`
	CurrentSituation   string           `json:"current_situation"`
	Alternative        string           `json:"alternative"`
	FinalQuestion      string           `json:"final_question"`
	BudgetSnapshot     BudgetSnapshot   `json:"budget_snapshot"`
	Status             string           `json:"status"`
	ActualAmount       *int64           `json:"actual_amount"`
	UserDecisionReason string           `json:"user_decision_reason"`
	SatisfactionScore  *int             `json:"satisfaction_score"`
	RegretScore        *int             `json:"regret_score"`
	Note               string           `json:"note"`
	ResponseSource     string           `json:"response_source"`
	NeedsFollowUp      bool             `json:"needs_follow_up"`
	FollowUpQuestion   string           `json:"follow_up_question"`
}

type ConsultationResultUpdate struct {
	Status             string `json:"status"`
	ActualAmount       *int64 `json:"actual_amount"`
	UserDecisionReason string `json:"user_decision_reason"`
	SatisfactionScore  *int   `json:"satisfaction_score"`
	RegretScore        *int   `json:"regret_score"`
	Note               string `json:"note"`
}

type ConsultationContext struct {
	Now                  time.Time      `json:"-"`
	Profile              Profile        `json:"profile"`
	Dashboard            Dashboard      `json:"dashboard"`
	RecentConsultations  []Consultation `json:"recent_consultations"`
	Memories             []MemoryItem   `json:"memories"`
	RelevantTransactions []Transaction  `json:"relevant_transactions"`
	AdviceFacts          AdviceFacts    `json:"advice_facts"`
}

// ConsultationUsage は利用者の対象月における新規相談数と有効な上限を表す。
type ConsultationUsage struct {
	Month string
	Count int
	Limit int
}

func (u ConsultationUsage) UseExternalModel() bool {
	return u.Count <= u.Limit
}

type MonthlyLimitEvent struct {
	Environment string
	UserID      int64
	Month       string
	Count       int
	Limit       int
	OccurredAt  time.Time
}

func (e MonthlyLimitEvent) DedupKey() string {
	return fmt.Sprintf("arran-monthly-consultation-limit-%s-%d-%s", e.Environment, e.UserID, e.Month)
}
