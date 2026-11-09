package consultation_test

import (
	"context"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"path/filepath"
	"testing"
	"time"
)

func activeValue(t *testing.T, state consultation.State, key string) string {
	t.Helper()
	for _, f := range state.Facts {
		if f.Active && f.Key == key {
			return f.Value
		}
	}
	t.Fatalf("missing %s", key)
	return ""
}
func needsClass(id string, amount int64, changes ...consultation.Change) consultation.Reply {
	r := paid(id, "needs_classification", "unknown", &amount, changes...)
	r.Status = "needs_information"
	r.Question = "仕事に必要な支出ですか？"
	r.Answer = r.Question
	return r
}

func TestKnownActualClassificationWaitBalanceAndAutomaticExclusion(t *testing.T) {
	for _, class := range []string{"free", "necessary"} {
		t.Run(class, func(t *testing.T) {
			ctx := identity.WithUser(context.Background(), 1)
			repo := openRepo(t, filepath.Join(t.TempDir(), "classification.db"))
			now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
			first := planned("道具", 3000)
			first.Changes = []consultation.Change{fact("current_balance", "50000円", "2026-10-12"), fact("monthly_free_budget", "30000円", "2026-10")}
			a := &scriptedAgent{replies: []consultation.Reply{first, reply("answered")}}
			s := consultation.Service{Repository: repo, Agent: a, Now: func() time.Time { return now }}
			thread, err := s.Send(ctx, 0, "3000円の道具")
			if err != nil {
				t.Fatal(err)
			}
			id := thread.Purchases[0].ID
			now = now.Add(6 * time.Hour)
			if err = s.DeliverDue(ctx); err != nil {
				t.Fatal(err)
			}
			_, err = s.Send(ctx, thread.ID, "別の話")
			if err != nil {
				t.Fatal(err)
			}
			// The review reproduction: unknown classification must not exempt the current balance.
			bad := needsClass(id, 2500)
			good := needsClass(id, 2500, fact("current_balance", "47500円", "2026-10-12"))
			staleAmount := int64(3000)
			stale := paid(id, "unanswered", "free", &staleAmount, fact("current_balance", "47000円", "2026-10-12"), fact("monthly_free_budget", "27000円", "2026-10"))
			a.replies = append(a.replies, bad, good, stale, reply("answered"), reply("answered"))
			_, err = s.Send(ctx, thread.ID, "2500円で買った")
			if err != nil {
				t.Fatal(err)
			}
			state, _ := repo.LoadChatState(ctx)
			p := state.Purchases[0]
			if activeValue(t, state, "current_balance") != "47500円" || activeValue(t, state, "monthly_free_budget") != "30000円" || p.BalanceAppliedAmount == nil || *p.BalanceAppliedAmount != 2500 || p.FreeBudgetAppliedAmount != nil || p.Events[0].BalanceAppliedAmount == nil {
				t.Fatalf("classification wait=%+v", state)
			}
			_, err = s.Send(ctx, thread.ID, "明日の天気は？")
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Send(ctx, thread.ID, "別の話を続ける")
			if err != nil {
				t.Fatal(err)
			}
			for _, in := range a.inputs[4:] {
				if len(in.AutoConfirmIDs) != 0 {
					t.Fatal("known actual remained automatic candidate")
				}
			}
			state, _ = repo.LoadChatState(ctx)
			if *state.Purchases[0].ActualAmount != 2500 || len(state.Purchases[0].Events) != 1 {
				t.Fatal("known payment replaced")
			}
			amount := int64(2500)
			done := paid(id, "purchased", class, &amount)
			if class == "free" {
				done.Changes = []consultation.Change{fact("monthly_free_budget", "27500円", "2026-10")}
			}
			// A classification-only double subtraction is rejected, then corrected by the model.
			twice := done
			twice.Changes = append(append([]consultation.Change(nil), done.Changes...), fact("current_balance", "45000円", "2026-10-12"))
			a.replies = append(a.replies, twice, done)
			_, err = s.Send(ctx, thread.ID, "分類は"+class)
			if err != nil {
				t.Fatal(err)
			}
			state, _ = repo.LoadChatState(ctx)
			p = state.Purchases[0]
			if activeValue(t, state, "current_balance") != "47500円" || *p.ActualAmount != 2500 || len(p.Events) != 2 {
				t.Fatal("classification double debited")
			}
			if class == "free" {
				if activeValue(t, state, "monthly_free_budget") != "27500円" || p.FreeBudgetAppliedAmount == nil {
					t.Fatal("free budget not applied")
				}
			} else if activeValue(t, state, "monthly_free_budget") != "30000円" || p.FreeBudgetAppliedAmount != nil {
				t.Fatal("necessary expense changed free budget")
			}
		})
	}
}

