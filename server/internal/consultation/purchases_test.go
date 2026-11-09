package consultation_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func planned(product string, amount int64) consultation.Reply {
	r := reply("answered")
	r.PurchaseChange = &consultation.PurchaseChange{Operation: "add", Description: &product, Amount: &amount}
	return r
}
func paid(id, outcome, class string, amount *int64, changes ...consultation.Change) consultation.Reply {
	r := reply("answered", changes...)
	r.PurchaseUpdates = []consultation.PurchaseUpdate{{ID: id, Outcome: outcome, Classification: class, Amount: amount}}
	return r
}

func TestPurchaseLifecycleMultipleDueSecondAutomaticCorrectionAndRestart(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	path := filepath.Join(t.TempDir(), "purchases.db")
	repo := openRepo(t, path)
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	a := &scriptedAgent{replies: []consultation.Reply{planned("イヤホン", 20000), planned("洗剤", 1000)}}
	a.replies[0].Changes = []consultation.Change{fact("current_balance", "50000円", "2026-10-12"), fact("monthly_free_budget", "30000円", "2026-10")}
	s := consultation.Service{Repository: repo, Agent: a, Now: func() time.Time { return now }}
	one, err := s.Send(ctx, 0, "2万円のイヤホンを買いたい")
	if err != nil {
		t.Fatal(err)
	}
	two, err := s.Send(ctx, 0, "千円の洗剤を買いたい")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := repo.LoadChatState(ctx)
	if len(state.Purchases) != 2 {
		t.Fatalf("purchases=%+v", state.Purchases)
	}
	id1, id2 := state.Purchases[0].ID, state.Purchases[1].ID
	now = now.Add(6*time.Hour - time.Second)
	if err = s.DeliverDue(ctx); err != nil {
		t.Fatal(err)
	}
	before, _ := repo.LoadChatThread(ctx, one.ID)
	if len(before.Messages) != 2 {
		t.Fatal("early reminder")
	}
	now = now.Add(time.Second)
	if err = s.DeliverDue(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.DeliverDue(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{one.ID, two.ID} {
		v, _ := repo.LoadChatThread(ctx, id)
		if len(v.Messages) != 3 || !strings.Contains(v.Messages[2].Content, "1回目") {
			t.Fatalf("first=%+v", v)
		}
	}
	now = now.Add(24 * time.Hour)
	if err = s.DeliverDue(ctx); err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	if state.Purchases[0].Status != "purchase_confirmation_pending" {
		t.Fatal("inactive purchase auto confirmed")
	}
	a.replies = append(a.replies, reply("answered"))
	turn, err := s.Send(ctx, two.ID, "他の相談をしたい")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(turn.Reply.Answer, "2回目") || !strings.Contains(turn.Reply.Answer, "イヤホン") || !strings.Contains(turn.Reply.Answer, "洗剤") || !strings.Contains(turn.Reply.Answer, "2026-10-12") {
		t.Fatalf("grouped confirmation=%s", turn.Reply.Answer)
	}
	if len(a.inputs[2].AutoConfirmIDs) != 0 {
		t.Fatal("same-turn assumption")
	}
	amount := int64(20000)
	auto := paid(id1, "unanswered", "free", &amount, fact("current_balance", "30000円", "2026-10-13"), fact("monthly_free_budget", "10000円", "2026-10"))
	auto.PurchaseUpdates = append(auto.PurchaseUpdates, consultation.PurchaseUpdate{ID: id2, Outcome: "not_purchased", Classification: "necessary"})
	a.replies = append(a.replies, auto)
	_, err = s.Send(ctx, two.ID, "洗剤は買ってない。別の話を続ける")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	if state.Purchases[0].Status != "unanswered" || state.Purchases[1].Status != "not_purchased" {
		t.Fatalf("outcomes=%+v", state.Purchases)
	}
	if len(a.inputs[3].AutoConfirmIDs) != 2 {
		t.Fatal("automatic candidates missing")
	}
	reopened := openRepo(t, path)
	s.Repository = reopened
	corrected := int64(18000)
	a.replies = append(a.replies, paid(id1, "purchased", "free", &corrected, fact("current_balance", "32000円", "2026-10-13"), fact("monthly_free_budget", "12000円", "2026-10")))
	_, err = s.Send(ctx, one.ID, "イヤホンは実際18000円だった")
	if err != nil {
		t.Fatal(err)
	}
	original, _ := reopened.LoadChatThread(ctx, one.ID)
	if len(original.Purchases) != 1 || len(original.Purchases[0].Events) != 2 || *original.Purchases[0].Events[0].Amount != 20000 || *original.Purchases[0].ActualAmount != 18000 {
		t.Fatalf("correction history=%+v", original.Purchases)
	}
	a.replies = append(a.replies, paid(id1, "not_purchased", "free", nil, fact("current_balance", "50000円", "2026-10-13"), fact("monthly_free_budget", "30000円", "2026-10")))
	_, err = s.Send(ctx, one.ID, "ごめん、イヤホンは買ってない")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = reopened.LoadChatState(ctx)
	if len(state.Purchases[0].Events) != 3 || state.Purchases[0].ActualAmount != nil {
		t.Fatal("correction not retained")
	}
	// A stale full state cannot erase events or overwrite reminders.
	state.Version--
	if _, err = reopened.SaveChatTurn(ctx, state, original); !errors.Is(err, consultation.ErrConflict) {
		t.Fatalf("stale=%v", err)
	}
	other := identity.WithUser(context.Background(), 2)
	if _, err = reopened.LoadChatThread(other, one.ID); !errors.Is(err, consultation.ErrNotFound) {
		t.Fatalf("owner=%v", err)
	}
}

func TestPurchaseFailureAndUnknownClassificationKeepActualAmount(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "failure.db"))
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	a := &scriptedAgent{replies: []consultation.Reply{planned("道具", 3000)}}
	s := consultation.Service{Repository: repo, Agent: a, Now: func() time.Time { return now }}
	thread, err := s.Send(ctx, 0, "3000円の道具を検討")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := repo.LoadChatState(ctx)
	id := state.Purchases[0].ID
	actual := int64(2500)
	r := paid(id, "needs_classification", "unknown", &actual)
	r.Status = "needs_information"
	r.Question = "仕事に必要な物ですか？"
	r.Answer = r.Question
	a.replies = append(a.replies, r)
	_, err = s.Send(ctx, thread.ID, "実際は2500円で買った")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	if state.Purchases[0].Status != "needs_classification" || *state.Purchases[0].ActualAmount != 2500 {
		t.Fatal("actual lost")
	}
	a.fail = true
	unavailable, err := s.Send(ctx, thread.ID, "仕事用です")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	if unavailable.Reply.Status != "unavailable" || *state.Purchases[0].ActualAmount != 2500 || state.Purchases[0].Amount != 3000 {
		t.Fatal("model failure lost amounts")
	}
	a.fail = false
	a.replies = append(a.replies, paid(id, "purchased", "necessary", &actual))
	_, err = s.Send(ctx, thread.ID, "仕事用です")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	if state.Purchases[0].Status != "purchased" || state.Purchases[0].Classification != "necessary" {
		t.Fatal("classification not stored")
	}
}

