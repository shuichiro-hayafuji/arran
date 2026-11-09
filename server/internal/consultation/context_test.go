package consultation

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestRelatedFactsExcludeOtherTopicsAndBoundContext(t *testing.T) {
	facts := []Fact{
		{Key: "preference:camera", Value: "写真撮影が好き", Active: true},
		{Key: "hobby:garden", Value: "園芸が好き", Active: true},
		{Key: "monthly_free_budget:2025-09", Value: "30000円", Period: "2025-09", Active: true},
		{Key: "current_balance", Value: "古い訂正前", Active: false},
	}
	selected := relevantFacts(facts, "写真撮影用のカメラを買いたい", nil)
	if len(selected) != 2 || selected[0].Key != "preference:camera" || selected[1].Period != "2025-09" {
		t.Fatalf("related facts = %#v", selected)
	}
	for i := 0; i < 100; i++ {
		facts = append(facts, Fact{Key: fmt.Sprintf("preference:camera:%d", i), Value: strings.Repeat("写真", 150), Active: true})
	}
	selected = relevantFacts(facts, "写真撮影用のカメラを買いたい", nil)
	size := 0
	for _, f := range selected {
		size += len(f.Key) + len(f.Value) + len(f.Period) + len(f.Source)
	}
	encoded, _ := json.Marshal(selected)
	if len(selected) > 12 || size > 6000 || len(encoded) > 6000 {
		t.Fatalf("unbounded context: count=%d size=%d", len(selected), size)
	}
}

func TestForgetCutoffRetainsOnlyNewConversationsAndRedactsDetails(t *testing.T) {
	facts := []Fact{{Key: "goal", Value: "", RecordedAt: "2026-10-12T00:00:00Z", Active: false}}
	messages := []Message{
		{Role: "assistant", Content: "forgotten goal", Reason: "old reason", CreatedAt: "2026-10-11T00:00:00Z"},
		{Role: "assistant", Content: "new topic", Reason: "パスワード:secret", Assumptions: "口座番号:1234567", CreatedAt: "2026-10-12T00:00:01Z"},
	}
	selected := recentHistory(messages, facts)
	if len(selected) != 1 || selected[0].Content != "new topic" || strings.Contains(selected[0].Reason, "secret") || strings.Contains(selected[0].Assumptions, "1234567") {
		t.Fatalf("forget cutoff = %#v", selected)
	}
	selectedFacts := relevantFacts([]Fact{{Key: "goal:1234567", Value: "写真の目標", Period: "認証コード:99999999", Source: "password:secret", Active: true}}, "写真", nil)
	if len(selectedFacts) != 1 || strings.Contains(selectedFacts[0].Key, "1234567") || strings.Contains(selectedFacts[0].Period, "99999999") || strings.Contains(selectedFacts[0].Source, "secret") {
		t.Fatalf("metadata redaction = %#v", selectedFacts)
	}
}

func TestRelatedPastAdviceBoundedByCountAndEncodedBytes(t *testing.T) {
	var threads []Thread
	for i := 0; i < 20; i++ {
		threads = append(threads, Thread{ID: int64(i + 1), Messages: []Message{{Role: "user", Content: "写真撮影のカメラ", CreatedAt: "2026-10-12T00:00:00Z"}, {Role: "assistant", Content: strings.Repeat("写真", 100), Reason: "写真の目標", CreatedAt: "2026-10-12T00:00:00Z"}}})
	}
	history := relatedHistory(threads, "写真撮影", nil)
	encoded, _ := json.Marshal(history)
	if len(history) == 0 || len(history) > 6 || len(encoded) > 6000 {
		t.Fatalf("unbounded past advice: %d/%d", len(history), len(encoded))
	}
}
