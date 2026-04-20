package main

import (
	"cmp"
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

	state := ui.LoadState()
	currentModel := cmp.Or(cfg.DefaultModel, state.Current, config.HardcodedDefaultModel)

	client := llm.NewClient(cfg.APIKey)
	p := tea.NewProgram(ui.New(cfg, client, currentModel, state))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
