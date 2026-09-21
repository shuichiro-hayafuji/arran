package config

import "testing"

func TestLoadUsesTerraMediumDefaults(t *testing.T) {
	t.Setenv("USE_MOCK_LLM", "false")
	t.Setenv("OPENAI_MODEL", "")
	t.Setenv("OPENAI_REASONING_EFFORT", "")

	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.OpenAIModel != "gpt-5.6-terra" {
		t.Fatalf("OpenAIModel = %q", config.OpenAIModel)
	}
	if config.OpenAIReasoningEffort != "medium" {
		t.Fatalf("OpenAIReasoningEffort = %q", config.OpenAIReasoningEffort)
	}
}

func TestLoadRejectsUnknownReasoningEffort(t *testing.T) {
	t.Setenv("USE_MOCK_LLM", "false")
	t.Setenv("OPENAI_REASONING_EFFORT", "extreme")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error")
	}
}

func TestLoadDatabaseURLBuildsLocalPostgresDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "spendable_today")
	t.Setenv("DB_USER", "spendable_today")
	t.Setenv("DB_PASSWORD", "local-only-password")
	t.Setenv("DB_SSLMODE", "disable")
	if got := loadDatabaseURL(); got != "postgres://spendable_today:local-only-password@127.0.0.1:5432/spendable_today?sslmode=disable" {
		t.Fatalf("database URL = %q", got)
	}
}
