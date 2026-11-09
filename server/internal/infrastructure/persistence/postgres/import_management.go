package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"strconv"
	"strings"
	"time"
)

type managementDB interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func managementID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func managementHash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func managementRows(ctx context.Context, db managementDB, ids []int64, active bool) ([]domain.Transaction, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	seen := map[int64]bool{}
	unique := []int64{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	ids = unique
	if len(ids) > 500 {
		var all []domain.Transaction
		for start := 0; start < len(ids); start += 500 {
			end := start + 500
			if end > len(ids) {
				end = len(ids)
			}
			part, err := managementRows(ctx, db, ids[start:end], active)
			if err != nil {
				return nil, err
			}
			all = append(all, part...)
		}
		return all, nil
	}
	args := []any{}
	marks := []string{}
	for _, id := range ids {
		marks = append(marks, "?")
		args = append(args, id)
	}
	table := "transactions"
	columns := identityColumns + ",t.revision"
	if active {
		table = "effective_transactions"
	} else {
		columns = "t.id,COALESCE(t.effective_date,t.transaction_date),t.description,t.normalized_merchant,COALESCE(t.effective_amount,t.amount),t.transaction_type,COALESCE(t.effective_category,t.category),t.source,t.source_account_name,t.fingerprint,t.imported_at,t.raw_data_json,t.identity_version,t.provider_transaction_id,t.import_file_key,t.legacy_fingerprint,t.revision"
	}
	rows, err := db.QueryContext(ctx, userQuery(ctx, "SELECT "+columns+" FROM "+table+" t WHERE t.user_id=:user_id AND t.id IN ("+strings.Join(marks, ",")+") ORDER BY t.id"), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Transaction
	for rows.Next() {
		var t domain.Transaction
		var raw string
		if err = rows.Scan(&t.ID, &t.TransactionDate, &t.Description, &t.NormalizedMerchant, &t.Amount, &t.TransactionType, &t.Category, &t.Source, &t.SourceAccountName, &t.Fingerprint, &t.ImportedAt, &raw, &t.IdentityVersion, &t.ProviderTransactionID, &t.ImportFileKey, &t.LegacyFingerprint, &t.Revision); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &t.RawData); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func recordImportBatch(ctx context.Context, tx *sql.Tx, items []domain.Transaction, result *domain.ImportCommitResult, prior map[int64]bool, state consultation.State) error {
	if len(items) == 0 {
		return nil
	}
	meta := domain.ImportMetadata{}
	if items[0].ImportMetadata != nil {
		meta = *items[0].ImportMetadata
	}
	id := managementID()
	if meta.CommitKey == "" {
		meta.CommitKey = id
	}
	result.ImportID = id
	recorded := time.Now().UTC().Format(time.RFC3339)
	metadata, _ := json.Marshal(meta)
	if _, err := tx.ExecContext(ctx, userQuery(ctx, "INSERT INTO csv_import_batches(id,user_id,commit_key,file_key,source_scope,account_scope,metadata,status,recorded_at,result) VALUES(?,:user_id,?,?,?,?,?,'active',?,'{}')"), id, meta.CommitKey, items[0].ImportFileKey, strings.ToLower(items[0].Source), strings.ToLower(items[0].SourceAccountName), string(metadata), recorded); err != nil {
		return err
	}
	counted := map[int64]bool{}
	for i, item := range items {
		target, err := resolveImportID(ctx, tx, item)
		if err != nil {
			return err
		}
		if target == 0 {
			return errors.New("CSV evidence target missing")
		}
		if active, known := prior[target]; known && !active && !counted[target] {
			if !meta.ConfirmReactivation {
				return fmt.Errorf("取消した明細の再有効化を確認してください")
			}
			result.ReactivatedCount++
			counted[target] = true
		}
		row := item.ImportRow
		if row < 2 {
			row = i + 2
		}
		observation, _ := json.Marshal(item)
		if _, err = tx.ExecContext(ctx, userQuery(ctx, "INSERT INTO csv_import_evidence(user_id,batch_id,row_number,transaction_type,transaction_id,observation) VALUES(:user_id,?,?,?,?,?)"), id, row, item.TransactionType, target, string(observation)); err != nil {
			return err
		}
	}
	if len(counted) > 0 {
		ids := []int64{}
		for id := range counted {
			ids = append(ids, id)
		}
		restored, e := managementRows(ctx, tx, ids, true)
		if e != nil {
			return e
		}
		mutationID := "import-restore:" + id
		if _, e = tx.ExecContext(ctx, userQuery(ctx, "INSERT INTO csv_mutations(user_id,id,payload_digest,kind,target,recorded_at,result) VALUES(:user_id,?,?,?,?,?,?)"), mutationID, managementHash(meta), "restore", id, recorded, "{}"); e != nil {
			return e
		}
		beforeJSON, _ := json.Marshal([]domain.Transaction{})
		afterJSON, _ := json.Marshal(restored)
		if _, e = tx.ExecContext(ctx, userQuery(ctx, "INSERT INTO transaction_change_events(user_id,id,mutation_id,kind,target,recorded_at,before_state,after_state) VALUES(:user_id,?,?,?,?,?,?,?)"), managementID(), mutationID, "restore", id, recorded, string(beforeJSON), string(afterJSON)); e != nil {
			return e
		}
		result.FinancialPending, e = writeDataChangeReceipts(ctx, tx, state, nil, restored, mutationID, "restore", recorded)
		if e != nil {
			return e
		}
		outcome, _ := json.Marshal(domain.MutationResult{ID: mutationID, Status: "saved", Affected: len(counted), Pending: result.FinancialPending, StateVersion: state.Version + 1})
		if _, e = tx.ExecContext(ctx, userQuery(ctx, "UPDATE csv_mutations SET result=? WHERE user_id=:user_id AND id=?"), string(outcome), mutationID); e != nil {
			return e
		}
	}
	payload, _ := json.Marshal(result)
	_, err := tx.ExecContext(ctx, userQuery(ctx, "UPDATE csv_import_batches SET result=? WHERE user_id=:user_id AND id=?"), string(payload), id)
	return err
}
func (r *Repository) ImportCommit(ctx context.Context, key string) (domain.ImportCommitResult, error) {
	var raw string
	err := r.db.QueryRowContext(ctx, userQuery(ctx, "SELECT result FROM csv_import_batches WHERE user_id=:user_id AND commit_key=?"), key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ImportCommitResult{}, domain.ErrNotFound
	}
	var result domain.ImportCommitResult
	if err == nil {
		err = json.Unmarshal([]byte(raw), &result)
	}
	return result, err
}
func loadBatch(ctx context.Context, db managementDB, id string, stateVersion int64) (domain.ImportBatch, error) {
	var b domain.ImportBatch
	var meta, result string
	err := db.QueryRowContext(ctx, userQuery(ctx, "SELECT id,file_key,source_scope,account_scope,metadata,status,version,recorded_at,result,legacy FROM csv_import_batches WHERE user_id=:user_id AND id=?"), id).Scan(&b.ID, &b.FileKey, &b.Source, &b.Account, &meta, &b.Status, &b.Version, &b.RecordedAt, &result, &b.Legacy)
	if errors.Is(err, sql.ErrNoRows) {
		return b, domain.ErrNotFound
	}
	if err != nil {
		return b, err
	}
	if err = json.Unmarshal([]byte(meta), &b.Metadata); err != nil {
		return b, err
	}
	if err = json.Unmarshal([]byte(result), &b.Result); err != nil {
		return b, err
	}
	b.StateVersion = stateVersion
	rows, err := db.QueryContext(ctx, userQuery(ctx, "SELECT row_number,transaction_id,observation FROM csv_import_evidence WHERE user_id=:user_id AND batch_id=? ORDER BY row_number,transaction_type"), id)
	if err != nil {
		return b, err
	}
	for rows.Next() {
		var e domain.ImportEvidence
		var raw string
		if err = rows.Scan(&e.Row, &e.TransactionID, &raw); err != nil {
			rows.Close()
			return b, err
		}
		if err = json.Unmarshal([]byte(raw), &e.Original); err != nil {
			rows.Close()
			return b, err
		}
		b.Rows = append(b.Rows, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return b, err
	}
	ids := []int64{}
	for _, e := range b.Rows {
		ids = append(ids, e.TransactionID)
	}
	current, err := managementRows(ctx, db, ids, false)
	if err != nil {
		return b, err
	}
	active, err := managementRows(ctx, db, ids, true)
	if err != nil {
		return b, err
	}
	values := map[int64]domain.Transaction{}
	enabled := map[int64]bool{}
	for _, t := range current {
		values[t.ID] = t
	}
	for _, t := range active {
		enabled[t.ID] = true
	}
	for i := range b.Rows {
		if b.Rows[i].Original.ID == 0 && b.Rows[i].Original.Description == "" && b.Rows[i].Original.TransactionDate == "" {
			original, e := scanIdentity(db.QueryRowContext(ctx, userQuery(ctx, "SELECT "+identityColumns+" FROM transactions t WHERE t.user_id=:user_id AND t.id=?"), b.Rows[i].TransactionID))
			if e != nil {
				return b, e
			}
			b.Rows[i].Original = original
		}
		b.Rows[i].Current = values[b.Rows[i].TransactionID]
		b.Rows[i].Active = enabled[b.Rows[i].TransactionID]
		if err = db.QueryRowContext(ctx, userQuery(ctx, "SELECT legacy_retained FROM transactions WHERE user_id=:user_id AND id=?"), b.Rows[i].TransactionID).Scan(&b.Rows[i].LegacyRetained); err != nil {
			return b, err
		}
	}
	countedIDs := map[int64]bool{}
	for _, row := range b.Rows {
		if countedIDs[row.TransactionID] || !row.Active {
			continue
		}
		countedIDs[row.TransactionID] = true
		var other int
		if err = db.QueryRowContext(ctx, userQuery(ctx, "SELECT count(*) FROM csv_import_evidence e JOIN csv_import_batches b ON b.user_id=e.user_id AND b.id=e.batch_id WHERE e.user_id=:user_id AND e.transaction_id=? AND b.status='active' AND b.id!=?"), row.TransactionID, b.ID).Scan(&other); err != nil {
			return b, err
		}
		if row.LegacyRetained || other > 0 {
			b.CancelProtected++
		} else if b.Status == "active" {
			b.CancelAffected++
		}
	}
	b.Snapshot = managementHash(b)
	return b, nil
}
func (r *Repository) ImportHistory(ctx context.Context, before string, limit int) ([]domain.ImportBatch, error) {
	state, err := r.LoadChatState(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.DB.QueryContext(ctx, userQuery(ctx, "SELECT id,file_key,source_scope,account_scope,metadata,status,version,recorded_at,result,legacy FROM csv_import_batches WHERE user_id=:user_id AND (?='' OR (recorded_at,id)<((SELECT recorded_at FROM csv_import_batches WHERE user_id=:user_id AND id=?),?)) ORDER BY recorded_at DESC,id DESC LIMIT ?"), before, before, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ImportBatch
	for rows.Next() {
		var b domain.ImportBatch
		var meta, result string
		if err = rows.Scan(&b.ID, &b.FileKey, &b.Source, &b.Account, &meta, &b.Status, &b.Version, &b.RecordedAt, &result, &b.Legacy); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(meta), &b.Metadata); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(result), &b.Result); err != nil {
			return nil, err
		}
		b.StateVersion = state.Version
		out = append(out, b)
	}
	return out, rows.Err()
}
func (r *Repository) ImportDetails(ctx context.Context, id string) (domain.ImportBatch, error) {
	state, err := r.LoadChatState(ctx)
	if err != nil {
		return domain.ImportBatch{}, err
	}
	return loadBatch(ctx, r.db, id, state.Version)
}
func (r *Repository) Mutation(ctx context.Context, id string) (domain.MutationResult, error) {
	var raw string
	err := r.db.QueryRowContext(ctx, userQuery(ctx, "SELECT result FROM csv_mutations WHERE user_id=:user_id AND id=?"), id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.MutationResult{}, domain.ErrNotFound
	}
	var result domain.MutationResult
	if err == nil {
		err = json.Unmarshal([]byte(raw), &result)
	}
	return result, err
}
func (r *Repository) ChangeAudit(ctx context.Context, target string) ([]domain.TransactionChangeEvent, error) {
	rows, err := r.db.QueryContext(ctx, userQuery(ctx, "SELECT id,mutation_id,kind,target,recorded_at,before_state,after_state FROM transaction_change_events WHERE user_id=:user_id AND target=? ORDER BY recorded_at,id"), target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.TransactionChangeEvent
	for rows.Next() {
		var e domain.TransactionChangeEvent
		if err = rows.Scan(&e.ID, &e.MutationID, &e.Kind, &e.Target, &e.RecordedAt, &e.Before, &e.After); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (r *Repository) MutateCSV(ctx context.Context, target string, in domain.MutationInput) (domain.MutationResult, error) {
	if in.ID == "" || (in.Action != "correct" && in.Action != "cancel" && in.Action != "restore") {
		return domain.MutationResult{}, fmt.Errorf("操作IDまたは種別が不正です")
	}
	seed, err := r.LoadChatState(ctx)
	if err != nil {
		return domain.MutationResult{}, err
	}
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.MutationResult{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, userQuery(ctx, "SELECT pg_advisory_xact_lock(?)"), -identity.UserID(ctx)-1); err != nil {
		return domain.MutationResult{}, err
	}
	digest := managementHash(struct {
		Target string
		Input  domain.MutationInput
	}{target, in})
	var prior, raw string
	err = tx.QueryRowContext(ctx, userQuery(ctx, "SELECT payload_digest,result FROM csv_mutations WHERE user_id=:user_id AND id=?"), in.ID).Scan(&prior, &raw)
	if err == nil {
		if prior != digest {
			return domain.MutationResult{}, consultation.ErrRequestPayload
		}
		var out domain.MutationResult
		err = json.Unmarshal([]byte(raw), &out)
		return out, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.MutationResult{}, err
	}
	state, err := reconciliationState(ctx, tx, &seed, true)
	if err != nil {
		return domain.MutationResult{}, err
	}
	ids := []int64{}
	protected := 0
	if in.Action == "correct" {
		id, e := strconv.ParseInt(target, 10, 64)
		if e != nil {
			return domain.MutationResult{}, fmt.Errorf("明細IDが不正です")
		}
		var revision int64
		if e = tx.QueryRowContext(ctx, userQuery(ctx, "SELECT revision FROM transactions WHERE user_id=:user_id AND id=?"), id).Scan(&revision); errors.Is(e, sql.ErrNoRows) {
			return domain.MutationResult{}, domain.ErrNotFound
		} else if e != nil {
			return domain.MutationResult{}, e
		}
		if revision != in.Version {
			return domain.MutationResult{}, consultation.ErrConflict
		}
		ids = append(ids, id)
	} else {
		b, e := loadBatch(ctx, tx, target, state.Version)
		if e != nil {
			return domain.MutationResult{}, e
		}
		if b.Version != in.Version || b.Snapshot != in.Snapshot {
			return domain.MutationResult{}, consultation.ErrConflict
		}
		for _, row := range b.Rows {
			ids = append(ids, row.TransactionID)
		}
	}
	if in.StateVersion != state.Version {
		return domain.MutationResult{}, consultation.ErrConflict
	}
	beforeRows, err := managementRows(ctx, tx, ids, true)
	if err != nil {
		return domain.MutationResult{}, err
	}
	originalRows, err := managementRows(ctx, tx, ids, false)
	if err != nil {
		return domain.MutationResult{}, err
	}
	if in.Action == "correct" {
		_, err = tx.ExecContext(ctx, userQuery(ctx, "UPDATE transactions SET effective_date=COALESCE(?,effective_date),effective_amount=COALESCE(?,effective_amount),effective_category=COALESCE(?,effective_category),revision=revision+1 WHERE user_id=:user_id AND id=?"), managementOptional(in.Date), managementOptional(in.Amount), managementOptional(in.Category), ids[0])
		if err == nil && in.Remember && in.Category != nil {
			_, err = tx.ExecContext(ctx, userQuery(ctx, "INSERT INTO merchant_rules(user_id,normalized_merchant,category,updated_at) SELECT user_id,normalized_merchant,?,? FROM transactions WHERE user_id=:user_id AND id=? ON CONFLICT(user_id,normalized_merchant) DO UPDATE SET category=excluded.category,updated_at=excluded.updated_at"), *in.Category, time.Now().UTC().Format(time.RFC3339), ids[0])
		}
	} else {
		status := "cancelled"
		if in.Action == "restore" {
			status = "active"
		}
		_, err = tx.ExecContext(ctx, userQuery(ctx, "UPDATE csv_import_batches SET status=?,version=version+1 WHERE user_id=:user_id AND id=?"), status, target)
	}
	if err != nil {
		return domain.MutationResult{}, err
	}
	afterRows, err := managementRows(ctx, tx, ids, true)
	if err != nil {
		return domain.MutationResult{}, err
	}
	updatedRows, err := managementRows(ctx, tx, ids, false)
	if err != nil {
		return domain.MutationResult{}, err
	}
	newValues := map[int64]domain.Transaction{}
	for _, t := range afterRows {
		newValues[t.ID] = t
	}
	oldValues := map[int64]domain.Transaction{}
	for _, t := range beforeRows {
		oldValues[t.ID] = t
	}
	changedBefore, changedAfter := []domain.Transaction{}, []domain.Transaction{}
	for _, t := range beforeRows {
		next, ok := newValues[t.ID]
		if ok && t.TransactionDate == next.TransactionDate && t.Amount == next.Amount && t.Category == next.Category {
			protected++
			continue
		}
		changedBefore = append(changedBefore, t)
		if ok {
			changedAfter = append(changedAfter, next)
		}
	}
	for _, t := range afterRows {
		if _, ok := oldValues[t.ID]; !ok {
			changedAfter = append(changedAfter, t)
		}
	}
	affectedIDs := map[int64]bool{}
	for _, t := range changedBefore {
		affectedIDs[t.ID] = true
	}
	for _, t := range changedAfter {
		affectedIDs[t.ID] = true
	}
	result := domain.MutationResult{ID: in.ID, Status: "saved", Affected: len(affectedIDs), Protected: protected, StateVersion: state.Version + 1}
	now := time.Now().UTC().Format(time.RFC3339)
	payload, _ := json.Marshal(result)
	if _, err = tx.ExecContext(ctx, userQuery(ctx, "INSERT INTO csv_mutations(user_id,id,payload_digest,kind,target,recorded_at,result) VALUES(:user_id,?,?,?,?,?,?)"), in.ID, digest, in.Action, target, now, string(payload)); err != nil {
		return result, err
	}
	oldJSON, _ := json.Marshal(originalRows)
	newJSON, _ := json.Marshal(updatedRows)
	if _, err = tx.ExecContext(ctx, userQuery(ctx, "INSERT INTO transaction_change_events(user_id,id,mutation_id,kind,target,recorded_at,before_state,after_state) VALUES(:user_id,?,?,?,?,?,?,?)"), managementID(), in.ID, in.Action, target, now, string(oldJSON), string(newJSON)); err != nil {
		return result, err
	}
	result.Pending, err = writeDataChangeReceipts(ctx, tx, state, changedBefore, changedAfter, in.ID, in.Action, now)
	if err != nil {
		return result, err
	}
	state.Version++
	stateJSON, _ := json.Marshal(state)
	if _, err = tx.ExecContext(ctx, userQuery(ctx, "UPDATE chat_states SET version=?,payload=? WHERE user_id=:user_id AND version=?"), state.Version, string(stateJSON), state.Version-1); err != nil {
		return result, err
	}
	payload, _ = json.Marshal(result)
	if _, err = tx.ExecContext(ctx, userQuery(ctx, "UPDATE csv_mutations SET result=? WHERE user_id=:user_id AND id=?"), string(payload), in.ID); err != nil {
		return result, err
	}
	err = tx.Commit()
	return result, err
}

func (r *Repository) ReactivationCandidates(ctx context.Context, items []domain.Transaction) ([]int64, error) {
	tx, err := r.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var result []int64
	seen := map[int64]bool{}
	for _, item := range items {
		id, e := resolveImportID(ctx, tx, item)
		if e != nil {
			return nil, e
		}
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		var n int
		if e = tx.QueryRowContext(ctx, userQuery(ctx, "SELECT count(*) FROM effective_transactions WHERE user_id=:user_id AND id=?"), id).Scan(&n); e != nil {
			return nil, e
		}
		if n == 0 {
			result = append(result, id)
		}
	}
	return result, nil
}

func managementOptional(value any) any {
	switch p := value.(type) {
	case *string:
		if p != nil {
			return *p
		}
	case *int64:
		if p != nil {
			return *p
		}
	}
	return nil
}

func (r *Repository) TransactionDetails(ctx context.Context, id int64) (domain.TransactionRecord, error) {
	state, err := r.LoadChatState(ctx)
	if err != nil {
		return domain.TransactionRecord{}, err
	}
	current, err := managementRows(ctx, r.db, []int64{id}, false)
	if err != nil {
		return domain.TransactionRecord{}, err
	}
	if len(current) == 0 {
		return domain.TransactionRecord{}, domain.ErrNotFound
	}
	active, err := managementRows(ctx, r.db, []int64{id}, true)
	if err != nil {
		return domain.TransactionRecord{}, err
	}
	original, err := scanIdentity(r.db.QueryRowContext(ctx, userQuery(ctx, "SELECT "+identityColumns+" FROM transactions t WHERE t.user_id=:user_id AND t.id=?"), id))
	if err != nil {
		return domain.TransactionRecord{}, err
	}
	return domain.TransactionRecord{Original: original, Current: current[0], Active: len(active) > 0, StateVersion: state.Version}, nil
}

func writeDataChangeReceipts(ctx context.Context, tx *sql.Tx, state consultation.State, beforeRows, afterRows []domain.Transaction, mutationID, operation, now string) (int, error) {
	pending := 0
	for _, direction := range []string{"expense", "income"} {
		old, next := []consultation.Transaction{}, []consultation.Transaction{}
		for _, t := range beforeRows {
			if t.TransactionType == direction && t.ImportedAt > forgottenCutoff(state) {
				old = append(old, consultation.Transaction{Date: t.TransactionDate, Amount: t.Amount, Category: t.Category})
			}
		}
		for _, t := range afterRows {
			if t.TransactionType == direction && t.ImportedAt > forgottenCutoff(state) {
				next = append(next, consultation.Transaction{Date: t.TransactionDate, Amount: t.Amount, Category: t.Category})
			}
		}
		if len(old)+len(next) == 0 {
			continue
		}
		total := 0
		for _, t := range beforeRows {
			if t.TransactionType == direction {
				total++
			}
		}
		for _, t := range afterRows {
			if t.TransactionType == direction {
				total++
			}
		}
		if len(old) > 20 {
			old = old[:20]
		}
		if len(next) > 20 {
			next = next[:20]
		}
		oldJSON, _ := json.Marshal(old)
		newJSON, _ := json.Marshal(next)
		ref := managementID()
		if _, err := tx.ExecContext(ctx, userQuery(ctx, "INSERT INTO csv_financial_receipts(user_id,ref,mutation_id,operation,before_data,after_data,included_count,omitted_count,status,recorded_at,state_version) VALUES(:user_id,?,?,?,?,?,?,?,'pending',?,?)"), ref, mutationID, operation+"_"+direction, string(oldJSON), string(newJSON), len(old)+len(next), total-len(old)-len(next), now, state.Version+1); err != nil {
			return 0, err
		}
		seen := map[int64]bool{}
		for _, row := range append(append([]domain.Transaction{}, beforeRows...), afterRows...) {
			if row.TransactionType != direction || seen[row.ID] {
				continue
			}
			seen[row.ID] = true
			if _, err := tx.ExecContext(ctx, userQuery(ctx, "INSERT INTO csv_financial_items(user_id,ref,transaction_id) VALUES(:user_id,?,?)"), ref, row.ID); err != nil {
				return 0, err
			}
		}
		pending++
	}
	return pending, nil
}
