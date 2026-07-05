# Go Terminal AI Agent

A production-ready terminal AI agent with multi-provider LLM support, tool execution, and a ChatGPT-style TUI.

## Features

- **Multi-provider**: OpenAI, Anthropic (Claude), Google Gemini, Groq, NVIDIA, OpenRouter
- **ChatGPT-style TUI**: Streaming responses, chat history, colored output
- **CLI mode**: One-shot prompts from the command line
- **Tool execution**: Read/write/edit files, grep, glob, web search/fetch, bash commands
- **Session persistence**: Chat history auto-saves across restarts
- **Response cache**: LRU cache with TTL for frequently asked questions
- **`go` wrapper**: Use `go "prompt"` as a system-wide alias alongside the real Go compiler

## One-Command Install

Works on any Linux distro, any Go version (1.22+), **no sudo required**:

```bash
curl -sL https://raw.githubusercontent.com/yesk993-ops/go-terminal-agent/master/scripts/setup-global.sh | bash
```

Then add to your `~/.bashrc` or `~/.zshrc`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Set at least one API key (choose one):

```bash
export NVIDIA_API_KEY="nvapi-..."    # free, reliable
export GROQ_API_KEY="gsk_..."          # free tier
export OPENAI_API_KEY="sk-..."        # paid
export ANTHROPIC_API_KEY="sk-ant-..." # paid
export GEMINI_API_KEY="AIza..."       # free tier
export OPENROUTER_API_KEY="sk-or-..." # paid
```

Run it:

```bash
go                          # Launch interactive TUI
go "what is docker?"        # Ask a question
go build ./...              # Real Go compiler (passthrough)
```

## Manual Setup

### 1. Build

```bash
git clone https://github.com/yesk993-ops/go-terminal-agent.git
cd go-terminal-agent
CGO_ENABLED=0 go build -ldflags="-s -w" -o ai-agent ./cmd/agent
```

### 2. Configure

```bash
mkdir -p ~/.config/agent
cp config.yaml ~/.config/agent/config.yaml
```

Edit `~/.config/agent/config.yaml` or set environment variables (see API keys above).

### 3. Run

```bash
# TUI mode
./ai-agent

# CLI mode
./ai-agent "explain goroutines in Go"
```

## Configuration

File: `~/.config/agent/config.yaml`

| Key | Description | Default |
|---|---|---|
| `provider.default` | Default LLM provider | `nvidia` |
| `providers[].api_key` | API key (or use env var) | — |
| `providers[].model` | Model name | — |
| `session.max_messages` | Max stored messages | `100` |
| `session.save_path` | Session save directory | `~/.local/share/agent/sessions` |
| `cache.enabled` | Enable response cache | `true` |
| `cache.max_size` | Max cache entries | `500` |
| `cache.default_ttl` | Cache TTL | `5m` |
| `logging.level` | Log level | `info` |

## Supported Providers

| Provider | Model | API Key |
|---|---|---|
| NVIDIA | meta/llama-3.1-8b-instruct (default) or qwen/qwen3.5-122b-a10b | `nvapi-...` |
| OpenAI | gpt-4o | `sk-...` |
| Anthropic | claude-sonnet-4-20250514 | `sk-ant-...` |
| Google Gemini | gemini-2.5-pro | `AIza...` |
| Groq | llama-3.3-70b-versatile | `gsk_...` |
| OpenRouter | openrouter/auto | `sk-or-...` |

## Project Structure

```
├── cmd/agent/main.go       # CLI entry point + rendering
├── internal/
│   ├── agent/              # Agent loop (streaming, tool calls)
│   ├── cache/              # LRU cache with TTL
│   ├── config/             # Viper-based config loader
│   ├── core/               # Shared types (Agent, Session, Token, etc.)
│   ├── logger/             # Structured logging (slog)
│   ├── plugin/             # Plugin system
│   ├── provider/           # LLM provider implementations (6 providers)
│   ├── session/            # Session persistence with auto-save
│   ├── tool/               # Tool implementations (8 tools)
│   └── tui/                # Bubble Tea TUI
├── config.yaml             # Example config with API key instructions
├── scripts/
│   ├── go-wrapper          # `go` command wrapper
│   ├── install.sh          # System-wide installer
│   └── setup-global.sh     # One-command installer (no sudo)
└── Makefile
```

## Tools (CLI mode only)

| Tool | Description |
|---|---|
| `read` | Read file contents |
| `write` | Write content to file |
| `edit` | Edit file via find-and-replace |
| `grep` | Search file contents with regex |
| `glob` | Find files by pattern |
| `bash` | Execute commands (destructive patterns blocked) |
| `web_search` | Search the web |
| `web_fetch` | Fetch and extract web page content |

## License

MIT