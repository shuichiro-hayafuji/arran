package consultation_test

import (
	"context"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/sqlite"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type scriptedAgent struct {
	inputs  []consultation.Input
	replies []consultation.Reply
	fail    bool
}

func (a *scriptedAgent) Advise(_ context.Context, input consultation.Input) (consultation.Reply, error) {
	a.inputs = append(a.inputs, input)
	if a.fail {
		return consultation.Reply{}, errors.New("model offline")
	}
	r := a.replies[0]
	a.replies = a.replies[1:]
	return r, nil
}
func reply(status string, changes ...consultation.Change) consultation.Reply {
	return consultation.Reply{Status: status, Answer: "分かっている範囲で考えてみます。", Changes: changes}
}
func fact(key, value, period string) consultation.Change {
	return consultation.Change{Key: key, Value: value, Kind: "fact", Period: period, Operation: "set"}
}
func openRepo(t *testing.T, path string) *sqlite.Repository {
	t.Helper()
	r, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}
func TestChatFactsCorrectionProvenanceAndRestart(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	path := filepath.Join(t.TempDir(), "chat.db")
	repo := openRepo(t, path)
	first := reply("needs_information", fact("current_balance", "約50000円", "2026-10-12"))
	first.Question = "今月の予算はどのくらいですか？"
	first.Answer = first.Question
	strict := 8
	second := reply("provisional", fact("current_balance", "約60000円", "2026-10-12"), fact("monthly_free_budget", "30000円", "2026-10"))
	second.Strictness = &strict
	a := &scriptedAgent{replies: []consultation.Reply{first, second, reply("answered")}}
	s := consultation.Service{Repository: repo, Agent: a, Now: func() time.Time { return time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC) }}
	thread, err := s.Send(ctx, 0, "残高は約5万円。イヤホンを買いたい")
	if err != nil {
		t.Fatal(err)
	}
	if thread.Reply.PlannedAmount != nil || a.inputs[0].Strictness != 5 || len(a.inputs[0].Facts) != 0 {
		t.Fatalf("missing data must remain missing: %#v", thread)
	}
	if strings.Contains(thread.Reply.Answer, "CSV") {
		t.Fatal("CSV must not precede the first answer")
	}
	thread, err = s.Send(ctx, thread.ID, "訂正、残高は約6万円、今月の自由予算3万円。厳しさ8で")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(thread.Reply.Answer, "CSV") {
		t.Fatal("CSV invitation missing after first result")
	}
	state, err := repo.LoadChatState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Strictness != 8 || len(state.Facts) != 3 || state.Facts[0].Active || !state.Facts[1].Active || state.Facts[1].RecordedAt == "" || state.Facts[1].Source == "chat:0:turn:1" {
		t.Fatalf("state=%#v", state)
	}
	reopened := openRepo(t, path)
	s.Repository = reopened
	thread, err = s.Send(ctx, thread.ID, "やっぱり別の買い物にする")
	if err != nil {
		t.Fatal(err)
	}
	last := a.inputs[2]
	if last.Strictness != 8 || len(last.Facts) != 2 || last.Facts[0].Value != "約60000円" || last.Facts[0].Period != "2026-10-12" || len(last.History) != 4 {
		t.Fatalf("context=%#v", last)
	}
	if strings.Contains(thread.Reply.Answer, "CSV") || len(thread.Messages) != 6 {
		t.Fatalf("one-off invitation/transcript=%#v", thread)
	}
	other := identity.WithUser(context.Background(), 2)
	if _, err = reopened.LoadChatThread(other, thread.ID); !errors.Is(err, consultation.ErrNotFound) {
		t.Fatalf("cross-owner read=%v", err)
	}
	otherState, err := reopened.LoadChatState(other)
	if err != nil || len(otherState.Facts) != 0 {
		t.Fatal("state owner isolation failed")
	}
}
func TestForgetExcludesOldTranscriptAndFacts(t *testing.T) {
	ctx := context.Background()
	repo := openRepo(t, ":memory:")
	forgotten := fact("goal", "", "")
	forgotten.Operation = "forget"
	a := &scriptedAgent{replies: []consultation.Reply{reply("answered", fact("goal", "秘密の目標", "")), reply("answered", forgotten), reply("answered")}}
	s := consultation.Service{Repository: repo, Agent: a}
	thread, err := s.Send(ctx, 0, "秘密の目標を覚えて")
	if err != nil {
		t.Fatal(err)
	}
	thread, err = s.Send(ctx, thread.ID, "目標を忘れて")
	if err != nil {
		t.Fatal(err)
	}
	thread, err = s.Send(ctx, thread.ID, "次の相談です")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.inputs[2].Facts) != 0 || len(a.inputs[2].History) != 0 {
		t.Fatal("forgotten data replayed")
	}
	if !strings.Contains(thread.Messages[0].Content, "秘密の目標") {
		t.Fatal("visible history was unexpectedly removed")
	}
}
func TestUnavailableRetainsMessageAndAllowsRetry(t *testing.T) {
	ctx := context.Background()
	repo := openRepo(t, ":memory:")
	a := &scriptedAgent{fail: true}
	s := consultation.Service{Repository: repo, Agent: a}
	thread, err := s.Send(ctx, 0, "収入が減りました。どうしよう")
	if err != nil {
		t.Fatal(err)
	}
	if thread.Reply.Status != "unavailable" || len(a.inputs) != 3 || len(thread.Messages) != 2 || thread.Reply.PlannedAmount != nil {
		t.Fatalf("failed response=%#v", thread)
	}
	state, err := repo.LoadChatState(ctx)
	if err != nil || state.CSVOffered || len(state.Facts) != 0 {
		t.Fatalf("failure changed facts=%#v %v", state, err)
	}
	a.fail = false
	a.replies = []consultation.Reply{reply("provisional")}
	retried, err := s.Send(ctx, thread.ID, "もう一度答えて")
	if err != nil {
		t.Fatal(err)
	}
	if retried.ID != thread.ID || len(retried.Messages) != 4 || a.inputs[3].History[0].Content != thread.Messages[0].Content {
		t.Fatal("retry lost original consultation")
	}
}
func TestStaleResponseCannotOverwriteFacts(t *testing.T) {
	ctx := context.Background()
	repo := openRepo(t, ":memory:")
	a := &scriptedAgent{replies: []consultation.Reply{reply("answered", fact("current_balance", "5000円", "今日"))}}
	thread, err := (consultation.Service{Repository: repo, Agent: a}).Send(ctx, 0, "残高5000円")
	if err != nil {
		t.Fatal(err)
	}
	stale, err := repo.LoadChatState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fresh := stale
	fresh.Strictness = 7
	if _, err = repo.SaveChatTurn(ctx, fresh, thread); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.SaveChatTurn(ctx, stale, thread); !errors.Is(err, consultation.ErrConflict) {
		t.Fatalf("stale save=%v", err)
	}
	latest, err := repo.LoadChatState(ctx)
	if err != nil || latest.Strictness != 7 {
		t.Fatal("stale save corrupted state")
	}
}
func TestPrivacyAndOutputValidation(t *testing.T) {
	redacted := consultation.Redact("口座番号:1234567 パスワードは secret カード番号:4111-1111-1111-1111 残高10000000円 対象期間2026-10-12")
	if strings.Contains(redacted, "1234567") || strings.Contains(redacted, "secret") || strings.Contains(redacted, "4111") || !strings.Contains(redacted, "10000000円") || !strings.Contains(redacted, "2026-10-12") {
		t.Fatalf("redaction=%s", redacted)
	}
	for _, bad := range []consultation.Reply{{Status: "safe", Answer: "safe"}, {Status: "needs_information", Answer: "質問"}, {Status: "answered", Answer: "回答", Changes: []consultation.Change{{Key: "balance", Value: "1", Kind: "unknown", Operation: "set"}}}} {
		if consultation.Validate(bad) == nil {
			t.Fatalf("invalid reply accepted=%#v", bad)
		}
	}
}

