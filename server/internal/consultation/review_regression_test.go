package consultation_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

func TestCommittedCSVChangesChatInputWithoutMerchantOrOtherOwnerData(t *testing.T) {
	repo := openRepo(t, ":memory:")
	owner := identity.WithUser(context.Background(), 1)
	other := identity.WithUser(context.Background(), 2)
	a := &scriptedAgent{replies: []consultation.Reply{reply("answered"), reply("answered"), reply("answered"), reply("answered")}}
	s := consultation.Service{Repository: repo, Agent: a}
	if _, err := s.Send(owner, 0, "イヤホンの支出について相談"); err != nil {
		t.Fatal(err)
	}
	if len(a.inputs[0].Transactions) != 0 || !a.inputs[0].TransactionCoverage.Partial {
		t.Fatal("empty CSV interpreted as full data")
	}
	rows := []domain.Transaction{
		{TransactionDate: "2026-10-10", Amount: 2000, TransactionType: "expense", Category: "趣味", Description: "private merchant", SourceAccountName: "private account", RawData: map[string]any{"line": "raw CSV"}, Fingerprint: "own-expense", ImportedAt: "2026-10-12T00:00:00Z"},
		{TransactionDate: "2026-10-11", Amount: 300000, TransactionType: "income", Category: "収入", Fingerprint: "income"},
	}
	if _, err := repo.ImportTransactions(owner, rows); err != nil {
		t.Fatal(err)
	}
	rows[0].Amount, rows[0].Category, rows[0].Fingerprint = 99999, "other owner", "other-expense"
	if _, err := repo.ImportTransactions(other, rows[:1]); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Send(owner, 0, "イヤホンの支出について相談"); err != nil {
		t.Fatal(err)
	}
	input := a.inputs[1]
	if len(input.Transactions) != 1 || input.Transactions[0].Amount != 2000 || input.TransactionCoverage.StartDate != "2026-10-10" || input.TransactionCoverage.EndDate != "2026-10-10" || !input.TransactionCoverage.Partial {
		t.Fatalf("CSV input = %#v", input)
	}
	encoded, _ := json.Marshal(input.Transactions)
	for _, forbidden := range []string{"merchant", "account", "raw CSV", "other owner", "transaction_type", "imported_at", "fingerprint"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("CSV sent %q: %s", forbidden, encoded)
		}
	}
	for _, oldRow := range []consultation.Fact{{Key: "goal", Value: "", Active: false, RecordedAt: "2026-10-12T00:00:01Z"}} {
		state, _ := repo.LoadChatState(owner)
		state.Facts = append(state.Facts, oldRow)
		thread, _ := repo.LoadChatThread(owner, 1)
		if _, err := repo.SaveChatTurn(owner, state, thread); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Send(owner, 0, "イヤホンについて相談"); err != nil {
		t.Fatal(err)
	}
	if len(a.inputs[2].Transactions) != 0 {
		t.Fatal("forgotten context resurrected by old CSV")
	}
	rows[0].Amount, rows[0].Category, rows[0].Fingerprint, rows[0].ImportedAt = 4000, "趣味", "new-expense", "2026-10-12T00:00:02Z"
	if _, err := repo.ImportTransactions(owner, rows[:1]); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Send(owner, 0, "イヤホンについて相談"); err != nil {
		t.Fatal(err)
	}
	if len(a.inputs[3].Transactions) != 1 || a.inputs[3].Transactions[0].Amount != 4000 {
		t.Fatal("newly imported data excluded after forgetting")
	}
}

