package domain

import "time"

type Consultation struct {
	ID                 int64          `json:"id"`
	CreatedAt          string         `json:"created_at"`
	UserMessage        string         `json:"user_message"`
	PlannedAmount      *int64         `json:"planned_amount"`
	InferredCategory   string         `json:"inferred_category"`
	AIRecommendation   string         `json:"ai_recommendation"`
	AIReasoningSummary string         `json:"ai_reasoning_summary"`
	CurrentSituation   string         `json:"current_situation"`
	Alternative        string         `json:"alternative"`
	FinalQuestion      string         `json:"final_question"`
	BudgetSnapshot     BudgetSnapshot `json:"budget_snapshot"`
	Status             string         `json:"status"`
	ActualAmount       *int64         `json:"actual_amount"`
	UserDecisionReason string         `json:"user_decision_reason"`
	SatisfactionScore  *int           `json:"satisfaction_score"`
	RegretScore        *int           `json:"regret_score"`
	Note               string         `json:"note"`
	ResponseSource     string         `json:"response_source"`
	NeedsFollowUp      bool           `json:"needs_follow_up"`
	FollowUpQuestion   string         `json:"follow_up_question"`
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
