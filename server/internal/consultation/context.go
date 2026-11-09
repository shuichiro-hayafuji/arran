package consultation

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Selection is a bounded lexical retrieval, not a financial decision. Dates and
// provenance remain intact so the model can judge relevance, staleness and risk.
func relevantFacts(facts []Fact, message string, history []Message) []Fact {
	query := message
	for _, m := range history {
		if m.Role == "user" {
			query += " " + m.Content
		}
	}
	financial := strings.ContainsAny(query, "円買支払予算残高貯蓄収入家計")
	aliases := map[string]string{
		"current_balance": "残高", "monthly_free_budget": "予算", "monthly_income": "月収収入",
		"monthly_fixed_costs": "固定費", "savings_goal": "貯蓄目標", "goal": "目標",
		"preference": "好み嗜好", "reduce_categories": "減らす", "allowed_categories": "許容",
	}
	type candidate struct {
		fact  Fact
		score int
		index int
	}
	var candidates []candidate
	queryTokens := tokens(query)
	for i, f := range facts {
		parts := strings.FieldsFunc(f.Key, func(r rune) bool { return r == ':' || r == '/' })
		if len(parts) == 0 {
			continue
		}
		canonical := parts[0]
		if !f.Active {
			continue
		}
		score := 0
		for token := range tokens(f.Key + " " + f.Value + " " + aliases[canonical]) {
			if queryTokens[token] {
				score++
			}
		}
		if financial && aliases[canonical] != "" && canonical != "preference" {
			score += 2
		}
		if score > 0 {
			candidates = append(candidates, candidate{f, score, i})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].index > candidates[j].index
	})
	var selected []candidate
	bytes := 2
	for _, c := range candidates {
		c.fact.Key, c.fact.Value, c.fact.Period, c.fact.Source = Redact(c.fact.Key), Redact(c.fact.Value), redactPeriod(c.fact.Period), Redact(c.fact.Source)
		encoded, _ := json.Marshal(c.fact)
		size := len(encoded) + 1
		if len(selected) == 12 {
			break
		}
		if bytes+size > 6000 {
			continue
		}
		bytes += size
		selected = append(selected, c)
	}
	// Preserve chronology after ranking, making same-period comparisons readable.
	sort.Slice(selected, func(i, j int) bool { return selected[i].index < selected[j].index })
	var result []Fact
	for _, c := range selected {
		f := c.fact
		result = append(result, f)
	}
	return result
}

func tokens(text string) map[string]bool {
	runes := []rune(strings.ToLower(text))
	result := map[string]bool{}
	for i := 0; i+1 < len(runes); i++ {
		if unicode.IsLetter(runes[i]) && unicode.IsLetter(runes[i+1]) {
			token := string(runes[i : i+2])
			switch token {
			case "です", "ます", "好き", "相談", "した", "たい", "こと":
				continue
			}
			result[token] = true
		}
	}
	return result
}

func recentHistory(messages []Message, facts []Fact) []Message {
	forgottenAt := latestForgetAt(facts)
	start := len(messages) - 8
	if start < 0 {
		start = 0
	}
	var result []Message
	for _, m := range messages[start:] {
		// Do not reintroduce forgotten material via prior answers or their reasons.
		if forgottenAt != "" && m.CreatedAt <= forgottenAt {
			continue
		}
		m.Content, m.Reason, m.Assumptions = Redact(m.Content), Redact(m.Reason), Redact(m.Assumptions)
		result = append(result, m)
	}
	return result
}

func redactPeriod(period string) string {
	for _, layout := range []string{"2006-01-02", "2006-01", time.RFC3339} {
		if _, err := time.Parse(layout, period); err == nil {
			return period
		}
	}
	return Redact(period)
}

func latestForgetAt(facts []Fact) string {
	latest := ""
	for _, f := range facts {
		if !f.Active && f.Value == "" && f.RecordedAt > latest {
			latest = f.RecordedAt
		}
	}
	return latest
}

// Search distinct previous consultations without exposing their whole transcripts.
func relatedHistory(threads []Thread, query string, facts []Fact) []HistoricalMessage {
	type candidate struct {
		message HistoricalMessage
		score   int
	}
	var candidates []candidate
	queryTokens := tokens(query)
	for _, thread := range threads {
		messages := recentHistory(thread.Messages, facts)
		userContent := ""
		for _, message := range messages {
			if message.Role == "user" {
				userContent = message.Content
			}
			score := 0
			for token := range tokens(userContent + " " + message.Content + " " + message.Reason + " " + message.Assumptions) {
				if queryTokens[token] {
					score++
				}
			}
			if score > 0 {
				candidates = append(candidates, candidate{HistoricalMessage{ConsultationID: thread.ID, Message: message}, score})
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	var result []HistoricalMessage
	bytes := 2
	for _, candidate := range candidates {
		encoded, _ := json.Marshal(candidate.message)
		if bytes+len(encoded)+1 > 6000 {
			continue
		}
		result = append(result, candidate.message)
		bytes += len(encoded) + 1
		if len(result) == 6 {
			break
		}
	}
	return result
}
