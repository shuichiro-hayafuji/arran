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

type trackedChatRepository struct {
	Repository
	begins, finishes int
	released         bool
}

func (r *trackedChatRepository) ReserveSyncConsultation(ctx context.Context, id, month string) (domain.ConsultationUsage, error) {
	r.begins++
	return r.Repository.ReserveMonthlyConsultation(ctx, month)
}
func (r *trackedChatRepository) BeginSyncContinuation(context.Context, string) error {
	r.begins++
	return nil
}
func (r *trackedChatRepository) FinishSyncConsultation(ctx context.Context, id string, release bool) error {
	r.finishes++
	r.released = release
	if release {
		return r.Repository.ReleaseMonthlyConsultation(ctx, "2026-11")
	}
	return nil
}

type failingChatRead struct{ consultation.Repository }

func (r failingChatRead) LoadChatState(context.Context) (consultation.State, error) {
	return consultation.State{}, errors.New("read failed")
}
func TestChatFenceClassifiesSafeAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		save                    bool
		wantFinish, wantRelease bool
	}{{"read", false, true, true}, {"save", true, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			_, base := testService(t)
			repo := &trackedChatRepository{Repository: base}
			var chatRepo consultation.Repository = failingChatRead{base}
			if tc.save {
				chatRepo = failingChatSave{base}
			}
			s := New(Config{Repository: repo, ChatRepository: chatRepo, ChatAgent: &countingChatAgent{}, Now: func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }})
			if _, err := s.SendChat(identity.WithUser(context.Background(), 1), 0, "hello", nil); err == nil {
				t.Fatal("failure expected")
			}
			if repo.begins != 1 || (repo.finishes == 1) != tc.wantFinish || repo.released != tc.wantRelease {
				t.Fatalf("fence %+v", repo)
			}
		})
	}
}
