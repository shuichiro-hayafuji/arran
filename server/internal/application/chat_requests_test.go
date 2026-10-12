package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/sqlite"
)

type requestModel struct {
	calls  atomic.Int32
	advise func(context.Context, consultation.Input) (consultation.Reply, error)
}

func (m *requestModel) Advise(ctx context.Context, in consultation.Input) (consultation.Reply, error) {
	m.calls.Add(1)
	return m.advise(ctx, in)
}
func requestRepo(t *testing.T) *sqlite.Repository {
	t.Helper()
	r, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "requests.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}
func waitReceipt(t *testing.T, app *application.Application, ctx context.Context, id string) application.ChatRequestResult {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		r, err := app.GetChatRequest(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "processing" {
			return r
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("receipt did not become terminal")
	return application.ChatRequestResult{}
}
func requestConfig(repo *sqlite.Repository, model consultation.Agent) application.Config {
	return application.Config{Repository: repo, ChatRepository: repo, RequestRepository: repo, ChatAgent: model, RequestBudget: 2 * time.Second, RequestSaveReserve: 300 * time.Millisecond}
}

func TestLogicalRequestDisconnectDuplicatePayloadAndExactSnapshot(t *testing.T) {
	repo := requestRepo(t)
	owner := identity.WithUser(context.Background(), 1)
	release := make(chan struct{})
	model := &requestModel{advise: func(ctx context.Context, in consultation.Input) (consultation.Reply, error) {
		encoded, _ := json.Marshal(in)
		if strings.Contains(string(encoded), "logical-request-001") {
			t.Error("logical request ID entered model input")
		}
		<-release
		return consultation.Reply{Status: "answered", Answer: "保存した回答"}, nil
	}}
	app := application.New(requestConfig(repo, model))
	defer app.WaitRequests()
	wire, cancel := context.WithCancel(owner)
	input := application.ChatRequestInput{Message: "元の相談"}
	first, err := app.SubmitChatRequest(wire, "logical-request-001", input)
	if err != nil || first.Status != "processing" {
		t.Fatalf("submit=%+v %v", first, err)
	}
	cancel()
	duplicate, err := app.SubmitChatRequest(owner, "logical-request-001", input)
	if err != nil || duplicate.RequestID != first.RequestID {
		t.Fatal("same-payload retry not linked")
	}
	if _, err = app.SubmitChatRequest(owner, "logical-request-001", application.ChatRequestInput{Message: "異なる相談"}); !errors.Is(err, consultation.ErrRequestPayload) {
		t.Fatalf("changed payload=%v", err)
	}
	if _, err = app.GetChatRequest(identity.WithUser(context.Background(), 2), first.RequestID); !errors.Is(err, consultation.ErrNotFound) {
		t.Fatalf("owner leak=%v", err)
	}
	close(release)
	result := waitReceipt(t, app, owner, first.RequestID)
	if result.Status != "completed" || result.Result == nil || len(result.Result.Messages) != 2 || model.calls.Load() != 1 {
		t.Fatalf("saved=%+v calls=%d", result, model.calls.Load())
	}
	usage, _ := repo.MonthlyConsultationUsage(owner, time.Now().In(time.FixedZone("JST", 9*3600)).Format("2006-01"))
	if usage.Count != 1 {
		t.Fatal("duplicate quota reservation")
	}
	if _, err = app.SendChat(owner, result.Result.ID, "あとからの別の発言", nil); err != nil {
		t.Fatal(err)
	}
	original, err := app.GetChatRequest(owner, first.RequestID)
	if err != nil || len(original.Result.Messages) != 2 || original.Result.UserMessage != "元の相談" {
		t.Fatal("receipt returned latest conversation rather than exact request")
	}
	afterCancel, err := app.CancelChatRequest(owner, first.RequestID)
	if err != nil || afterCancel.Status != "completed" || afterCancel.CancelRequested {
		t.Fatal("post-commit cancellation changed a completed outcome")
	}
}

func TestRequestCancellationFencesFinancialCommitAndUnknownID(t *testing.T) {
	repo := requestRepo(t)
	ctx := identity.WithUser(context.Background(), 1)
	started := make(chan struct{})
	release := make(chan struct{})
	model := &requestModel{advise: func(_ context.Context, in consultation.Input) (consultation.Reply, error) {
		if in.Message == "初回" {
			return consultation.Reply{Status: "answered", Answer: "初回", Changes: []consultation.Change{{Key: "current_balance", Value: "50000円", Operation: "set", Kind: "fact", Basis: "user_report"}}}, nil
		}
		close(started)
		<-release
		return consultation.Reply{Status: "answered", Answer: "変更", Changes: []consultation.Change{{Key: "current_balance", Value: "47500円", Operation: "set", Kind: "fact", Basis: "user_report"}}}, nil
	}}
	app := application.New(requestConfig(repo, model))
	defer app.WaitRequests()
	_, err := app.SubmitChatRequest(ctx, "cancel-request-first", application.ChatRequestInput{Message: "初回"})
	if err != nil {
		t.Fatal(err)
	}
	first := waitReceipt(t, app, ctx, "cancel-request-first")
	_, err = app.SubmitChatRequest(ctx, "cancel-request-second", application.ChatRequestInput{ConsultationID: first.Result.ID, Message: "変更"})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err = app.CancelChatRequest(ctx, "cancel-request-second"); err != nil {
		t.Fatal(err)
	}
	close(release)
	cancelled := waitReceipt(t, app, ctx, "cancel-request-second")
	if cancelled.Status != "cancelled" || cancelled.Result == nil || cancelled.Result.Status != "unavailable" {
		t.Fatalf("cancel=%+v", cancelled)
	}
	state, _ := repo.LoadChatState(ctx)
	for _, f := range state.Facts {
		if f.Active && f.Key == "current_balance" && f.Value != "50000円" {
			t.Fatal("cancelled request committed financial change")
		}
	}
	tombstone, err := app.CancelChatRequest(ctx, "never-accepted-request")
	if err != nil || tombstone.Status != "cancelled" || tombstone.Result != nil {
		t.Fatal("unknown cancel did not retire request ID")
	}
	if _, err = app.SubmitChatRequest(ctx, "never-accepted-request", application.ChatRequestInput{Message: "変更"}); !errors.Is(err, consultation.ErrRequestPayload) {
		t.Fatalf("cancelled unaccepted request was submitted: %v", err)
	}
	if model.calls.Load() != 2 {
		t.Fatal("cancelled tombstone executed model")
	}
}

func TestTwoRepairsAndIgnoringModelStillRespectAbsoluteBudget(t *testing.T) {
	t.Run("two repairs", func(t *testing.T) {
		repo := requestRepo(t)
		ctx := identity.WithUser(context.Background(), 1)
		model := &requestModel{}
		model.advise = func(_ context.Context, in consultation.Input) (consultation.Reply, error) {
			if model.calls.Load() < 3 {
				return consultation.Reply{Status: "invalid", Answer: "invalid"}, nil
			}
			if in.ValidationFeedback == "" {
				t.Error("repair feedback missing")
			}
			return consultation.Reply{Status: "answered", Answer: "修正後の回答"}, nil
		}
		app := application.New(requestConfig(repo, model))
		defer app.WaitRequests()
		_, err := app.SubmitChatRequest(ctx, "repair-request-001", application.ChatRequestInput{Message: "修正"})
		if err != nil {
			t.Fatal(err)
		}
		result := waitReceipt(t, app, ctx, "repair-request-001")
		if model.calls.Load() != 3 || result.Result == nil || result.Result.Status != "answered" {
			t.Fatal("bounded repairs did not save result")
		}
	})
	t.Run("ignoring model", func(t *testing.T) {
		repo := requestRepo(t)
		ctx := identity.WithUser(context.Background(), 1)
		finished := make(chan struct{})
		model := &requestModel{advise: func(_ context.Context, _ consultation.Input) (consultation.Reply, error) {
			time.Sleep(230 * time.Millisecond)
			close(finished)
			return consultation.Reply{Status: "answered", Answer: "too late", Changes: []consultation.Change{{Key: "current_balance", Value: "1円", Operation: "set", Kind: "fact", Basis: "user_report"}}}, nil
		}}
		cfg := requestConfig(repo, model)
		cfg.RequestBudget = 160 * time.Millisecond
		cfg.RequestSaveReserve = 70 * time.Millisecond
		app := application.New(cfg)
		defer app.WaitRequests()
		_, err := app.SubmitChatRequest(ctx, "deadline-request-001", application.ChatRequestInput{Message: "遅延"})
		if err != nil {
			t.Fatal(err)
		}
		result := waitReceipt(t, app, ctx, "deadline-request-001")
		if result.Result == nil || result.Result.Status != "unavailable" {
			t.Fatalf("deadline=%+v", result)
		}
		<-finished
		time.Sleep(10 * time.Millisecond)
		state, _ := repo.LoadChatState(ctx)
		if len(state.Facts) != 0 {
			t.Fatal("ignored-context model saved financial changes after deadline")
		}
		saved, _ := repo.LoadChatThread(ctx, result.Result.ID)
		if len(saved.Messages) != 2 {
			t.Fatal("original and fallback both saved")
		}
	})
}

type unreadableReceipt struct{ *sqlite.Repository }

func (r unreadableReceipt) GetChatRequest(ctx context.Context, id string) (consultation.RequestReceipt, error) {
	value, err := r.Repository.GetChatRequest(ctx, id)
	if value.Result != nil {
		return value, errors.New("temporary receipt read failure")
	}
	return value, err
}
func TestCommittedReceiptReadFailureDoesNotReleaseQuota(t *testing.T) {
	repo := requestRepo(t)
	ctx := identity.WithUser(context.Background(), 1)
	model := &requestModel{advise: func(context.Context, consultation.Input) (consultation.Reply, error) {
		return consultation.Reply{Status: "answered", Answer: "回答"}, nil
	}}
	cfg := requestConfig(repo, model)
	cfg.RequestRepository = unreadableReceipt{repo}
	app := application.New(cfg)
	_, err := app.SubmitChatRequest(ctx, "receipt-read-failure", application.ChatRequestInput{Message: "相談"})
	if err != nil {
		t.Fatal(err)
	}
	app.WaitRequests()
	receipt, err := repo.GetChatRequest(ctx, "receipt-read-failure")
	if err != nil || receipt.Result == nil || receipt.Status != "completed" {
		t.Fatal("completed receipt damaged by result read failure")
	}
	usage, _ := repo.MonthlyConsultationUsage(ctx, receipt.Month)
	if usage.Count != 1 {
		t.Fatal("committed save released monthly quota")
	}
}

func TestExpiredReceiptDoesNotRestartAndNewIDCanRetry(t *testing.T) {
	repo := requestRepo(t)
	ctx := identity.WithUser(context.Background(), 1)
	month := time.Now().In(time.FixedZone("JST", 9*3600)).Format("2006-01")
	_, claimed, err := repo.ClaimChatRequest(ctx, consultation.RequestReceipt{ID: "crashed-request-001", PayloadHash: "original", Month: month, NewConsultation: true, ExpiresAt: time.Now().Add(-time.Second)})
	if err != nil || !claimed {
		t.Fatal(err)
	}
	r, err := repo.GetChatRequest(ctx, "crashed-request-001")
	if err != nil || r.Status != "failed" || r.ErrorCode != "deadline_exceeded" {
		t.Fatalf("expiry: %+v %v", r, err)
	}
	same, claimed, err := repo.ClaimChatRequest(ctx, consultation.RequestReceipt{ID: r.ID, PayloadHash: r.PayloadHash, Month: month, NewConsultation: true, ExpiresAt: time.Now().Add(time.Second)})
	if err != nil || claimed || same.Status != "failed" {
		t.Fatal("expired request restarted")
	}
	usage, _ := repo.MonthlyConsultationUsage(ctx, month)
	if usage.Count != 0 {
		t.Fatal("failed reservation not released")
	}
	model := &requestModel{advise: func(context.Context, consultation.Input) (consultation.Reply, error) {
		return consultation.Reply{Status: "answered", Answer: "再送結果"}, nil
	}}
	app := application.New(requestConfig(repo, model))
	defer app.WaitRequests()
	_, err = app.SubmitChatRequest(ctx, "explicit-new-request", application.ChatRequestInput{Message: "利用者の明示再送"})
	if err != nil {
		t.Fatal(err)
	}
	result := waitReceipt(t, app, ctx, "explicit-new-request")
	if result.Result == nil || model.calls.Load() != 1 {
		t.Fatal("explicit retry failed")
	}
}

type slowPreparation struct {
	*sqlite.Repository
	calls atomic.Int32
}

func (r *slowPreparation) MonthlyConsultationUsage(ctx context.Context, month string) (domain.ConsultationUsage, error) {
	if r.calls.Add(1) == 1 {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return domain.ConsultationUsage{}, ctx.Err()
		}
	}
	return r.Repository.MonthlyConsultationUsage(ctx, month)
}
func TestPreparationConsumesAbsoluteBudget(t *testing.T) {
	repo := requestRepo(t)
	ctx := identity.WithUser(context.Background(), 1)
	observed := make(chan time.Duration, 1)
	model := &requestModel{advise: func(ctx context.Context, _ consultation.Input) (consultation.Reply, error) {
		deadline, _ := ctx.Deadline()
		observed <- time.Until(deadline)
		<-ctx.Done()
		return consultation.Reply{}, ctx.Err()
	}}
	cfg := requestConfig(repo, model)
	cfg.Repository = &slowPreparation{Repository: repo}
	cfg.RequestBudget = 300 * time.Millisecond
	cfg.RequestSaveReserve = 100 * time.Millisecond
	app := application.New(cfg)
	defer app.WaitRequests()
	started := time.Now()
	_, err := app.SubmitChatRequest(ctx, "slow-prepare-request", application.ChatRequestInput{Message: "準備が遅い"})
	if err != nil {
		t.Fatal(err)
	}
	result := waitReceipt(t, app, ctx, "slow-prepare-request")
	remaining := <-observed
	if remaining > 130*time.Millisecond || time.Since(started) > 350*time.Millisecond || result.Result == nil || result.Result.Status != "unavailable" {
		t.Fatalf("remaining=%v elapsed=%v result=%+v", remaining, time.Since(started), result)
	}
}

type cancelAfterAcceptance struct {
	*sqlite.Repository
	cancel context.CancelFunc
}

func (r cancelAfterAcceptance) ClaimChatRequest(ctx context.Context, in consultation.RequestReceipt) (consultation.RequestReceipt, bool, error) {
	receipt, claimed, err := r.Repository.ClaimChatRequest(ctx, in)
	if claimed {
		r.cancel()
	}
	return receipt, claimed, err
}
func TestAcceptanceCommitSurvivesImmediateWireCancellation(t *testing.T) {
	repo := requestRepo(t)
	owner := identity.WithUser(context.Background(), 1)
	wire, cancel := context.WithCancel(owner)
	defer cancel()
	model := &requestModel{advise: func(context.Context, consultation.Input) (consultation.Reply, error) {
		return consultation.Reply{Status: "answered", Answer: "保存済"}, nil
	}}
	cfg := requestConfig(repo, model)
	cfg.RequestRepository = cancelAfterAcceptance{repo, cancel}
	app := application.New(cfg)
	defer app.WaitRequests()
	first, err := app.SubmitChatRequest(wire, "commit-wire-cancel", application.ChatRequestInput{Message: "相談"})
	if err != nil || first.Status != "processing" || wire.Err() == nil {
		t.Fatalf("commit return %+v %v", first, err)
	}
	retry, err := app.SubmitChatRequest(owner, "commit-wire-cancel", application.ChatRequestInput{Message: "相談"})
	if err != nil {
		t.Fatal(err)
	}
	result := waitReceipt(t, app, owner, retry.RequestID)
	if result.Result == nil || model.calls.Load() != 1 {
		t.Fatal("accepted request did not execute exactly once")
	}
}
func TestWaitRequestsOwnsLateModelAndRepositoryAccess(t *testing.T) {
	repo := requestRepo(t)
	ctx := identity.WithUser(context.Background(), 1)
	release := make(chan struct{})
	exited := make(chan struct{})
	model := &requestModel{advise: func(context.Context, consultation.Input) (consultation.Reply, error) {
		<-release
		close(exited)
		return consultation.Reply{Status: "answered", Answer: "late"}, nil
	}}
	cfg := requestConfig(repo, model)
	cfg.RequestBudget = 200 * time.Millisecond
	cfg.RequestSaveReserve = 100 * time.Millisecond
	app := application.New(cfg)
	_, err := app.SubmitChatRequest(ctx, "lifetime-request-001", application.ChatRequestInput{Message: "相談"})
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	waitReceipt(t, app, ctx, "lifetime-request-001")
	waited := make(chan struct{})
	go func() { app.WaitRequests(); close(waited) }()
	returned := false
	select {
	case <-waited:
		returned = true
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	<-exited
	<-waited
	if returned {
		t.Fatal("repository could close while model/send still running")
	}
}

type blockedClaim struct {
	*sqlite.Repository
	entered, release chan struct{}
}

func (r blockedClaim) ClaimChatRequest(ctx context.Context, in consultation.RequestReceipt) (consultation.RequestReceipt, bool, error) {
	close(r.entered)
	<-r.release
	return r.Repository.ClaimChatRequest(ctx, in)
}
func TestShutdownWaitIncludesAdmissionAndRejectsNewCalls(t *testing.T) {
	repo := requestRepo(t)
	ctx := identity.WithUser(context.Background(), 1)
	life, stop := context.WithCancel(context.Background())
	defer stop()
	claim := blockedClaim{repo, make(chan struct{}), make(chan struct{})}
	cfg := requestConfig(repo, nil)
	cfg.RequestContext = life
	cfg.RequestRepository = claim
	app := application.New(cfg)
	submitted := make(chan error, 1)
	go func() {
		_, err := app.SubmitChatRequest(ctx, "shutdown-admission-001", application.ChatRequestInput{Message: "相談"})
		submitted <- err
	}()
	<-claim.entered
	stop()
	waited := make(chan struct{})
	go func() { app.WaitRequests(); close(waited) }()
	early := false
	select {
	case <-waited:
		early = true
	case <-time.After(20 * time.Millisecond):
	}
	if _, err := app.SubmitChatRequest(ctx, "shutdown-new-id-001", application.ChatRequestInput{Message: "新規"}); !errors.Is(err, application.ErrRequestsStopping) {
		t.Errorf("new admission: %v", err)
	}
	if _, err := app.GetChatRequest(ctx, "shutdown-admission-001"); !errors.Is(err, application.ErrRequestsStopping) {
		t.Errorf("new read: %v", err)
	}
	if _, err := app.CancelChatRequest(ctx, "shutdown-admission-001"); !errors.Is(err, application.ErrRequestsStopping) {
		t.Errorf("new cancel: %v", err)
	}
	close(claim.release)
	if err := <-submitted; !errors.Is(err, application.ErrRequestsStopping) {
		t.Errorf("blocked claim: %v", err)
	}
	<-waited
	if early {
		t.Fatal("shutdown returned before admission completed")
	}
}
