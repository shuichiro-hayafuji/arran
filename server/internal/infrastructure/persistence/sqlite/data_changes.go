package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"time"
)

func (r *Repository) LoadDataChanges(ctx context.Context, cutoff string) ([]consultation.DataChangeContext, error) {
	rows, err := r.db.QueryContext(ctx, chatQuery(ctx, "SELECT ref,operation,before_data,after_data,included_count,omitted_count,balance_resolved,budget_resolved,mutation_id FROM csv_financial_receipts WHERE user_id=:user_id AND status IN ('pending','needs_information') AND recorded_at>? ORDER BY recorded_at,ref LIMIT 20"), cutoff)
	if err != nil {
		return nil, err
	}
	var out []consultation.DataChangeContext
	for rows.Next() {
		var c consultation.DataChangeContext
		var before, after, mutation string
		if err = rows.Scan(&c.Ref, &c.Operation, &before, &after, &c.Included, &c.Omitted, &c.BalanceResolved, &c.BudgetResolved, &mutation); err != nil {
			rows.Close()
			return nil, err
		}
		if err = json.Unmarshal([]byte(before), &c.Before); err != nil {
			rows.Close()
			return nil, err
		}
		if err = json.Unmarshal([]byte(after), &c.After); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range out {
		for _, items := range [][]consultation.Transaction{out[i].Before, out[i].After} {
			for j := range items {
				items[j].Date = consultation.Redact(items[j].Date)
				category := []rune(consultation.Redact(items[j].Category))
				if len(category) > 100 {
					category = category[:100]
				}
				items[j].Category = string(category)
			}
		}
	}
	remaining := 100
	for i := range out {
		c := &out[i]
		total := len(c.Before) + len(c.After)
		if len(c.Before) > remaining {
			c.Before = c.Before[:remaining]
		}
		remaining -= len(c.Before)
		if len(c.After) > remaining {
			c.After = c.After[:remaining]
		}
		remaining -= len(c.After)
		c.Included = len(c.Before) + len(c.After)
		c.Omitted += total - c.Included
	}
	state, err := r.LoadChatState(ctx)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, p := range state.Purchases {
		if p.RecordedAt > cutoff {
			allowed[p.ID] = true
		}
	}
	for i := range out {
		rows, e := r.db.QueryContext(ctx, chatQuery(ctx, "SELECT DISTINCT d.purchase_id FROM csv_financial_items i JOIN purchase_csv_decisions d ON d.user_id=i.user_id AND d.transaction_id=i.transaction_id JOIN transactions t ON t.user_id=i.user_id AND t.id=i.transaction_id WHERE i.user_id=:user_id AND i.ref=? AND d.action='same' AND t.imported_at>?"), out[i].Ref, cutoff)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var pid string
			if e = rows.Scan(&pid); e != nil {
				rows.Close()
				return nil, e
			}
			if allowed[pid] {
				out[i].RelatedPurchaseIDs = append(out[i].RelatedPurchaseIDs, pid)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}
func syncDataChangeReceipts(ctx context.Context, tx *sql.Tx, state consultation.State, reply consultation.Reply) error {
	for _, u := range reply.DataChangeUpdates {
		var status, mutation string
		var balance, budget bool
		var omitted int
		err := tx.QueryRowContext(ctx, chatQuery(ctx, "SELECT status,mutation_id,balance_resolved,budget_resolved,omitted_count FROM csv_financial_receipts WHERE user_id=:user_id AND ref=? AND recorded_at>?"), u.Ref, forgottenCutoff(state)).Scan(&status, &mutation, &balance, &budget, &omitted)
		if err != nil || (status == "applied" || status == "baseline_adopted") {
			return consultation.ErrConflict
		}
		if u.Status != "applied" && u.Status != "needs_information" && u.Status != "baseline_adopted" {
			return consultation.ErrConflict
		}
		for _, c := range reply.Changes {
			if c.Operation == "set" && (c.Basis == "reconciliation" || c.Basis == "user_report") {
				if c.Key == "current_balance" {
					balance = true
				}
				if c.Key == "monthly_free_budget" {
					budget = true
				}
			}
		}
		if u.Status == "baseline_adopted" {
			if err := consultation.ValidateDataChangeBaseline(reply.Changes, time.Now()); err != nil {
				return consultation.ErrConflict
			}
		}
		if u.Status == "applied" && (omitted > 0 || !balance || !budget) {
			return consultation.ErrConflict
		}
		before, _ := json.Marshal(map[string]any{"status": status})
		after, _ := json.Marshal(map[string]any{"status": u.Status, "balance_resolved": balance, "budget_resolved": budget, "state_version": state.Version + 1})
		if _, err = tx.ExecContext(ctx, chatQuery(ctx, "UPDATE csv_financial_receipts SET status=?,balance_resolved=?,budget_resolved=?,state_version=? WHERE user_id=:user_id AND ref=? AND status IN ('pending','needs_information')"), u.Status, balance, budget, state.Version+1, u.Ref); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, chatQuery(ctx, "INSERT INTO transaction_change_events(user_id,id,mutation_id,kind,target,recorded_at,before_state,after_state) VALUES(:user_id,?,?,?,?,?,?,?)"), managementID(), mutation, "financial_confirmation", u.Ref, time.Now().UTC().Format(time.RFC3339), string(before), string(after)); err != nil {
			return err
		}
	}
	return nil
}
