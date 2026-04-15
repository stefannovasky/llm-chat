package main

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/stefannovasky/llm-chat/internal/config"
	"github.com/stefannovasky/llm-chat/internal/llm"
	"github.com/stefannovasky/llm-chat/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrFirstRun) {
			fmt.Fprintf(os.Stderr, "Config file created at %s\nAdd your OpenRouter API key to get started.\n", config.ConfigPath())
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}

	client := llm.NewClient(cfg.APIKey, cfg.DefaultModel)
	p := tea.NewProgram(ui.New(cfg, client))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
