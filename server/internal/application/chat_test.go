package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

type singleChatLimit struct{ Repository }

func (r singleChatLimit) ReserveMonthlyConsultation(ctx context.Context, month string) (domain.ConsultationUsage, error) {
	u, err := r.Repository.ReserveMonthlyConsultation(ctx, month)
	u.Limit = 1
	return u, err
}
func (r singleChatLimit) MonthlyConsultationUsage(ctx context.Context, month string) (domain.ConsultationUsage, error) {
	u, err := r.Repository.MonthlyConsultationUsage(ctx, month)
	u.Limit = 1
	return u, err
}

type countingChatAgent struct{ calls int }

func (a *countingChatAgent) Advise(context.Context, consultation.Input) (consultation.Reply, error) {
	a.calls++
	return consultation.Reply{Status: "answered", Answer: "相談を確認しました。"}, nil
}

type chatNotification struct{ calls int }

func (n *chatNotification) NotifyMonthlyLimit(context.Context, domain.MonthlyLimitEvent) error {
	n.calls++
	return nil
}

type failingChatSave struct{ consultation.Repository }

func (r failingChatSave) SaveChatTurn(context.Context, consultation.State, consultation.Thread) (consultation.Thread, error) {
	return consultation.Thread{}, errors.New("database offline")
}

func TestChatMaintainsQuotaNotificationsOwnershipAndJSTMonth(t *testing.T) {
	_, repo := testService(t)
	ctx := identity.WithUser(context.Background(), 1)
	clock := time.Date(2026, 7, 31, 14, 59, 0, 0, time.UTC)
	agent, notifier := &countingChatAgent{}, &chatNotification{}
	s := New(Config{Repository: singleChatLimit{repo}, ChatRepository: repo, ChatAgent: agent, AdminNotifier: notifier, Now: func() time.Time { return clock }})
	first, err := s.SendChat(ctx, 0, "初回の相談です", nil)
	if err != nil {
		t.Fatal(err)
	}
	if agent.calls != 1 || notifier.calls != 1 {
		t.Fatalf("calls agent=%d notifier=%d", agent.calls, notifier.calls)
	}
	continued, err := s.SendChat(ctx, first.ID, "追加の相談です", nil)
	if err != nil || agent.calls != 1 || continued.ResponseSource != "quota_fallback" || len(continued.Messages) != 4 {
		t.Fatalf("quota continuation = %#v %v calls=%d", continued, err, agent.calls)
	}
	usage, err := repo.MonthlyConsultationUsage(ctx, "2026-07")
	if err != nil || usage.Count != 1 {
		t.Fatalf("continuation counted = %#v %v", usage, err)
	}
	second, err := s.SendChat(ctx, 0, "上限後の新規相談です", nil)
	if err != nil || second.ResponseSource != "quota_fallback" || agent.calls != 1 || notifier.calls != 1 {
		t.Fatalf("quota new = %#v %v", second, err)
	}
	other := identity.WithUser(context.Background(), 2)
	if _, err = s.SendChat(other, first.ID, "他人の相談", nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ownership = %v", err)
	}
	otherUsage, _ := repo.MonthlyConsultationUsage(other, "2026-07")
	if otherUsage.Count != 0 {
		t.Fatal("unauthorized chat consumed quota")
	}
	clock = clock.Add(time.Minute) // August in JST, still July in UTC.
	if _, err = s.SendChat(ctx, 0, "新しい月の相談", nil); err != nil || agent.calls != 2 || notifier.calls != 2 {
		t.Fatalf("month reset = %v agent=%d notifications=%d", err, agent.calls, notifier.calls)
	}
}

func TestChatFailedSaveReleasesNewConsultationReservation(t *testing.T) {
	_, repo := testService(t)
	ctx := identity.WithUser(context.Background(), 1)
	s := New(Config{Repository: repo, ChatRepository: failingChatSave{repo}, ChatAgent: &countingChatAgent{}})
	if _, err := s.SendChat(ctx, 0, "保存失敗の相談", nil); err == nil {
		t.Fatal("save unexpectedly succeeded")
	}
	u, err := repo.MonthlyConsultationUsage(ctx, s.now().In(jst).Format("2006-01"))
	if err != nil || u.Count != 0 {
		t.Fatalf("reservation leaked = %#v %v", u, err)
	}
}
