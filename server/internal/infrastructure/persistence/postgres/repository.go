// Package postgres is the production adapter for a Docker-hosted PostgreSQL database.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

type Repository struct {
	db *database
}

func Open(ctx context.Context, url string) (*Repository, error) {
	db, err := OpenDatabase(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

// Migrate は明示的なスキーマ確認用の窓口。Open 時にも同じ移行を実行する。
func (r *Repository) Migrate(ctx context.Context) error {
	return Migrate(ctx, r.db.DB)
}

// MonthlyConsultationLimit は利用者別設定を優先し、未設定なら全体の既定値を返す。
func (r *Repository) MonthlyConsultationLimit(ctx context.Context) (int, error) {
	var limit int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(user_limits.monthly_limit, settings.integer_value)
		FROM service_settings AS settings
		LEFT JOIN user_consultation_limits AS user_limits
		  ON user_limits.user_id = :user_id
		WHERE settings.key = 'default_monthly_consultation_limit'
	`).Scan(&limit)
	return limit, err
}

func (r *Repository) MonthlyConsultationUsage(ctx context.Context, month string) (domain.ConsultationUsage, error) {
	usage := domain.ConsultationUsage{Month: month}
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(usage.consultation_count, 0),
		       COALESCE(user_limits.monthly_limit, settings.integer_value)
		FROM service_settings AS settings
		LEFT JOIN user_consultation_limits AS user_limits
		  ON user_limits.user_id = :user_id
		LEFT JOIN monthly_consultation_usage AS usage
		  ON usage.user_id = :user_id AND usage.month = ?
		WHERE settings.key = 'default_monthly_consultation_limit'
	`, month).Scan(&usage.Count, &usage.Limit)
	return usage, err
}

// ReserveMonthlyConsultation は同一利用者・月の同時要求をDBのupsertで直列化する。
func (r *Repository) ReserveMonthlyConsultation(ctx context.Context, month string) (domain.ConsultationUsage, error) {
	userID := identity.UserID(ctx)
	if userID <= 0 {
		return domain.ConsultationUsage{}, errors.New("authenticated user is required")
	}
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.ConsultationUsage{}, err
	}
	defer tx.Rollback()
	usage := domain.ConsultationUsage{Month: month}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO monthly_consultation_usage(user_id, month, consultation_count)
		VALUES($1, $2, 1)
		ON CONFLICT (user_id, month) DO UPDATE SET
		  consultation_count = monthly_consultation_usage.consultation_count + 1,
		  updated_at = CURRENT_TIMESTAMP
		RETURNING consultation_count
	`, userID, month).Scan(&usage.Count)
	if err != nil {
		return domain.ConsultationUsage{}, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(user_limits.monthly_limit, settings.integer_value)
		FROM service_settings AS settings
		LEFT JOIN user_consultation_limits AS user_limits ON user_limits.user_id = $1
		WHERE settings.key = 'default_monthly_consultation_limit'
	`, userID).Scan(&usage.Limit)
	if err != nil {
		return domain.ConsultationUsage{}, err
	}
	if err = tx.Commit(); err != nil {
		return domain.ConsultationUsage{}, err
	}
	return usage, nil
}

