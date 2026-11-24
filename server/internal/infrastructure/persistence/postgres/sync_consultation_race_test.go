package postgres

import (
	"context"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"os"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

type synchronizedStateRepository struct {
	consultation.Repository
	loaded  chan struct{}
	proceed <-chan struct{}
}

func (r synchronizedStateRepository) LoadChatState(ctx context.Context) (consultation.State, error) {
	state, err := r.Repository.LoadChatState(ctx)
	select {
	case r.loaded <- struct{}{}:
	case <-ctx.Done():
		return state, ctx.Err()
	}
	select {
	case <-r.proceed:
		return state, err
	case <-ctx.Done():
		return state, ctx.Err()
	}
}

type successfulRaceAgent struct{}

func (successfulRaceAgent) Advise(context.Context, consultation.Input) (consultation.Reply, error) {
	return consultation.Reply{Status: "answered", Answer: "ok"}, nil
}

func TestSyncConsultationConflictRollback(t *testing.T) {
	if os.Getenv("ARRAN_TEST_DATABASE_URL") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	for _, withModel := range []bool{false, true} {
		t.Run(map[bool]string{false: "fallback", true: "model"}[withModel], func(t *testing.T) {
			r := openReconciliationRepository(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			owner := identity.WithUser(ctx, 701)
			if err := r.RefreshWithdrawalBatchHealth(ctx); err != nil {
				t.Fatal(err)
			}
			loaded := make(chan struct{}, 2)
			proceed := make(chan struct{})
			results := make(chan error, 2)
			config := application.Config{Repository: r, ChatRepository: synchronizedStateRepository{Repository: r, loaded: loaded, proceed: proceed}, Now: func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }}
			if withModel {
				config.ChatAgent = successfulRaceAgent{}
			}
			app := application.New(config)
			for i := 0; i < 2; i++ {
				go func() { _, err := app.SendChat(owner, 0, "hello", nil); results <- err }()
			}
			for i := 0; i < 2; i++ {
				select {
				case <-loaded:
				case <-ctx.Done():
					t.Fatal("timed out waiting for concurrent state reads")
				}
			}
			close(proceed)
			successes, conflicts := 0, 0
			for i := 0; i < 2; i++ {
				var err error
				select {
				case err = <-results:
				case <-ctx.Done():
					t.Fatal("timed out waiting for consultations")
				}
				if err == nil {
					successes++
				} else if errors.Is(err, consultation.ErrConflict) {
					conflicts++
				} else {
					t.Fatalf("chat failure: %v", err)
				}
			}
			if successes != 1 || conflicts != 1 {
				t.Fatalf("results success=%d conflict=%d", successes, conflicts)
			}
			usage, err := r.MonthlyConsultationUsage(owner, "2026-11")
			expectedUsage := 1
			expectedActive := 0
			if withModel {
				expectedUsage = 2
				expectedActive = 1
			}
			if err != nil || usage.Count != expectedUsage {
				t.Fatalf("usage after rollback: %+v %v", usage, err)
			}
			var active int
			if err = r.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM sync_consultation_reservations WHERE user_id=701 AND state='active'`).Scan(&active); err != nil || active != expectedActive {
				t.Fatalf("active markers=%d %v", active, err)
			}
			err = r.RequestWithdrawal(owner, 701)
			if withModel && !errors.Is(err, identity.ErrWithdrawalBlocked) {
				t.Fatalf("model outcome admitted: %v", err)
			}
			if !withModel && err != nil {
				t.Fatalf("withdrawal blocked after known rollback: %v", err)
			}
		})
	}
}
