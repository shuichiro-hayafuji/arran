package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/shuichiro-hayafuji/arran_agent"
	"github.com/shuichirohayafuji/spendable-today/server/internal/agentadapter"
	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/auth"
	"github.com/shuichirohayafuji/spendable-today/server/internal/config"
	"github.com/shuichirohayafuji/spendable-today/server/internal/consultation"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"github.com/shuichirohayafuji/spendable-today/server/internal/handler"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/notification"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/openai"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/postgres"
)

// App owns the resources created during application composition.
type App struct {
	Server       *http.Server
	repository   *postgres.Repository
	stopRequests context.CancelFunc
	application  *application.Application
	stopWorker   context.CancelFunc
	workerDone   chan struct{}
}

// Enabling intake is deliberately blocked until all authenticated work is
// drained by the batch and restored account status is reconciled safely.
const withdrawalRuntimeReady = false

// Migrate runs the release migration with only the one-off process credentials.
func Migrate(ctx context.Context, cfg config.Config) error {
	return postgres.MigrateURL(ctx, cfg.DatabaseURL)
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	repo, err := postgres.Open(ctx, cfg.DatabaseURL, cfg.AutoMigrate())
	if err != nil {
		return nil, err
	}

	consentGate := identity.AIConsentGate{Store: repo, Version: cfg.AIConsentVersion}
	primary, source := modelClient(cfg, guardedUsageRecorder(repo), consentGate)
	fallback := agent.MockClient{}
	agents := agentadapter.NewWithFailureCheck(primary, fallback, source, func(err error) bool {
		return errors.Is(err, consultation.ErrConsentRequired) || errors.Is(err, consultation.ErrConsentUnavailable) || errors.Is(err, consultation.ErrAIOutcomeUnknown)
	})
	var chatAgent consultation.Agent
	if !cfg.UseMockLLM {
		client := openai.NewOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIReasoningEffort)
		client.SetUsageRecorder(guardedUsageRecorder(repo))
		client.SetConsentGate(consentExecutor{consentGate})
		chatAgent = client
	}
	fallbackAgent := agentadapter.New(fallback, fallback, "quota_fallback")
	adminNotifier := notification.NewPagerDuty(cfg.PagerDutyRoutingKey)
	requestCtx, stopRequests := context.WithCancel(ctx)
	apiApplication := application.New(application.Config{ImportManagement: repo, ChatDataChanges: repo, ChatReconciliation: repo, ReconciliationRepository: repo, RequestRepository: repo, RequestContext: requestCtx, Repository: repo, ChatRepository: repo, ChatAgent: chatAgent,
		ConsultationAgent: agents, FallbackAgent: fallbackAgent,
		ReviewAgent: agents, FallbackReview: fallbackAgent,
		MemoryAgent: agents, FallbackMemory: fallbackAgent, AdminNotifier: adminNotifier,
		Environment: cfg.Environment})

	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	worker := consultation.ReminderWorker{Repository: repo, Deliver: func(ctx context.Context, id int64) error {
		deliveryCtx, cancel := context.WithTimeout(identity.WithUser(ctx, id), 10*time.Second)
		defer cancel()
		return (consultation.Service{Repository: repo}).DeliverDue(deliveryCtx)
	}, OnError: func(error) { log.Print("purchase confirmation delivery failed; will retry") }}
	go func() { defer close(workerDone); worker.Run(workerCtx) }()
	return &App{
		Server: &http.Server{
			Addr:              cfg.APIAddr,
			Handler:           auth.New(repo).Wrap(handler.NewWithServices(apiApplication, consentService{consentGate}, withdrawalRequestService{repository: repo, enabled: cfg.WithdrawalBatchEnabled && withdrawalRuntimeReady})),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		repository: repo, stopRequests: stopRequests, application: apiApplication, stopWorker: stopWorker, workerDone: workerDone,
	}, nil
}

func usageRecorder(repository interface {
	RecordLLMUsage(context.Context, domain.LLMUsage) error
}) openai.UsageRecorder {
	return func(ctx context.Context, usage openai.UsageRecord) error {
		return repository.RecordLLMUsage(ctx, domain.LLMUsage{
			UserID: identity.UserID(ctx), Operation: usage.Operation, Model: usage.Model,
			InputTokens: usage.InputTokens, CachedInputTokens: usage.CachedInputTokens,
			OutputTokens: usage.OutputTokens, ReasoningTokens: usage.ReasoningTokens,
			TotalTokens: usage.TotalTokens, OccurredAt: usage.OccurredAt,
		})
	}
}