func (r *Repository) ReleaseMonthlyConsultation(ctx context.Context, month string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE monthly_consultation_usage
		SET consultation_count = consultation_count - 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = :user_id AND month = ? AND consultation_count > 0
	`, month)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) ClaimMonthlyLimitNotification(ctx context.Context, event domain.MonthlyLimitEvent) (bool, error) {
	var claimed bool
	err := r.db.QueryRowContext(ctx, `
		WITH claimed AS (
		  INSERT INTO admin_notifications(
		    user_id, month, notification_type, consultation_count,
		    consultation_limit, status, attempts, occurred_at
		  ) VALUES(:user_id, ?, 'monthly_consultation_limit', ?, ?, 'sending', 1, ?)
		  ON CONFLICT (user_id, month, notification_type) DO UPDATE SET
		    consultation_count = excluded.consultation_count,
		    consultation_limit = excluded.consultation_limit,
		    status = 'sending', attempts = admin_notifications.attempts + 1,
		    last_error = '', updated_at = CURRENT_TIMESTAMP
		  WHERE admin_notifications.status = 'failed'
		  RETURNING 1
		)
		SELECT EXISTS(SELECT 1 FROM claimed)
	`, event.Month, event.Count, event.Limit, event.OccurredAt).Scan(&claimed)
	return claimed, err
}

func (r *Repository) CompleteMonthlyLimitNotification(ctx context.Context, event domain.MonthlyLimitEvent, notifyErr error) error {
	status := "delivered"
	lastError := ""
	var deliveredAt any = event.OccurredAt
	if notifyErr != nil {
		status = "failed"
		lastError = notifyErr.Error()
		deliveredAt = nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE admin_notifications
		SET status = ?, last_error = ?, delivered_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = :user_id AND month = ?
		  AND notification_type = 'monthly_consultation_limit'
	`, status, lastError, deliveredAt, event.Month)
	return err
}

func (r *Repository) RecordLLMUsage(ctx context.Context, usage domain.LLMUsage) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO llm_usage(
		  user_id, operation, model, input_tokens, cached_input_tokens,
		  output_tokens, reasoning_tokens, total_tokens, occurred_at
		) VALUES(:user_id, ?, ?, ?, ?, ?, ?, ?, ?)
	`, usage.Operation, usage.Model, usage.InputTokens, usage.CachedInputTokens,
		usage.OutputTokens, usage.ReasoningTokens, usage.TotalTokens, usage.OccurredAt)
	return err
}

func (r *Repository) GetProfile(ctx context.Context) (domain.Profile, error) {
	var profile domain.Profile
	var reduceJSON, allowedJSON string
	err := r.db.QueryRowContext(ctx, `
		SELECT monthly_income, current_balance, monthly_fixed_costs,
		       monthly_free_budget, monthly_savings_goal, reduce_categories_json,
		       allowed_categories_json, long_term_goal, advice_strictness, updated_at
		FROM profiles WHERE user_id = :user_id
	`).Scan(
		&profile.MonthlyIncome,
		&profile.CurrentBalance,
		&profile.MonthlyFixedCosts,
		&profile.MonthlyFreeBudget,
		&profile.MonthlySavingsGoal,
		&reduceJSON,
		&allowedJSON,
		&profile.LongTermGoal,
		&profile.AdviceStrictness,
		&profile.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Profile{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Profile{}, err
	}
	if err := json.Unmarshal([]byte(reduceJSON), &profile.ReduceCategories); err != nil {
		return domain.Profile{}, err
	}
	if err := json.Unmarshal([]byte(allowedJSON), &profile.AllowedCategories); err != nil {
		return domain.Profile{}, err
	}
	return profile, nil
}

func (r *Repository) PutProfile(ctx context.Context, profile domain.Profile) error {
	reduceJSON, _ := json.Marshal(profile.ReduceCategories)
	allowedJSON, _ := json.Marshal(profile.AllowedCategories)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO profiles(
			user_id, id, monthly_income, current_balance, monthly_fixed_costs,
			monthly_free_budget, monthly_savings_goal, reduce_categories_json,
			allowed_categories_json, long_term_goal, advice_strictness, updated_at
		) VALUES(:user_id, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			monthly_income = excluded.monthly_income,
			current_balance = excluded.current_balance,
			monthly_fixed_costs = excluded.monthly_fixed_costs,
			monthly_free_budget = excluded.monthly_free_budget,
			monthly_savings_goal = excluded.monthly_savings_goal,
			reduce_categories_json = excluded.reduce_categories_json,
			allowed_categories_json = excluded.allowed_categories_json,
			long_term_goal = excluded.long_term_goal,
			advice_strictness = excluded.advice_strictness,
			updated_at = excluded.updated_at
	`, profile.MonthlyIncome, profile.CurrentBalance, profile.MonthlyFixedCosts,
		profile.MonthlyFreeBudget, profile.MonthlySavingsGoal, string(reduceJSON),
		string(allowedJSON), profile.LongTermGoal, profile.AdviceStrictness, profile.UpdatedAt)
	return err
}

