package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type AIConfig struct {
	Endpoint string `yaml:"ai_endpoint"`
	Model    string `yaml:"ai_model"`
	APIKey   string `yaml:"ai_api_key"`
}

type Config struct {
	AI       AIConfig `yaml:"ai"`
	Warnings []string `yaml:"-"` // non-fatal warnings collected during load
}

func (c Config) AIEnabled() bool {
	return c.AI.Endpoint != "" && c.AI.Model != ""
}

func configDir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d, nil
	}
	return os.UserConfigDir()
}

func Load() (Config, error) {
	dir, err := configDir()
	if err != nil {
		return Config{}, fmt.Errorf("finding user config dir: %w", err)
	}

	configPath := filepath.Join(dir, "asql", "config.yaml")

	var cfg Config

	// Check file permissions — warn if too permissive
	if info, statErr := os.Stat(configPath); statErr == nil {
		if perm := info.Mode().Perm(); perm&0077 != 0 {
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("config file %s has permissions %o, recommend 0600", configPath, perm))
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("reading config: %w", err)
		}
		// File not found — continue with zero-value cfg; env vars may still apply
	} else if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	// Environment variables override config file values
	if v := os.Getenv("ASQL_AI_API_KEY"); v != "" {
		cfg.AI.APIKey = v
	}
	if v := os.Getenv("ASQL_AI_ENDPOINT"); v != "" {
		cfg.AI.Endpoint = v
	}
	if v := os.Getenv("ASQL_AI_MODEL"); v != "" {
		cfg.AI.Model = v
	}

	return cfg, nil
}
