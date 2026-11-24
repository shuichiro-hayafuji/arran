package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"log"
	"strings"
	"time"
)

// syncConsultationFence is implemented by the production repository. A
// missing implementation preserves standalone test adapters; production
// withdrawal admission stays disabled until every write path is fenced.
type syncConsultationFence interface {
	ReserveSyncConsultation(context.Context, string, string) (domain.ConsultationUsage, error)
	BeginSyncContinuation(context.Context, string) error
	FinishSyncConsultation(context.Context, string, bool) error
}

func (s *Application) SendChat(ctx context.Context, id int64, message string, amount *int64) (domain.Consultation, error) {
	if amount != nil {
		if *amount < 0 {
			return domain.Consultation{}, fmt.Errorf("予定額は0円以上にしてください")
		}
		message += fmt.Sprintf("\n購入予定額は%d円です。", *amount)
	}
	if s.chat.Repository == nil {
		return domain.Consultation{}, fmt.Errorf("chat repository unavailable")
	}
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 16000 {
		return domain.Consultation{}, fmt.Errorf("メッセージを1〜16000バイトで入力してください")
	}
	if id != 0 {
		if _, err := s.chat.Repository.LoadChatThread(ctx, id); err != nil {
			if errors.Is(err, consultation.ErrNotFound) {
				return domain.Consultation{}, domain.ErrNotFound
			}
			return domain.Consultation{}, err
		}
	}
	month := s.now().In(jst).Format("2006-01")
	var usage domain.ConsultationUsage
	var err error
	fence, fenced := s.repository.(syncConsultationFence)
	operationID := ""
	if fenced {
		var idBytes [16]byte
		if _, err = rand.Read(idBytes[:]); err != nil {
			return domain.Consultation{}, err
		}
		operationID = hex.EncodeToString(idBytes[:])
		if id == 0 {
			usage, err = fence.ReserveSyncConsultation(ctx, operationID, month)
		} else {
			err = fence.BeginSyncContinuation(ctx, operationID)
			began := err == nil
			if began {
				usage, err = s.repository.MonthlyConsultationUsage(ctx, month)
			}
			// A read failure after confirmed admission has no external side effect.
			if began && err != nil {
				finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer cancel()
				if finishErr := fence.FinishSyncConsultation(finishCtx, operationID, false); finishErr != nil {
					log.Print("synchronous consultation fence could not be completed")
				}
			}
		}
	} else if id == 0 {
		usage, err = s.repository.ReserveMonthlyConsultation(ctx, month)
	} else {
		usage, err = s.repository.MonthlyConsultationUsage(ctx, month)
	}
	if err != nil {
		return domain.Consultation{}, err
	}
	committed := false
	defer func() {
		if !fenced && id == 0 && !committed {
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = s.repository.ReleaseMonthlyConsultation(releaseCtx, month)
		}
	}()
	chat := s.chat
	if (id == 0 && !usage.UseExternalModel()) || (id != 0 && usage.Count >= usage.Limit) {
		chat.Agent = nil
		chat.UnavailableReason = "今月の相談上限に達しました。相談内容は保存しました。外部モデルを使う助言は翌月に再開できます。"
	}
	thread, err := chat.Send(ctx, id, message)
	if fenced {
		var before *consultation.BeforeChatSideEffects
		if err == nil || errors.As(err, &before) {
			finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			finishErr := fence.FinishSyncConsultation(finishCtx, operationID, err != nil && id == 0)
			cancel()
			if finishErr != nil {
				log.Print("synchronous consultation fence could not be completed")
			}
		}
	}
	if errors.Is(err, consultation.ErrNotFound) {
		return domain.Consultation{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Consultation{}, err
	}
	committed = true
	if id == 0 {
		s.notifyMonthlyLimit(ctx, usage)
	}
	return s.chatConsultation(ctx, thread)
}
func (s *Application) chatConsultation(ctx context.Context, t consultation.Thread) (domain.Consultation, error) {
	item, err := s.repository.GetConsultation(ctx, t.ID)
	if err != nil {
		return domain.Consultation{}, err
	}
	return chatProjection(item, t), nil
}
func chatProjection(item domain.Consultation, t consultation.Thread) domain.Consultation {
	// Keep advice and transcript from the same saved version even if another turn
	// finishes between the save and this summary read. Result fields remain separate.
	item.PlannedAmount = t.Reply.PlannedAmount
	if t.PendingPurchase != nil {
		item.PlannedAmount = &t.PendingPurchase.Amount
		item.PendingPurchase = &domain.PendingPurchase{Description: t.PendingPurchase.Description, Amount: t.PendingPurchase.Amount, RecordedAt: t.PendingPurchase.RecordedAt, Status: t.PendingPurchase.Status}
	}
	for _, p := range t.Purchases {
		out := domain.Purchase{ActualAmountBasis: p.ActualAmountBasis, PurchaseDate: p.PurchaseDate, CSVReconciliationPending: p.CSVReconciliationPending, BalanceAppliedAmount: p.BalanceAppliedAmount, FreeBudgetAppliedAmount: p.FreeBudgetAppliedAmount, ID: p.ID, ConsultationID: p.ConsultationID, Description: p.Description, Amount: p.Amount, RecordedAt: p.RecordedAt, Status: p.Status, FirstAskedAt: p.FirstAskedAt, SecondAskedAt: p.SecondAskedAt, ActualAmount: p.ActualAmount, Classification: p.Classification}
		for _, e := range p.Events {
			out.Events = append(out.Events, domain.PurchaseEvent{BalanceAppliedAmount: e.BalanceAppliedAmount, FreeBudgetAppliedAmount: e.FreeBudgetAppliedAmount, BalanceBasis: e.BalanceBasis, Outcome: e.Outcome, Amount: e.Amount, Classification: e.Classification, RecordedAt: e.RecordedAt, Source: e.Source})
		}
		item.Purchases = append(item.Purchases, out)
	}
	item.AIRecommendation = t.Reply.Answer
	item.AIReasoningSummary = t.Reply.Reason
	item.CurrentSituation = t.Reply.Assumptions
	item.ResponseSource = t.Source
	item.NeedsFollowUp = t.Reply.Status == "needs_information"
	item.FollowUpQuestion = t.Reply.Question
	switch item.Status {
	case "answered", "provisional", "needs_information", "unavailable":
		item.Status = t.Reply.Status
	}
	for _, m := range t.Messages {
		item.Messages = append(item.Messages, domain.ChatMessage{Role: m.Role, Content: m.Content, CreatedAt: m.CreatedAt, Reason: m.Reason, Assumptions: m.Assumptions})
	}
	return item
}
func (s *Application) GetChat(ctx context.Context, id int64) (domain.Consultation, error) {
	if s.chat.Repository != nil {
		t, err := s.chat.Repository.LoadChatThread(ctx, id)
		if err == nil {
			return s.chatConsultation(ctx, t)
		}
		if !errors.Is(err, consultation.ErrNotFound) {
			return domain.Consultation{}, err
		}
	}
	return s.repository.GetConsultation(ctx, id)
}

func (s *Application) ListChats(ctx context.Context, limit int) ([]domain.Consultation, error) {
	items, err := s.ListConsultations(ctx, limit)
	if err != nil {
		return nil, err
	}
	for i, item := range items {
		items[i], err = s.GetChat(ctx, item.ID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}
