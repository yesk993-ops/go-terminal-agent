# Go Terminal AI Agent

A production-ready terminal AI assistant with multi-provider LLM support, tool execution, and a ChatGPT-style TUI.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)](#)

---

## Features

| Feature | Description |
|---------|-------------|
| **Multi-provider LLM support** | OpenAI, Anthropic (Claude), Google Gemini, Groq, NVIDIA, OpenRouter |
| **ChatGPT-style TUI** | Streaming responses, chat history, colored output, interactive commands |
| **CLI mode** | One-shot prompts from command line for scripting/automation |
| **Tool execution** | 8 built-in tools: read/write/edit files, grep, glob, bash, web search/fetch |
| **Session persistence** | Chat history auto-saves to `~/.local/share/agent/sessions/` across restarts |
| **Response caching** | LRU cache with TTL (default: 5 min, 500 entries) for repeated queries |
| **Fallback provider chain** | Auto-retry with backoff, then failover to next provider on rate limits |
| **`go` wrapper** | System-wide alias: `go "prompt"` runs AI, `go build` passes to real Go compiler |
| **Cross-platform** | Linux, macOS, Windows (amd64) |

---

## Quick Install

### One-Command Install (Linux/macOS, no sudo)

```bash
curl -sL https://raw.githubusercontent.com/yesk993-ops/go-terminal-agent/master/scripts/setup-global.sh | bash
```

Then add to your shell config (`~/.bashrc`, `~/.zshrc`):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### Manual Build

```bash
git clone https://github.com/yesk993-ops/go-terminal-agent.git
cd go-terminal-agent
make setup          # Builds and installs to ~/.local/bin with config
# or: make build     # Just builds binary as ./ai-agent
```

---

## Configuration

### API Keys (choose at least one)

```bash
export NVIDIA_API_KEY="nvapi-..."    # Free, reliable (default)
export GROQ_API_KEY="gsk_..."        # Free tier
export OPENAI_API_KEY="sk-..."       # Paid
export ANTHROPIC_API_KEY="sk-ant-..." # Paid
export GEMINI_API_KEY="AIza..."      # Free tier
export OPENROUTER_API_KEY="sk-or-..." # Paid (access to all models)
```

Or edit `~/.config/agent/config.yaml` directly.

### Config File: `~/.config/agent/config.yaml`

```yaml
provider:
  default: nvidia
  max_tokens: 8192
  temperature: 0.7

providers:
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "gpt-4o"
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    model: "claude-sonnet-4-20250514"
  gemini:
    api_key: "${GEMINI_API_KEY}"
    model: "gemini-2.5-pro"
  groq:
    api_key: "${GROQ_API_KEY}"
    model: "llama-3.3-70b-versatile"
  nvidia:
    api_key: "${NVIDIA_API_KEY}"
    model: "meta/llama-3.1-8b-instruct"
  openrouter:
    api_key: "${OPENROUTER_API_KEY}"
    model: "openrouter/auto"

ui:
  theme: "catppuccin-mocha"
  show_cost: true
  max_history_ui: 50

session:
  max_messages: 100
  max_age: 24h
  auto_save: true
  save_path: "~/.local/share/agent/sessions"

cache:
  enabled: true
  max_size: 500
  default_ttl: 5m

logging:
  level: "info"
  format: "text"
  output: "stderr"
```

---

## Usage

### Interactive TUI

```bash
go                    # Launch TUI
```

**TUI Commands:**
| Key / Command | Action |
|---------------|--------|
| Type + Enter | Send message |
| `/provider <name>` | Switch provider (openai, anthropic, gemini, groq, nvidia, openrouter) |
| `/model <name>` | Switch model |
| `/think` | Toggle extended thinking mode |
| `/clear` | Clear chat history |
| `/status` | Show provider/model/cache/session status |
| `/help` | Show help |
| `/exit` or `Ctrl+C` | Exit TUI |

### CLI Mode (One-shot)

```bash
go "explain goroutines in Go"
go -p groq -m llama-3.3-70b-versatile "write a http server in go"
go --list-providers
```

### Real Go Compiler Passthrough

```bash
go build ./...
go test ./...
go mod tidy
# Any go subcommand passes through to real Go
```

---

## Supported Providers

| Provider | Default Model | API Key Format | Notes |
|----------|--------------|----------------|-------|
| **NVIDIA** | `meta/llama-3.1-8b-instruct` | `nvapi-...` | **Default** — free, reliable |
| **Groq** | `llama-3.3-70b-versatile` | `gsk_...` | Free tier, very fast |
| **OpenRouter** | `openrouter/auto` | `sk-or-...` | Access to Claude, GPT-4o, etc. |
| **OpenAI** | `gpt-4o` | `sk-...` | Paid |
| **Anthropic** | `claude-sonnet-4-20250514` | `sk-ant-...` | Paid |
| **Google Gemini** | `gemini-2.5-pro` | `AIza...` | Free tier available |

Override per-request: `go -p openai -m gpt-4o "prompt"`

---

## Tools (Available in CLI Mode)

| Tool | Description |
|------|-------------|
| `read` | Read file contents with optional line range |
| `write` | Write content to file (creates parent dirs) |
| `edit` | Find-and-replace edit in file |
| `grep` | Regex search file contents (with file filter) |
| `glob` | Find files by glob pattern |
| `bash` | Execute shell commands (destructive commands blocked) |
| `web_search` | DuckDuckGo instant answer API search |
| `web_fetch` | Fetch and extract web page content |

> **Note:** Tools are only available in CLI mode (`go "prompt"`), not in the interactive TUI.

---

## Project Structure

```
go-terminal-agent/
├── cmd/agent/main.go          # CLI entry point (TUI/CLI mode dispatch)
├── internal/
│   ├── agent/agent.go         # Agent loop: streaming, tool calls, caching
│   ├── cache/cache.go         # LRU cache with TTL
│   ├── config/config.go       # Viper config loader (env + YAML)
│   ├── core/                  # Core interfaces & types
│   │   ├── agent.go           # Agent interface
│   │   ├── cache.go           # Cache interface
│   │   ├── config.go          # Config structs
│   │   ├── provider.go        # Provider interface, Message, ToolCall, etc.
│   │   ├── session.go         # Session/Store interfaces
│   │   └── tool.go            # Tool/Registry interfaces
│   ├── logger/logger.go       # Structured slog logger
│   ├── plugin/plugin.go       # Go plugin system (.so files)
│   ├── provider/              # 6 LLM providers + fallback chain
│   │   ├── provider.go        # Registry, base provider, SSE streaming
│   │   ├── anthropic.go       # Anthropic Claude
│   │   ├── fallback.go        # Retry + fallback chain
│   │   ├── gemini.go          # Google Gemini
│   │   ├── groq.go            # Groq (OpenAI-compatible)
│   │   ├── nvidia.go          # NVIDIA (OpenAI-compatible)
│   │   ├── openai.go          # OpenAI
│   │   └── openrouter.go      # OpenRouter
│   ├── session/session.go     # JSON file persistence with auto-save
│   ├── tool/                  # 8 tool implementations
│   │   ├── bash.go            # Shell exec (destructive blocked)
│   │   ├── edit.go            # File edit tool
│   │   ├── glob.go            # Glob pattern search
│   │   ├── grep.go            # Regex content search
│   │   ├── read.go            # File read tool
│   │   ├── registry.go        # Tool registry + JSON schema
│   │   ├── webfetch.go        # HTTP fetch tool
│   │   ├── websearch.go       # DuckDuckGo search tool
│   │   └── write.go           # File write tool
│   └── tui/                   # Bubble Tea TUI
│       ├── tui.go             # Main TUI model & commands
│       ├── markdown.go        # Message styling
│       ├── render.go          # CLI frame writer (markdown, word-wrap)
│       └── styles.go          # Lip Gloss styles
├── config.yaml                # Example config
├── scripts/
│   ├── go-wrapper             # Smart `go` command wrapper
│   ├── install.sh             # System-wide install (sudo)
│   └── setup-global.sh        # One-command install (no sudo)
└── Makefile
```

---

## Architecture

```
cmd/agent/main.go          ← Entry point
    │
    ├── config.Load()      ← Viper: env vars + YAML + defaults
    ├── logger.Init()      ← Structured slog
    ├── provider.Get()     ← Factory + fallback chain
    ├── session.NewStore() ← JSON file persistence
    └── cache.New()        ← LRU + TTL
    │
    ├── [CLI mode] agent.New() → runOnce() → FrameWriter streaming
    └── [TUI mode] tui.New()   → tea.NewProgram() → Bubble Tea loop
```

**Clean architecture:** All interfaces in `internal/core/`, implementations depend only on interfaces.

---

## Development

```bash
make deps        # go mod tidy + verify
make build       # Build for current platform
make build-all   # Cross-compile Linux/macOS/Windows
make test        # Run all tests with race detector
make test-short  # Run tests without race
make lint        # go vet ./...
make run         # Build and run
make run-dev     # go run ./cmd/agent (hot reload)
make clean       # Remove build artifacts
```

### Tests

```bash
go test ./... -v -count=1 -race      # Full test suite with race detection
go test ./internal/cache/... -v      # Run specific package tests
```

**Test coverage:** 28 tests across cache, session, tool registry, TUI rendering, bash tool.

---

## Installation Options

| Method | Command | Location | Requires sudo |
|--------|---------|----------|---------------|
| One-command | `curl ... setup-global.sh \| bash` | `~/.local/bin` | No |
| Make target | `make setup` | `~/.local/bin` | No |
| System-wide | `make install` | `/usr/local/bin` | Yes |
| Manual | `go build -o ai-agent ./cmd/agent` | `./ai-agent` | No |

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "command not found: go" | Add `export PATH="$HOME/.local/bin:$PATH"` to shell config, restart shell |
| "no API key configured" | Export one of the API key env vars or edit `~/.config/agent/config.yaml` |
| "provider not found" | Run `go --list-providers` to see available; check config.yaml spelling |
| TUI rendering issues | Ensure terminal supports true color; try `export TERM=xterm-256color` |
| Go wrapper conflicts | Use `go build` (passthrough) vs `go "prompt"` (AI); wrapper detects subcommands |

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

## Contributing

1. Fork the repo
2. Create a feature branch
3. Run `make test` and `make lint`
4. Submit a PR

---

## Related

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Terminal styling
- [Viper](https://github.com/spf13/viper) — Config management