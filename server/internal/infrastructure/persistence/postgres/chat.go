package postgres

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
)

func chatQuery(ctx context.Context, query string) string {
	query = strings.ReplaceAll(query, ":user_id", strconv.FormatInt(identity.UserID(ctx), 10))
	return bind(query)
}
func (r *Repository) LoadChatState(ctx context.Context) (consultation.State, error) {
	var payload string
	err := r.db.QueryRowContext(ctx, chatQuery(ctx, "SELECT payload FROM chat_states WHERE user_id=:user_id")).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return r.initialChatState(ctx)
	}
	if err != nil {
		return consultation.State{}, err
	}
	var state consultation.State
	err = json.Unmarshal([]byte(payload), &state)
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
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return consultation.Thread{}, err
	}
	defer tx.Rollback()
	query := func(q string) string { return chatQuery(ctx, q) }
	if _, err = tx.ExecContext(ctx, query("INSERT INTO chat_states(user_id,version,payload) VALUES(:user_id,0,'{}') ON CONFLICT(user_id) DO NOTHING")); err != nil {
		return consultation.Thread{}, err
	}
	var version int64
	if err = tx.QueryRowContext(ctx, query("SELECT version FROM chat_states WHERE user_id=:user_id FOR UPDATE")).Scan(&version); err != nil {
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
		err = tx.QueryRowContext(ctx, query(`INSERT INTO consultations(user_id,created_at,user_message,planned_amount,inferred_category,ai_recommendation,ai_reasoning_summary,current_situation,alternative,final_question,budget_snapshot_json,status,actual_amount,response_source,needs_follow_up,follow_up_question) VALUES(:user_id,? ,? ,NULL,'' ,'','','','','','{}','pending',NULL,'',FALSE,'') RETURNING id`), thread.CreatedAt, thread.Messages[0].Content).Scan(&thread.ID)
		if err != nil {
			return consultation.Thread{}, err
		}
	}
	reply := thread.Reply
	if reply.PlannedAmount == nil && thread.PendingPurchase != nil {
		reply.PlannedAmount = &thread.PendingPurchase.Amount
	}
	_, err = tx.ExecContext(ctx, query(`UPDATE consultations SET planned_amount=?, ai_recommendation=?,ai_reasoning_summary=?,current_situation=?,status=?,response_source=?,needs_follow_up=?,follow_up_question=? WHERE user_id=:user_id AND id=?`), nullableInt64(reply.PlannedAmount), reply.Answer, reply.Reason, reply.Assumptions, reply.Status, thread.Source, reply.Status == "needs_information", reply.Question, thread.ID)
	if err != nil {
		return consultation.Thread{}, err
	}
	for i := range state.Facts {
		if strings.HasPrefix(state.Facts[i].Source, "chat:0:") {
			state.Facts[i].Source = strings.Replace(state.Facts[i].Source, "chat:0:", "chat:"+strconv.FormatInt(thread.ID, 10)+":", 1)
		}
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
	rows, err := r.ListTransactions(ctx, "", limit)
	if err != nil {
		return nil, coverage, err
	}
	var result []consultation.Transaction
	for _, row := range rows {
		if row.TransactionType != "expense" || (forgottenAt != "" && row.ImportedAt <= forgottenAt) {
			continue
		}
		category := []rune(consultation.Redact(row.Category))
		if len(category) > 100 {
			category = category[:100]
		}
		result = append(result, consultation.Transaction{Date: row.TransactionDate, Amount: row.Amount, Category: string(category)})
		if coverage.StartDate == "" || row.TransactionDate < coverage.StartDate {
			coverage.StartDate = row.TransactionDate
		}
		if row.TransactionDate > coverage.EndDate {
			coverage.EndDate = row.TransactionDate
		}
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