func TestMultiplePurchasesInSameThreadAndIDRequired(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "multiple.db"))
	a := &scriptedAgent{replies: []consultation.Reply{planned("本", 1000), planned("傘", 1000)}}
	s := consultation.Service{Repository: repo, Agent: a}
	thread, err := s.Send(ctx, 0, "1000円の本")
	if err != nil {
		t.Fatal(err)
	}
	thread, err = s.Send(ctx, thread.ID, "さらに1000円の傘")
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.Purchases) != 2 || thread.Purchases[0].ID == thread.Purchases[1].ID {
		t.Fatalf("same thread=%+v", thread.Purchases)
	}
	amount := int64(800)
	r := paid(thread.Purchases[0].ID, "purchased", "free", &amount)
	a.replies = append(a.replies, r)
	thread, err = s.Send(ctx, thread.ID, "本だけ800円で買った")
	if err != nil {
		t.Fatal(err)
	}
	if thread.Purchases[0].Status != "purchased" || thread.Purchases[1].Status != "purchase_confirmation_pending" {
		t.Fatal("misattributed")
	}
}

func TestInvalidPurchaseOutputRetriedWithoutLosingPendingOrFacts(t *testing.T) {
	for _, name := range []string{"unknown ID", "duplicate ID", "early automatic", "missing recalculation", "invalid classification"} {
		t.Run(name, func(t *testing.T) {
			ctx := identity.WithUser(context.Background(), 1)
			repo := openRepo(t, filepath.Join(t.TempDir(), "invalid.db"))
			first := planned("商品", 1000)
			first.Changes = []consultation.Change{fact("current_balance", "50000円", "2026-10"), fact("monthly_free_budget", "30000円", "2026-10")}
			a := &scriptedAgent{replies: []consultation.Reply{first}}
			s := consultation.Service{Repository: repo, Agent: a}
			thread, err := s.Send(ctx, 0, "1000円の商品")
			if err != nil {
				t.Fatal(err)
			}
			id := thread.Purchases[0].ID
			amount := int64(1000)
			bad := paid(id, "purchased", "free", &amount, fact("current_balance", "49000円", "2026-10"), fact("monthly_free_budget", "29000円", "2026-10"))
			switch name {
			case "unknown ID":
				bad.PurchaseUpdates[0].ID = "someone-elses-purchase"
			case "duplicate ID":
				bad.PurchaseUpdates = append(bad.PurchaseUpdates, bad.PurchaseUpdates[0])
			case "early automatic":
				bad.PurchaseUpdates[0].Outcome = "unanswered"
			case "missing recalculation":
				bad.Changes = nil
			case "invalid classification":
				bad.PurchaseUpdates[0].Classification = "anything"
			}
			a.replies = append(a.replies, bad, bad, bad)
			thread, err = s.Send(ctx, thread.ID, "購入の結果を報告")
			if err != nil {
				t.Fatal(err)
			}
			state, _ := repo.LoadChatState(ctx)
			if thread.Reply.Status != "unavailable" || len(a.inputs) != 4 || state.Purchases[0].Amount != 1000 || state.Purchases[0].Status != "purchase_confirmation_pending" || len(state.Purchases[0].Events) != 0 || len(state.Facts) != 2 {
				t.Fatalf("invalid response lost state: %+v", state)
			}
		})
	}
}

