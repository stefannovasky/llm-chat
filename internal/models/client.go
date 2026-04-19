package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Undocumented but public endpoint that powers openrouter.ai/rankings.
// Returns models ordered by weekly usage. Used here because the official
// /api/v1/models endpoint has no popularity signal.
const endpoint = "https://openrouter.ai/api/frontend/models/find?order=top-weekly"

type Model struct {
	ID             string
	Name           string
	ContextLength  int
	PromptPrice    string
	CompletionPrice string
}

type apiResponse struct {
	Data struct {
		Models []apiModel `json:"models"`
	} `json:"data"`
}

type apiModel struct {
	Slug          string       `json:"slug"`
	Name          string       `json:"name"`
	ContextLength int          `json:"context_length"`
	HasTextOutput bool         `json:"has_text_output"`
	Endpoint      *apiEndpoint `json:"endpoint"`
}

type apiEndpoint struct {
	Pricing pricing `json:"pricing"`
}

type pricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}

func Fetch(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("models: build request: %w", err)
	}

	cli := &http.Client{Timeout: 30 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models: unexpected status %d", resp.StatusCode)
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("models: decode: %w", err)
	}

	out := make([]Model, 0, len(body.Data.Models))
	for _, m := range body.Data.Models {
		if !m.HasTextOutput || m.Endpoint == nil {
			continue
		}
		out = append(out, Model{
			ID:              m.Slug,
			Name:            m.Name,
			ContextLength:   m.ContextLength,
			PromptPrice:     m.Endpoint.Pricing.Prompt,
			CompletionPrice: m.Endpoint.Pricing.Completion,
		})
	}
	return out, nil
}
