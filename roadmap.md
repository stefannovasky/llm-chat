# llm-chat — Roadmap

Interactive LLM chat in the terminal via TUI, integrated with OpenRouter. Unix only.

---

## Architecture

```
llm-chat/
├── cmd/
│   └── llm-chat/
│       └── main.go
├── internal/
│   ├── config/
│   ├── domain/
│   ├── llm/
│   └── ui/
├── config.toml.example
├── go.mod
└── go.sum
```

**Layers:**
- **config** — reads and validates `~/.config/llm-chat/config.toml` (respects `$XDG_CONFIG_HOME`)
- **domain** — Message, Conversation, system prompt
- **llm** — native HTTP client for OpenRouter, SSE parsing, streaming control
- **ui** — Bubble Tea model/update/view, layout, keybindings, rendering

---

## Stack

- Go 1.25.0
- Bubble Tea v2.0.5
- Bubbles v2.1.0
- Lip Gloss v2.0.3
- Glamour (version TBD)
- TOML library (TBD)

---

## MVP

### Phase 1 — Project setup

**Goal:** project compiles with final folder structure and working config.

- [ ] Initialize Go module and add dependencies with exact versions
- [ ] Create folder structure
- [ ] Create `config.toml.example` with `api_key` and `model` fields
- [ ] Implement config reading from `~/.config/llm-chat/config.toml`
- [ ] On first run, create template config and display guidance message
- [ ] Validate config: error and exit if `api_key` is missing; hardcoded default if `model` is missing
- [ ] Entrypoint at `cmd/llm-chat/main.go`

**Result:** `go run ./cmd/llm-chat` loads config and prints a message. No UI yet.

---

### Phase 2 — Basic TUI layout

**Goal:** functional TUI with layout, multiline input, and scroll.

- [ ] Bubble Tea model with 3 areas: header, history, input
- [ ] Header: simple line with app name
- [ ] History: scrollable area taking up all remaining space
- [ ] Input: fixed at the bottom, dynamic height (up to ~6 lines)
- [ ] Enter sends message, Shift+Enter inserts new line
- [ ] History scroll via mouse scroll and PageUp/PageDown
- [ ] Colored dots to differentiate user and assistant messages
- [ ] Ctrl+C exits the app

**Result:** TUI opens, user types messages that appear in history with colored dots. No LLM yet.

---

### Phase 3 — OpenRouter integration

**Goal:** working conversation with the LLM, no streaming yet.

- [ ] Define Message and Conversation types in the domain layer
- [ ] Implement generic chat system prompt
- [ ] Native HTTP client for OpenRouter chat completions endpoint (no streaming)
- [ ] Send full conversation history with each request
- [ ] Capture usage data (prompt_tokens, completion_tokens) from response
- [ ] Display API errors inline in red
- [ ] Connect UI to domain: sending a message triggers the client, response appears in history

**Result:** user chats with the LLM. Responses appear complete (no streaming).

---

### Phase 4 — Streaming

**Goal:** responses appear token by token in real time.

- [ ] SSE parsing in the HTTP client
- [ ] Incremental UI updates with each received token
- [ ] Block input while streaming is active
- [ ] Ctrl+C during streaming cancels the current response (text already received is kept)
- [ ] Second Ctrl+C exits the app
- [ ] Visual "typing" indicator while streaming is active

**Result:** responses flow token by token. Input blocked during generation. Ctrl+C cancels.

---

### Phase 5 — Markdown rendering

**Goal:** assistant responses are rendered with proper formatting.

- [ ] Integrate Glamour for Markdown rendering in assistant responses
- [ ] Code blocks, bold, italic, lists rendered correctly in the terminal

**Result:** MVP complete. User can chat with an LLM in the terminal with streaming and formatted responses.

---

## Post-MVP

### Phase 6 — Slash commands

- [ ] Slash command parser: input starting with `/` is interpreted as a command
- [ ] `/model` — switch active model
- [ ] `/cost` — display accumulated session cost (uses captured usage data)
- [ ] `/compact` — manually compact conversation history
- [ ] Invalid command errors displayed inline
- [ ] Command output displayed as a system message with distinct color

---

### Phase 7 — Conversation persistence + resume

- [ ] Save conversations to disk (format TBD)
- [ ] `/resume` — list and re-enter previous conversations

---

### Phase 8 — Token management

- [ ] Token count display for current conversation
- [ ] Fetch model context window limits from OpenRouter API
- [ ] Autocompact — automatically compact history when approaching token limit

---

### Phase 9 — Advanced config

- [ ] `temperature` and `max_tokens` configurable via `config.toml`
- [ ] Configurable HTTP timeout via `config.toml`
- [ ] Customizable system prompt via `config.toml`

---

### Phase 10 — UX polish

- [ ] Header displaying active model
- [ ] Unblocked input during streaming

---

## Workflow

Commit after each completed task using conventional commits. Mark the task checkbox with `[x]` in this file before committing.

---

## Technical decisions

- **State:** Bubble Tea model is the single source of truth. All mutations go through `Update`.
- **Streaming:** goroutine + channel pattern. No UI manipulation outside the update loop.
- **Separation:** `internal/llm` does not import `internal/ui` and vice-versa. Communication via `internal/domain` types.
- **Config:** XDG Base Directory. Single read at startup.
- **Errors:** displayed inline in the chat, no automatic retry.
- **OpenRouter:** vanilla usage only — no fallback routing, provider preferences, or transforms.
- **Target:** Unix only. No Windows support.