func guardedUsageRecorder(repository interface {
	RecordGuardedLLMUsage(context.Context, domain.LLMUsage) error
}) openai.UsageRecorder {
	return func(ctx context.Context, usage openai.UsageRecord) error {
		return repository.RecordGuardedLLMUsage(ctx, domain.LLMUsage{
			UserID: identity.UserID(ctx), Operation: usage.Operation, Model: usage.Model,
			InputTokens: usage.InputTokens, CachedInputTokens: usage.CachedInputTokens,
			OutputTokens: usage.OutputTokens, ReasoningTokens: usage.ReasoningTokens,
			TotalTokens: usage.TotalTokens, OccurredAt: usage.OccurredAt,
		})
	}
}

func (a *App) Close() error {
	a.stopRequests()
	a.application.WaitRequests()
	a.stopWorker()
	<-a.workerDone
	return a.repository.Close()
}

func modelClient(cfg config.Config, usageRecorder openai.UsageRecorder, gate identity.AIConsentGate) (agent.Model, string) {
	if cfg.UseMockLLM {
		return agent.MockClient{}, "mock"
	}
	client := openai.NewOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIReasoningEffort)
	client.SetUsageRecorder(usageRecorder)
	client.SetConsentGate(consentExecutor{gate})
	return client, "openai"
}

type consentExecutor struct{ gate identity.AIConsentGate }

func (c consentExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	called := false
	var callErr error
	err := c.gate.Execute(identity.RequireUsageTracking(ctx), func(inner context.Context) error { called = true; callErr = fn(inner); return callErr })
	if err != nil && called && callErr == nil {
		return fmt.Errorf("%w: completion record unavailable", consultation.ErrAIOutcomeUnknown)
	}
	if err != nil && !called {
		if errors.Is(err, identity.ErrConsentRequired) || errors.Is(err, identity.ErrInvalidUser) {
			return consultation.ErrConsentRequired
		}
		return fmt.Errorf("%w: authorization unavailable", consultation.ErrConsentUnavailable)
	}
	return err
}

type consentService struct{ gate identity.AIConsentGate }

func (c consentService) Status(ctx context.Context) (any, error) { return c.gate.Status(ctx) }
func (c consentService) Grant(ctx context.Context, version string) error {
	err := c.gate.Grant(ctx, version)
	if errors.Is(err, identity.ErrWithdrawalPending) {
		return consultation.ErrWithdrawalPending
	}
	if errors.Is(err, identity.ErrConsentRequired) {
		return consultation.ErrConsentRequired
	}
	return err
}
func (c consentService) Withdraw(ctx context.Context) error { return c.gate.Withdraw(ctx) }

// withdrawalRequestService binds all operations to the authenticated session.
type withdrawalRequestService struct {
	repository interface {
		WithdrawalRequested(context.Context, int64) (bool, error)
		RequestWithdrawal(context.Context, int64) error
	}
	enabled bool
}

func (s withdrawalRequestService) WithdrawalAcceptanceEnabled() bool { return s.enabled }

func (s withdrawalRequestService) WithdrawalRequested(ctx context.Context) (bool, error) {
	userID := identity.UserID(ctx)
	if userID <= 0 {
		return false, errors.New("authenticated user required")
	}
	return s.repository.WithdrawalRequested(ctx, userID)
}
func (s withdrawalRequestService) RequestWithdrawal(ctx context.Context) error {
	userID := identity.UserID(ctx)
	if userID <= 0 {
		return errors.New("authenticated user required")
	}
	if !s.enabled {
		return errors.New("withdrawal batch unavailable")
	}
	err := s.repository.RequestWithdrawal(ctx, userID)
	if errors.Is(err, identity.ErrWithdrawalBlocked) {
		return consultation.ErrRequestBusy
	}
	return err
}

// RunWithdrawalBatch executes a one-off scheduled run using the runtime DB role.
func RunWithdrawalBatch(ctx context.Context, cfg config.Config, now time.Time) error {
	if !cfg.WithdrawalBatchEnabled || !withdrawalRuntimeReady {
		return errors.New("withdrawal batch is disabled")
	}
	// docker compose exec exiting does not guarantee that the container-side
	// process is killed. Bound DB work independently of the systemd timeout.
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	repo, err := postgres.Open(ctx, cfg.DatabaseURL, false)
	if err != nil {
		return err
	}
	defer repo.Close()
	return repo.RunWithdrawalBatch(ctx, now)
}

func RefreshWithdrawalBatchHealth(ctx context.Context, cfg config.Config) error {
	if !cfg.WithdrawalBatchEnabled || !withdrawalRuntimeReady {
		return errors.New("withdrawal batch is disabled")
	}
	repo, err := postgres.Open(ctx, cfg.DatabaseURL, false)
	if err != nil {
		return err
	}
	defer repo.Close()
	return repo.RefreshWithdrawalBatchHealth(ctx)
}
