package domain

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

const DefaultSystemPrompt = "You are a helpful assistant."

type Message struct {
	Role    Role
	Content string
}

type Conversation struct {
	Messages []Message
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

type ChatResult struct {
	Message Message
	Usage   Usage
}
