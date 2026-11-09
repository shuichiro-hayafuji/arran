package consultation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Purchase is owned by the consultation module. Events retain every posted outcome.
type Purchase struct {
	ActualAmountBasis        string          `json:"actual_amount_basis"`
	CSVReconciliationPending bool            `json:"csv_reconciliation_pending"`
	PurchaseDate             string          `json:"purchase_date"`
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
	AccountingTracked        bool            `json:"accounting_tracked"`
	BalanceAppliedAmount     *int64          `json:"balance_applied_amount"`
	FreeBudgetAppliedAmount  *int64          `json:"free_budget_applied_amount"`
	Events                   []PurchaseEvent `json:"events"`
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
type PurchaseUpdate struct {
	AmountBasis    string  `json:"amount_basis"`
	PurchaseDate   *string `json:"purchase_date"`
	ID             string  `json:"id"`
	Outcome        string  `json:"outcome"`
	Amount         *int64  `json:"amount"`
	Classification string  `json:"classification"`
}

func pending(p Purchase) bool {
	return p.CSVReconciliationPending || p.Status == "purchase_confirmation_pending" || (p.Status == "needs_classification" || p.Status == "classification_required")
}

// LegacyPurchases imports old single-purchase threads once, including before any UI use.
func LegacyPurchases(state *State, threads []Thread) {
	if state.PurchasesMigrated {
		return
	}
	for _, t := range threads {
		if p := t.PendingPurchase; p != nil {
			state.Purchases = append(state.Purchases, Purchase{ID: fmt.Sprintf("legacy:%d", t.ID), ConsultationID: t.ID, Description: p.Description, Amount: p.Amount, RecordedAt: p.RecordedAt, Status: p.Status})
		}
	}
	state.PurchasesMigrated = true
	state.purchasesHydrated = true
}

type PurchaseCoverage struct {
	Partial      bool `json:"partial"`
	PendingLimit int  `json:"pending_limit"`
	HistoryLimit int  `json:"history_limit"`
	EventLimit   int  `json:"event_limit"`
	ByteLimit    int  `json:"byte_limit"`
	Included     int  `json:"included"`
	Omitted      int  `json:"omitted"`
}

func modelPurchases(state State, message string, currentID int64, forced ...string) ([]Purchase, PurchaseCoverage) {
	coverage := PurchaseCoverage{PendingLimit: 20, HistoryLimit: 6, EventLimit: 6, ByteLimit: 16000}
	type candidate struct {
		p    Purchase
		rank int
	}
	var candidates []candidate
	cutoff := latestForgetAt(state.Facts)
	for _, p := range state.Purchases {
		if p.RecordedAt <= cutoff {
			continue
		}
		rank := 0
		for _, id := range forced {
			if p.ID == id {
				rank += 100
			}
		}
		if pending(p) {
			rank += 20
		}
		if currentID != 0 && p.ConsultationID == currentID {
			rank += 10
		}
		common := false
		for token := range tokens(p.Description) {
			if tokens(message)[token] {
				common = true
				break
			}
		}
		if common {
			rank += 40
		}
		if rank == 0 {
			coverage.Omitted++
			continue
		}
		candidates = append(candidates, candidate{p, rank})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].rank > candidates[j].rank })
	var result []Purchase
	bytes, pendingCount, historyCount := 2, 0, 0
	for _, c := range candidates {
		p := c.p
		p.BalanceAppliedAmount = balanceApplied(p)
		p.FreeBudgetAppliedAmount = budgetApplied(p)
		p.AccountingTracked = true
		p.ActualAmountBasis = ActualAmountProvenance(p)
		if (pending(p) && pendingCount == coverage.PendingLimit) || (!pending(p) && historyCount == coverage.HistoryLimit) {
			coverage.Omitted++
			continue
		}
		p.Description = Redact(p.Description)
		events := p.Events
		if len(events) > coverage.EventLimit {
			// Include the most recent posted outcome even after repeated classification questions.
			previous := -1
			for i := len(events) - 1; i >= 0; i-- {
				if events[i].BalanceAppliedAmount != nil || events[i].Outcome != "needs_classification" {
					previous = i
					break
				}
			}
			start := len(events) - coverage.EventLimit
			if previous >= 0 && previous < start {
				events = append([]PurchaseEvent{events[previous]}, events[start+1:]...)
			} else {
				events = events[start:]
			}
		}
		p.Events = append([]PurchaseEvent(nil), events...)
		for i := range p.Events {
			p.Events[i].Source = Redact(p.Events[i].Source)
		}
		encoded, _ := json.Marshal(p)
		if bytes+len(encoded)+1 > coverage.ByteLimit {
			coverage.Omitted++
			continue
		}
		bytes += len(encoded) + 1
		result = append(result, p)
		if pending(p) {
			pendingCount++
		} else {
			historyCount++
		}
	}
	coverage.Included = len(result)
	coverage.Partial = coverage.Omitted > 0
	return result, coverage
}

