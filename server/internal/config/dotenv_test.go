package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnvLine(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		key   string
		value string
		ok    bool
	}{
		{name: "plain", line: "USE_MOCK_LLM=false", key: "USE_MOCK_LLM", value: "false", ok: true},
		{name: "single quoted", line: "OPENAI_MODEL='gpt-5-mini'", key: "OPENAI_MODEL", value: "gpt-5-mini", ok: true},
		{name: "double quoted", line: `OPENAI_MODEL="gpt-5-mini"`, key: "OPENAI_MODEL", value: "gpt-5-mini", ok: true},
		{name: "export", line: "export API_ADDR=0.0.0.0:8080", key: "API_ADDR", value: "0.0.0.0:8080", ok: true},
		{name: "comment", line: "# local settings", ok: false},
		{name: "inline comment", line: "USE_MOCK_LLM=false # OpenAI", key: "USE_MOCK_LLM", value: "false", ok: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key, value, ok, err := parseDotEnvLine(test.line)
			if err != nil {
				t.Fatalf("parseDotEnvLine() error = %v", err)
			}
			if key != test.key || value != test.value || ok != test.ok {
				t.Fatalf("parseDotEnvLine() = (%q, %q, %v), want (%q, %q, %v)", key, value, ok, test.key, test.value, test.ok)
			}
		})
	}
}

func TestLoadDotEnvFileDoesNotOverrideProcessEnvironment(t *testing.T) {
	const existingKey = "SPENDABLE_TEST_EXISTING"
	const loadedKey = "SPENDABLE_TEST_LOADED"
	restoreEnv(t, existingKey)
	restoreEnv(t, loadedKey)
	if err := os.Setenv(existingKey, "from-process"); err != nil {
		t.Fatal(err)
	}
	if err := os.Unsetenv(loadedKey); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "settings")
	content := existingKey + "=from-file\n" + loadedKey + "=loaded\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadDotEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(existingKey); got != "from-process" {
		t.Fatalf("existing environment was overridden: %q", got)
	}
	if got := os.Getenv(loadedKey); got != "loaded" {
		t.Fatalf("dotenv value was not loaded: %q", got)
	}
}

func TestParseDotEnvLineRejectsInvalidAssignment(t *testing.T) {
	if _, _, _, err := parseDotEnvLine("OPENAI API KEY=secret"); err == nil {
		t.Fatal("expected invalid key error")
	}
}

func restoreEnv(t *testing.T, key string) {
	t.Helper()
	value, exists := os.LookupEnv(key)
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(key, value)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}