func TestClassificationAndAmountCorrectionsAndExplicitBalanceReport(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "corrections.db"))
	first := planned("道具", 3000)
	first.Changes = []consultation.Change{fact("current_balance", "50000円", "2026-10-12"), fact("monthly_free_budget", "30000円", "2026-10")}
	a := &scriptedAgent{replies: []consultation.Reply{first}}
	s := consultation.Service{Repository: repo, Agent: a}
	thread, err := s.Send(ctx, 0, "3000円の道具")
	if err != nil {
		t.Fatal(err)
	}
	id := thread.Purchases[0].ID
	amount := int64(2500)
	a.replies = append(a.replies, needsClass(id, 2500, fact("current_balance", "47500円", "2026-10-12")))
	_, err = s.Send(ctx, thread.ID, "2500円で買った")
	if err != nil {
		t.Fatal(err)
	}
	userBalance := fact("current_balance", "60000円", "2026-10-12")
	userBalance.Basis = "user_report"
	a.replies = append(a.replies, paid(id, "purchased", "free", &amount, userBalance, fact("monthly_free_budget", "27500円", "2026-10")))
	_, err = s.Send(ctx, thread.ID, "趣味用です。現在残高は新しく確認したら6万円です")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := repo.LoadChatState(ctx)
	if activeValue(t, state, "current_balance") != "60000円" || state.Purchases[0].Events[1].BalanceBasis != "user_report" {
		t.Fatal("explicit balance report rejected")
	}
	a.replies = append(a.replies, paid(id, "purchased", "necessary", &amount, fact("monthly_free_budget", "30000円", "2026-10")))
	_, err = s.Send(ctx, thread.ID, "訂正、仕事に必要でした")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	if activeValue(t, state, "current_balance") != "60000円" || activeValue(t, state, "monthly_free_budget") != "30000円" || state.Purchases[0].FreeBudgetAppliedAmount != nil {
		t.Fatal("free to necessary correction")
	}
	a.replies = append(a.replies, paid(id, "purchased", "free", &amount, fact("monthly_free_budget", "27500円", "2026-10")))
	_, err = s.Send(ctx, thread.ID, "やっぱり趣味用です")
	if err != nil {
		t.Fatal(err)
	}
	corrected := int64(2200)
	a.replies = append(a.replies, paid(id, "purchased", "free", &corrected, fact("current_balance", "60300円", "2026-10-12"), fact("monthly_free_budget", "27800円", "2026-10")))
	_, err = s.Send(ctx, thread.ID, "実額は2200円でした")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	p := state.Purchases[0]
	if len(p.Events) != 5 || *p.ActualAmount != 2200 || *p.BalanceAppliedAmount != 2200 || *p.FreeBudgetAppliedAmount != 2200 || *p.Events[0].Amount != 2500 || activeValue(t, state, "current_balance") != "60300円" {
		t.Fatal("real amount correction lost accounting history")
	}
}

func TestAmbiguousTargetDoesNotChangeMultiplePurchases(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "ambiguous.db"))
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	a := &scriptedAgent{replies: []consultation.Reply{planned("本", 1000), planned("傘", 1000), reply("answered")}}
	s := consultation.Service{Repository: repo, Agent: a, Now: func() time.Time { return now }}
	one, err := s.Send(ctx, 0, "1000円の本")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Send(ctx, one.ID, "1000円の傘も")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(6 * time.Hour)
	if err = s.DeliverDue(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = s.Send(ctx, one.ID, "別の話")
	if err != nil {
		t.Fatal(err)
	}
	unclear := reply("needs_information")
	unclear.PurchaseTargetUnclear = true
	unclear.Question = "本と傘のどちらですか？"
	unclear.Answer = unclear.Question
	amount := int64(1000)
	bad := unclear
	bad.PurchaseUpdates = []consultation.PurchaseUpdate{{ID: one.Purchases[0].ID, Outcome: "needs_classification", Amount: &amount, Classification: "unknown"}}
	a.replies = append(a.replies, bad, unclear)
	_, err = s.Send(ctx, one.ID, "それを買った")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := repo.LoadChatState(ctx)
	for _, p := range state.Purchases {
		if p.ActualAmount != nil || len(p.Events) != 0 || p.Status != "purchase_confirmation_pending" {
			t.Fatalf("ambiguous target mutated: %+v", p)
		}
	}
}