func TestForgottenPurchaseNotReplayedOrConfirmed(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "forgotten.db"))
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	a := &scriptedAgent{replies: []consultation.Reply{planned("秘密の商品", 1000), reply("answered", consultation.Change{Key: "preference", Kind: "fact", Operation: "forget"}), reply("answered")}}
	s := consultation.Service{Repository: repo, Agent: a, Now: func() time.Time { return now }}
	thread, err := s.Send(ctx, 0, "1000円の秘密の商品")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	_, err = s.Send(ctx, thread.ID, "忘れて")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(7 * time.Hour)
	if err = s.DeliverDue(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = s.Send(ctx, thread.ID, "別の話")
	if err != nil {
		t.Fatal(err)
	}
	in := a.inputs[2]
	if len(in.Purchases) != 0 || len(in.AutoConfirmIDs) != 0 || in.PendingPurchase != nil {
		t.Fatalf("forgotten purchase replayed: %+v", in)
	}
	stored, _ := repo.LoadChatThread(ctx, thread.ID)
	for _, m := range stored.Messages {
		if strings.Contains(m.Content, "1回目") {
			t.Fatal("forgotten purchase confirmed")
		}
	}
}

func TestRepeatedPurchaseAndSameAmountCorrectionCannotDoublePost(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "idempotent.db"))
	first := planned("本", 1000)
	first.Changes = []consultation.Change{fact("current_balance", "50000円", "2026-10"), fact("monthly_free_budget", "30000円", "2026-10")}
	a := &scriptedAgent{replies: []consultation.Reply{first}}
	s := consultation.Service{Repository: repo, Agent: a}
	thread, err := s.Send(ctx, 0, "1000円の本")
	if err != nil {
		t.Fatal(err)
	}
	id := thread.Purchases[0].ID
	amount := int64(1000)
	a.replies = append(a.replies, paid(id, "purchased", "free", &amount, fact("current_balance", "49000円", "2026-10"), fact("monthly_free_budget", "29000円", "2026-10")))
	_, err = s.Send(ctx, thread.ID, "予定どおり買った")
	if err != nil {
		t.Fatal(err)
	}
	duplicate := paid(id, "purchased", "free", &amount, fact("current_balance", "48000円", "2026-10"), fact("monthly_free_budget", "28000円", "2026-10"))
	a.replies = append(a.replies, duplicate, reply("answered"), reply("answered"))
	_, err = s.Send(ctx, thread.ID, "同じ本、予定どおり買いました")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Send(ctx, thread.ID, "実額は訂正しても1000円です")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := repo.LoadChatState(ctx)
	if len(state.Purchases[0].Events) != 1 || len(state.Facts) != 4 || state.Facts[2].Value != "49000円" || state.Facts[3].Value != "29000円" {
		t.Fatalf("duplicate posting=%+v", state)
	}
}

