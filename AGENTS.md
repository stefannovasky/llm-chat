# llm-chat

Interactive LLM chat application for the terminal, built as a TUI with Bubble Tea and integrated with OpenRouter. Unix only, no Windows support.

## Running

```
make run          # run via go run
make build        # compile to bin/llm-chat
./bin/llm-chat    # run compiled binary
```

Config lives at `~/.config/llm-chat/config.toml` (respects `$XDG_CONFIG_HOME`). On first run with no config, the app creates a template and exits with instructions.

## Project structure

Code is organized under `internal/` with four packages: `config` (TOML parsing and validation), `domain` (Message, Conversation types), `llm` (OpenRouter HTTP client and SSE streaming), and `ui` (Bubble Tea model, view, update). The entrypoint is `cmd/llm-chat/main.go`. These layers must stay decoupled — `llm` and `ui` never import each other, they communicate through `domain` types.

## Stack

Go 1.25.0, Bubble Tea v2.0.5, Bubbles v2.1.0, Lip Gloss v2.0.3. Do not use different versions. Glamour and TOML library versions are TBD.

## Roadmap

Development is tracked in `roadmap.md`. The MVP spans phases 1 through 5 (setup, TUI layout, OpenRouter integration, streaming, Markdown rendering). Post-MVP phases 6 through 10 cover slash commands, conversation persistence, token management, advanced config, and UX polish. Mark tasks with `[x]` as they are completed.

## Commits

Follow conventional commits. Examples: `feat: add config loading`, `fix: handle empty API response`, `refactor: extract SSE parser`. Keep commits atomic — one logical change per commit.

## Key constraints

All terminal output goes through Bubble Tea. No `fmt.Println` or direct prints. No UI updates outside the update loop. No business logic in the view layer. Use existing Bubbles components instead of reimplementing them. The OpenRouter client uses native `net/http` with no third-party HTTP or OpenAI client libraries.
