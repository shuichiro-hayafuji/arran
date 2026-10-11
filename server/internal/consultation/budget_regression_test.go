package consultation_test

import (
	"context"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBudgetAlreadyAppliedThroughUnknownClassification(t *testing.T) {
	ctx := identity.WithUser(context.Background(), 1)
	repo := openRepo(t, filepath.Join(t.TempDir(), "budget.db"))
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
	a.replies = append(a.replies, paid(id, "purchased", "free", &amount, fact("current_balance", "47500円", "2026-10-12"), fact("monthly_free_budget", "27500円", "2026-10")), needsClass(id, 2500))
	_, err = s.Send(ctx, thread.ID, "2500円で趣味用に買った")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Send(ctx, thread.ID, "分類が分からなくなった。同じ2500円です")
	if err != nil {
		t.Fatal(err)
	}
	bad := paid(id, "purchased", "free", &amount, fact("monthly_free_budget", "25000円", "2026-10"))
	a.replies = append(a.replies, bad, paid(id, "purchased", "free", &amount))
	_, err = s.Send(ctx, thread.ID, "やはり趣味用です")
	if err != nil {
		t.Fatal(err)
	}
	state, _ := repo.LoadChatState(ctx)
	if activeValue(t, state, "monthly_free_budget") != "27500円" || activeValue(t, state, "current_balance") != "47500円" || *state.Purchases[0].FreeBudgetAppliedAmount != 2500 {
		t.Fatal("same amount posted to budget twice")
	}
	// Correct the actual while classification is unknown; retain the old budget
	// posting until the model resolves classification and applies the difference.
	a.replies = append(a.replies, needsClass(id, 2200, fact("current_balance", "47800円", "2026-10-12")))
	_, err = s.Send(ctx, thread.ID, "2200円に訂正。分類は不明")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	if *state.Purchases[0].FreeBudgetAppliedAmount != 2500 || activeValue(t, state, "monthly_free_budget") != "27500円" {
		t.Fatal("unknown classification erased old budget posting")
	}
	corrected := int64(2200)
	a.replies = append(a.replies, paid(id, "purchased", "free", &corrected, fact("monthly_free_budget", "27800円", "2026-10")))
	_, err = s.Send(ctx, thread.ID, "自由支出でした")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	if *state.Purchases[0].FreeBudgetAppliedAmount != 2200 || activeValue(t, state, "monthly_free_budget") != "27800円" {
		t.Fatal("corrected free amount not applied")
	}
	// A user explicitly supplying a new budget is a separate report, not a second charge.
	a.replies = append(a.replies, needsClass(id, 2200))
	_, err = s.Send(ctx, thread.ID, "分類を考え直したい")
	if err != nil {
		t.Fatal(err)
	}
	newBudget := fact("monthly_free_budget", "40000円", "2026-10")
	newBudget.Basis = "user_report"
	a.replies = append(a.replies, paid(id, "purchased", "free", &corrected, newBudget))
	_, err = s.Send(ctx, thread.ID, "自由支出です。今の自由予算は4万円と確認しました")
	if err != nil {
		t.Fatal(err)
	}
	state, _ = repo.LoadChatState(ctx)
	if activeValue(t, state, "monthly_free_budget") != "40000円" || *state.Purchases[0].FreeBudgetAppliedAmount != 2200 {
		t.Fatal("explicit new budget report rejected")
	}
}

func TestBudgetReversalAfterUnknownClassification(t *testing.T) {
	for _, outcome := range []string{"necessary", "not_purchased"} {
		t.Run(outcome, func(t *testing.T) {
			ctx := identity.WithUser(context.Background(), 1)
			repo := openRepo(t, filepath.Join(t.TempDir(), "reverse.db"))
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
			a.replies = append(a.replies, paid(id, "purchased", "free", &amount, fact("current_balance", "47500円", "2026-10-12"), fact("monthly_free_budget", "27500円", "2026-10")), needsClass(id, 2500))
			_, err = s.Send(ctx, thread.ID, "2500円で買った")
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Send(ctx, thread.ID, "分類不明")
			if err != nil {
				t.Fatal(err)
			}
			correction := paid(id, "purchased", "necessary", &amount, fact("monthly_free_budget", "30000円", "2026-10"))
			if outcome == "not_purchased" {
				correction = paid(id, "not_purchased", "unknown", nil, fact("current_balance", "50000円", "2026-10-12"), fact("monthly_free_budget", "30000円", "2026-10"))
			}
			bad := correction
			bad.Changes = nil
			a.replies = append(a.replies, bad, correction)
			_, err = s.Send(ctx, thread.ID, "訂正"+outcome)
			if err != nil {
				t.Fatal(err)
			}
			state, _ := repo.LoadChatState(ctx)
			if state.Purchases[0].FreeBudgetAppliedAmount != nil || activeValue(t, state, "monthly_free_budget") != "30000円" {
				t.Fatal("previous budget posting not reversed")
			}
		})
	}
}

func TestFirstConfirmationAnswerOnlyReasksUnansweredPurchases(t *testing.T) {
	for _, answer := range []string{"purchased", "not_purchased", "cancel", "needs_classification", "partial"} {
		t.Run(answer, func(t *testing.T) {
			ctx := identity.WithUser(context.Background(), 1)
			repo := openRepo(t, filepath.Join(t.TempDir(), "confirmation.db"))
			now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
			a := &scriptedAgent{replies: []consultation.Reply{planned("本", 1000)}}
			s := consultation.Service{Repository: repo, Agent: a, Now: func() time.Time { return now }}
			thread, err := s.Send(ctx, 0, "1000円の本")
			if err != nil {
				t.Fatal(err)
			}
			id := thread.Purchases[0].ID
			if answer == "partial" {
				a.replies = append(a.replies, planned("傘", 1000))
				thread, err = s.Send(ctx, thread.ID, "1000円の傘も")
				if err != nil {
					t.Fatal(err)
				}
			}
			now = now.Add(6 * time.Hour)
			if err = s.DeliverDue(ctx); err != nil {
				t.Fatal(err)
			}
			amount := int64(1000)
			r := paid(id, "purchased", "free", &amount)
			switch answer {
			case "not_purchased":
				r = paid(id, "not_purchased", "unknown", nil)
			case "cancel":
				r = reply("answered")
				r.PurchaseChange = &consultation.PurchaseChange{ID: id, Operation: "cancel"}
			case "needs_classification":
				r = needsClass(id, 1000)
			}
			a.replies = append(a.replies, r)
			updated, err := s.Send(ctx, thread.ID, "本の購入結果を回答します")
			if err != nil {
				t.Fatal(err)
			}
			if len(a.inputs[len(a.inputs)-1].AutoConfirmIDs) != 0 {
				t.Fatal("same-turn automatic confirmation")
			}
			state, _ := repo.LoadChatState(ctx)
			if state.Purchases[0].SecondAskedAt != "" {
				t.Fatal("answered purchase asked twice")
			}
			if answer == "partial" {
				if state.Purchases[1].SecondAskedAt == "" || !strings.Contains(updated.Reply.Answer, "傘") || strings.Contains(updated.Reply.Answer, "・本") {
					t.Fatal("partial answer reasked answered purchase")
				}
			} else if strings.Contains(updated.Reply.Answer, "2回目") {
				t.Fatal("clear first response triggered second confirmation")
			}
		})
	}
}
