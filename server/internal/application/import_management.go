package application

import (
	"context"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"strings"
	"time"
)

type ImportManagementRepository interface {
	TransactionDetails(context.Context, int64) (domain.TransactionRecord, error)
	ImportCommit(context.Context, string) (domain.ImportCommitResult, error)
	ImportHistory(context.Context, string, int) ([]domain.ImportBatch, error)
	ImportDetails(context.Context, string) (domain.ImportBatch, error)
	Mutation(context.Context, string) (domain.MutationResult, error)
	ChangeAudit(context.Context, string) ([]domain.TransactionChangeEvent, error)
	MutateCSV(context.Context, string, domain.MutationInput) (domain.MutationResult, error)
	ReactivationCandidates(context.Context, []domain.Transaction) ([]int64, error)
}

func (s *Application) ImportHistory(ctx context.Context, before string) ([]domain.ImportBatch, error) {
	if s.imports == nil {
		return nil, fmt.Errorf("取込履歴は利用できません")
	}
	return s.imports.ImportHistory(ctx, before, 20)
}
func (s *Application) ImportDetails(ctx context.Context, id string) (domain.ImportBatch, error) {
	if s.imports == nil {
		return domain.ImportBatch{}, fmt.Errorf("取込履歴は利用できません")
	}
	return s.imports.ImportDetails(ctx, id)
}
func (s *Application) CSVMutation(ctx context.Context, id string) (domain.MutationResult, error) {
	if s.imports == nil {
		return domain.MutationResult{}, fmt.Errorf("訂正結果は利用できません")
	}
	return s.imports.Mutation(ctx, id)
}
func (s *Application) ChangeAudit(ctx context.Context, target string) ([]domain.TransactionChangeEvent, error) {
	if s.imports == nil {
		return nil, fmt.Errorf("監査は利用できません")
	}
	return s.imports.ChangeAudit(ctx, target)
}
func (s *Application) MutateCSV(ctx context.Context, target string, in domain.MutationInput) (domain.MutationResult, error) {
	if s.imports == nil {
		return domain.MutationResult{}, fmt.Errorf("訂正は利用できません")
	}
	if strings.TrimSpace(in.ID) == "" || len(in.ID) > 128 {
		return domain.MutationResult{}, fmt.Errorf("操作IDが不正です")
	}
	if len(in.Reason) > 2000 {
		return domain.MutationResult{}, fmt.Errorf("理由が長すぎます")
	}
	if in.Action != "correct" && in.Action != "cancel" && in.Action != "restore" {
		return domain.MutationResult{}, fmt.Errorf("操作の指定が不正です")
	}
	if in.Amount != nil && (*in.Amount < 0 || *in.Amount > 1_000_000_000_000) {
		return domain.MutationResult{}, fmt.Errorf("金額が不正です")
	}
	if in.Date != nil {
		if _, err := time.Parse("2006-01-02", *in.Date); err != nil {
			return domain.MutationResult{}, fmt.Errorf("日付が不正です")
		}
	}
	if in.Category != nil {
		valid := false
		for _, c := range domain.Categories {
			if *in.Category == c {
				valid = true
			}
		}
		if !valid {
			return domain.MutationResult{}, fmt.Errorf("カテゴリが不正です")
		}
	}
	if in.Action == "correct" && in.Date == nil && in.Amount == nil && in.Category == nil {
		return domain.MutationResult{}, fmt.Errorf("訂正する項目を指定してください")
	}
	return s.imports.MutateCSV(ctx, target, in)
}

func (s *Application) TransactionDetails(ctx context.Context, id int64) (domain.TransactionRecord, error) {
	if s.imports == nil {
		return domain.TransactionRecord{}, fmt.Errorf("明細を利用できません")
	}
	return s.imports.TransactionDetails(ctx, id)
}
