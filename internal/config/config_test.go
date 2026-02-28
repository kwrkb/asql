package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("missing file returns zero config", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.AIEnabled() {
			t.Error("expected AI disabled for missing file")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)

		asqlDir := filepath.Join(dir, "asql")
		os.MkdirAll(asqlDir, 0o755)
		os.WriteFile(filepath.Join(asqlDir, "config.yaml"), []byte(`
ai:
  ai_endpoint: http://localhost:11434/v1
  ai_model: llama3
  ai_api_key: sk-test
`), 0o644)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.AIEnabled() {
			t.Error("expected AI enabled")
		}
		if cfg.AI.Endpoint != "http://localhost:11434/v1" {
			t.Errorf("unexpected endpoint: %q", cfg.AI.Endpoint)
		}
		if cfg.AI.Model != "llama3" {
			t.Errorf("unexpected model: %q", cfg.AI.Model)
		}
		if cfg.AI.APIKey != "sk-test" {
			t.Errorf("unexpected api key: %q", cfg.AI.APIKey)
		}
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)

		asqlDir := filepath.Join(dir, "asql")
		os.MkdirAll(asqlDir, 0o755)
		os.WriteFile(filepath.Join(asqlDir, "config.yaml"), []byte(`{invalid`), 0o644)

		_, err := Load()
		if err == nil {
			t.Error("expected error for invalid YAML")
		}
	})

	t.Run("partial config with missing model", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)

		asqlDir := filepath.Join(dir, "asql")
		os.MkdirAll(asqlDir, 0o755)
		os.WriteFile(filepath.Join(asqlDir, "config.yaml"), []byte(`
ai:
  ai_endpoint: http://localhost:11434/v1
`), 0o644)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.AIEnabled() {
			t.Error("expected AI disabled when model is missing")
		}
	})
}

func TestAIEnabled(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		model    string
		want     bool
	}{
		{"both set", "http://localhost", "model", true},
		{"empty endpoint", "", "model", false},
		{"empty model", "http://localhost", "", false},
		{"both empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{AI: AIConfig{Endpoint: tt.endpoint, Model: tt.model}}
			if got := cfg.AIEnabled(); got != tt.want {
				t.Errorf("AIEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
