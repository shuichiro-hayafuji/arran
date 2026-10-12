package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"strings"
	"time"
)

func reconciliationState(ctx context.Context, tx *sql.Tx, seed *consultation.State, lock bool) (consultation.State, error) {
	q := func(s string) string { return chatQuery(ctx, s) }
	suffix := ""
	if lock {
		suffix = ""
	}
	var version int64
	var payload string
	err := tx.QueryRowContext(ctx, q("SELECT version,payload FROM chat_states WHERE user_id=:user_id"+suffix)).Scan(&version, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		if seed == nil {
			return consultation.State{}, nil
		}
		data, e := json.Marshal(seed)
		if e != nil {
			return consultation.State{}, e
		}
		_, e = tx.ExecContext(ctx, q("INSERT INTO chat_states(user_id,version,payload) VALUES(:user_id,?,?)"), seed.Version, string(data))
		return *seed, e
	}
	if err != nil {
		return consultation.State{}, err
	}
	var state consultation.State
	err = json.Unmarshal([]byte(payload), &state)
	state.Version = version
	return state, err
}
func resolveImportID(ctx context.Context, tx *sql.Tx, item domain.Transaction) (int64, error) {
	q := func(s string) string { return chatQuery(ctx, s) }
	var id int64
	err := tx.QueryRowContext(ctx, q("SELECT id FROM transactions WHERE user_id=:user_id AND fingerprint=?"), item.Fingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, q("SELECT transaction_id FROM csv_identity_aliases WHERE user_id=:user_id AND row_key=?"), item.Fingerprint).Scan(&id)
	}
	if errors.Is(err, sql.ErrNoRows) && item.IdentityAction == "same" {
		id = item.IdentityTransactionID
		err = nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}
