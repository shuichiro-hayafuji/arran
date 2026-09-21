package app

import (
	"context"
	"net/http"
	"time"

	"github.com/shuichiro-hayafuji/arran_agent"
	"github.com/shuichirohayafuji/spendable-today/server/internal/agentadapter"
	"github.com/shuichirohayafuji/spendable-today/server/internal/application"
	"github.com/shuichirohayafuji/spendable-today/server/internal/auth"
	"github.com/shuichirohayafuji/spendable-today/server/internal/config"
	"github.com/shuichirohayafuji/spendable-today/server/internal/handler"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/notification"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/openai"
	"github.com/shuichirohayafuji/spendable-today/server/internal/infrastructure/persistence/postgres"
)

// App owns the resources created during application composition.
type App struct {
	Server     *http.Server
	repository *postgres.Repository
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	repo, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	primary, source := modelClient(cfg, repo)
	fallback := agent.MockClient{}
	agents := agentadapter.New(primary, fallback, source)
	fallbackAgent := agentadapter.New(fallback, fallback, "quota_fallback")
	adminNotifier := notification.NewPagerDuty(cfg.PagerDutyRoutingKey)
	apiApplication := application.New(application.Config{Repository: repo,
		ConsultationAgent: agents, FallbackAgent: fallbackAgent,
		ReviewAgent: agents, FallbackReview: fallbackAgent,
		MemoryAgent: agents, FallbackMemory: fallbackAgent, AdminNotifier: adminNotifier,
		Environment: cfg.Environment})

	return &App{
		Server: &http.Server{
			Addr:              cfg.APIAddr,
			Handler:           auth.New(repo).Wrap(handler.New(apiApplication)),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		repository: repo,
	}, nil
}

func (a *App) Close() error {
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
