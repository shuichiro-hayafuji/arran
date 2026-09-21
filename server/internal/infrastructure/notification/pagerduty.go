package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)

const pagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

type PagerDuty struct {
	routingKey string
	endpoint   string
	client     *http.Client
}

func NewPagerDuty(routingKey string) *PagerDuty {
	return &PagerDuty{
		routingKey: routingKey,
		endpoint:   pagerDutyEventsURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *PagerDuty) NotifyMonthlyLimit(ctx context.Context, event domain.MonthlyLimitEvent) error {
	if p.routingKey == "" {
		return fmt.Errorf("PagerDuty routing key is not configured")
	}
	body, err := json.Marshal(map[string]any{
		"routing_key":  p.routingKey,
		"event_action": "trigger",
		"dedup_key":    event.DedupKey(),
		"payload": map[string]any{
			"summary":   fmt.Sprintf("Arran monthly consultation limit reached: user %d", event.UserID),
			"source":    "arran-api",
			"severity":  "warning",
			"timestamp": event.OccurredAt.Format(time.RFC3339),
			"custom_details": map[string]any{
				"environment":        event.Environment,
				"internal_user_id":   event.UserID,
				"month":              event.Month,
				"consultation_count": event.Count,
				"consultation_limit": event.Limit,
			},
		},
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return fmt.Errorf("PagerDuty returned %s: %s", response.Status, string(message))
	}
	return nil
}
