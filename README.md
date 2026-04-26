# llm-chat

A terminal-first chat client for LLMs, using OpenRouter.

## Features

- Chat with any model available on OpenRouter (GPT, Claude, Gemini, Llama, etc.)
- Streaming responses rendered live in the terminal
- Markdown rendering (code blocks, lists, headings) via Glamour
- Persistent conversation history stored locally
- Simple TOML configuration
- Single static binary, no runtime dependencies

## Installation

Linux and macOS. Windows is not supported.

Build from source:

```
git clone https://github.com/stefannovasky/llm-chat
cd llm-chat
make build
./bin/llm-chat
```

## Configuration

Get an API key from [OpenRouter](https://openrouter.ai/keys).

On first run, `llm-chat` creates a config template at `~/.config/llm-chat/config.toml` (respects `$XDG_CONFIG_HOME`) and exits with instructions. Open the file and paste your key:

```toml
# OpenRouter API key (required)
api_key = "sk-or-..."

# Model to use (optional, defaults to openai/gpt-4o-mini)
# default_model = "openai/gpt-4o-mini"
```

Browse available model identifiers at [openrouter.ai/models](https://openrouter.ai/models).

## Usage

Launch with `llm-chat`. Type a message and press Enter to send. Alt+Enter inserts a newline for multi-line input.

Slash commands:

```
/new       start a fresh conversation
/model     switch the active model
/cost      show session cost and token usage
/compact   compact conversation history
/resume    list and reopen a previous conversation
/help      list available commands
```

## License

MIT — see [LICENSE](LICENSE).
