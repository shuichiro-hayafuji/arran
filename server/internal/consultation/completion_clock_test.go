package consultation_test

import (
	"context"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"path/filepath"
	"testing"
	"time"
)

type advancingChatAgent struct {
	scripted *scriptedAgent
	now      *time.Time
}

func (a advancingChatAgent) Advise(ctx context.Context, in consultation.Input) (consultation.Reply, error) {
	r, err := a.scripted.Advise(ctx, in)
	*a.now = (*a.now).Add(15 * time.Second)
	return r, err
}

func TestPurchaseDueTimeUsesAdviceCompletionAndKeepsReceivedTime(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "completion.db"))
	received := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	now := received
	scripted := &scriptedAgent{replies: []consultation.Reply{planned("本", 1000)}}
	s := consultation.Service{Repository: repo, Agent: advancingChatAgent{scripted: scripted, now: &now}, Now: func() time.Time { return now }}
	thread, err := s.Send(ctx, 0, "1000円の本")
	if err != nil {
		t.Fatal(err)
	}
	completed := received.Add(15 * time.Second)
	if thread.CreatedAt != received.Format(time.RFC3339) || thread.Messages[0].CreatedAt != received.Format(time.RFC3339) || thread.Messages[1].CreatedAt != completed.Format(time.RFC3339) || thread.Purchases[0].RecordedAt != completed.Format(time.RFC3339) {
		t.Fatalf("received/completed timestamps mixed: %+v", thread)
	}
	now = completed.Add(6*time.Hour - time.Second)
	if err = s.DeliverDue(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := repo.LoadChatThread(ctx, thread.ID)
	if err != nil || len(before.Messages) != 2 {
		t.Fatal("first reminder sent before advice completion plus six hours")
	}
	now = completed.Add(6 * time.Hour)
	if err = s.DeliverDue(ctx); err != nil {
		t.Fatal(err)
	}
	boundary, err := repo.LoadChatThread(ctx, thread.ID)
	if err != nil || len(boundary.Messages) != 3 {
		t.Fatal("first reminder missing at completion plus six hours")
	}
	amount := int64(1000)
	scripted.replies = append(scripted.replies, paid(thread.Purchases[0].ID, "purchased", "necessary", &amount))
	turnReceived := now
	updated, err := s.Send(ctx, thread.ID, "予定どおり買った")
	if err != nil {
		t.Fatal(err)
	}
	event := updated.Purchases[0].Events[0]
	if event.RecordedAt != turnReceived.Add(15*time.Second).Format(time.RFC3339) || updated.Messages[len(updated.Messages)-2].CreatedAt != turnReceived.Format(time.RFC3339) || updated.Messages[len(updated.Messages)-1].CreatedAt != event.RecordedAt {
		t.Fatal("event and assistant did not use completion time")
	}
}
