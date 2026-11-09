package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"strings"
)

const identityColumns = "t.id,t.transaction_date,t.description,t.normalized_merchant,t.amount,t.transaction_type,t.category,t.source,t.source_account_name,t.fingerprint,t.imported_at,t.raw_data_json,t.identity_version,t.provider_transaction_id,t.import_file_key,t.legacy_fingerprint"

func scanIdentity(row interface{ Scan(...any) error }) (domain.Transaction, error) {
	var t domain.Transaction
	var raw string
	err := row.Scan(&t.ID, &t.TransactionDate, &t.Description, &t.NormalizedMerchant, &t.Amount, &t.TransactionType, &t.Category, &t.Source, &t.SourceAccountName, &t.Fingerprint, &t.ImportedAt, &raw, &t.IdentityVersion, &t.ProviderTransactionID, &t.ImportFileKey, &t.LegacyFingerprint)
	if err == nil {
		err = json.Unmarshal([]byte(raw), &t.RawData)
	}
	return t, err
}
func sameIdentityContent(a, b domain.Transaction) bool {
	return a.TransactionDate == b.TransactionDate && a.Amount == b.Amount && a.TransactionType == b.TransactionType && a.Description == b.Description && a.Source == b.Source && a.SourceAccountName == b.SourceAccountName
}
func (r *Repository) PreviewTransactionIdentities(ctx context.Context, items []domain.Transaction) ([]domain.ImportIdentityConflict, error) {
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, conflicts, err := processIdentities(ctx, tx, items, false)
	return conflicts, err
}
func (r *Repository) ImportTransactions(ctx context.Context, items []domain.Transaction) (domain.ImportCommitResult, error) {
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.ImportCommitResult{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, userQuery(ctx, "SELECT pg_advisory_xact_lock(?)"), -identity.UserID(ctx)-1); err != nil {
		return domain.ImportCommitResult{}, err
	}
	result, conflicts, err := processIdentities(ctx, tx, items, true)
	if err != nil {
		return domain.ImportCommitResult{}, err
	}
	if len(conflicts) > 0 {
		return domain.ImportCommitResult{}, fmt.Errorf("CSVの同一明細か別明細か確認が必要です。再度プレビューしてください")
	}
	if err = tx.Commit(); err != nil {
		return domain.ImportCommitResult{}, err
	}
	return result, nil
}
func processIdentities(ctx context.Context, tx *sql.Tx, items []domain.Transaction, write bool) (domain.ImportCommitResult, []domain.ImportIdentityConflict, error) {
	query := func(q string) string { return userQuery(ctx, q) }
	// Validate both directions on every path, including provider-key duplicates.
	fileBinding := func(item domain.Transaction, id int64) error {
		var target int64
		err := tx.QueryRowContext(ctx, query("SELECT transaction_id FROM csv_identity_file_rows WHERE user_id=:user_id AND file_key=? AND source_scope=? AND account_scope=? AND transaction_type=? AND row_number=?"), item.ImportFileKey, strings.ToLower(item.Source), strings.ToLower(item.SourceAccountName), item.TransactionType, item.ImportRow).Scan(&target)
		if err == nil && target != id {
			return fmt.Errorf("CSVの物理行は別の明細へ確定済みです。再プレビューしてください")
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var row int
		err = tx.QueryRowContext(ctx, query("SELECT row_number FROM csv_identity_file_rows WHERE user_id=:user_id AND file_key=? AND source_scope=? AND account_scope=? AND transaction_type=? AND transaction_id=?"), item.ImportFileKey, strings.ToLower(item.Source), strings.ToLower(item.SourceAccountName), item.TransactionType, id).Scan(&row)
		if err == nil && row != item.ImportRow {
			return fmt.Errorf("CSVの複数物理行へ同じ明細を割り当てられません")
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if write {
			_, err = tx.ExecContext(ctx, query("INSERT INTO csv_identity_file_rows(user_id,file_key,source_scope,account_scope,transaction_type,row_number,transaction_id) VALUES(:user_id,?,?,?,?,?,?) ON CONFLICT(user_id,file_key,source_scope,account_scope,transaction_type,row_number) DO NOTHING"), item.ImportFileKey, strings.ToLower(item.Source), strings.ToLower(item.SourceAccountName), item.TransactionType, item.ImportRow, id)
			if err != nil {
				return err
			}
			// Check the row again even if a concurrent insert won the conflict.
			err = tx.QueryRowContext(ctx, query("SELECT transaction_id FROM csv_identity_file_rows WHERE user_id=:user_id AND file_key=? AND source_scope=? AND account_scope=? AND transaction_type=? AND row_number=?"), item.ImportFileKey, strings.ToLower(item.Source), strings.ToLower(item.SourceAccountName), item.TransactionType, item.ImportRow).Scan(&target)
			if err != nil {
				return err
			}
			if target != id {
				return fmt.Errorf("CSVの物理行の対応が競合しました。再プレビューしてください")
			}
		}
		return nil
	}
	// A provider ID may identify one physical row per import. Reject the whole
	// batch before saving anything; silently collapsing repeats loses provenance.
	providerIDs := map[string]bool{}
	for _, item := range items {
		if item.IdentityVersion != 2 || item.ProviderTransactionID == "" {
			continue
		}
		key, _ := json.Marshal([]string{item.Source, item.SourceAccountName, item.TransactionType, item.ProviderTransactionID})
		if providerIDs[string(key)] {
			return domain.ImportCommitResult{}, nil, fmt.Errorf("CSV内で同じ取引IDが繰り返されています。取引ID列と重複行を確認してください")
		}
		providerIDs[string(key)] = true
	}
	result := domain.ImportCommitResult{}
	conflicts := []domain.ImportIdentityConflict{}
	used := map[int64]string{}
	seen := map[string]domain.Transaction{}

	group := func(item domain.Transaction) string {
		value, _ := json.Marshal([]any{item.TransactionDate, item.Amount, item.TransactionType, strings.ToLower(item.Source), strings.ToLower(item.SourceAccountName)})
		return string(value)
	}
	candidateGroups := map[string][]domain.Transaction{}
	// Capture candidates before changing any legacy row, so a prior match cannot
	// make a later unresolved occurrence disappear from confirmation.
	for _, item := range items {
		if item.IdentityVersion != 2 {
			continue
		}
		key := group(item)
		if _, ok := candidateGroups[key]; ok {
			continue
		}
		candidates := []domain.Transaction{}
		rows, e := tx.QueryContext(ctx, query("SELECT "+identityColumns+" FROM transactions t WHERE t.user_id=:user_id AND t.transaction_date=? AND t.amount=? AND t.transaction_type=? AND lower(t.source)=lower(?) AND lower(t.source_account_name)=lower(?) ORDER BY t.id"), item.TransactionDate, item.Amount, item.TransactionType, item.Source, item.SourceAccountName)
		if e != nil {
			return result, nil, e
		}
		for rows.Next() {
			candidate, e := scanIdentity(rows)
			if e != nil {
				rows.Close()
				return result, nil, e
			}
			candidates = append(candidates, candidate)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return result, nil, e
		}
		candidateGroups[key] = candidates
	}
	for _, item := range items {
		if previous, ok := seen[item.Fingerprint]; ok {
			if !sameIdentityContent(previous, item) {
				return result, nil, fmt.Errorf("CSVの同じ取引IDで内容が異なります")
			}
			result.DuplicateCount++
			continue
		}
		seen[item.Fingerprint] = item
		var boundID int64
		if item.IdentityVersion == 2 {
			e := tx.QueryRowContext(ctx, query("SELECT transaction_id FROM csv_identity_file_rows WHERE user_id=:user_id AND file_key=? AND source_scope=? AND account_scope=? AND transaction_type=? AND row_number=?"), item.ImportFileKey, strings.ToLower(item.Source), strings.ToLower(item.SourceAccountName), item.TransactionType, item.ImportRow).Scan(&boundID)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return result, nil, e
			}
		}
		existing, err := scanIdentity(tx.QueryRowContext(ctx, query("SELECT "+identityColumns+" FROM transactions t WHERE t.user_id=:user_id AND t.fingerprint=?"), item.Fingerprint))
		if errors.Is(err, sql.ErrNoRows) && item.IdentityVersion == 2 {
			existing, err = scanIdentity(tx.QueryRowContext(ctx, query("SELECT "+identityColumns+" FROM csv_identity_aliases a JOIN transactions t ON t.id=a.transaction_id AND t.user_id=a.user_id WHERE a.user_id=:user_id AND a.row_key=?"), item.Fingerprint))
		}

		aliasFound := false
		var aliasID int64
		var aliasAction, aliasIncoming, aliasSnapshot string
		if item.IdentityVersion == 2 {
			e := tx.QueryRowContext(ctx, query("SELECT transaction_id,decision_action,incoming_snapshot,candidate_snapshot FROM csv_identity_aliases WHERE user_id=:user_id AND row_key=?"), item.Fingerprint).Scan(&aliasID, &aliasAction, &aliasIncoming, &aliasSnapshot)
			if e == nil {
				aliasFound = true
				if (item.IdentityAction == "same" && aliasID != item.IdentityTransactionID) || (item.IdentityAction == "distinct" && aliasAction == "same") {
					return result, nil, fmt.Errorf("CSVの確認結果は別の取込で確定済みです。再プレビューしてください")
				}
			} else if !errors.Is(e, sql.ErrNoRows) {
				return result, nil, e
			}
		}
		if err == nil {
			if item.IdentityVersion == 2 && ((aliasFound && (aliasIncoming != domain.ImportIdentitySnapshot(item) || aliasSnapshot != domain.ImportIdentitySnapshot(existing))) || (!aliasFound && !sameIdentityContent(existing, item))) {
				return result, nil, fmt.Errorf("CSVの取引IDが既存明細と異なる内容です")
			}
			if previous, ok := used[existing.ID]; ok && previous != item.Fingerprint {
				return result, nil, fmt.Errorf("同じ既存明細をCSVの複数行へ割り当てることはできません")
			}
			used[existing.ID] = item.Fingerprint
			if item.IdentityVersion == 2 {
				if e := fileBinding(item, existing.ID); e != nil {
					return result, nil, e
				}
			}
			result.DuplicateCount++
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return result, nil, err
		}
		candidates := []domain.Transaction{}
		if item.IdentityVersion == 2 {
			for _, candidate := range candidateGroups[group(item)] {
				if candidate.ID != boundID && item.ProviderTransactionID != "" && candidate.ProviderTransactionID != "" && candidate.ProviderTransactionID != item.ProviderTransactionID {
					continue
				}
				candidates = append(candidates, candidate)
			}
		}
		if boundID != 0 {
			found := false
			for _, candidate := range candidates {
				if candidate.ID == boundID {
					found = true
				}
			}
			if !found {
				return result, nil, fmt.Errorf("CSVの確定済み物理行の内容が変わりました。取込を確認してください")
			}
		}
		var chosen *domain.Transaction
		switch item.IdentityAction {
		case "same":
			for i := range candidates {
				if candidates[i].ID == item.IdentityTransactionID {
					chosen = &candidates[i]
					break
				}
			}
			if chosen == nil {
				return result, nil, fmt.Errorf("CSVの同一明細の指定が不正です。候補を再確認してください")
			}

			suffix := ""
			if write {
				suffix = " FOR UPDATE"
			}
			fresh, e := scanIdentity(tx.QueryRowContext(ctx, query("SELECT "+identityColumns+" FROM transactions t WHERE t.user_id=:user_id AND t.id=?"+suffix), chosen.ID))
			if e != nil {
				return result, nil, e
			}
			if item.IdentityCandidateSnapshot == "" || item.IdentityCandidateSnapshot != domain.ImportIdentitySnapshot(fresh) {
				return result, nil, fmt.Errorf("CSVの既存候補が変更されました。再プレビューしてください")
			}
			chosen = &fresh
			if boundID != 0 && boundID != chosen.ID {
				return result, nil, fmt.Errorf("CSVの物理行は別の明細へ確定済みです")
			}
			if !write {
				if e := fileBinding(item, chosen.ID); e != nil {
					return result, nil, e
				}
			}
			if previous, ok := used[chosen.ID]; ok && previous != item.Fingerprint {
				return result, nil, fmt.Errorf("同じ既存明細をCSVの複数行へ割り当てることはできません")
			}
			used[chosen.ID] = item.Fingerprint
		case "distinct":
			if boundID != 0 {
				return result, nil, fmt.Errorf("CSVの物理行は既存明細へ確定済みです。別明細として追加できません")
			}
			if item.IdentityTransactionID != 0 {
				return result, nil, fmt.Errorf("別明細に既存IDを指定できません")
			}
		case "":
			if len(candidates) > 0 {
				snapshots := map[int64]string{}
				for _, candidate := range candidates {
					snapshots[candidate.ID] = domain.ImportIdentitySnapshot(candidate)
				}
				conflicts = append(conflicts, domain.ImportIdentityConflict{Row: item.ImportRow, Transaction: item, Candidates: candidates, CandidateSnapshots: snapshots})
				continue
			}
		default:
			return result, nil, fmt.Errorf("CSVの確認指定が不正です")
		}
		if !write {
			if chosen != nil {
				result.DuplicateCount++
			} else {
				result.ImportedCount++
			}
			continue
		}
		id := int64(0)
		if chosen != nil {
			id = chosen.ID
			// Keep the original row, raw record, category corrections and timestamps.
			if chosen.IdentityVersion < 2 {
				if _, err = tx.ExecContext(ctx, query("UPDATE transactions SET legacy_fingerprint=fingerprint,fingerprint=?,identity_version=2,provider_transaction_id=?,import_file_key=? WHERE user_id=:user_id AND id=? AND identity_version<2"), item.Fingerprint, item.ProviderTransactionID, item.ImportFileKey, id); err != nil {
					return result, nil, err
				}
			}
			result.DuplicateCount++
		} else {
			raw, e := json.Marshal(item.RawData)
			if e != nil {
				return result, nil, e
			}
			version := item.IdentityVersion
			if version == 0 {
				version = 1
			}
			insert := "INSERT INTO transactions(user_id,transaction_date,description,normalized_merchant,amount,transaction_type,category,source,source_account_name,fingerprint,imported_at,raw_data_json,identity_version,provider_transaction_id,import_file_key) VALUES(:user_id,?,?,?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id"
			err = tx.QueryRowContext(ctx, query(insert), item.TransactionDate, item.Description, item.NormalizedMerchant, item.Amount, item.TransactionType, item.Category, item.Source, item.SourceAccountName, item.Fingerprint, item.ImportedAt, string(raw), version, item.ProviderTransactionID, item.ImportFileKey).Scan(&id)
			if err != nil {
				return result, nil, err
			}

			result.ImportedCount++
		}
		if item.IdentityVersion == 2 {
			if err = fileBinding(item, id); err != nil {
				return result, nil, err
			}
			if _, err = tx.ExecContext(ctx, query("INSERT INTO csv_identity_aliases(user_id,row_key,row_number,file_key,source_scope,account_scope,transaction_type,transaction_id,decision_action,incoming_snapshot,candidate_snapshot) VALUES(:user_id,?,?,?,?,?,?,?,?,?,?)"), item.Fingerprint, item.ImportRow, item.ImportFileKey, strings.ToLower(item.Source), strings.ToLower(item.SourceAccountName), item.TransactionType, id, func() string {
				if chosen != nil {
					return "same"
				}
				return "distinct"
			}(), domain.ImportIdentitySnapshot(item), func() string {
				if chosen != nil {
					return domain.ImportIdentitySnapshot(*chosen)
				}
				saved := item
				saved.ID = id
				return domain.ImportIdentitySnapshot(saved)
			}()); err != nil {
				return result, nil, fmt.Errorf("CSVの同一明細の割当が競合しました。再プレビューしてください: %w", err)
			}
		}
	}
	return result, conflicts, nil
}
