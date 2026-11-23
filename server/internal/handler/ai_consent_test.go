package handler

import (
	"context"
	"errors"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type consentFake struct {
	granted   string
	withdrawn bool
	err       error
}

type waitingConsent struct{ consentFake }

func (*waitingConsent) Withdraw(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }

func TestWithdrawalHasServerDeadline(t *testing.T) {
	h := &handler{consent: &waitingConsent{}, withdrawalTimeout: 10 * time.Millisecond}
	w := httptest.NewRecorder()
	h.withdrawAIConsent(w, httptest.NewRequest(http.MethodDelete, "/ai-consent", nil))
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "withdrawal_unconfirmed") {
		t.Fatalf("timeout: %d %s", w.Code, w.Body.String())
	}
}

func (f *consentFake) Status(context.Context) (any, error) {
	return map[string]any{"version": "v1", "description": "test policy", "granted": f.granted != "", "withdrawal_pending": false}, f.err
}
func (f *consentFake) Grant(_ context.Context, version string) error {
	if version != "v1" {
		return consultation.ErrConsentRequired
	}
	f.granted = version
	return f.err
}
func (f *consentFake) Withdraw(context.Context) error {
	if f.err != nil {
		return f.err
	}
	f.withdrawn = true
	f.granted = ""
	return nil
}

func TestConsentEndpointsUseDisplayedVersionAndReportPending(t *testing.T) {
	f := &consentFake{}
	h := NewWithConsent(nil, f)
	request := func(method, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/ai-consent", strings.NewReader(body)))
		return w
	}
	if w := request(http.MethodGet, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "test policy") {
		t.Fatalf("get %d %s", w.Code, w.Body.String())
	}
	if w := request(http.MethodPut, `{"version":"old"}`); w.Code != 409 || f.granted != "" {
		t.Fatalf("old version: %d", w.Code)
	}
	if w := request(http.MethodPut, `{"version":"v1"}`); w.Code != 200 || f.granted != "v1" {
		t.Fatalf("grant: %d", w.Code)
	}
	f.err = errors.New("waiting")
	if w := request(http.MethodDelete, ""); w.Code != 503 || f.withdrawn {
		t.Fatalf("pending: %d", w.Code)
	}
	f.err = nil
	if w := request(http.MethodDelete, ""); w.Code != 200 || !f.withdrawn {
		t.Fatalf("withdraw: %d", w.Code)
	}
}
