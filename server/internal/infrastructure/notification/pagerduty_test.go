package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)

func TestPagerDutySendsOnlyOperationalLimitDetails(t *testing.T) {
	var requestBody map[string]any
	notifier := NewPagerDuty("test-routing-key")
	notifier.endpoint = "https://pagerduty.example.test/enqueue"
	notifier.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader(`{"status":"success"}`)),
			Header:     make(http.Header),
		}, nil
	})}
	event := domain.MonthlyLimitEvent{
		Environment: "test", UserID: 42, Month: "2026-09", Count: 30, Limit: 30,
		OccurredAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
	}
	if err := notifier.NotifyMonthlyLimit(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"user@example.com", "相談内容", "current_balance", "monthly_income"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("notification contains forbidden field %q: %s", forbidden, text)
		}
	}
	for _, required := range []string{"internal_user_id", "consultation_count", "consultation_limit", "2026-09"} {
		if !strings.Contains(text, required) {
			t.Fatalf("notification missing %q: %s", required, text)
		}
	}
}

func TestPagerDutyRequiresRoutingKey(t *testing.T) {
	if err := NewPagerDuty("").NotifyMonthlyLimit(context.Background(), domain.MonthlyLimitEvent{}); err == nil {
		t.Fatal("expected missing routing key error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