func validatePurchases(r Reply, in Input) error {
	if r.PurchaseTargetUnclear && (r.Question == "" || len(r.PurchaseUpdates) > 0 || r.PurchaseChange != nil) {
		return errors.New("unclear target requires a question without purchase mutations")
	}
	for _, c := range r.Changes {
		if !financialKey(c.Key) || c.Operation != "set" || c.Basis == "user_report" || unchangedFinancialFact(in.Facts, c) {
			continue
		}
		if r.PurchaseTargetUnclear || (c.Basis == "purchase" && len(r.PurchaseUpdates) == 0) || (c.Basis == "memory" && r.PurchaseChange != nil) {
			return errors.New("purchase-derived financial changes require an identified purchase outcome; explicit user reports remain separate")
		}
	}
	balanceNeeded, budgetNeeded := false, false
	for _, u := range r.PurchaseUpdates {
		if u.AmountBasis != "" && u.AmountBasis != "reported" && u.AmountBasis != "estimated" && u.AmountBasis != "unchanged" {
			return errors.New("invalid actual amount basis")
		}
		if u.PurchaseDate != nil && *u.PurchaseDate != "" {
			if _, err := time.Parse("2006-01-02", *u.PurchaseDate); err != nil {
				return errors.New("invalid explicit purchase date")
			}
		}
		for _, p := range in.Purchases {
			if p.ID == u.ID && u.AmountBasis == "unchanged" && !sameAmount(p.ActualAmount, u.Amount) {
				return errors.New("unchanged actual amount differs from saved actual")
			}
			if p.ID == u.ID && u.Outcome == "unanswered" && u.AmountBasis != "" && u.AmountBasis != "estimated" {
				return errors.New("unanswered actual amount must remain estimated")
			}
			if p.ID == u.ID && u.Outcome != "classification_required" && !containsPurchaseID(in.ReconciliationRequiredIDs, u.ID) {
				if u.Outcome != "needs_classification" && (u.Classification != "unknown" || u.Outcome == "not_purchased") {
					var target *int64
					if u.Classification == "free" && u.Outcome != "not_purchased" {
						target = u.Amount
					}
					applied := budgetApplied(p)
					if (target == nil) != (applied == nil) || (target != nil && applied != nil && *target != *applied) {
						budgetNeeded = true
					}
				}
				applied := balanceApplied(p)
				if (u.Amount != nil && (applied == nil || *applied != *u.Amount)) || (u.Outcome == "not_purchased" && applied != nil) {
					balanceNeeded = true
				}
			}
		}
	}

	visible := map[string]Purchase{}
	for _, p := range in.Purchases {
		visible[p.ID] = p
	}
	if c := r.PurchaseChange; c != nil && c.ID != "" {
		p, ok := visible[c.ID]
		if !ok || !pending(p) || p.ActualAmount != nil {
			return errors.New("unknown pending purchase")
		}
	}
	if c := r.PurchaseChange; c != nil {
		if c.Operation == "add" && c.ID != "" {
			return errors.New("new purchase must not reuse an ID")
		}
		matches := 0
		if c.ID == "" && c.Operation != "add" {
			for _, p := range in.Purchases {
				if pending(p) && in.PendingPurchase != nil {
					matches++
				}
			}
		}
		if c.ID == "" && c.Operation != "add" && in.PendingPurchase != nil && in.PendingPurchase.Status == "needs_classification" {
			return errors.New("known payment must use a purchase outcome correction")
		}
		if matches > 1 {
			return errors.New("purchase change requires an unambiguous ID")
		}
		if c.Operation != "cancel" && (c.Description == nil || strings.TrimSpace(*c.Description) == "") && c.ID == "" && (c.Operation == "add" || in.PendingPurchase == nil) {
			return errors.New("new purchase requires a product")
		}
	}
	if len(r.PurchaseUpdates) > 100 {
		return errors.New("too many purchase updates")
	}
	seen := map[string]bool{}
	for _, u := range r.PurchaseUpdates {
		p, ok := visible[u.ID]
		if !ok || seen[u.ID] || (r.PurchaseChange != nil && r.PurchaseChange.ID == u.ID) {
			return errors.New("unknown or duplicate purchase outcome")
		}
		if samePostedOutcome(p, u) {
			return errors.New("purchase result already recorded; omit duplicate update and do not recalculate")
		}
		seen[u.ID] = true
		if u.Amount != nil && *u.Amount < 0 {
			return errors.New("negative actual amount")
		}
		if u.Classification != "free" && u.Classification != "necessary" && u.Classification != "unknown" {
			return errors.New("invalid purchase classification")
		}
		switch u.Outcome {
		case "purchased", "unanswered":
			if u.Amount == nil || (u.Outcome == "purchased" && u.Classification != "free" && u.Classification != "necessary") {
				return errors.New("purchase requires amount and classification")
			}
			if u.Outcome == "unanswered" {
				if u.Classification == "unknown" && r.Question == "" {
					return errors.New("assumed purchase requires a classification question")
				}
				if !pending(p) || p.ActualAmount != nil || p.SecondAskedAt == "" || *u.Amount != p.Amount {
					return errors.New("purchase cannot be assumed yet")
				}
			}
		case "not_purchased":
			if u.Amount != nil {
				return errors.New("unbought purchase must have no payment")
			}
		case "classification_required":
			for _, id := range in.AutoConfirmIDs {
				if id == u.ID {
					return errors.New("automatic purchase must apply the planned amount to current balance even before classification")
				}
			}
			if p.ActualAmount != nil || u.Amount != nil || u.Classification != "unknown" || r.Question == "" {
				return errors.New("unknown planned classification cannot contain an actual payment")
			}
		case "needs_classification":
			if u.Amount == nil || u.Classification != "unknown" || r.Question == "" {
				return errors.New("classification question required")
			}
		default:
			return errors.New("invalid purchase outcome")
		}
		// The model computes money. This validates which bookkeeping operation is
		// needed and which amount was already applied, without computing a balance.
		if containsPurchaseID(in.ReconciliationRequiredIDs, u.ID) && u.Amount != nil {
			continue
		}
		for _, f := range in.Facts {
			if f.Key != "current_balance" && f.Key != "monthly_free_budget" {
				continue
			}
			needed := false
			if f.Key == "current_balance" {
				needed = balanceNeeded
			} else if u.Outcome != "needs_classification" && u.Outcome != "classification_required" && (u.Classification != "unknown" || u.Outcome == "not_purchased") {
				needed = budgetNeeded
			}
			if u.Outcome == "not_purchased" && balanceApplied(p) == nil && budgetApplied(p) == nil {
				needed = false
			}
			found := false
			for _, c := range r.Changes {
				if c.Key != f.Key || c.Operation != "set" {
					continue
				}
				found = true
				if ((f.Key == "current_balance" && !balanceNeeded) || (f.Key == "monthly_free_budget" && !budgetNeeded)) && (c.Value != f.Value || c.Period != f.Period) && c.Basis != "user_report" {
					return errors.New("payment already applied; only an explicit user balance correction can change the balance")
				}
			}
			if needed && !found {
				return errors.New("model must recalculate known financial facts")
			}
		}

	}
	for _, id := range in.AutoConfirmIDs {
		if !seen[id] && !r.PurchaseTargetUnclear {
			return errors.New("second unanswered confirmation requires an explicit model outcome")
		}
	}
	if c := r.PurchaseChange; c != nil && c.ID == "" && c.Operation == "cancel" && len(in.Purchases) > 1 {
		return errors.New("ambiguous purchase cancellation")
	}
	return nil
}

