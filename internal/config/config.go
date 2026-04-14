package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const defaultModel = "openai/gpt-4o-mini"

// ErrFirstRun is returned when no config file exists and a template was created.
var ErrFirstRun = errors.New("first run")

type Config struct {
	APIKey       string `toml:"api_key"`
	DefaultModel string `toml:"default_model"`
}

func configPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "llm-chat", "config.toml")
}

const templateConfig = `# OpenRouter API key (required)
api_key = ""

# Model to use (optional, defaults to openai/gpt-4o-mini)
# default_model = "openai/gpt-4o-mini"
`

// Load reads the config file, creates a template on first run, and validates required fields.
func Load() (*Config, error) {
	path := configPath()

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(templateConfig), 0644); err != nil {
			return nil, fmt.Errorf("failed to create config file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Config file created at %s\nAdd your OpenRouter API key to get started.\n", path)
		return nil, ErrFirstRun
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("error: api_key is required. Set it in %s", path)
	}

	if cfg.DefaultModel == "" {
		cfg.DefaultModel = defaultModel
	}

	return &cfg, nil
}
