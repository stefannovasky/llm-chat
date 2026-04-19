package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
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
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	return &Client{
		apiKey:  apiKey,
		model:   model,
		httpCli: &http.Client{Transport: transport},
	}
}

type chatRequest struct {
	Model         string        `json:"model"`
	Messages      []chatMessage `json:"messages"`
	Stream        bool          `json:"stream"`
	StreamOptions streamOptions `json:"stream_options"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type apiErrorBody struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Stream starts a streaming chat completion. The returned channel is closed
// when the stream ends (success, error, or ctx cancel). Errors before the
// channel can be returned are returned directly.
func (c *Client) Stream(ctx context.Context, conv domain.Conversation) (<-chan domain.StreamEvent, error) {
	msgs := make([]chatMessage, len(conv.Messages))
	for i, m := range conv.Messages {
		msgs[i] = chatMessage{Role: string(m.Role), Content: m.Content}
	}

	body, err := json.Marshal(chatRequest{
		Model:         c.model,
		Messages:      msgs,
		Stream:        true,
		StreamOptions: streamOptions{IncludeUsage: true},
	})
	if err != nil {
		return nil, fmt.Errorf("openrouter: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openrouter: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		var apiErr apiErrorBody
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("openrouter: %s", apiErr.Error.Message)
		}
		return nil, fmt.Errorf("openrouter: %d %s", resp.StatusCode, respBody)
	}

	ch := make(chan domain.StreamEvent)
	go parseStream(ctx, resp.Body, ch)
	return ch, nil
}

func parseStream(ctx context.Context, body io.ReadCloser, ch chan<- domain.StreamEvent) {
	defer close(ch)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	// Some chunks (especially with usage payloads) can exceed default 64KB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			ch <- domain.StreamEvent{Err: fmt.Errorf("openrouter: parse chunk: %w", err)}
			return
		}

		if chunk.Error != nil {
			ch <- domain.StreamEvent{Err: fmt.Errorf("openrouter: %s", chunk.Error.Message)}
			return
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			ch <- domain.StreamEvent{Delta: chunk.Choices[0].Delta.Content}
		}

		if chunk.Usage != nil {
			ch <- domain.StreamEvent{Usage: &domain.Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
			}}
		}
	}

	if ctx.Err() != nil {
		return
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
		ch <- domain.StreamEvent{Err: fmt.Errorf("openrouter: read stream: %w", err)}
	}
}