func TestPendingProductSupplementCorrectionReplacementAndCancellation(t *testing.T) {
	repo := openRepo(t, ":memory:")
	amount, corrected := int64(20000), int64(25000)
	headphone, camera := "イヤホン", "カメラ"
	makeReply := func(change *consultation.PurchaseChange) consultation.Reply {
		r := reply("answered")
		r.PurchaseChange = change
		return r
	}
	a := &scriptedAgent{replies: []consultation.Reply{
		makeReply(&consultation.PurchaseChange{Operation: "set", Description: &headphone, Amount: &amount}),
		makeReply(nil),
		makeReply(&consultation.PurchaseChange{Operation: "set", Amount: &corrected}),
		makeReply(&consultation.PurchaseChange{Operation: "set", Description: &camera, Amount: &corrected}),
		makeReply(&consultation.PurchaseChange{Operation: "cancel"}),
	}}
	s := consultation.Service{Repository: repo, Agent: a}
	thread, err := s.Send(context.Background(), 0, "イヤホンを買いたい。予算は2万円")
	if err != nil {
		t.Fatal(err)
	}
	for i, message := range []string{"詳しく教えて", "金額は2万5千円に訂正", "同じ額のカメラにする", "買うのをやめた"} {
		thread, err = s.Send(context.Background(), thread.ID, message)
		if err != nil {
			t.Fatal(err)
		}
		switch i {
		case 0:
			if thread.PendingPurchase.Description != headphone || thread.PendingPurchase.Amount != amount {
				t.Fatal("null supplement erased product")
			}
		case 1:
			if thread.PendingPurchase.Description != headphone || thread.PendingPurchase.Amount != corrected {
				t.Fatal("amount correction lost product")
			}
		case 2:
			if thread.PendingPurchase.Description != camera || thread.PendingPurchase.Amount != corrected {
				t.Fatal("same price replacement ignored")
			}
		case 3:
			if thread.PendingPurchase != nil || thread.Reply.PlannedAmount != nil {
				t.Fatal("cancel retained pending purchase")
			}
		}
	}
	if a.inputs[3].PendingPurchase == nil || a.inputs[3].PendingPurchase.Description != headphone {
		t.Fatal("model did not receive current purchase target")
	}
	stored, _ := repo.GetConsultation(context.Background(), thread.ID)
	if stored.PlannedAmount != nil {
		t.Fatal("cancel left legacy API planned amount")
	}
}

func TestNewConversationRetrievesRelatedAdviceWithPrivacyOwnershipAndForgetCutoff(t *testing.T) {
	repo := openRepo(t, ":memory:")
	owner, other := identity.WithUser(context.Background(), 1), identity.WithUser(context.Background(), 2)
	first := reply("answered")
	first.Reason = "写真を続ける目標がある"
	first.Assumptions = "パスワード:secret は判断に使わない"
	a := &scriptedAgent{replies: []consultation.Reply{first, reply("answered"), reply("answered"), reply("answered"), reply("answered"), reply("answered")}}
	clock := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	s := consultation.Service{Repository: repo, Agent: a, Now: func() time.Time { return clock }}
	photo, err := s.Send(owner, 0, "写真撮影のカメラを検討したい")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Send(owner, 0, "園芸の種を選ぼう"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Send(other, 0, "写真撮影の他人の内容"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Send(owner, 0, "写真撮影のカメラについて前の助言を聞きたい"); err != nil {
		t.Fatal(err)
	}
	input := a.inputs[3]
	found := false
	for _, message := range input.RelatedHistory {
		if message.ConsultationID != photo.ID {
			t.Fatalf("irrelevant/other owner history included = %#v", message)
		}
		if message.Reason == first.Reason {
			found = true
		}
		if strings.Contains(message.Assumptions, "secret") {
			t.Fatal("past advice secret leaked")
		}
	}
	if !found {
		t.Fatal("new conversation lost past related advice")
	}
	state, _ := repo.LoadChatState(owner)
	state.Facts = append(state.Facts, consultation.Fact{Key: "goal", Value: "", Active: false, RecordedAt: clock.Format(time.RFC3339)})
	if _, err := repo.SaveChatTurn(owner, state, photo); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Second)
	if _, err := s.Send(owner, 0, "写真撮影のカメラを改めて相談"); err != nil {
		t.Fatal(err)
	}
	if len(a.inputs[4].RelatedHistory) != 0 {
		t.Fatal("forgotten past advice was recovered")
	}
}

func TestPurchaseChangesRejectMalformedOutputAndForgottenPendingIsNotSent(t *testing.T) {
	amount := int64(20000)
	description := "秘密の写真機材"
	for _, bad := range []*consultation.PurchaseChange{{Operation: "delete"}, {Operation: "set"}, {Operation: "cancel", Amount: &amount}, {Operation: "set", Amount: func() *int64 { value := int64(-1); return &value }()}} {
		r := reply("answered")
		r.PurchaseChange = bad
		if consultation.Validate(r) == nil {
			t.Fatalf("malformed purchase accepted = %#v", bad)
		}
	}
	repo := openRepo(t, ":memory:")
	clock := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	initial := reply("answered")
	initial.PurchaseChange = &consultation.PurchaseChange{Operation: "set", Description: &description, Amount: &amount}
	forgotten := reply("answered", consultation.Change{Key: "goal", Value: "", Kind: "fact", Operation: "forget"})
	agent := &scriptedAgent{replies: []consultation.Reply{initial, forgotten, reply("answered")}}
	service := consultation.Service{Repository: repo, Agent: agent, Now: func() time.Time { return clock }}
	thread, err := service.Send(context.Background(), 0, "写真機材を買う予定です")
	if err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Second)
	thread, err = service.Send(context.Background(), thread.ID, "写真の目標を忘れて")
	if err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Second)
	if _, err = service.Send(context.Background(), thread.ID, "別のことを相談したい"); err != nil {
		t.Fatal(err)
	}
	if agent.inputs[2].PendingPurchase != nil {
		t.Fatal("forgotten pending description resent")
	}
}

