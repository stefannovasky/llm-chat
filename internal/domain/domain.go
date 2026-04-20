package domain

import "time"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

const DefaultSystemPrompt = "You are a helpful assistant."

const CompactPrompt = "You are summarizing this conversation so it can be continued with less context. " +
	"Preserve: the user's underlying goal, key facts and decisions, any code or concrete details referenced, " +
	"and any open questions or pending actions. Drop: pleasantries, tangents, and verbose explanations that " +
	"have already been acknowledged. Respond with the summary only — no preamble, no meta-commentary."

type Message struct {
	Role             Role       `json:"role"`
	Content          string     `json:"content"`
	Model            string     `json:"model,omitempty"`
	PromptTokens     int        `json:"prompt_tokens,omitempty"`
	CompletionTokens int        `json:"completion_tokens,omitempty"`
	Cost             float64    `json:"cost,omitempty"`
	CompactedAt      *time.Time `json:"compacted_at,omitempty"`
}

type Conversation struct {
	Messages []Message
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	Cost             float64
}

type StreamEvent struct {
	Delta string
	Usage *Usage
	Err   error
}
