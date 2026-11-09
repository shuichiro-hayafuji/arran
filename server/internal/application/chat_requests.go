package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)

type ChatRequestInput struct {
	ConsultationID int64  `json:"consultation_id"`
	Message        string `json:"message"`
	PlannedAmount  *int64 `json:"planned_amount"`
}
type ChatRequestResult struct {
	RequestID       string               `json:"request_id"`
	Status          string               `json:"status"`
	ExpiresAt       string               `json:"expires_at"`
	CancelRequested bool                 `json:"cancel_requested"`
	ErrorCode       string               `json:"error_code"`
	Result          *domain.Consultation `json:"result"`
}

var ErrRequestsStopping = errors.New("consultation request service is stopping")

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,100}$`)

func (s *Application) SubmitChatRequest(ctx context.Context, id string, in ChatRequestInput) (ChatRequestResult, error) {
	started := time.Now()
	if err := s.beginRequest(); err != nil {
		return ChatRequestResult{}, err
	}
	defer s.requestsWG.Done()
	if s.requests == nil {
		return ChatRequestResult{}, errors.New("request repository unavailable")
	}
	if !requestIDPattern.MatchString(id) {
		return ChatRequestResult{}, fmt.Errorf("リクエストIDの指定が不正です")
	}
	in.Message = strings.TrimSpace(in.Message)
	if in.Message == "" || len(in.Message) > 16000 || in.ConsultationID < 0 || (in.PlannedAmount != nil && *in.PlannedAmount < 0) {
		return ChatRequestResult{}, fmt.Errorf("相談内容・金額の入力が不正です")
	}
	if in.PlannedAmount != nil && len(in.Message)+len(fmt.Sprintf("\n購入予定額は%d円です。", *in.PlannedAmount)) > 16000 {
		return ChatRequestResult{}, fmt.Errorf("相談内容が長すぎます")
	}
	prepareEnd := started.Add(5 * time.Second)
	if totalEnd := started.Add(s.requestBudget); totalEnd.Before(prepareEnd) {
		prepareEnd = totalEnd
	}
	prepareCtx, cancel := context.WithDeadline(ctx, prepareEnd)
	defer cancel()
	stopPreparation := context.AfterFunc(s.requestsContext, cancel)
	defer stopPreparation()
	if in.ConsultationID != 0 {
		if _, err := s.chat.Repository.LoadChatThread(prepareCtx, in.ConsultationID); err != nil {
			if s.requestsContext.Err() != nil {
				return ChatRequestResult{}, ErrRequestsStopping
			}
			return ChatRequestResult{}, err
		}
	}
	if _, err := s.repository.MonthlyConsultationUsage(prepareCtx, s.now().In(jst).Format("2006-01")); err != nil {
		if s.requestsContext.Err() != nil {
			return ChatRequestResult{}, ErrRequestsStopping
		}
		return ChatRequestResult{}, err
	}
	payload, _ := json.Marshal(in)
	digest := sha256.Sum256(payload)
	receipt, claimed, err := s.requests.ClaimChatRequest(prepareCtx, consultation.RequestReceipt{ID: id, PayloadHash: hex.EncodeToString(digest[:]), Month: s.now().In(jst).Format("2006-01"), NewConsultation: in.ConsultationID == 0, ExpiresAt: started.Add(s.requestBudget)})
	if err != nil {
		if s.requestsContext.Err() != nil {
			return ChatRequestResult{}, ErrRequestsStopping
		}
		return ChatRequestResult{}, err
	}
	if claimed {
		s.requestsWG.Add(1)
		go func() { defer s.requestsWG.Done(); s.executeChatRequest(context.WithoutCancel(ctx), receipt, in) }()
	}
	return requestResult(receipt), nil
}
func (s *Application) GetChatRequest(ctx context.Context, id string) (ChatRequestResult, error) {
	if err := s.beginRequest(); err != nil {
		return ChatRequestResult{}, err
	}
	defer s.requestsWG.Done()
	receipt, err := s.requests.GetChatRequest(ctx, id)
	return requestResult(receipt), err
}
func (s *Application) CancelChatRequest(ctx context.Context, id string) (ChatRequestResult, error) {
	if err := s.beginRequest(); err != nil {
		return ChatRequestResult{}, err
	}
	defer s.requestsWG.Done()
	if !requestIDPattern.MatchString(id) {
		return ChatRequestResult{}, fmt.Errorf("リクエストIDの指定が不正です")
	}
	receipt, err := s.requests.CancelChatRequest(ctx, id)
	return requestResult(receipt), err
}
func (s *Application) beginRequest() error {
	s.requestsMu.Lock()
	defer s.requestsMu.Unlock()
	if s.requestsStopping || s.requestsContext.Err() != nil {
		return ErrRequestsStopping
	}
	s.requestsWG.Add(1)
	return nil
}

// WaitRequests closes admission before waiting, including preparation and result
// API calls. Child goroutines are added while their registered parent is alive.
func (s *Application) WaitRequests() {
	s.requestsMu.Lock()
	s.requestsStopping = true
	s.requestsMu.Unlock()
	s.requestsWG.Wait()
}

func requestResult(r consultation.RequestReceipt) ChatRequestResult {
	out := ChatRequestResult{RequestID: r.ID, Status: r.Status, ExpiresAt: r.ExpiresAt.UTC().Format(time.RFC3339Nano), CancelRequested: r.CancelRequested, ErrorCode: r.ErrorCode}
	if r.Result != nil {
		t := *r.Result
		item := domain.Consultation{ID: t.ID, CreatedAt: t.CreatedAt, Status: t.Reply.Status}
		if len(t.Messages) > 0 {
			item.UserMessage = t.Messages[0].Content
		}
		projected := chatProjection(item, t)
		out.Result = &projected
	}
	return out
}

func (s *Application) executeChatRequest(owner context.Context, receipt consultation.RequestReceipt, in ChatRequestInput) {
	totalCtx, endTotal := context.WithDeadline(owner, receipt.ExpiresAt)
	defer endTotal()
	modelEnd := receipt.ExpiresAt.Add(-s.requestSaveReserve)
	modelCtx, endModel := context.WithDeadline(totalCtx, modelEnd)
	defer endModel()
	if s.requestsContext.Err() != nil {
		endModel()
	}
	var usage domain.ConsultationUsage
	var err error
	usage, err = s.repository.MonthlyConsultationUsage(modelCtx, receipt.Month)
	if err != nil {
		s.finishRequestFailure(owner, receipt.ID, "preparation_failed")
		return
	}
	message := in.Message
	if in.PlannedAmount != nil {
		message += fmt.Sprintf("\n購入予定額は%d円です。", *in.PlannedAmount)
	}
	chat := s.chat
	if (in.ConsultationID == 0 && !usage.UseExternalModel()) || (in.ConsultationID != 0 && usage.Count >= usage.Limit) {
		chat.Agent = nil
		chat.UnavailableReason = "今月の相談上限に達しました。相談内容は保存しました。外部モデルを使う助言は翌月に再開できます。"
	}
	ref := consultation.RequestReference{ID: receipt.ID, Deadline: receipt.ExpiresAt}
	done := make(chan error, 1)
	s.requestsWG.Add(1)
	go func() {
		defer s.requestsWG.Done()
		_, e := chat.Send(consultation.WithRequestReference(modelCtx, ref), in.ConsultationID, message)
		done <- e
	}()
	stopWatch := make(chan struct{})
	defer close(stopWatch)
	s.requestsWG.Add(1)
	go func() {
		defer s.requestsWG.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopWatch:
				return
			case <-s.requestsContext.Done():
				endModel()
				return
			case <-modelCtx.Done():
				return
			case <-ticker.C:
				check, cancel := context.WithTimeout(totalCtx, 300*time.Millisecond)
				r, e := s.requests.GetChatRequest(check, receipt.ID)
				cancel()
				if e == nil && (r.CancelRequested || r.Status != "processing") {
					endModel()
					return
				}
			}
		}
	}()
	select {
	case err = <-done:
	case <-modelCtx.Done():
		err = modelCtx.Err()
	}
	saved := err == nil
	defer func() {
		if saved && receipt.NewConsultation {
			notify, cancel := context.WithTimeout(owner, 5*time.Second)
			defer cancel()
			s.notifyMonthlyLimit(notify, usage)
		}
	}()
	if err != nil && (errors.Is(err, consultation.ErrRequestCancelled) || modelCtx.Err() != nil) {
		fallback := s.chat
		fallback.Agent = nil
		fallback.SuppressConfirmation = true
		fallback.UnavailableReason = "回答の作成を時間内に完了できませんでした。相談内容は保存しました。"
		if errors.Is(err, consultation.ErrRequestCancelled) || errors.Is(modelCtx.Err(), context.Canceled) {
			fallback.UnavailableReason = "相談の処理を中止しました。相談内容は保存しました。"
		}
		ref.AllowCancelled = true
		_, err = fallback.Send(consultation.WithRequestReference(totalCtx, ref), in.ConsultationID, message)
		saved = err == nil
	}

	check, cancel := context.WithTimeout(owner, 5*time.Second)
	fresh, e := s.requests.GetChatRequest(check, receipt.ID)
	cancel()
	if e == nil && fresh.Result != nil {
		saved = true
		return
	}

	code := "save_failed"
	if errors.Is(err, consultation.ErrConflict) {
		code = "conversation_changed"
	}
	if time.Now().After(receipt.ExpiresAt) {
		code = "deadline_exceeded"
	}
	s.finishRequestFailure(owner, receipt.ID, code)
}
func (s *Application) finishRequestFailure(ctx context.Context, id, code string) {
	cleanup, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = s.requests.FailChatRequest(cleanup, id, code)
}
