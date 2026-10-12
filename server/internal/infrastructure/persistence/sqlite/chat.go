package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"strconv"
	"strings"
	"time"
)

func chatQuery(ctx context.Context, query string) string {
	query = strings.ReplaceAll(query, ":user_id", strconv.FormatInt(identity.UserID(ctx), 10))
	return query
}
func (r *Repository) LoadChatState(ctx context.Context) (consultation.State, error) {
	var payload string
	err := r.db.QueryRowContext(ctx, chatQuery(ctx, "SELECT payload FROM chat_states WHERE user_id=:user_id")).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		state, err := r.initialChatState(ctx)
		if err == nil {
			err = r.hydratePurchases(ctx, &state)
		}
		return state, err
	}
	if err != nil {
		return consultation.State{}, err
	}
	var state consultation.State
	err = json.Unmarshal([]byte(payload), &state)
	if err == nil {
		err = r.hydratePurchases(ctx, &state)
	}
	return state, err
}
func (r *Repository) LoadChatThread(ctx context.Context, id int64) (consultation.Thread, error) {
	var payload string
	err := r.db.QueryRowContext(ctx, chatQuery(ctx, "SELECT payload FROM chat_threads WHERE user_id=:user_id AND consultation_id=?"), id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return consultation.Thread{}, consultation.ErrNotFound
	}
	if err != nil {
		return consultation.Thread{}, err
	}
	var thread consultation.Thread
	err = json.Unmarshal([]byte(payload), &thread)
	return thread, err
}