// ImportTransactions は全明細を単一トランザクションで登録する。
// 利用者内の fingerprint 重複だけを除外し、それ以外のDBエラーでは全件を戻す。
func (r *Repository) ImportTransactions(
	ctx context.Context,
	transactions []domain.Transaction,
) (domain.ImportCommitResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ImportCommitResult{}, err
	}
	defer tx.Rollback()

	result := domain.ImportCommitResult{}
	for _, transaction := range transactions {
		rawJSON, _ := json.Marshal(transaction.RawData)
		execResult, err := tx.ExecContext(ctx, userQuery(ctx, `
			INSERT INTO transactions(user_id, 
				transaction_date, description, normalized_merchant, amount,
				transaction_type, category, source, source_account_name,
				fingerprint, imported_at, raw_data_json
			) VALUES(:user_id, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (user_id, fingerprint) DO NOTHING
		`), transaction.TransactionDate, transaction.Description,
			transaction.NormalizedMerchant, transaction.Amount,
			transaction.TransactionType, transaction.Category, transaction.Source,
			transaction.SourceAccountName, transaction.Fingerprint,
			transaction.ImportedAt, string(rawJSON))
		if err != nil {
			return domain.ImportCommitResult{}, err
		}
		affected, _ := execResult.RowsAffected()
		if affected == 0 {
			result.DuplicateCount++
		} else {
			result.ImportedCount++
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.ImportCommitResult{}, err
	}
	return result, nil
}

func (r *Repository) ListTransactions(
	ctx context.Context,
	month string,
	limit int,
) ([]domain.Transaction, error) {
	query := `
		SELECT id, transaction_date, description, normalized_merchant, amount,
		       transaction_type, category, source, source_account_name,
		       fingerprint, imported_at, raw_data_json
		FROM transactions WHERE user_id = :user_id
	`
	args := []any{}
	if month != "" {
		query += " AND substr(transaction_date, 1, 7) = ?"
		args = append(args, month)
	}
	query += " ORDER BY transaction_date DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var transactions []domain.Transaction
	for rows.Next() {
		transaction, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}
	return transactions, rows.Err()
}

func (r *Repository) UpdateTransactionCategory(
	ctx context.Context,
	id int64,
	category string,
	rememberMerchant bool,
) (domain.Transaction, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE transactions SET category = ? WHERE user_id = :user_id AND id = ?
	`, category, id)
	if err != nil {
		return domain.Transaction{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.Transaction{}, domain.ErrNotFound
	}
	transaction, err := r.GetTransaction(ctx, id)
	if err != nil {
		return domain.Transaction{}, err
	}
	if rememberMerchant {
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO merchant_rules(user_id, normalized_merchant, category, updated_at)
			VALUES(:user_id, ?, ?, ?)
			ON CONFLICT(user_id, normalized_merchant) DO UPDATE SET
				category = excluded.category,
				updated_at = excluded.updated_at
		`, transaction.NormalizedMerchant, category, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return domain.Transaction{}, err
		}
	}
	return transaction, nil
}

