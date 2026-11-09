package application

import (
	"context"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"time"
)

type ReconciliationRepository interface {
	PreviewReconciliations(context.Context, []domain.Transaction) ([]domain.ReconciliationConflict, error)
	ExpenseCount(context.Context, string) (domain.ExpenseCount, error)
	PurchaseCSVCandidates(context.Context, string) ([]domain.ReconciliationConflict, error)
	ResolvePurchaseCSV(context.Context, int64, domain.ReconciliationDecision) error
}

func (s *Application) PreviewImport(ctx context.Context, data []byte, mapping domain.CSVMapping, source, account string, decisions ...[]domain.ImportIdentityDecision) (domain.ImportPreview, error) {
	var identityDecisions []domain.ImportIdentityDecision
	if len(decisions) > 0 {
		identityDecisions = decisions[0]
	}
	return s.previewImport(ctx, data, mapping, source, account, identityDecisions, nil)
}
func (s *Application) PreviewReconciledImport(ctx context.Context, data []byte, mapping domain.CSVMapping, source, account string, identityDecisions []domain.ImportIdentityDecision, decisions []domain.ReconciliationDecision) (domain.ImportPreview, error) {
	return s.previewImport(ctx, data, mapping, source, account, identityDecisions, decisions)
}
func (s *Application) ExpenseCount(ctx context.Context, month string) (domain.ExpenseCount, error) {
	if month != "" {
		if _, err := time.Parse("2006-01", month); err != nil {
			return domain.ExpenseCount{}, fmt.Errorf("月の指定が不正です")
		}
	}
	if s.reconciliation == nil {
		return domain.ExpenseCount{}, fmt.Errorf("reconciliation repository unavailable")
	}
	return s.reconciliation.ExpenseCount(ctx, month)
}
func (s *Application) PurchaseCSVCandidates(ctx context.Context, id string) ([]domain.ReconciliationConflict, error) {
	if s.reconciliation == nil {
		return nil, fmt.Errorf("reconciliation repository unavailable")
	}
	return s.reconciliation.PurchaseCSVCandidates(ctx, id)
}
func (s *Application) ResolvePurchaseCSV(ctx context.Context, id int64, d domain.ReconciliationDecision) error {
	if s.reconciliation == nil {
		return fmt.Errorf("reconciliation repository unavailable")
	}
	return s.reconciliation.ResolvePurchaseCSV(ctx, id, d)
}