func TestAdviceDetailsAndPendingPurchaseSurviveFollowUpAndRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "chat.db")
	repo := openRepo(t, path)
	amount := int64(20000)
	first := reply("needs_information")
	description := "イヤホン"
	first.PurchaseChange = &consultation.PurchaseChange{Operation: "set", Description: &description, Amount: &amount}
	first.Question = "予算は？"
	first.Reason, first.Assumptions = "残高だけでは判断できません", "購入はまだしていません"
	a := &scriptedAgent{replies: []consultation.Reply{first, reply("provisional")}}
	s := consultation.Service{Repository: repo, Agent: a}
	thread, err := s.Send(ctx, 0, "2万円のイヤホンを買いたい")
	if err != nil {
		t.Fatal(err)
	}
	if thread.PendingPurchase == nil || thread.PendingPurchase.Amount != amount || thread.PendingPurchase.Status != "purchase_confirmation_pending" {
		t.Fatalf("pending = %#v", thread.PendingPurchase)
	}
	s.Repository = openRepo(t, path)
	thread, err = s.Send(ctx, thread.ID, "まだ予算は分かりません")
	if err != nil {
		t.Fatal(err)
	}
	if thread.PendingPurchase == nil || thread.PendingPurchase.Amount != amount || thread.Messages[1].Reason != first.Reason || thread.Messages[1].Assumptions != first.Assumptions {
		t.Fatalf("restored thread = %#v", thread)
	}
	if a.inputs[1].History[1].Reason != first.Reason || a.inputs[1].History[1].Assumptions != first.Assumptions {
		t.Fatal("previous advice evidence missing from input")
	}
	stored, err := s.Repository.LoadChatThread(ctx, thread.ID)
	if err != nil || stored.PendingPurchase == nil || stored.Messages[1].Reason != first.Reason {
		t.Fatalf("persisted = %#v %v", stored, err)
	}
}

func TestLegacyFactsAreImportedOnceWithoutInventingMissingZeros(t *testing.T) {
	ctx := context.Background()
	repo := openRepo(t, ":memory:")
	if err := repo.PutProfile(ctx, domain.Profile{CurrentBalance: 50000, MonthlyFreeBudget: 0, UpdatedAt: "2026-09-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	a := &scriptedAgent{replies: []consultation.Reply{reply("answered"), reply("answered", fact("current_balance", "", ""))}}
	s := consultation.Service{Repository: repo, Agent: a}
	thread, err := s.Send(ctx, 0, "残高について相談です")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.inputs[0].Facts) != 1 || a.inputs[0].Facts[0].Value != "50000円" || a.inputs[0].Facts[0].RecordedAt != "2026-09-01T00:00:00Z" {
		t.Fatalf("legacy context=%#v", a.inputs[0])
	}
	a.replies[0].Changes[0].Operation = "forget"
	if _, err = s.Send(ctx, thread.ID, "残高を忘れて"); err != nil {
		t.Fatal(err)
	}
	state, err := repo.LoadChatState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range state.Facts {
		if f.Active && f.Key == "current_balance" {
			t.Fatal("forgotten legacy balance was imported again")
		}
	}
}