func (r *Repository) GetTransaction(
	ctx context.Context,
	id int64,
) (domain.Transaction, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, transaction_date, description, normalized_merchant, amount,
		       transaction_type, category, source, source_account_name,
		       fingerprint, imported_at, raw_data_json
		FROM transactions WHERE user_id = :user_id AND id = ?
	`, id)
	transaction, err := scanTransaction(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Transaction{}, domain.ErrNotFound
	}
	return transaction, err
}

func (r *Repository) MerchantRules(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT normalized_merchant, category FROM merchant_rules WHERE user_id = :user_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := map[string]string{}
	for rows.Next() {
		var merchant, category string
		if err := rows.Scan(&merchant, &category); err != nil {
			return nil, err
		}
		rules[merchant] = category
	}
	return rules, rows.Err()
}

func (r *Repository) MonthlyDashboard(
	ctx context.Context,
	month string,
	profile domain.Profile,
) (domain.Dashboard, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT category, COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE user_id = :user_id AND transaction_type = 'expense'
		  AND substr(transaction_date, 1, 7) = ?
		GROUP BY category
		ORDER BY SUM(amount) DESC
	`, month)
	if err != nil {
		return domain.Dashboard{}, err
	}
	defer rows.Close()

	dashboard := domain.Dashboard{
		Month:              month,
		MonthlyFreeBudget:  profile.MonthlyFreeBudget,
		ByCategory:         []domain.CategoryAmount{},
		RecentTransactions: []domain.Transaction{},
	}
	fixedCategories := map[string]bool{
		"家賃": true, "光熱費": true, "通信": true,
	}
	for rows.Next() {
		var item domain.CategoryAmount
		if err := rows.Scan(&item.Category, &item.Amount); err != nil {
			return domain.Dashboard{}, err
		}
		dashboard.ByCategory = append(dashboard.ByCategory, item)
		dashboard.ExpenseTotal += item.Amount
		if fixedCategories[item.Category] {
			dashboard.FixedExpenseEstimate += item.Amount
		} else {
			dashboard.FreeExpenseTotal += item.Amount
		}
		switch item.Category {
		case "酒・飲み会":
			dashboard.DrinkingTotal = item.Amount
		case "サブスクリプション":
			dashboard.SubscriptionTotal = item.Amount
		}
	}
	if err := rows.Err(); err != nil {
		return domain.Dashboard{}, err
	}
	dashboard.FreeBudgetRemaining = profile.MonthlyFreeBudget - dashboard.FreeExpenseTotal
	recent, err := r.ListTransactions(ctx, month, 5)
	if err != nil {
		return domain.Dashboard{}, err
	}
	dashboard.RecentTransactions = recent
	return dashboard, nil
}

