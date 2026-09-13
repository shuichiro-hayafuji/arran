package config

import "testing"

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
