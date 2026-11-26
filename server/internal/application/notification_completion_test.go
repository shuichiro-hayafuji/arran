package application

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)

type notificationCompletionRepo struct {
	Repository
	completeErr error
	completed   bool
	contextErr  error
}

func (r *notificationCompletionRepo) ClaimMonthlyLimitNotification(context.Context, domain.MonthlyLimitEvent) (bool, error) {
	return true, nil
}
func (r *notificationCompletionRepo) CompleteMonthlyLimitNotification(ctx context.Context, _ domain.MonthlyLimitEvent, _ error) error {
	r.completed = true
	r.contextErr = ctx.Err()
	return r.completeErr
}

type cancellingNotifier struct{ cancel context.CancelFunc }

func (n cancellingNotifier) NotifyMonthlyLimit(context.Context, domain.MonthlyLimitEvent) error {
	n.cancel()
	return nil
}
func TestNotificationCompletionAfterCancellationAndFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		repo := &notificationCompletionRepo{}
		if fail {
			repo.completeErr = errors.New("private provider payload")
		}
		service := &Application{repository: repo, adminNotifier: cancellingNotifier{cancel}, now: time.Now}
		var logs bytes.Buffer
		old := log.Writer()
		log.SetOutput(&logs)
		service.notifyMonthlyLimit(ctx, domain.ConsultationUsage{Count: 1, Limit: 1})
		log.SetOutput(old)
		if !repo.completed || repo.contextErr != nil {
			t.Fatalf("completion not attempted with live context: %+v", repo)
		}
		if fail && (!strings.Contains(logs.String(), "completion could not be recorded") || strings.Contains(logs.String(), "private provider payload")) {
			t.Fatalf("unsafe or missing failure log: %q", logs.String())
		}
		if !fail && logs.Len() != 0 {
			t.Fatalf("unexpected log %q", logs.String())
		}
	}
}