func (r *Repository) CreateConsultation(
	ctx context.Context,
	consultation domain.Consultation,
) (domain.Consultation, error) {
	snapshotJSON, _ := json.Marshal(consultation.BudgetSnapshot)
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO consultations(user_id, 
			created_at, user_message, planned_amount, inferred_category,
			ai_recommendation, ai_reasoning_summary, current_situation,
			alternative, final_question, budget_snapshot_json, status,
			actual_amount, user_decision_reason, satisfaction_score,
			regret_score, note, response_source, needs_follow_up,
			follow_up_question
		) VALUES(:user_id, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id
	`, consultation.CreatedAt, consultation.UserMessage,
		nullableInt64(consultation.PlannedAmount), consultation.InferredCategory,
		consultation.AIRecommendation, consultation.AIReasoningSummary,
		consultation.CurrentSituation, consultation.Alternative,
		consultation.FinalQuestion, string(snapshotJSON), consultation.Status,
		nullableInt64(consultation.ActualAmount), consultation.UserDecisionReason,
		nullableInt(consultation.SatisfactionScore), nullableInt(consultation.RegretScore),
		consultation.Note, consultation.ResponseSource, consultation.NeedsFollowUp,
		consultation.FollowUpQuestion).Scan(&consultation.ID)
	if err != nil {
		return domain.Consultation{}, err
	}
	return consultation, nil
}

func (r *Repository) UpdateConsultationAdvice(
	ctx context.Context,
	consultation domain.Consultation,
) (domain.Consultation, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE consultations SET
			planned_amount = ?, inferred_category = ?, ai_recommendation = ?,
			ai_reasoning_summary = ?, current_situation = ?, alternative = ?,
			final_question = ?, budget_snapshot_json = ?, status = ?,
			response_source = ?, needs_follow_up = ?, follow_up_question = ?
		WHERE user_id = :user_id AND id = ?
	`, nullableInt64(consultation.PlannedAmount), consultation.InferredCategory,
		consultation.AIRecommendation, consultation.AIReasoningSummary,
		consultation.CurrentSituation, consultation.Alternative,
		consultation.FinalQuestion, mustJSON(consultation.BudgetSnapshot),
		consultation.Status, consultation.ResponseSource, consultation.NeedsFollowUp,
		consultation.FollowUpQuestion, consultation.ID)
	if err != nil {
		return domain.Consultation{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.Consultation{}, domain.ErrNotFound
	}
	return r.GetConsultation(ctx, consultation.ID)
}

func (r *Repository) ListConsultations(
	ctx context.Context,
	limit int,
) ([]domain.Consultation, error) {
	rows, err := r.db.QueryContext(ctx, consultationSelect+`
		ORDER BY created_at DESC, id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var consultations []domain.Consultation
	for rows.Next() {
		consultation, err := scanConsultation(rows)
		if err != nil {
			return nil, err
		}
		consultations = append(consultations, consultation)
	}
	return consultations, rows.Err()
}

func (r *Repository) GetConsultation(
	ctx context.Context,
	id int64,
) (domain.Consultation, error) {
	consultation, err := scanConsultation(r.db.QueryRowContext(
		ctx,
		consultationSelect+" AND id = ?",
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Consultation{}, domain.ErrNotFound
	}
	return consultation, err
}

func (r *Repository) UpdateConsultationResult(
	ctx context.Context,
	id int64,
	update domain.ConsultationResultUpdate,
) (domain.Consultation, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE consultations SET
			status = ?, actual_amount = ?, user_decision_reason = ?,
			satisfaction_score = ?, regret_score = ?, note = ?
		WHERE user_id = :user_id AND id = ?
	`, update.Status, nullableInt64(update.ActualAmount), update.UserDecisionReason,
		nullableInt(update.SatisfactionScore), nullableInt(update.RegretScore),
		update.Note, id)
	if err != nil {
		return domain.Consultation{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.Consultation{}, domain.ErrNotFound
	}
	return r.GetConsultation(ctx, id)
}

func (r *Repository) AddMessage(
	ctx context.Context,
	consultationID int64,
	role string,
	content string,
) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO consultation_messages(user_id, consultation_id, role, content, created_at)
		VALUES(:user_id, ?, ?, ?, ?)
	`, consultationID, role, content, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (r *Repository) ListMemories(ctx context.Context) ([]domain.MemoryItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, type, content, evidence, confidence, created_at, updated_at
		FROM memories WHERE user_id = :user_id ORDER BY updated_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.MemoryItem
	for rows.Next() {
		var item domain.MemoryItem
		if err := rows.Scan(
			&item.ID, &item.Type, &item.Content, &item.Evidence,
			&item.Confidence, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) SaveMemory(
	ctx context.Context,
	item domain.MemoryItem,
) (domain.MemoryItem, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if item.ID == 0 {
		item.CreatedAt = now
		item.UpdatedAt = now
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO memories(user_id, type, content, evidence, confidence, created_at, updated_at)
			VALUES(:user_id, ?, ?, ?, ?, ?, ?)
			RETURNING id
		`, item.Type, item.Content, item.Evidence, item.Confidence,
			item.CreatedAt, item.UpdatedAt).Scan(&item.ID)
		if err != nil {
			return domain.MemoryItem{}, err
		}
		return item, nil
	}
	item.UpdatedAt = now
	result, err := r.db.ExecContext(ctx, `
		UPDATE memories SET type = ?, content = ?, evidence = ?,
			confidence = ?, updated_at = ? WHERE user_id = :user_id AND id = ?
	`, item.Type, item.Content, item.Evidence, item.Confidence, item.UpdatedAt, item.ID)
	if err != nil {
		return domain.MemoryItem{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.MemoryItem{}, domain.ErrNotFound
	}
	return item, nil
}