func TestActualCorrectionWhileClassificationPendingAndUnknownPlannedClassification(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "pending-correction.db"))
	first := planned("道具", 3000)
	first.Changes = []consultation.Change{fact("current_balance", "50000円", "2026-10-12"), fact("monthly_free_budget", "30000円", "2026-10")}
	a := &scriptedAgent{replies: []consultation.Reply{first}}
	s := consultation.Service{Repository: repo, Agent: a}
	thread, err := s.Send(ctx, 0, "3000円の道具")
	if err != nil {
		t.Fatal(err)
	}
	id := thread.Purchases[0].ID
	unverified := paid(id, "classification_required", "unknown", nil)
	unverified.Status = "needs_information"
	unverified.Question = "仕事用ですか？"
	a.replies = append(a.replies, unverified)
	_, err = s.Send(ctx, thread.ID, "まだ買ったか答えない")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := repo.LoadChatState(ctx)
	if state.Purchases[0].ActualAmount != nil || state.Purchases[0].BalanceAppliedAmount != nil || activeValue(t, state, "current_balance") != "50000円" {
		t.Fatal("unverified plan turned into actual payment")
	}
	a.replies = append(a.replies, needsClass(id, 2500, fact("current_balance", "47500円", "2026-10-12")), needsClass(id, 2200, fact("current_balance", "47800円", "2026-10-12")))
	_, err = s.Send(ctx, thread.ID, "実際2500円で買った")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Send(ctx, thread.ID, "訂正、2200円でした")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	p := state.Purchases[0]
	if *p.ActualAmount != 2200 || *p.BalanceAppliedAmount != 2200 || activeValue(t, state, "current_balance") != "47800円" || activeValue(t, state, "monthly_free_budget") != "30000円" || len(p.Events) != 3 || *p.Events[1].Amount != 2500 {
		t.Fatal("classification-pending correction lost state")
	}
}

func TestAutomaticUnknownClassificationAppliesBalanceAndKeepsBudgetPending(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "automatic-unknown.db"))
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	first := planned("道具", 3000)
	first.Changes = []consultation.Change{fact("current_balance", "50000円", "2026-10-12"), fact("monthly_free_budget", "30000円", "2026-10")}
	a := &scriptedAgent{replies: []consultation.Reply{first, reply("answered")}}
	s := consultation.Service{Repository: repo, Agent: a, Now: func() time.Time { return now }}
	thread, err := s.Send(ctx, 0, "3000円の道具")
	if err != nil {
		t.Fatal(err)
	}
	id := thread.Purchases[0].ID
	now = now.Add(6 * time.Hour)
	if err = s.DeliverDue(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = s.Send(ctx, thread.ID, "別の話")
	if err != nil {
		t.Fatal(err)
	}
	deferOnly := paid(id, "classification_required", "unknown", nil)
	deferOnly.Question = "仕事用ですか？"
	amount := int64(3000)
	assumed := paid(id, "unanswered", "unknown", &amount, fact("current_balance", "47000円", "2026-10-12"))
	assumed.Question = "仕事用ですか？"
	assumed.Status = "needs_information"
	a.replies = append(a.replies, deferOnly, assumed, reply("answered"))
	_, err = s.Send(ctx, thread.ID, "明日の天気は？")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := repo.LoadChatState(ctx)
	p := state.Purchases[0]
	if activeValue(t, state, "current_balance") != "47000円" || activeValue(t, state, "monthly_free_budget") != "30000円" || *p.ActualAmount != 3000 || p.Status != "needs_classification" || p.Events[0].Outcome != "unanswered" || *p.BalanceAppliedAmount != 3000 || p.FreeBudgetAppliedAmount != nil {
		t.Fatal("automatic classification wait failed to post balance")
	}
	_, err = s.Send(ctx, thread.ID, "まだ別の話")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.inputs[len(a.inputs)-1].AutoConfirmIDs) != 0 {
		t.Fatal("assumed paid amount remained automatic candidate")
	}
	a.replies = append(a.replies, paid(id, "purchased", "free", &amount, fact("monthly_free_budget", "27000円", "2026-10")))
	_, err = s.Send(ctx, thread.ID, "趣味用です")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	if activeValue(t, state, "current_balance") != "47000円" || activeValue(t, state, "monthly_free_budget") != "27000円" {
		t.Fatal("automatic classification caused double payment")
	}
}
