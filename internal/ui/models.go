package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const modelsEndpoint = "https://openrouter.ai/api/frontend/models/find?order=top-weekly"

type llmInfo struct {
	ID              string
	Name            string
	ContextLength   int
	PromptPrice     string
	CompletionPrice string
}

type apiModelsResponse struct {
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

func fetchModels(ctx context.Context) ([]llmInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint, nil)
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

	var body apiModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("models: decode: %w", err)
	}

	out := make([]llmInfo, 0, len(body.Data.Models))
	for _, m := range body.Data.Models {
		if !m.HasTextOutput || m.Endpoint == nil {
			continue
		}
		out = append(out, llmInfo{
			ID:              m.Slug,
			Name:            m.Name,
			ContextLength:   m.ContextLength,
			PromptPrice:     m.Endpoint.Pricing.Prompt,
			CompletionPrice: m.Endpoint.Pricing.Completion,
		})
	}
	return out, nil
}

func orderModels(all []llmInfo, recent []string) []llmInfo {
	byID := make(map[string]llmInfo, len(all))
	for _, m := range all {
		byID[m.ID] = m
	}
	out := make([]llmInfo, 0, len(all))
	seen := make(map[string]bool)
	for _, id := range recent {
		if m, ok := byID[id]; ok && !seen[id] {
			out = append(out, m)
			seen[id] = true
		}
	}
	for _, m := range all {
		if !seen[m.ID] {
			out = append(out, m)
		}
	}
	return out
}
