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
	AI AIConfig `yaml:"ai"`
}

func (c Config) AIEnabled() bool {
	return c.AI.Endpoint != "" && c.AI.Model != ""
}

func Load() (Config, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Config{}, fmt.Errorf("finding user config dir: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "asql", "config.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}
