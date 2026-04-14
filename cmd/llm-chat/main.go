package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/stefannovasky/llm-chat/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		if !errors.Is(err, config.ErrFirstRun) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}

	fmt.Printf("config loaded. model: %s\n", cfg.DefaultModel)
}
