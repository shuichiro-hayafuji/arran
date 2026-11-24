package consultation_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

type outcomeRepository struct {
	consultation.Repository
	failRead, failSave, conflict bool
}

func (r outcomeRepository) LoadChatState(ctx context.Context) (consultation.State, error) {
	if r.failRead {
		return consultation.State{}, errors.New("read failed")
	}
	return r.Repository.LoadChatState(ctx)
}
func (r outcomeRepository) SaveChatTurn(ctx context.Context, s consultation.State, t consultation.Thread) (consultation.Thread, error) {
	if r.conflict {
		return consultation.Thread{}, errors.Join(consultation.ErrConflict, consultation.ErrChatSaveRolledBack)
	}
	if r.failSave {
		return consultation.Thread{}, &consultation.BeforeChatSideEffects{Err: errors.New("misleading lower-level error")}
	}
	return r.Repository.SaveChatTurn(ctx, s, t)
}
func TestChatBeforeSideEffectsClassification(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "outcome.db"))
	service := consultation.Service{Repository: outcomeRepository{Repository: repo, failRead: true}}
	_, err := service.Send(ctx, 0, "hello")
	var safe *consultation.BeforeChatSideEffects
	if !errors.As(err, &safe) {
		t.Fatalf("read failure not classified safe: %v", err)
	}
	service.Repository = outcomeRepository{Repository: repo, failSave: true}
	_, err = service.Send(ctx, 0, "hello")
	safe = nil
	if err == nil || errors.As(err, &safe) {
		t.Fatalf("save failure incorrectly safe: %v", err)
	}
	service.Repository = outcomeRepository{Repository: repo, conflict: true}
	_, err = service.Send(ctx, 0, "hello")
	safe = nil
	if !errors.As(err, &safe) || !errors.Is(err, consultation.ErrConflict) {
		t.Fatalf("no-model confirmed rollback not safe: %v", err)
	}
	service.Agent = &scriptedAgent{replies: []consultation.Reply{reply("answered")}}
	_, err = service.Send(ctx, 0, "hello")
	safe = nil
	if err == nil || errors.As(err, &safe) || !errors.Is(err, consultation.ErrConflict) {
		t.Fatalf("model-called conflict must remain unknown: %v", err)
	}

}
