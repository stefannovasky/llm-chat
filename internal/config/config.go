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

func ConfigPath() string {
	base, _ := os.UserConfigDir()
	return filepath.Join(base, "llm-chat", "config.toml")
}

const templateConfig = `# OpenRouter API key (required)
api_key = ""

# Model to use (optional, defaults to openai/gpt-4o-mini)
# default_model = "openai/gpt-4o-mini"
`

func Load() (*Config, error) {
	path := ConfigPath()

	var cfg Config
	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create config file: %w", err)
		}
		_, werr := f.WriteString(templateConfig)
		f.Close()
		if werr != nil {
			return nil, fmt.Errorf("failed to write config file: %w", werr)
		}
		return nil, ErrFirstRun
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required. Set it in %s", path)
	}

	if cfg.DefaultModel == "" {
		cfg.DefaultModel = defaultModel
	}

	return &cfg, nil
}