func (r *Repository) DeleteMemory(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM memories WHERE user_id = :user_id AND id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) SaveReview(
	ctx context.Context,
	review domain.MonthlyReview,
) (domain.MonthlyReview, error) {
	candidatesJSON, _ := json.Marshal(review.Candidates)
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO monthly_reviews(user_id, month, created_at, summary, candidates_json)
		VALUES(:user_id, ?, ?, ?, ?)
		RETURNING id
	`, review.Month, review.CreatedAt, review.Summary, string(candidatesJSON)).Scan(&review.ID)
	if err != nil {
		return domain.MonthlyReview{}, err
	}
	return review, nil
}

func (r *Repository) LatestReview(ctx context.Context) (domain.MonthlyReview, error) {
	var review domain.MonthlyReview
	var candidatesJSON string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, month, created_at, summary, candidates_json
		FROM monthly_reviews WHERE user_id = :user_id ORDER BY created_at DESC, id DESC LIMIT 1
	`).Scan(&review.ID, &review.Month, &review.CreatedAt, &review.Summary, &candidatesJSON)
	if err != nil {
		return domain.MonthlyReview{}, err
	}
	if err := json.Unmarshal([]byte(candidatesJSON), &review.Candidates); err != nil {
		return domain.MonthlyReview{}, err
	}
	return review, nil
}

type scanner interface {
	Scan(...any) error
}

func scanTransaction(row scanner) (domain.Transaction, error) {
	var transaction domain.Transaction
	var rawJSON string
	err := row.Scan(
		&transaction.ID, &transaction.TransactionDate, &transaction.Description,
		&transaction.NormalizedMerchant, &transaction.Amount,
		&transaction.TransactionType, &transaction.Category, &transaction.Source,
		&transaction.SourceAccountName, &transaction.Fingerprint,
		&transaction.ImportedAt, &rawJSON,
	)
	if err != nil {
		return domain.Transaction{}, err
	}
	if err := json.Unmarshal([]byte(rawJSON), &transaction.RawData); err != nil {
		return domain.Transaction{}, err
	}
	return transaction, nil
}

const consultationSelect = `
	SELECT id, created_at, user_message, planned_amount, inferred_category,
	       ai_recommendation, ai_reasoning_summary, current_situation,
	       alternative, final_question, budget_snapshot_json, status,
	       actual_amount, user_decision_reason, satisfaction_score,
	       regret_score, note, response_source, needs_follow_up,
	       follow_up_question
	FROM consultations WHERE user_id = :user_id
`

func scanConsultation(row scanner) (domain.Consultation, error) {
	var consultation domain.Consultation
	var plannedAmount, actualAmount sql.NullInt64
	var satisfaction, regret sql.NullInt64
	var snapshotJSON string
	err := row.Scan(
		&consultation.ID, &consultation.CreatedAt, &consultation.UserMessage,
		&plannedAmount, &consultation.InferredCategory,
		&consultation.AIRecommendation, &consultation.AIReasoningSummary,
		&consultation.CurrentSituation, &consultation.Alternative,
		&consultation.FinalQuestion, &snapshotJSON, &consultation.Status,
		&actualAmount, &consultation.UserDecisionReason, &satisfaction,
		&regret, &consultation.Note, &consultation.ResponseSource,
		&consultation.NeedsFollowUp, &consultation.FollowUpQuestion,
	)
	if err != nil {
		return domain.Consultation{}, err
	}
	if plannedAmount.Valid {
		consultation.PlannedAmount = &plannedAmount.Int64
	}
	if actualAmount.Valid {
		consultation.ActualAmount = &actualAmount.Int64
	}
	if satisfaction.Valid {
		value := int(satisfaction.Int64)
		consultation.SatisfactionScore = &value
	}
	if regret.Valid {
		value := int(regret.Int64)
		consultation.RegretScore = &value
	}
	if err := json.Unmarshal([]byte(snapshotJSON), &consultation.BudgetSnapshot); err != nil {
		return domain.Consultation{}, err
	}
	return consultation, nil
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return int64(*value)
}

func mustJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func IsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func ValidateDatabaseURL(url string) error {
	if strings.TrimSpace(url) == "" {
		return errors.New("database URL is empty")
	}
	return nil
}
