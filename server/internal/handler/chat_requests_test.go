package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/sqlite"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type delayedRequestAgent struct {
	release chan struct{}
	calls   atomic.Int32
}

func (a *delayedRequestAgent) Advise(ctx context.Context, in consultation.Input) (consultation.Reply, error) {
	a.calls.Add(1)
	select {
	case <-a.release:
		return consultation.Reply{Status: "answered", Answer: "wire-independent result"}, nil
	case <-ctx.Done():
		return consultation.Reply{}, ctx.Err()
	}
}
func TestHTTPReceiptDisconnectAndOwnership(t *testing.T) {
	repo, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "http.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	model := &delayedRequestAgent{release: make(chan struct{})}
	app := application.New(application.Config{Repository: repo, ChatRepository: repo, RequestRepository: repo, ChatAgent: model})
	defer app.WaitRequests()
	api := New(app)
	request := func(ctx context.Context, method, path, payload string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(payload)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		api.ServeHTTP(out, r)
		return out
	}
	owner := identity.WithUser(context.Background(), 1)
	wire, cancel := context.WithCancel(owner)
	first := request(wire, http.MethodPost, "/consultation-requests/http-request-0001", `{"message":"11秒以上かかる相談"}`)
	if first.Code != 202 {
		t.Fatalf("accept status=%d %s", first.Code, first.Body.String())
	}
	cancel()
	var accepted application.ChatRequestResult
	decodeResponse(t, first, &accepted)
	if accepted.Status != "processing" {
		t.Fatal("POST waited for result")
	}
	other := request(identity.WithUser(context.Background(), 2), http.MethodGet, "/consultation-requests/http-request-0001", "")
	if other.Code != 404 {
		t.Fatal("HTTP owner leak")
	}
	changed := request(owner, http.MethodPost, "/consultation-requests/http-request-0001", `{"message":"different"}`)
	if changed.Code != 409 {
		t.Fatal("changed payload")
	}
	close(model.release)
	for start := time.Now(); time.Since(start) < time.Second; {
		resp := request(owner, http.MethodGet, "/consultation-requests/http-request-0001", "")
		var result application.ChatRequestResult
		if err = json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Status == "completed" {
			if result.Result == nil || model.calls.Load() != 1 {
				t.Fatal("result mismatch")
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("disconnected wire prevented result")
}

func TestHTTPReceiptDuringShutdownReturnsUnavailable(t *testing.T) {
	repo, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "stop.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	app := application.New(application.Config{Repository: repo, ChatRepository: repo, RequestRepository: repo})
	app.WaitRequests()
	api := New(app)
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
		request := httptest.NewRequest(method, "/consultation-requests/stopping-request-001", bytes.NewBufferString(`{"message":"相談"}`)).WithContext(identity.WithUser(context.Background(), 1))
		request.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		api.ServeHTTP(out, request)
		if out.Code != 503 {
			t.Fatalf("%s status=%d", method, out.Code)
		}
	}
}
