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

	primary, source := modelClient(cfg)
	fallback := agent.MockClient{}
	agents := agentadapter.New(primary, fallback, source)
	apiApplication := application.New(application.Config{Repository: repo,
		ConsultationAgent: agents, ReviewAgent: agents, MemoryAgent: agents})

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

func modelClient(cfg config.Config) (agent.Model, string) {
	if cfg.UseMockLLM {
		return agent.MockClient{}, "mock"
	}
	return openai.NewOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIModel), "openai"
}