func TestPurchaseContextLimitsAndRelatedFinalizedHistory(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "bounded.db"))
	state, _ := repo.LoadChatState(ctx)
	state.Facts = []consultation.Fact{{Key: "current_balance", Value: "api_key=hidden-value 残高50000円", Source: "access_token=hidden-source", Kind: "fact", Active: true, RecordedAt: "2026-10-12T00:00:00Z"}}
	for i := 0; i < 40; i++ {
		state.Purchases = append(state.Purchases, consultation.Purchase{ID: fmt.Sprintf("pending:%d", i), Description: strings.Repeat("商品", 80), Amount: 1000, RecordedAt: "2026-10-12T00:00:00Z", Status: "purchase_confirmation_pending"})
	}
	state.Purchases = append(state.Purchases, consultation.Purchase{ID: "related", Description: "関連イヤホン", Amount: 1000, RecordedAt: "2026-10-12T00:00:00Z", Status: "purchased"}, consultation.Purchase{ID: "unrelated", Description: "無関係の家具", Amount: 1000, RecordedAt: "2026-10-12T00:00:00Z", Status: "purchased"})
	for i := 0; i < 20; i++ {
		state.Purchases[0].Events = append(state.Purchases[0].Events, consultation.PurchaseEvent{Outcome: "needs_classification", Source: "access_token=event-secret"})
	}
	_, err := repo.SaveChatTurn(ctx, state, consultation.Thread{CreatedAt: "2026-10-12T00:00:00Z", Messages: []consultation.Message{{Role: "user", Content: "相談"}}, Reply: reply("answered")})
	if err != nil {
		t.Fatal(err)
	}
	a := &scriptedAgent{replies: []consultation.Reply{reply("answered")}}
	s := consultation.Service{Repository: repo, Agent: a}
	_, err = s.Send(ctx, 0, "関連イヤホンの話")
	if err != nil {
		t.Fatal(err)
	}
	in := a.inputs[0]
	encoded, _ := json.Marshal(in.Purchases)
	if len(encoded) > 16000 || !in.PurchaseCoverage.Partial || in.PurchaseCoverage.Omitted == 0 || len(in.Purchases) > 26 {
		t.Fatalf("unbounded=%+v", in.PurchaseCoverage)
	}
	related := false
	for _, p := range in.Purchases {
		if len(p.Events) > 6 {
			t.Fatal("events unbounded")
		}
		if p.ID == "unrelated" {
			t.Fatal("unrelated posted history transmitted")
		}
		if p.ID == "related" {
			related = true
		}
	}
	if !related {
		t.Fatal("related correction history omitted")
	}
	factEncoded, _ := json.Marshal(in.Facts)
	if strings.Contains(string(factEncoded), "hidden-value") || strings.Contains(string(factEncoded), "hidden-source") || strings.Contains(string(encoded), "event-secret") {
		t.Fatal("secret escaped redaction")
	}
}