func TestAmountOnlyAnswerCreatesPendingWithProductFromConversation(t *testing.T) {
	repo := openRepo(t, ":memory:")
	question := reply("needs_information")
	question.Question = "予定額は？"
	amount := int64(20000)
	product := "イヤホン"
	answer := reply("answered")
	answer.PurchaseChange = &consultation.PurchaseChange{Operation: "set", Description: &product, Amount: &amount}
	agent := &scriptedAgent{replies: []consultation.Reply{question, answer}}
	service := consultation.Service{Repository: repo, Agent: agent}
	thread, err := service.Send(context.Background(), 0, "イヤホンを買いたい")
	if err != nil {
		t.Fatal(err)
	}
	thread, err = service.Send(context.Background(), thread.ID, "2万円です")
	if err != nil {
		t.Fatal(err)
	}
	if thread.PendingPurchase == nil || thread.PendingPurchase.Description != product || thread.PendingPurchase.Amount != amount {
		t.Fatalf("amount-only supplement = %#v", thread.PendingPurchase)
	}
	if len(agent.inputs[1].History) == 0 || agent.inputs[1].History[0].Content != "イヤホンを買いたい" {
		t.Fatal("model could not identify product from original conversation")
	}
}

func TestAmountOnlyChangeCannotRefreshForgottenProductForNextModelTurn(t *testing.T) {
	repo := openRepo(t, ":memory:")
	amount, correction := int64(20000), int64(25000)
	product := "削除した写真機材"
	initial := reply("answered")
	initial.PurchaseChange = &consultation.PurchaseChange{Operation: "set", Description: &product, Amount: &amount}
	forget := reply("answered", consultation.Change{Key: "goal", Value: "", Kind: "fact", Operation: "forget"})
	invalid := reply("answered")
	invalid.PurchaseChange = &consultation.PurchaseChange{Operation: "set", Amount: &correction}
	agent := &scriptedAgent{replies: []consultation.Reply{initial, forget, invalid, invalid, invalid, reply("answered")}}
	clock := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	service := consultation.Service{Repository: repo, Agent: agent, Now: func() time.Time { return clock }}
	thread, err := service.Send(context.Background(), 0, "写真機材を2万円で買いたい")
	if err != nil {
		t.Fatal(err)
	}
	originalRecordedAt := thread.PendingPurchase.RecordedAt
	clock = clock.Add(time.Second)
	thread, err = service.Send(context.Background(), thread.ID, "写真の目標を忘れて")
	if err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Second)
	thread, err = service.Send(context.Background(), thread.ID, "金額は2万5千円です")
	if err != nil {
		t.Fatal(err)
	}
	if thread.Reply.Status != "unavailable" || len(agent.inputs) != 5 {
		t.Fatalf("forgotten implicit product accepted: status=%s calls=%d", thread.Reply.Status, len(agent.inputs))
	}
	if thread.PendingPurchase == nil || thread.PendingPurchase.Description != product || thread.PendingPurchase.Amount != amount || thread.PendingPurchase.RecordedAt != originalRecordedAt {
		t.Fatalf("forgotten display history was refreshed = %#v", thread.PendingPurchase)
	}
	clock = clock.Add(time.Second)
	if _, err = service.Send(context.Background(), thread.ID, "別の相談を続けます"); err != nil {
		t.Fatal(err)
	}
	for _, input := range agent.inputs[2:] {
		if input.PendingPurchase != nil {
			t.Fatal("forgotten purchase was reintroduced to external context")
		}
		encoded, _ := json.Marshal(input)
		if strings.Contains(string(encoded), product) {
			t.Fatal("forgotten product resurrected through another context field")
		}
	}
}