func financialKey(key string) bool {
	return key == "current_balance" || key == "monthly_free_budget"
}

func unchangedFinancialFact(facts []Fact, change Change) bool {
	for _, fact := range facts {
		if fact.Active && fact.Key == change.Key && fact.Value == change.Value && fact.Period == change.Period {
			return true
		}
	}
	return false
}

func applyPurchases(state *State, thread *Thread, r Reply, in Input, timestamp string) {
	if c := r.PurchaseChange; c != nil {
		index := -1
		for i, p := range state.Purchases {
			if p.ID == c.ID && c.ID != "" {
				index = i
			}
		}
		// Compatibility for amount-only supplementation of an old single pending item.
		if index < 0 && c.ID == "" && c.Operation != "add" {
			for _, v := range in.Purchases {
				if v.ConsultationID == thread.ID && pending(v) {
					for i, p := range state.Purchases {
						if p.ID == v.ID {
							index = i
						}
					}
				}
			}
		}
		if c.Operation == "cancel" {
			if index >= 0 {
				state.Purchases[index].Status = "cancelled"
			}
		} else {
			description := ""
			if index >= 0 {
				description = state.Purchases[index].Description
			} else if in.PendingPurchase != nil {
				description = in.PendingPurchase.Description
			}
			if c.Description != nil && strings.TrimSpace(*c.Description) != "" {
				description = Redact(strings.TrimSpace(*c.Description))
			}
			if index < 0 {
				var b [16]byte
				if _, err := rand.Read(b[:]); err != nil {
					panic(err)
				}
				state.Purchases = append(state.Purchases, Purchase{ID: hex.EncodeToString(b[:]), ConsultationID: thread.ID})
				index = len(state.Purchases) - 1
			}
			p := &state.Purchases[index]
			p.Description = description
			p.Amount = *c.Amount
			p.RecordedAt = timestamp
			p.Status = "purchase_confirmation_pending"
			p.FirstAskedAt = ""
			p.SecondAskedAt = ""
		}
	}
	for _, u := range r.PurchaseUpdates {
		for i := range state.Purchases {
			p := &state.Purchases[i]
			if p.ID != u.ID {
				continue
			}
			p.ActualAmountBasis = ActualAmountProvenance(*p)
			appliedBalance, appliedBudget := balanceApplied(*p), budgetApplied(*p)
			basis := ""
			for _, c := range r.Changes {
				if containsPurchaseID(in.ReconciliationRequiredIDs, u.ID) && u.Amount != nil {
					continue
				}
				if c.Operation != "set" {
					continue
				}
				if c.Key == "current_balance" && u.Outcome != "classification_required" {
					appliedBalance = u.Amount
					basis = c.Basis
				}
				if c.Key == "monthly_free_budget" && u.Outcome != "needs_classification" && u.Outcome != "classification_required" && (u.Classification != "unknown" || u.Outcome == "not_purchased") {
					if u.Classification == "free" && u.Outcome != "not_purchased" {
						appliedBudget = u.Amount
					} else {
						appliedBudget = nil
					}
				}
			}
			p.AccountingTracked = true
			p.BalanceAppliedAmount = appliedBalance
			p.FreeBudgetAppliedAmount = appliedBudget
			p.Events = append(p.Events, PurchaseEvent{Outcome: u.Outcome, Amount: u.Amount, Classification: u.Classification, RecordedAt: timestamp, Source: fmt.Sprintf("chat:%d", thread.ID), BalanceAppliedAmount: appliedBalance, FreeBudgetAppliedAmount: appliedBudget, BalanceBasis: basis})
			if u.Outcome != "classification_required" {
				if u.AmountBasis != "unchanged" {
					if u.AmountBasis != "" {
						p.ActualAmountBasis = u.AmountBasis
					} else if u.Outcome == "unanswered" {
						p.ActualAmountBasis = "estimated"
					} else {
						p.ActualAmountBasis = "reported"
					}
				}
				p.ActualAmount = u.Amount
			}
			if u.PurchaseDate != nil {
				p.PurchaseDate = *u.PurchaseDate
			}
			p.Classification = u.Classification
			p.Status = u.Outcome
			if u.Outcome == "unanswered" && u.Classification == "unknown" {
				p.Status = "needs_classification"
			}
		}
	}
}