// SaveChatTurn commits the thread, summary and remembered facts together. Optimistic
// versions prevent overlapping model responses from overwriting newer corrections.
func (r *Repository) SaveChatTurn(ctx context.Context, state consultation.State, thread consultation.Thread) (consultation.Thread, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return consultation.Thread{}, err
	}
	defer tx.Rollback()
	query := func(q string) string { return chatQuery(ctx, q) }
	requestRef, hasRequest := consultation.RequestReferenceFrom(ctx)
	requestCancelled := false
	var requestExpiry int64
	if hasRequest {
		var status string
		var expires int64
		if err = tx.QueryRowContext(ctx, query("SELECT status,expires_at,cancel_requested FROM chat_requests WHERE user_id=:user_id AND request_id=?"), requestRef.ID).Scan(&status, &expires, &requestCancelled); err != nil {
			return consultation.Thread{}, err
		}
		requestExpiry = expires
		if status != "processing" || time.Now().UnixMilli() >= expires {
			return consultation.Thread{}, consultation.ErrRequestFinished
		}
		if requestCancelled && !requestRef.AllowCancelled {
			return consultation.Thread{}, consultation.ErrRequestCancelled
		}
		if requestRef.AllowCancelled && thread.Reply.Status != "unavailable" {
			return consultation.Thread{}, consultation.ErrRequestCancelled
		}
	}

	if _, err = tx.ExecContext(ctx, query("INSERT INTO chat_states(user_id,version,payload) VALUES(:user_id,0,'{}') ON CONFLICT(user_id) DO NOTHING")); err != nil {
		return consultation.Thread{}, err
	}
	var version int64
	if err = tx.QueryRowContext(ctx, query("SELECT version FROM chat_states WHERE user_id=:user_id")).Scan(&version); err != nil {
		return consultation.Thread{}, err
	}
	if version != state.Version {
		return consultation.Thread{}, consultation.ErrConflict
	}
	if thread.ID != 0 {
		var threadVersion int64
		err = tx.QueryRowContext(ctx, query("SELECT version FROM chat_threads WHERE user_id=:user_id AND consultation_id=?"), thread.ID).Scan(&threadVersion)
		if errors.Is(err, sql.ErrNoRows) {
			return consultation.Thread{}, consultation.ErrNotFound
		}
		if err != nil {
			return consultation.Thread{}, err
		}
		if thread.Version != threadVersion {
			return consultation.Thread{}, consultation.ErrConflict
		}
	} else {
		err = tx.QueryRowContext(ctx, query(`INSERT INTO consultations(created_at,user_message,planned_amount,inferred_category,ai_recommendation,ai_reasoning_summary,current_situation,alternative,final_question,budget_snapshot_json,status,actual_amount,response_source,needs_follow_up,follow_up_question) VALUES(? ,? ,NULL,'' ,'','','','','','{}','pending',NULL,'',FALSE,'') RETURNING id`), thread.CreatedAt, thread.Messages[0].Content).Scan(&thread.ID)
		if err != nil {
			return consultation.Thread{}, err
		}
	}
	for i := range state.Purchases {
		if state.Purchases[i].ConsultationID == 0 {
			state.Purchases[i].ConsultationID = thread.ID
		}
	}
	consultation.ProjectPurchases(state, &thread)
	reply := thread.Reply
	if reply.PlannedAmount == nil && thread.PendingPurchase != nil {
		reply.PlannedAmount = &thread.PendingPurchase.Amount
	}
	_, err = tx.ExecContext(ctx, query(`UPDATE consultations SET planned_amount=?, ai_recommendation=?,ai_reasoning_summary=?,current_situation=?,status=?,response_source=?,needs_follow_up=?,follow_up_question=? WHERE id=?`), nullableInt64(reply.PlannedAmount), reply.Answer, reply.Reason, reply.Assumptions, reply.Status, thread.Source, reply.Status == "needs_information", reply.Question, thread.ID)
	if err != nil {
		return consultation.Thread{}, err
	}
	for i := range state.Facts {
		if strings.HasPrefix(state.Facts[i].Source, "chat:0:") {
			state.Facts[i].Source = strings.Replace(state.Facts[i].Source, "chat:0:", "chat:"+strconv.FormatInt(thread.ID, 10)+":", 1)
		}
	}
	if err = syncReconciliationReceipts(ctx, tx, state, thread.Reply); err != nil {
		return consultation.Thread{}, err
	}
	if err = syncDataChangeReceipts(ctx, tx, state, thread.Reply); err != nil {
		return consultation.Thread{}, err
	}
	state.Version++
	thread.Version++
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return consultation.Thread{}, err
	}
	threadJSON, err := json.Marshal(thread)
	if err != nil {
		return consultation.Thread{}, err
	}
	result, err := tx.ExecContext(ctx, query("UPDATE chat_states SET version=?,payload=? WHERE user_id=:user_id AND version=?"), state.Version, string(stateJSON), state.Version-1)
	if err != nil {
		return consultation.Thread{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return consultation.Thread{}, err
	}
	if affected != 1 {
		return consultation.Thread{}, consultation.ErrConflict
	}
	_, err = tx.ExecContext(ctx, query("INSERT INTO chat_threads(consultation_id,user_id,version,payload) VALUES(?,:user_id,?,?) ON CONFLICT(consultation_id) DO UPDATE SET version=excluded.version,payload=excluded.payload"), thread.ID, thread.Version, string(threadJSON))
	if err != nil {
		return consultation.Thread{}, err
	}

	if _, err = tx.ExecContext(ctx, query("DELETE FROM chat_purchase_schedule WHERE user_id=:user_id")); err != nil {
		return consultation.Thread{}, err
	}
	visited := map[int64]bool{thread.ID: true}
	for _, p := range state.Purchases {
		if p.RecordedAt <= forgottenCutoff(state) {
			continue
		}
		if p.Status == "purchase_confirmation_pending" || p.Status == "classification_required" {
			at, e := time.Parse(time.RFC3339, p.RecordedAt)
			if e != nil {
				return consultation.Thread{}, e
			}
			_, err = tx.ExecContext(ctx, query("INSERT INTO chat_purchase_schedule(user_id,purchase_id,consultation_id,due_at,first_sent) VALUES(:user_id,?,?,?,?) ON CONFLICT(user_id,purchase_id) DO UPDATE SET consultation_id=excluded.consultation_id,due_at=excluded.due_at,first_sent=excluded.first_sent"), p.ID, p.ConsultationID, at.Add(6*time.Hour).UTC().Format(time.RFC3339), p.FirstAskedAt != "")
			if err != nil {
				return consultation.Thread{}, err
			}
		}
		if visited[p.ConsultationID] {
			continue
		}
		visited[p.ConsultationID] = true
		var payload string
		err = tx.QueryRowContext(ctx, query("SELECT payload FROM chat_threads WHERE user_id=:user_id AND consultation_id=?"), p.ConsultationID).Scan(&payload)
		if err != nil {
			return consultation.Thread{}, err
		}
		var original consultation.Thread
		if err = json.Unmarshal([]byte(payload), &original); err != nil {
			return consultation.Thread{}, err
		}
		consultation.ProjectPurchases(state, &original)
		original.Version++
		encoded, e := json.Marshal(original)
		if e != nil {
			return consultation.Thread{}, e
		}
		if _, err = tx.ExecContext(ctx, query("UPDATE chat_threads SET version=?,payload=? WHERE user_id=:user_id AND consultation_id=?"), original.Version, string(encoded), original.ID); err != nil {
			return consultation.Thread{}, err
		}
	}
	if hasRequest {
		if time.Now().UnixMilli() >= requestExpiry {
			return consultation.Thread{}, consultation.ErrRequestFinished
		}
		status := "completed"
		if requestCancelled {
			status = "cancelled"
		}
		_, err = tx.ExecContext(ctx, query("UPDATE chat_requests SET status=?,result_json=?,error_code='' WHERE user_id=:user_id AND request_id=? AND status='processing'"), status, string(threadJSON), requestRef.ID)
		if err != nil {
			return consultation.Thread{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return consultation.Thread{}, err
	}
	return thread, nil
}

// The one-time bridge preserves known nonzero legacy profile values. Legacy zero
// fields cannot distinguish missing inputs from explicit zero, so they stay unknown.
func (r *Repository) initialChatState(ctx context.Context) (consultation.State, error) {
	state := consultation.State{Strictness: 5}
	profile, err := r.GetProfile(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return state, err
	}
	if err == nil {
		values := []struct {
			key    string
			amount int64
		}{
			{"current_balance", profile.CurrentBalance}, {"monthly_income", profile.MonthlyIncome},
			{"monthly_free_budget", profile.MonthlyFreeBudget}, {"monthly_fixed_costs", profile.MonthlyFixedCosts}, {"savings_goal", profile.MonthlySavingsGoal},
		}
		for _, v := range values {
			if v.amount != 0 {
				state.Facts = append(state.Facts, consultation.Fact{Key: v.key, Value: strconv.FormatInt(v.amount, 10) + "円", Kind: "fact", Source: "legacy_profile", RecordedAt: profile.UpdatedAt, Active: true})
			}
		}
		if profile.LongTermGoal != "" {
			state.Facts = append(state.Facts, consultation.Fact{Key: "goal", Value: profile.LongTermGoal, Kind: "fact", Source: "legacy_profile", RecordedAt: profile.UpdatedAt, Active: true})
		}
		if len(profile.ReduceCategories) > 0 {
			state.Facts = append(state.Facts, consultation.Fact{Key: "reduce_categories", Value: strings.Join(profile.ReduceCategories, ", "), Kind: "fact", Source: "legacy_profile", RecordedAt: profile.UpdatedAt, Active: true})
		}
		if len(profile.AllowedCategories) > 0 {
			state.Facts = append(state.Facts, consultation.Fact{Key: "allowed_categories", Value: strings.Join(profile.AllowedCategories, ", "), Kind: "fact", Source: "legacy_profile", RecordedAt: profile.UpdatedAt, Active: true})
		}
	}
	memories, err := r.ListMemories(ctx)
	if err != nil {
		return state, err
	}
	for _, m := range memories {
		state.Facts = append(state.Facts, consultation.Fact{Key: "legacy_memory:" + strconv.FormatInt(m.ID, 10), Value: m.Content, Kind: "inference", Source: "legacy_memory:" + strconv.FormatInt(m.ID, 10), RecordedAt: m.UpdatedAt, Active: true})
	}
	return state, nil
}

// LoadChatTransactions projects only confirmed expense data permitted by B-S08.
// Candidate limit applies before filtering; coverage is always partial.
func (r *Repository) LoadChatTransactions(ctx context.Context, limit int, forgottenAt string) ([]consultation.Transaction, consultation.TransactionCoverage, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	coverage := consultation.TransactionCoverage{Partial: true, CandidateLimit: limit}
	rows, err := r.db.QueryContext(ctx, `SELECT t.transaction_date,t.amount,t.category,t.transaction_type,t.imported_at,EXISTS(SELECT 1 FROM purchase_csv_decisions d WHERE d.user_id=o.user_id AND d.transaction_id=t.id AND d.action='same'),EXISTS(SELECT 1 FROM csv_financial_items i JOIN csv_financial_receipts f ON f.user_id=i.user_id AND f.ref=i.ref WHERE i.user_id=o.user_id AND i.transaction_id=t.id AND f.status IN ('pending','needs_information') AND f.recorded_at>?)
 FROM effective_transactions t JOIN chat_transaction_owners o ON o.transaction_id=t.id
 WHERE o.user_id=? ORDER BY t.transaction_date DESC,t.id DESC LIMIT ?`, forgottenAt, identity.UserID(ctx), limit)
	if err != nil {
		return nil, coverage, err
	}
	defer rows.Close()
	var result []consultation.Transaction
	for rows.Next() {
		var item consultation.Transaction
		var transactionType, importedAt string
		var linked, dataPending bool
		if err := rows.Scan(&item.Date, &item.Amount, &item.Category, &transactionType, &importedAt, &linked, &dataPending); err != nil {
			return nil, coverage, err
		}
		if transactionType != "expense" || (forgottenAt != "" && importedAt <= forgottenAt) {
			continue
		}
		if dataPending {
			coverage.DataChangeExcluded++
			continue
		}
		if linked {
			coverage.LinkedExcluded++
			continue
		}
		category := []rune(consultation.Redact(item.Category))
		if len(category) > 100 {
			category = category[:100]
		}
		item.Category = string(category)
		result = append(result, item)
		if coverage.StartDate == "" || item.Date < coverage.StartDate {
			coverage.StartDate = item.Date
		}
		if item.Date > coverage.EndDate {
			coverage.EndDate = item.Date
		}
	}
	if err := rows.Err(); err != nil {
		return nil, coverage, err
	}
	return result, coverage, nil
}
func (r *Repository) ListRecentChatThreads(ctx context.Context, excludedID int64, limit int) ([]consultation.Thread, error) {
	if limit < 1 || limit > 20 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, chatQuery(ctx, "SELECT payload FROM chat_threads WHERE user_id=:user_id AND consultation_id<>? ORDER BY consultation_id DESC LIMIT ?"), excludedID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var threads []consultation.Thread
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var thread consultation.Thread
		if err := json.Unmarshal([]byte(payload), &thread); err != nil {
			return nil, err
		}
		threads = append(threads, thread)
	}
	return threads, rows.Err()
}

func forgottenCutoff(state consultation.State) string {
	var cutoff string
	for _, f := range state.Facts {
		if !f.Active && f.Value == "" && f.RecordedAt > cutoff {
			cutoff = f.RecordedAt
		}
	}
	return cutoff
}
func (r *Repository) hydratePurchases(ctx context.Context, state *consultation.State) error {
	if state.PurchasesMigrated {
		return nil
	}
	rows, err := r.db.QueryContext(ctx, chatQuery(ctx, "SELECT payload FROM chat_threads WHERE user_id=:user_id ORDER BY consultation_id"))
	if err != nil {
		return err
	}
	defer rows.Close()
	var threads []consultation.Thread
	for rows.Next() {
		var payload string
		if err = rows.Scan(&payload); err != nil {
			return err
		}
		var t consultation.Thread
		if err = json.Unmarshal([]byte(payload), &t); err != nil {
			return err
		}
		threads = append(threads, t)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	consultation.LegacyPurchases(state, threads)
	return nil
}
