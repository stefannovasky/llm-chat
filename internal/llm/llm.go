package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stefannovasky/llm-chat/internal/domain"
)

const endpoint = "https://openrouter.ai/api/v1/chat/completions"

type Client struct {
	apiKey  string
	model   string
	httpCli *http.Client
}

func NewClient(apiKey, model string) *Client {
	return &Client{
		apiKey:  apiKey,
		model:   model,
		httpCli: &http.Client{Timeout: 60 * time.Second},
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (c *Client) Chat(ctx context.Context, conv domain.Conversation) (domain.ChatResult, error) {
	msgs := make([]chatMessage, len(conv.Messages))
	for i, m := range conv.Messages {
		msgs[i] = chatMessage{Role: string(m.Role), Content: m.Content}
	}

	body, err := json.Marshal(chatRequest{Model: c.model, Messages: msgs})
	if err != nil {
		return domain.ChatResult{}, fmt.Errorf("openrouter: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return domain.ChatResult{}, fmt.Errorf("openrouter: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return domain.ChatResult{}, fmt.Errorf("openrouter: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.ChatResult{}, fmt.Errorf("openrouter: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return domain.ChatResult{}, fmt.Errorf("openrouter: %d %s", resp.StatusCode, respBody)
	}

	var result chatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return domain.ChatResult{}, fmt.Errorf("openrouter: unmarshal response: %w", err)
	}

	if len(result.Choices) == 0 {
		return domain.ChatResult{}, fmt.Errorf("openrouter: empty response")
	}

	return domain.ChatResult{
		Message: domain.Message{
			Role:    domain.RoleAssistant,
			Content: result.Choices[0].Message.Content,
		},
		Usage: domain.Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
		},
	}, nil
}
