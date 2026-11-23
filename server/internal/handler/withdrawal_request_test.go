package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

type withdrawalFake struct {
	records map[int64]bool
	fail    bool
}

func (s *withdrawalFake) WithdrawalRequested(ctx context.Context) (bool, error) {
	if s.fail {
		return false, errors.New("db")
	}
	return s.records[identity.UserID(ctx)], nil
}
func (s *withdrawalFake) RequestWithdrawal(ctx context.Context) error {
	if s.fail {
		return errors.New("db")
	}
	s.records[identity.UserID(ctx)] = true
	return nil
}
func TestWithdrawalRequestOwnerRetryAndFailure(t *testing.T) {
	store := &withdrawalFake{records: map[int64]bool{}}
	h := NewWithServices(nil, nil, store)
	call := func(method string, user int64) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/withdrawal-request", nil)
		if user > 0 {
			r = r.WithContext(identity.WithUser(r.Context(), user))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for i := 0; i < 2; i++ {
		if w := call(http.MethodPut, 1); w.Code != 200 || !strings.Contains(w.Body.String(), `"requested":true`) {
			t.Fatalf("retry %d %s", w.Code, w.Body)
		}
	}
	if w := call(http.MethodGet, 2); w.Code != 200 || !strings.Contains(w.Body.String(), `"requested":false`) {
		t.Fatalf("owner isolation %d %s", w.Code, w.Body)
	}
	store.fail = true
	if w := call(http.MethodPut, 2); w.Code != 503 || strings.Contains(w.Body.String(), `"requested":true`) {
		t.Fatalf("error %d %s", w.Code, w.Body)
	}
}
