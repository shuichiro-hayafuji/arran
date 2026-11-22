package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"time"
)

// previewPayload is a versioned storage format, never an API or LLM DTO.
// Transaction's JSON deliberately omits internal identity and confirmation fields.
type previewPayload struct {
	Version int                  `json:"version"`
	Rows    []previewTransaction `json:"rows"`
}
type previewMetadata struct {
	Metadata            domain.ImportMetadata `json:"metadata"`
	CommitKey           string                `json:"commit_key"`
	ConfirmReactivation bool                  `json:"confirm_reactivation"`
}
type previewTransaction struct {
	Transaction               domain.Transaction `json:"transaction"`
	Metadata                  *previewMetadata   `json:"metadata"`
	Reconciliation            *domain.ReconciliationDecision
	IdentityVersion           int
	ProviderTransactionID     string
	ImportFileKey             string
	ImportRow                 int
	Occurrence                int
	LegacyFingerprint         string
	IdentityAction            string
	IdentityCandidateSnapshot string
	IdentityTransactionID     int64
}

func encodeImportPreview(items []domain.Transaction) ([]byte, error) {
	payload := previewPayload{Version: 1, Rows: make([]previewTransaction, len(items))}
	for i, item := range items {
		row := previewTransaction{Transaction: item}
		row.Reconciliation = item.Reconciliation
		row.IdentityVersion = item.IdentityVersion
		row.ProviderTransactionID = item.ProviderTransactionID
		row.ImportFileKey = item.ImportFileKey
		row.ImportRow = item.ImportRow
		row.Occurrence = item.Occurrence
		row.LegacyFingerprint = item.LegacyFingerprint
		row.IdentityAction = item.IdentityAction
		row.IdentityCandidateSnapshot = item.IdentityCandidateSnapshot
		row.IdentityTransactionID = item.IdentityTransactionID
		if item.ImportMetadata != nil {
			row.Metadata = &previewMetadata{Metadata: *item.ImportMetadata, CommitKey: item.ImportMetadata.CommitKey, ConfirmReactivation: item.ImportMetadata.ConfirmReactivation}
		}
		payload.Rows[i] = row
	}
	return json.Marshal(payload)
}
func decodeImportPreview(data []byte, id string) ([]domain.Transaction, error) {
	var payload previewPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if payload.Version != 1 || payload.Rows == nil {
		return nil, fmt.Errorf("保存済みプレビューの形式に対応していません。再度プレビューしてください")
	}
	items := make([]domain.Transaction, len(payload.Rows))
	for i, row := range payload.Rows {
		item := row.Transaction
		item.Reconciliation = row.Reconciliation
		item.IdentityVersion = row.IdentityVersion
		item.ProviderTransactionID = row.ProviderTransactionID
		item.ImportFileKey = row.ImportFileKey
		item.ImportRow = row.ImportRow
		item.Occurrence = row.Occurrence
		item.LegacyFingerprint = row.LegacyFingerprint
		item.IdentityAction = row.IdentityAction
		item.IdentityCandidateSnapshot = row.IdentityCandidateSnapshot
		item.IdentityTransactionID = row.IdentityTransactionID
		if row.Metadata == nil || row.Metadata.CommitKey != id {
			return nil, fmt.Errorf("保存済みプレビューの確定情報が不正です")
		}
		meta := row.Metadata.Metadata
		meta.CommitKey = row.Metadata.CommitKey
		meta.ConfirmReactivation = row.Metadata.ConfirmReactivation
		item.ImportMetadata = &meta
		items[i] = item
	}
	return items, nil
}

func (r *Repository) SaveImportPreview(ctx context.Context, id string, items []domain.Transaction, now, expires time.Time) error {
	if id == "" || !expires.After(now) {
		return fmt.Errorf("プレビューの期限が不正です")
	}
	payload, err := encodeImportPreview(items)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Expired uncommitted data is removed on the owner's next preview. Empty
	// successful receipts are retained, just like normal csv_import_batches.
	if _, err = tx.ExecContext(ctx, chatQuery(ctx, "DELETE FROM csv_import_previews WHERE user_id=:user_id AND expires_at<=? AND result IS NULL"), now.UnixNano()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, chatQuery(ctx, "INSERT INTO csv_import_previews(user_id,id,payload,expires_at) VALUES(:user_id,?,?,?)"), id, string(payload), expires.UnixNano()); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *Repository) CommitImportPreview(ctx context.Context, id string, now time.Time) (domain.ImportCommitResult, error) {
	if id == "" {
		return domain.ImportCommitResult{}, fmt.Errorf("プレビューIDが必要です")
	}
	return r.importTransactions(ctx, nil, id, now)
}

func loadImportPreview(ctx context.Context, tx *sql.Tx, id string, now time.Time) ([]domain.Transaction, *domain.ImportCommitResult, error) {
	started := time.Now()
	// A saved result always wins over expiry or a stale reconciliation snapshot.
	var receipt string
	err := tx.QueryRowContext(ctx, chatQuery(ctx, "SELECT result FROM csv_import_batches WHERE user_id=:user_id AND commit_key=?"), id).Scan(&receipt)
	if err == nil {
		var result domain.ImportCommitResult
		if err = json.Unmarshal([]byte(receipt), &result); err != nil {
			return nil, nil, err
		}
		return nil, &result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, err
	}
	var payload string
	var expiry int64
	var emptyReceipt sql.NullString
	err = tx.QueryRowContext(ctx, chatQuery(ctx, "SELECT payload,expires_at,result FROM csv_import_previews WHERE user_id=:user_id AND id=?"), id).Scan(&payload, &expiry, &emptyReceipt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("プレビューが見つからないか、有効期限が切れています。再度プレビューしてください")
	}
	if err != nil {
		return nil, nil, err
	}
	if emptyReceipt.Valid {
		var result domain.ImportCommitResult
		if err = json.Unmarshal([]byte(emptyReceipt.String), &result); err != nil {
			return nil, nil, err
		}
		return nil, &result, nil
	}
	if now.Add(time.Since(started)).UnixNano() >= expiry {
		return nil, nil, fmt.Errorf("プレビューの有効期限が切れています。再度プレビューしてください")
	}
	items, err := decodeImportPreview([]byte(payload), id)
	return items, nil, err
}
func finishImportPreview(ctx context.Context, tx *sql.Tx, id string, items []domain.Transaction, result domain.ImportCommitResult) error {
	if id == "" {
		return nil
	}
	var changed sql.Result
	var err error
	if len(items) == 0 {
		// Empty imports have no bank batch, so keep their result while clearing raw data.
		var data []byte
		data, err = json.Marshal(result)
		if err != nil {
			return err
		}
		changed, err = tx.ExecContext(ctx, chatQuery(ctx, "UPDATE csv_import_previews SET payload='',result=? WHERE user_id=:user_id AND id=?"), string(data), id)
	} else {
		changed, err = tx.ExecContext(ctx, chatQuery(ctx, "DELETE FROM csv_import_previews WHERE user_id=:user_id AND id=?"), id)
	}
	if err != nil {
		return err
	}
	count, err := changed.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("プレビューの保存状態が変わりました。確定結果を再確認してください")
	}
	return nil
}
