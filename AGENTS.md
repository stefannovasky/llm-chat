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

Code is organized under `internal/` with five packages: `config` (TOML parsing and validation), `llm` (OpenRouter HTTP client and SSE streaming), `sessions` (Message and Conversation types, session persistence logic), `storage` (XDG path helpers and atomic JSON writes), and `ui` (Bubble Tea model, view, update, slash commands). The entrypoint is `cmd/llm-chat/main.go`.

Follow package-oriented design: each package owns a clear responsibility and exposes a minimal API. Don't create utility/helper grab-bag packages.

## Stack

Go 1.25, Bubble Tea v2.0.5, Bubbles v2.1.0, Lip Gloss v2.0.3, Glamour v2.0.0, BurntSushi/toml v1.6.0. Do not use different versions.

## Commits

Follow conventional commits. Examples: `feat: add config loading`, `fix: handle empty API response`, `refactor: extract SSE parser`. Keep commits atomic — one logical change per commit.

## Key constraints

All terminal output goes through Bubble Tea — no `fmt.Println` or direct prints. The OpenRouter client uses native `net/http` with no third-party HTTP or OpenAI client libraries.
