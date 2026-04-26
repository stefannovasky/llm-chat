package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const HardcodedDefaultModel = "openai/gpt-4o-mini"

const DefaultSystemPrompt = "You are a helpful assistant."

// ErrFirstRun is returned when no config file exists and a template was created.
var ErrFirstRun = errors.New("first run")

type Config struct {
	APIKey       string `toml:"api_key"`
	DefaultModel string `toml:"default_model"`
	SystemPrompt string `toml:"system_prompt"`
}

func ConfigPath() string {
	base, _ := os.UserConfigDir()
	return filepath.Join(base, "llm-chat", "config.toml")
}

const templateConfig = `# OpenRouter API key (required)
api_key = ""

# Model to use (optional, defaults to openai/gpt-4o-mini)
# default_model = "openai/gpt-4o-mini"

# System prompt (optional)
# system_prompt = """
# Line one of the prompt.
# Line two of the prompt.
# """
`

func Load() (*Config, error) {
	path := ConfigPath()

	var cfg Config
	_, err := toml.DecodeFile(path, &cfg)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("create config dir: %w", err)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("create config file: %w", err)
		}
		_, writeErr := f.WriteString(templateConfig)
		if err := f.Close(); writeErr == nil && err != nil {
			writeErr = err
		}
		if writeErr != nil {
			return nil, fmt.Errorf("write config: %w", writeErr)
		}
		return nil, ErrFirstRun
	case err != nil:
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required. Set it in %s", path)
	}
	cfg.DefaultModel = strings.TrimSpace(cfg.DefaultModel)
	cfg.SystemPrompt = strings.TrimSpace(cfg.SystemPrompt)
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = DefaultSystemPrompt
	}
	return &cfg, nil
}