func (r *Repository) PreviewReconciliations(ctx context.Context, items []domain.Transaction) ([]domain.ReconciliationConflict, error) {
	seed, err := r.LoadChatState(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	state, err := reconciliationState(ctx, tx, nil, false)
	if err != nil {
		return nil, err
	}
	if state.Version == 0 && len(state.Purchases) == 0 {
		state = seed
	}
	return reconcileImports(ctx, tx, items, state, false, nil)
}
func reconcileImports(ctx context.Context, tx *sql.Tx, items []domain.Transaction, state consultation.State, write bool, newRows map[string]bool) ([]domain.ReconciliationConflict, error) {
	q := func(s string) string { return chatQuery(ctx, s) }
	var conflicts []domain.ReconciliationConflict
	seenPurchases := map[string]bool{}
	for _, item := range items {
		if item.TransactionType != "expense" {
			if item.Reconciliation != nil {
				return nil, fmt.Errorf("入金は購入照合の対象外です")
			}
			continue
		}
		id, err := resolveImportID(ctx, tx, item)
		if err != nil {
			return nil, err
		}
		if id != 0 {
			var importedAt string
			if e := tx.QueryRowContext(ctx, q("SELECT imported_at FROM transactions WHERE user_id=:user_id AND id=?"), id).Scan(&importedAt); e != nil {
				return nil, e
			}
			if importedAt <= forgottenCutoff(state) {
				if item.Reconciliation != nil {
					return nil, consultation.ErrNotFound
				}
				continue
			}
		}
		// An existing same decision is stable across import keys and purchase revisions.
		var linked string
		if id != 0 {
			err = tx.QueryRowContext(ctx, q("SELECT purchase_id FROM purchase_csv_decisions WHERE user_id=:user_id AND transaction_id=? AND action='same'"), id).Scan(&linked)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}
		if linked != "" {
			if d := item.Reconciliation; d != nil && (d.Action != "same" || d.PurchaseID != linked) {
				return nil, fmt.Errorf("CSVの購入照合は確定済みです。再確認してください")
			}
			continue
		}
		candidates := []domain.ReconciliationCandidate{}
		for _, p := range state.Purchases {
			if p.RecordedAt <= forgottenCutoff(state) {
				continue
			}
			if p.ActualAmount == nil && p.Status != "not_purchased" {
				continue
			}
			if id != 0 {
				var action string
				e := tx.QueryRowContext(ctx, q("SELECT action FROM purchase_csv_decisions WHERE user_id=:user_id AND transaction_id=? AND purchase_id=?"), id, p.ID).Scan(&action)
				if e == nil && action == "distinct" {
					continue
				}
				if e != nil && !errors.Is(e, sql.ErrNoRows) {
					return nil, e
				}
			}
			candidates = append(candidates, domain.ReconciliationCandidate{ID: p.ID, Description: p.Description, PlannedAmount: p.Amount, ActualAmount: p.ActualAmount, Status: p.Status, Classification: p.Classification, RecordedAt: p.RecordedAt, PurchaseDate: p.PurchaseDate, Snapshot: consultation.PurchaseSnapshot(p)})
		}
		if d := item.Reconciliation; d != nil {
			if d.StateVersion != state.Version {
				return nil, consultation.ErrConflict
			}
			if d.Action != "same" && d.Action != "distinct" {
				return nil, fmt.Errorf("CSVの購入照合の指定が不正です")
			}
			chosen := []domain.ReconciliationCandidate{}
			for _, c := range candidates {
				if d.PurchaseID == c.ID || (d.Action == "distinct" && d.PurchaseID == "") {
					chosen = append(chosen, c)
				}
			}
			if len(chosen) == 0 {
				return nil, fmt.Errorf("CSVの購入候補が変更されました。再確認してください")
			}
			if d.PurchaseID != "" && (len(chosen) != 1 || chosen[0].Snapshot != d.Snapshot) {
				return nil, consultation.ErrConflict
			}
			if d.Action == "same" {
				if seenPurchases[d.PurchaseID] {
					return nil, fmt.Errorf("同じ購入をCSVの複数明細へ割り当てられません")
				}
				seenPurchases[d.PurchaseID] = true
				var previous int64
				e := tx.QueryRowContext(ctx, q("SELECT transaction_id FROM purchase_csv_decisions WHERE user_id=:user_id AND purchase_id=? AND action='same'"), d.PurchaseID).Scan(&previous)
				if e == nil && previous != id {
					return nil, fmt.Errorf("この購入は別のCSV明細と照合済みです")
				}
				if e != nil && !errors.Is(e, sql.ErrNoRows) {
					return nil, e
				}
			}
			if write {
				if id == 0 {
					return nil, fmt.Errorf("CSV明細が保存されていません")
				}
				original, e := scanIdentity(tx.QueryRowContext(ctx, q("SELECT "+identityColumns+" FROM transactions t WHERE t.user_id=:user_id AND t.id=?"), id))
				if e != nil {
					return nil, e
				}
				for _, c := range chosen {
					var b [16]byte
					if _, e = rand.Read(b[:]); e != nil {
						return nil, e
					}
					ref := hex.EncodeToString(b[:])
					status := "applied"
					if d.Action == "same" {
						status = "pending"
					}
					inclusion := "unknown"
					if newRows[item.Fingerprint] {
						inclusion = "new_csv"
					}
					resolved := false
					if d.Action == "same" && inclusion == "new_csv" {
						for _, p := range state.Purchases {
							// Identity is already explicit. Equal applied metadata proves
							// this new bank observation needs no additional money operation.
							if p.ID == c.ID && p.AccountingTracked && p.Status != "not_purchased" && p.ActualAmount != nil && *p.ActualAmount == original.Amount && sameAppliedAmount(p.BalanceAppliedAmount, p.ActualAmount) && ((p.Classification == "free" && sameAppliedAmount(p.FreeBudgetAppliedAmount, p.ActualAmount)) || (p.Classification == "necessary" && p.FreeBudgetAppliedAmount == nil)) {
								resolved, status = true, "applied"
							}
						}
					}
					_, e = tx.ExecContext(ctx, q("INSERT INTO purchase_csv_decisions(user_id,transaction_id,purchase_id,action,ref,csv_snapshot,purchase_revision,prior_inclusion,financial_status,balance_resolved,budget_resolved,recorded_at) VALUES(:user_id,?,?,?,?,?,?,?,?,?,?,?)"), id, c.ID, d.Action, ref, domain.ImportIdentitySnapshot(original), c.Snapshot, inclusion, status, resolved, resolved, time.Now().UTC().Format(time.RFC3339))
					if e != nil {
						return nil, e
					}
					if resolved {
						_, e = tx.ExecContext(ctx, q("INSERT INTO purchase_csv_receipts(user_id,ref,purchase_revision,status,balance_resolved,budget_resolved,state_version,recorded_at) VALUES(:user_id,?,?,'applied',?,?,?,?)"), ref, c.Snapshot, true, true, state.Version+1, time.Now().UTC().Format(time.RFC3339))
						if e != nil {
							return nil, e
						}
					}
				}
			}
			// A pair-specific distinct answer can leave other purchase candidates unresolved.
			if d.Action == "distinct" && d.PurchaseID != "" && len(candidates) > 1 {
				remaining := []domain.ReconciliationCandidate{}
				for _, c := range candidates {
					if c.ID != d.PurchaseID {
						remaining = append(remaining, c)
					}
				}
				conflicts = append(conflicts, domain.ReconciliationConflict{Row: item.ImportRow, Transaction: item, Candidates: remaining, StateVersion: state.Version})
			}
			continue
		}
		if len(candidates) > 0 {
			item.RawData = nil
			conflicts = append(conflicts, domain.ReconciliationConflict{Row: item.ImportRow, Transaction: item, Candidates: candidates, StateVersion: state.Version})
		}
	}
	return conflicts, nil
}
func (r *Repository) LoadReconciliations(ctx context.Context, cutoff string) ([]consultation.ReconciliationContext, error) {
	state, err := r.LoadChatState(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, chatQuery(ctx, "SELECT d.ref,d.purchase_id,t.transaction_date,t.amount,t.category,d.prior_inclusion,d.balance_resolved,d.budget_resolved FROM purchase_csv_decisions d JOIN effective_transactions t ON t.id=d.transaction_id AND t.user_id=d.user_id WHERE d.user_id=:user_id AND d.action='same' AND d.financial_status!='applied' AND t.imported_at>? ORDER BY d.recorded_at LIMIT 20"), cutoff)
	if err != nil {
		return nil, err
	}
	var result []consultation.ReconciliationContext
	for rows.Next() {
		var c consultation.ReconciliationContext
		if err = rows.Scan(&c.Ref, &c.PurchaseID, &c.CSV.Date, &c.CSV.Amount, &c.CSV.Category, &c.PriorInclusion, &c.BalanceResolved, &c.BudgetResolved); err != nil {
			rows.Close()
			return nil, err
		}
		for _, p := range state.Purchases {
			if p.ID == c.PurchaseID && p.RecordedAt > cutoff {
				c.CSV.Category = consultation.Redact(c.CSV.Category)
				category := []rune(c.CSV.Category)
				if len(category) > 100 {
					c.CSV.Category = string(category[:100])
				}
				c.BalanceResolved = (c.BalanceResolved || c.PriorInclusion == "new_csv") && sameAppliedAmount(p.BalanceAppliedAmount, p.ActualAmount)
				c.BudgetResolved = (c.BudgetResolved || c.PriorInclusion == "new_csv") && ((p.Classification == "free" && sameAppliedAmount(p.FreeBudgetAppliedAmount, p.ActualAmount)) || ((p.Classification == "necessary" || p.Status == "not_purchased") && p.FreeBudgetAppliedAmount == nil))
				c.PurchaseRevision = consultation.PurchaseSnapshot(p)
				result = append(result, c)
				break
			}
		}
	}
	err = rows.Err()
	rows.Close()
	return result, err
}
func syncReconciliationReceipts(ctx context.Context, tx *sql.Tx, state consultation.State, reply consultation.Reply) error {
	q := func(s string) string { return chatQuery(ctx, s) }
	for _, p := range state.Purchases {
		revision := consultation.PurchaseSnapshot(p)
		targeted := false
		for _, u := range reply.PurchaseUpdates {
			if u.ID == p.ID {
				targeted = true
			}
		}
		if targeted {
			balance, budget := false, false
			for _, c := range reply.Changes {
				if c.Operation != "set" || (c.Basis != "purchase" && c.Basis != "user_report") {
					continue
				}
				if c.Key == "current_balance" && sameAppliedAmount(p.BalanceAppliedAmount, p.ActualAmount) {
					balance = true
				}
				if c.Key == "monthly_free_budget" && ((p.Classification == "free" && sameAppliedAmount(p.FreeBudgetAppliedAmount, p.ActualAmount)) || ((p.Classification == "necessary" || p.Status == "not_purchased") && p.FreeBudgetAppliedAmount == nil)) {
					budget = true
				}
			}
			if _, err := tx.ExecContext(ctx, q("UPDATE purchase_csv_decisions SET balance_resolved=(balance_resolved OR ?),budget_resolved=(budget_resolved OR ?) WHERE user_id=:user_id AND purchase_id=? AND action='same'"), balance, budget, p.ID); err != nil {
				return err
			}
		}

		// A fully reflected correction of an already applied link is another completed
		// revision. Preserve that result without reinterpreting the bank amount.
		complete := targeted && len(reply.ReconciliationUpdates) == 0 && !p.CSVReconciliationPending && p.ActualAmount != nil && p.Classification != "unknown" && sameAppliedAmount(p.BalanceAppliedAmount, p.ActualAmount) && ((p.Classification == "free" && sameAppliedAmount(p.FreeBudgetAppliedAmount, p.ActualAmount)) || (p.Classification == "necessary" && p.FreeBudgetAppliedAmount == nil))
		if _, err := tx.ExecContext(ctx, q("UPDATE purchase_csv_decisions SET financial_status=CASE WHEN financial_status='applied' AND ? THEN 'applied' ELSE 'pending' END,purchase_revision=? WHERE user_id=:user_id AND purchase_id=? AND action='same' AND purchase_revision!=?"), complete, revision, p.ID, revision); err != nil {
			return err
		}
		if complete {
			if _, err := tx.ExecContext(ctx, q("INSERT INTO purchase_csv_receipts(user_id,ref,purchase_revision,status,balance_resolved,budget_resolved,state_version,recorded_at) SELECT user_id,ref,purchase_revision,financial_status,balance_resolved,budget_resolved,?,? FROM purchase_csv_decisions WHERE user_id=:user_id AND purchase_id=? AND action='same' AND financial_status='applied' ON CONFLICT(user_id,ref,purchase_revision) DO NOTHING"), state.Version+1, time.Now().UTC().Format(time.RFC3339), p.ID); err != nil {
				return err
			}
		}
	}
	for _, u := range reply.ReconciliationUpdates {
		var revision string
		err := tx.QueryRowContext(ctx, q("SELECT purchase_revision FROM purchase_csv_decisions WHERE user_id=:user_id AND ref=? AND action='same' AND financial_status!='applied'"), u.Ref).Scan(&revision)
		if err != nil {
			return consultation.ErrConflict
		}
		balance, budget := false, false
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
		if u.Status == "applied" {
			// The validator has established both dimensions, including unchanged
			// already-reflected values. Persist that evidence explicitly.
			balance, budget = true, true
		}
		if _, err = tx.ExecContext(ctx, q("UPDATE purchase_csv_decisions SET financial_status=?,balance_resolved=(balance_resolved OR ?),budget_resolved=(budget_resolved OR ?) WHERE user_id=:user_id AND ref=?"), u.Status, balance, budget, u.Ref); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, q("INSERT INTO purchase_csv_receipts(user_id,ref,purchase_revision,status,balance_resolved,budget_resolved,state_version,recorded_at) SELECT user_id,ref,purchase_revision,financial_status,balance_resolved,budget_resolved,?,? FROM purchase_csv_decisions WHERE user_id=:user_id AND ref=? ON CONFLICT(user_id,ref,purchase_revision) DO UPDATE SET status=excluded.status,balance_resolved=excluded.balance_resolved,budget_resolved=excluded.budget_resolved,state_version=excluded.state_version,recorded_at=excluded.recorded_at"), state.Version+1, time.Now().UTC().Format(time.RFC3339), u.Ref); err != nil {
			return err
		}
	}
	return nil
}
func (r *Repository) ExpenseCount(ctx context.Context, month string) (domain.ExpenseCount, error) {
	state, err := r.LoadChatState(ctx)
	if err != nil {
		return domain.ExpenseCount{}, err
	}
	result := domain.ExpenseCount{Month: month, Scope: "dated_confirmed_and_undated_are_disjoint; estimated_is_subset_of_dated_confirmed; bank_statement_date_when_purchase_date_unknown"}
	add := func(date string, estimated bool) {
		if date == "" {
			result.Undated++
			return
		}
		if month == "" || strings.HasPrefix(date, month) {
			result.Confirmed++
			if estimated {
				result.Estimated++
			}
		}
	}
	rows, err := r.db.QueryContext(ctx, chatQuery(ctx, "SELECT t.transaction_date,COALESCE(d.purchase_id,''),COALESCE(d.financial_status,'') FROM effective_transactions t LEFT JOIN purchase_csv_decisions d ON d.transaction_id=t.id AND d.user_id=t.user_id AND d.action='same' WHERE t.user_id=:user_id AND t.transaction_type='expense'"))
	if err != nil {
		return result, err
	}
	dates := map[string]string{}
	statuses := map[string]string{}
	for rows.Next() {
		var date, pid, status string
		if err = rows.Scan(&date, &pid, &status); err != nil {
			rows.Close()
			return result, err
		}
		if pid == "" {
			add(date, false)
		} else {
			dates[pid] = date
			statuses[pid] = status
			if status != "applied" {
				result.PendingRecalculation++
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	for _, p := range state.Purchases {
		if p.Status == "not_purchased" {
			if dates[p.ID] != "" && statuses[p.ID] != "applied" {
				result.Unresolved++
			}
			continue
		}
		if p.ActualAmount == nil {
			continue
		}
		if p.CSVReconciliationPending && dates[p.ID] == "" {
			unresolved, e := r.RequiresCSVReconciliation(ctx, p.ID)
			if e != nil {
				return result, e
			}
			if unresolved {
				result.Unresolved++
				continue
			}
			result.PendingRecalculation++
		}

		date := p.PurchaseDate
		if date == "" {
			date = dates[p.ID]
		}
		estimated := consultation.ActualAmountProvenance(p) == "estimated"
		add(date, estimated)
	}
	var pending int
	if err = r.db.QueryRowContext(ctx, chatQuery(ctx, "SELECT count(*) FROM csv_financial_receipts WHERE user_id=:user_id AND status IN ('pending','needs_information') AND recorded_at>?"), forgottenCutoff(state)).Scan(&pending); err != nil {
		return result, err
	}
	result.PendingRecalculation += pending
	return result, nil
}
func (r *Repository) PurchaseCSVCandidates(ctx context.Context, purchaseID string) ([]domain.ReconciliationConflict, error) {
	state, err := r.LoadChatState(ctx)
	if err != nil {
		return nil, err
	}
	var purchase *consultation.Purchase
	for i := range state.Purchases {
		if state.Purchases[i].ID == purchaseID {
			purchase = &state.Purchases[i]
		}
	}
	if purchase == nil || purchase.RecordedAt <= forgottenCutoff(state) {
		return nil, consultation.ErrNotFound
	}
	rows, err := r.ListTransactions(ctx, "", 100)
	if err != nil {
		return nil, err
	}
	var result []domain.ReconciliationConflict
	for _, t := range rows {
		if t.TransactionType != "expense" || t.ImportedAt <= forgottenCutoff(state) {
			continue
		}
		var count int
		if err = r.db.QueryRowContext(ctx, chatQuery(ctx, "SELECT count(*) FROM purchase_csv_decisions WHERE user_id=:user_id AND transaction_id=? AND (action='same' OR purchase_id=?)"), t.ID, purchaseID).Scan(&count); err != nil {
			return nil, err
		}
		if count > 0 {
			continue
		}
		t.RawData = nil
		result = append(result, domain.ReconciliationConflict{Row: int(t.ID), Transaction: t, StateVersion: state.Version, Candidates: []domain.ReconciliationCandidate{{ID: purchase.ID, Description: purchase.Description, ActualAmount: purchase.ActualAmount, PlannedAmount: purchase.Amount, Status: purchase.Status, Classification: purchase.Classification, RecordedAt: purchase.RecordedAt, PurchaseDate: purchase.PurchaseDate, Snapshot: consultation.PurchaseSnapshot(*purchase)}}})
	}
	return result, nil
}
func (r *Repository) ResolvePurchaseCSV(ctx context.Context, id int64, d domain.ReconciliationDecision) error {
	seed, err := r.LoadChatState(ctx)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	state, err := reconciliationState(ctx, tx, &seed, true)
	if err != nil {
		return err
	}
	var items []domain.Transaction
	if id == 0 {
		if d.Action != "distinct" || d.PurchaseID == "" {
			return fmt.Errorf("CSV distinct-all purchase choice is invalid")
		}
		rows, e := tx.QueryContext(ctx, chatQuery(ctx, "SELECT "+identityColumns+" FROM effective_transactions t WHERE t.user_id=:user_id AND t.transaction_type='expense' AND t.imported_at>? AND NOT EXISTS(SELECT 1 FROM purchase_csv_decisions x WHERE x.user_id=t.user_id AND x.transaction_id=t.id AND (x.action='same' OR x.purchase_id=?)) ORDER BY t.id LIMIT 1001"), forgottenCutoff(state), d.PurchaseID)
		if e != nil {
			return e
		}
		for rows.Next() {
			item, e := scanIdentity(rows)
			if e != nil {
				rows.Close()
				return e
			}
			items = append(items, item)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(items) == 0 || len(items) > 1000 {
			return fmt.Errorf("CSV candidates changed or exceed the confirmation limit")
		}
	} else {
		item, e := scanIdentity(tx.QueryRowContext(ctx, chatQuery(ctx, "SELECT "+identityColumns+" FROM effective_transactions t WHERE t.user_id=:user_id AND t.id=?"), id))
		if e != nil || item.ImportedAt <= forgottenCutoff(state) {
			return consultation.ErrNotFound
		}
		items = append(items, item)
	}
	for i := range items {
		decision := d
		items[i].Reconciliation = &decision
	}
	_, err = reconcileImports(ctx, tx, items, state, true, map[string]bool{})
	if err != nil {
		return err
	}

	state.Version++
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, chatQuery(ctx, "UPDATE chat_states SET version=?,payload=? WHERE user_id=:user_id AND version=?"), state.Version, string(payload), state.Version-1); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) RequiresCSVReconciliation(ctx context.Context, purchaseID string) (bool, error) {
	state, err := r.LoadChatState(ctx)
	if err != nil {
		return false, err
	}
	cutoff := forgottenCutoff(state)
	var count int
	err = r.db.QueryRowContext(ctx, chatQuery(ctx, "SELECT count(*) FROM effective_transactions t WHERE t.user_id=:user_id AND t.transaction_type='expense' AND t.imported_at>? AND NOT EXISTS(SELECT 1 FROM purchase_csv_decisions d WHERE d.user_id=t.user_id AND d.transaction_id=t.id AND (d.action='same' OR d.purchase_id=?))"), cutoff, purchaseID).Scan(&count)
	return count > 0, err
}

func sameAppliedAmount(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
