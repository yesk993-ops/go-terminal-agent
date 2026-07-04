# Go Terminal AI Agent

A production-ready terminal AI agent with multi-provider LLM support, tool execution, and a ChatGPT-style TUI.

## Features

- **Multi-provider**: OpenAI, Anthropic (Claude), Google Gemini, Groq, NVIDIA, OpenRouter
- **ChatGPT-style TUI**: Streaming responses, chat history, colored output
- **CLI mode**: One-shot prompts from the command line
- **Tool execution**: Read/write/edit files, grep, glob, web search/fetch, bash commands
- **Session persistence**: Chat history auto-saves across restarts
- **Response cache**: LRU cache with TTL for frequently asked questions
- **`go` wrapper**: Use `go "prompt"` as a system-wide alias

## Quick Start

### 1. Configure

```bash
cp config.yaml ~/.config/agent/config.yaml
```

Edit and set at least one API key:

```yaml
providers:
  - name: nvidia
    api_key: nvapi-xxx
    model: meta/llama-3.1-8b-instruct
  - name: openai
    api_key: sk-xxx
    model: gpt-4o

provider:
  default: nvidia
```

### 2. Run

```bash
# TUI mode (interactive chat)
./ai-agent

# CLI mode (one-shot prompt)
./ai-agent "explain goroutines in Go"
```

### 3. Install system-wide (optional)

```bash
bash scripts/install.sh
```

This installs the `go` wrapper — use it as both the Go compiler and an AI assistant:

```bash
go "how do I write a REST API in Go?"   # AI agent
go build ./...                           # Go compiler (passthrough)
```

## Configuration

File: `~/.config/agent/config.yaml`

| Key | Description | Default |
|---|---|---|
| `provider.default` | Default LLM provider | `nvidia` |
| `providers[].name` | Provider name | — |
| `providers[].api_key` | API key | — |
| `providers[].model` | Model name | — |
| `session.max_messages` | Max stored messages | `100` |
| `session.save_path` | Session save directory | `~/.config/agent/sessions` |
| `cache.enabled` | Enable response cache | `true` |
| `cache.max_size` | Max cache entries | `100` |
| `cache.default_ttl` | Cache TTL | `5m` |
| `logging.level` | Log level | `info` |

## Supported Providers

| Provider | Model | API Key |
|---|---|---|
| NVIDIA | meta/llama-3.1-8b-instruct (default) | `nvapi-xxx` |
| OpenAI | gpt-4o, gpt-4o-mini | `sk-xxx` |
| Anthropic | claude-sonnet-4-20250514 | `sk-ant-xxx` |
| Google Gemini | gemini-2.0-flash | AIza... |
| Groq | llama-3.3-70b-versatile | `gsk_xxx` |
| OpenRouter | openai/o3-mini | `sk-or-xxx` |

Set via environment variable or config file:

```bash
export NVIDIA_API_KEY=nvapi-xxx
export OPENAI_API_KEY=sk-xxx
```

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
├── config.yaml             # Example config
├── scripts/
│   ├── go-wrapper          # `go` command wrapper
│   └── install.sh          # System-wide installer
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