func confirmation(items []Purchase, number int) string {
	var lines []string
	for _, p := range items {
		lines = append(lines, fmt.Sprintf("・%s（予定%d円、相談日時%s、購入ID %s）", p.Description, p.Amount, p.RecordedAt, p.ID))
	}
	return fmt.Sprintf("購入の確認（%d回目）です。次の予定は購入しましたか？購入済みなら実際の支払額か「予定どおり」、未購入ならその旨を教えてください。\n%s", number, strings.Join(lines, "\n"))
}

func secondConfirmation(state *State, in Input, timestamp string) string {
	visible := map[string]bool{}
	for _, p := range in.Purchases {
		visible[p.ID] = true
	}
	var items []Purchase
	for i := range state.Purchases {
		p := &state.Purchases[i]
		if visible[p.ID] && unconfirmed(*p) && p.FirstAskedAt != "" && p.SecondAskedAt == "" {
			p.SecondAskedAt = timestamp
			items = append(items, *p)
		}
	}
	if len(items) == 0 {
		return ""
	}
	return confirmation(items, 2)
}

// ProjectPurchases updates the original transcript idempotently within the save transaction.
func ProjectPurchases(state State, thread *Thread) {
	old := map[string]Purchase{}
	for _, p := range thread.Purchases {
		old[p.ID] = p
	}
	thread.Purchases = nil
	thread.PendingPurchase = nil
	var first []Purchase
	for _, p := range state.Purchases {
		if p.ConsultationID != thread.ID {
			continue
		}
		thread.Purchases = append(thread.Purchases, p)
		if pending(p) {
			thread.PendingPurchase = &PendingPurchase{Description: p.Description, Amount: p.Amount, RecordedAt: p.RecordedAt, Status: p.Status}
		}
		if p.FirstAskedAt != "" && old[p.ID].FirstAskedAt == "" {
			first = append(first, p)
		}
	}
	if len(first) > 0 {
		thread.Messages = append(thread.Messages, Message{Role: "assistant", Content: confirmation(first, 1), CreatedAt: first[0].FirstAskedAt})
	}
}

