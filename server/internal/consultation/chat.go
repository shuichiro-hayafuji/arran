// Package consultation owns chat advice, remembered information and conversation history.
package consultation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrConflict = errors.New("conversation changed; retry the request")
var ErrNotFound = errors.New("conversation not found")

// Fact retains provenance and history; only the latest active value enters model context.
type Fact struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Kind       string `json:"kind"`
	Period     string `json:"period"`
	Source     string `json:"source"`
	RecordedAt string `json:"recorded_at"`
	Active     bool   `json:"active"`
}
type State struct {
	Version    int64  `json:"version"`
	Facts      []Fact `json:"facts"`
	Strictness int    `json:"strictness"`
	CSVOffered bool   `json:"csv_offered"`
}
type Message struct {
	Role        string `json:"role"`
	Content     string `json:"content"`
	CreatedAt   string `json:"created_at"`
	Reason      string `json:"reason,omitempty"`
	Assumptions string `json:"assumptions,omitempty"`
}
type PendingPurchase struct {
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
	RecordedAt  string `json:"recorded_at"`
	Status      string `json:"status"`
}
type Thread struct {
	PendingPurchase *PendingPurchase `json:"pending_purchase,omitempty"`
	ID              int64            `json:"id"`
	Version         int64            `json:"version"`
	CreatedAt       string           `json:"created_at"`
	Messages        []Message        `json:"messages"`
	Reply           Reply            `json:"reply"`
	Source          string           `json:"source"`
}
type Change struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Kind      string `json:"kind"`
	Period    string `json:"period"`
	Operation string `json:"operation"`
}
type PurchaseChange struct {
	Operation   string  `json:"operation"`
	Description *string `json:"description"`
	Amount      *int64  `json:"amount"`
}
type Transaction struct {
	Date     string `json:"date"`
	Amount   int64  `json:"amount"`
	Category string `json:"category"`
}
type TransactionCoverage struct {
	Partial        bool   `json:"partial"`
	CandidateLimit int    `json:"candidate_limit"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
}
type HistoricalMessage struct {
	ConsultationID int64 `json:"consultation_id"`
	Message
}
type Reply struct {
	PurchaseChange *PurchaseChange `json:"purchase_change"`
	Status         string          `json:"status"`
	Answer         string          `json:"answer"`
	Reason         string          `json:"reason"`
	Assumptions    string          `json:"assumptions"`
	Question       string          `json:"question"`
	PlannedAmount  *int64          `json:"planned_amount"`
	Changes        []Change        `json:"changes"`
	Strictness     *int            `json:"strictness"`
}
type Input struct {
	PendingPurchase     *PendingPurchase    `json:"pending_purchase"`
	Transactions        []Transaction       `json:"transactions"`
	TransactionCoverage TransactionCoverage `json:"transaction_coverage"`
	RelatedHistory      []HistoricalMessage `json:"related_history"`
	Now                 string              `json:"now"`
	Message             string              `json:"message"`
	History             []Message           `json:"history"`
	Facts               []Fact              `json:"facts"`
	Strictness          int                 `json:"strictness"`
}
type Agent interface {
	Advise(context.Context, Input) (Reply, error)
}
type Repository interface {
	LoadChatTransactions(context.Context, int, string) ([]Transaction, TransactionCoverage, error)
	ListRecentChatThreads(context.Context, int64, int) ([]Thread, error)
	LoadChatState(context.Context) (State, error)
	LoadChatThread(context.Context, int64) (Thread, error)
	SaveChatTurn(context.Context, State, Thread) (Thread, error)
}
type Service struct {
	Repository        Repository
	Agent             Agent
	Now               func() time.Time
	UnavailableReason string
}

// Send keeps missing amounts missing and never substitutes a programmed financial verdict.
func (s Service) Send(ctx context.Context, id int64, message string) (Thread, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return Thread{}, fmt.Errorf("メッセージを入力してください")
	}
	if len(message) > 16000 {
		return Thread{}, fmt.Errorf("メッセージは16000バイト以内で入力してください")
	}
	state, err := s.Repository.LoadChatState(ctx)
	if err != nil {
		return Thread{}, err
	}
	if state.Strictness == 0 {
		state.Strictness = 5
	}
	thread := Thread{}
	if id != 0 {
		thread, err = s.Repository.LoadChatThread(ctx, id)
		if err != nil {
			return Thread{}, err
		}
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	timestamp := now.UTC().Format(time.RFC3339)
	if thread.CreatedAt == "" {
		thread.CreatedAt = timestamp
	}
	// Forgotten material is never replayed from the transcript. History includes only the last
	// turns; remembered financial information is supplied through active facts with provenance.
	input := Input{Now: now.In(time.FixedZone("Asia/Tokyo", 9*3600)).Format(time.RFC3339), Message: Redact(message), Strictness: state.Strictness}
	input.History = recentHistory(thread.Messages, state.Facts)
	input.Facts = relevantFacts(state.Facts, input.Message, input.History)
	input.Transactions, input.TransactionCoverage, err = s.Repository.LoadChatTransactions(ctx, 100, latestForgetAt(state.Facts))
	if err != nil {
		return Thread{}, err
	}
	pastThreads, err := s.Repository.ListRecentChatThreads(ctx, id, 20)
	if err != nil {
		return Thread{}, err
	}
	input.RelatedHistory = relatedHistory(pastThreads, input.Message, state.Facts)
	if thread.PendingPurchase != nil && thread.PendingPurchase.RecordedAt > latestForgetAt(state.Facts) {
		pending := *thread.PendingPurchase
		pending.Description = Redact(pending.Description)
		input.PendingPurchase = &pending
	}
	var reply Reply
	agentErr := errors.New("chat model unavailable")
	modelCtx, cancelModel := context.WithTimeout(ctx, 22*time.Second)
	defer cancelModel()
	if s.Agent != nil {
		for attempt := 0; attempt < 3; attempt++ {
			if modelCtx.Err() != nil {
				break
			}
			reply, agentErr = s.Agent.Advise(modelCtx, input)
			if agentErr == nil {
				agentErr = Validate(reply)
				if agentErr == nil && reply.PurchaseChange != nil && reply.PurchaseChange.Operation == "set" && input.PendingPurchase == nil && (reply.PurchaseChange.Description == nil || strings.TrimSpace(*reply.PurchaseChange.Description) == "") {
					agentErr = errors.New("new purchase requires a product")
				}
			}
			if agentErr == nil {
				break
			}
		}
	}
	thread.Source = "openai"
	if agentErr != nil {
		reply = Reply{Status: "unavailable", Answer: "今は助言を作成できません。相談内容は保存しました。同じ相談で再試行できます。"}
		thread.Source = "unavailable"
		if s.UnavailableReason != "" {
			reply.Answer = s.UnavailableReason
			thread.Source = "quota_fallback"
		}
	} else {
		for _, change := range reply.Changes {
			for i := range state.Facts {
				if state.Facts[i].Key == change.Key {
					state.Facts[i].Active = false
				}
			}
			state.Facts = append(state.Facts, Fact{Key: change.Key, Value: Redact(change.Value), Kind: change.Kind, Period: change.Period, Source: fmt.Sprintf("chat:%d:turn:%d", thread.ID, len(thread.Messages)/2+1), RecordedAt: timestamp, Active: change.Operation == "set"})
		}
		if reply.Strictness != nil {
			state.Strictness = *reply.Strictness
		}
		if !state.CSVOffered && (reply.Status == "answered" || reply.Status == "provisional") {
			reply.Answer += "\n\nより正確に相談したい場合は、任意でCSV明細を取り込めます。CSVがなくても相談できます。"
			state.CSVOffered = true
		}
	}
	if agentErr == nil && reply.PurchaseChange != nil {
		change := reply.PurchaseChange
		if change.Operation == "cancel" {
			thread.PendingPurchase = nil
		} else {
			description := ""
			if input.PendingPurchase != nil {
				description = input.PendingPurchase.Description
			}
			if change.Description != nil && strings.TrimSpace(*change.Description) != "" {
				description = Redact(strings.TrimSpace(*change.Description))
			}
			thread.PendingPurchase = &PendingPurchase{Description: description, Amount: *change.Amount, RecordedAt: timestamp, Status: "purchase_confirmation_pending"}
		}
	}
	reply.PlannedAmount = nil
	if thread.PendingPurchase != nil {
		reply.PlannedAmount = &thread.PendingPurchase.Amount
	}
	thread.Messages = append(thread.Messages, Message{Role: "user", Content: message, CreatedAt: timestamp}, Message{Role: "assistant", Content: reply.Answer, Reason: reply.Reason, Assumptions: reply.Assumptions, CreatedAt: timestamp})
	thread.Reply = reply
	// Store the consultation even when model calls exhaust the request's deadline.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return s.Repository.SaveChatTurn(saveCtx, state, thread)
}
func Validate(r Reply) error {
	switch r.Status {
	case "answered", "provisional", "needs_information", "unavailable":
	default:
		return errors.New("invalid advice status")
	}
	if len(r.Reason) > 16000 || len(r.Assumptions) > 16000 || len(r.Question) > 16000 {
		return errors.New("advice detail too long")
	}
	if strings.TrimSpace(r.Answer) == "" || len(r.Answer) > 16000 {
		return errors.New("invalid advice answer")
	}
	if r.Status == "needs_information" && strings.TrimSpace(r.Question) == "" {
		return errors.New("missing follow-up question")
	}
	if change := r.PurchaseChange; change != nil {
		if change.Operation != "set" && change.Operation != "cancel" {
			return errors.New("invalid purchase operation")
		}
		if change.Description != nil && len(*change.Description) > 2000 {
			return errors.New("purchase description too long")
		}
		if change.Operation == "set" && (change.Amount == nil || *change.Amount < 0) {
			return errors.New("invalid purchase amount")
		}
		if change.Operation == "cancel" && (change.Amount != nil || change.Description != nil) {
			return errors.New("cancel must not contain purchase details")
		}
	}
	if r.PlannedAmount != nil && *r.PlannedAmount < 0 {
		return errors.New("invalid planned amount")
	}
	if r.Strictness != nil && (*r.Strictness < 1 || *r.Strictness > 10) {
		return errors.New("invalid strictness")
	}
	if len(r.Changes) > 20 {
		return errors.New("too many fact changes")
	}
	for _, c := range r.Changes {
		if c.Key == "" || len(c.Key) > 120 || len(c.Value) > 2000 || len(c.Period) > 100 {
			return errors.New("invalid fact")
		}
		if c.Operation != "set" && c.Operation != "forget" {
			return errors.New("invalid fact operation")
		}
		if c.Operation == "forget" && c.Value != "" {
			return errors.New("forgotten values must be empty")
		}
		if c.Operation == "set" && strings.TrimSpace(c.Value) == "" {
			return errors.New("empty fact")
		}
		if c.Kind != "fact" && c.Kind != "inference" {
			return errors.New("invalid provenance kind")
		}
	}
	return nil
}
