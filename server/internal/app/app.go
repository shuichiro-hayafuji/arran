package app

import (
	"context"
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

func New(ctx context.Context, cfg config.Config) (*App, error) {
	repo, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	primary, source := modelClient(cfg, usageRecorder(repo))
	fallback := agent.MockClient{}
	agents := agentadapter.New(primary, fallback, source)
	var chatAgent consultation.Agent
	if !cfg.UseMockLLM {
		client := openai.NewOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIReasoningEffort)
		client.SetUsageRecorder(usageRecorder(repo))
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
			Handler:           auth.New(repo).Wrap(handler.New(apiApplication)),
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

func (a *App) Close() error {
	a.stopRequests()
	a.application.WaitRequests()
	a.stopWorker()
	<-a.workerDone
	return a.repository.Close()
}

func modelClient(cfg config.Config, usageRecorder openai.UsageRecorder) (agent.Model, string) {
	if cfg.UseMockLLM {
		return agent.MockClient{}, "mock"
	}
	client := openai.NewOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIModel, cfg.OpenAIReasoningEffort)
	client.SetUsageRecorder(usageRecorder)
	return client, "openai"
}