// DeliverDue does not call any model or post a financial outcome. It is safe to retry.
func (s Service) DeliverDue(ctx context.Context) error {
	state, err := s.Repository.LoadChatState(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	timestamp := now.UTC().Format(time.RFC3339)
	var id int64
	for i := range state.Purchases {
		p := &state.Purchases[i]
		at, err := time.Parse(time.RFC3339, p.RecordedAt)
		if err != nil || !unconfirmed(*p) || p.FirstAskedAt != "" || now.Before(at.Add(6*time.Hour)) || p.RecordedAt <= latestForgetAt(state.Facts) {
			continue
		}
		p.FirstAskedAt = timestamp
		id = p.ConsultationID
	}
	if id == 0 {
		// Persist the one-time bridge even if forgotten legacy plans are not deliverable.
		if state.purchasesHydrated && len(state.Purchases) > 0 {
			thread, err := s.Repository.LoadChatThread(ctx, state.Purchases[0].ConsultationID)
			if err != nil {
				return err
			}
			_, err = s.Repository.SaveChatTurn(ctx, state, thread)
			return err
		}
		return nil
	}
	thread, err := s.Repository.LoadChatThread(ctx, id)
	if err != nil {
		return err
	}
	_, err = s.Repository.SaveChatTurn(ctx, state, thread)
	return err
}

// ReminderRepository exposes only owner IDs with due work; the composition root
// supplies the authenticated owner context used for every read and save.
type ReminderRepository interface {
	DuePurchaseOwners(context.Context, time.Time) ([]int64, error)
}
type ReminderWorker struct {
	Repository ReminderRepository
	Deliver    func(context.Context, int64) error
	OnError    func(error)
}

func (w ReminderWorker) Run(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		w.Tick(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func (w ReminderWorker) Tick(ctx context.Context, now time.Time) {
	ids, err := w.Repository.DuePurchaseOwners(ctx, now)
	if err == nil {
		for _, id := range ids {
			if ctx.Err() != nil {
				return
			}
			if e := w.Deliver(ctx, id); e != nil && w.OnError != nil {
				w.OnError(e)
			}
		}
	} else if w.OnError != nil {
		w.OnError(err)
	}
}

func financialFacts(all, selected []Fact) []Fact {
	for _, f := range all {
		if !f.Active || (f.Key != "current_balance" && f.Key != "monthly_free_budget") {
			continue
		}
		found := false
		for _, s := range selected {
			if s.Key == f.Key {
				found = true
			}
		}
		if !found {
			if len(selected) >= 12 {
				selected = selected[:11]
			}
			f.Key = Redact(f.Key)
			f.Value = Redact(f.Value)
			f.Period = redactPeriod(f.Period)
			f.Source = Redact(f.Source)
			selected = append([]Fact{f}, selected...)
		}
	}
	return selected
}

func postedClassification(p Purchase) string {
	for i := len(p.Events) - 1; i >= 0; i-- {
		e := p.Events[i]
		if e.Outcome != "needs_classification" && e.Outcome != "classification_required" {
			return e.Classification
		}
	}
	return ""
}
func postedAmount(p Purchase) *int64 {
	for i := len(p.Events) - 1; i >= 0; i-- {
		e := p.Events[i]
		if e.Outcome != "needs_classification" && e.Outcome != "classification_required" {
			return e.Amount
		}
	}
	return nil
}

func samePostedOutcome(p Purchase, u PurchaseUpdate) bool {
	if (u.PurchaseDate != nil && *u.PurchaseDate != p.PurchaseDate) || (u.AmountBasis != "" && u.AmountBasis != "unchanged" && u.AmountBasis != p.ActualAmountBasis) {
		return false
	}
	if p.CSVReconciliationPending && p.BalanceAppliedAmount == nil {
		return false
	}
	if u.Outcome == "needs_classification" || u.Outcome == "classification_required" || p.Status == "needs_classification" || p.Status == "classification_required" {
		return false
	}
	for i := len(p.Events) - 1; i >= 0; i-- {
		e := p.Events[i]
		if e.Outcome == "needs_classification" || e.Outcome == "classification_required" {
			continue
		}
		paid := func(outcome string) bool { return outcome == "purchased" || outcome == "unanswered" }
		outcomeSame := e.Outcome == u.Outcome || (paid(e.Outcome) && paid(u.Outcome))
		amountSame := (e.Amount == nil && u.Amount == nil) || (e.Amount != nil && u.Amount != nil && *e.Amount == *u.Amount)
		return outcomeSame && amountSame && e.Classification == u.Classification
	}
	return false
}

func balanceApplied(p Purchase) *int64 {
	if p.AccountingTracked {
		return p.BalanceAppliedAmount
	}
	return postedAmount(p)
}
func budgetApplied(p Purchase) *int64 {
	if p.AccountingTracked {
		return p.FreeBudgetAppliedAmount
	}
	if postedClassification(p) == "free" {
		return postedAmount(p)
	}
	return nil
}

func unconfirmed(p Purchase) bool {
	return p.ActualAmount == nil && (p.Status == "purchase_confirmation_pending" || p.Status == "classification_required")
}

func containsPurchaseID(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// ActualAmountProvenance retains legacy unanswered estimates through classification-only
// events. Only a changed amount or explicit reported basis establishes new provenance.
func ActualAmountProvenance(p Purchase) string {
	if p.ActualAmountBasis != "" {
		return p.ActualAmountBasis
	}
	if p.ActualAmount == nil {
		return ""
	}
	basis := "reported"
	var previous *int64
	for _, event := range p.Events {
		if event.Amount == nil {
			continue
		}
		if event.Outcome == "unanswered" {
			basis = "estimated"
		} else if previous == nil || !sameAmount(previous, event.Amount) {
			basis = "reported"
		}
		previous = event.Amount
	}
	if previous != nil && !sameAmount(previous, p.ActualAmount) {
		return "reported"
	}
	return basis
}
