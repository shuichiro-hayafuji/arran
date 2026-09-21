package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
)

// Config contains the runtime settings required to start the API.
//
// Environment-variable parsing belongs here so the application composition
// layer does not need to know how settings are represented externally.
type Config struct {
	APIAddr               string
	DatabaseURL           string
	UseMockLLM            bool
	OpenAIAPIKey          string
	OpenAIModel           string
	OpenAIReasoningEffort string
	Environment           string
	PagerDutyRoutingKey   string
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	useMock, err := strconv.ParseBool(envOrDefault("USE_MOCK_LLM", "true"))
	if err != nil {
		return Config{}, fmt.Errorf("USE_MOCK_LLM must be true or false")
	}

	reasoningEffort := envOrDefault("OPENAI_REASONING_EFFORT", "medium")
	switch reasoningEffort {
	case "none", "low", "medium", "high", "xhigh", "max":
	default:
		return Config{}, fmt.Errorf("OPENAI_REASONING_EFFORT must be one of none, low, medium, high, xhigh, or max")
	}

	return Config{
		APIAddr:               envOrDefault("API_ADDR", "127.0.0.1:8080"),
		DatabaseURL:           loadDatabaseURL(),
		UseMockLLM:            useMock,
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:           envOrDefault("OPENAI_MODEL", "gpt-5.6-terra"),
		OpenAIReasoningEffort: reasoningEffort,
		Environment:           envOrDefault("APP_ENVIRONMENT", "local"),
		PagerDutyRoutingKey:   os.Getenv("PAGERDUTY_ROUTING_KEY"),
	}, nil
}

func loadDatabaseURL() string {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}
	databaseURL := url.URL{
		Scheme: "postgres", User: url.UserPassword(envOrDefault("DB_USER", "spendable_today"), envOrDefault("DB_PASSWORD", "local-only-password")),
		Host: net.JoinHostPort(envOrDefault("DB_HOST", "127.0.0.1"), envOrDefault("DB_PORT", "5432")), Path: envOrDefault("DB_NAME", "spendable_today"),
	}
	query := databaseURL.Query()
	query.Set("sslmode", envOrDefault("DB_SSLMODE", "disable"))
	databaseURL.RawQuery = query.Encode()
	return databaseURL.String()
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
