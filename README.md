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
| **Session persistence** | Chat history auto-saves across restarts (platform-specific data dir) |
| **Response caching** | LRU cache with TTL (default: 5 min, 500 entries) for repeated queries |
| **Fallback provider chain** | Auto-retry with backoff, then failover to next provider on rate limits |
| **`go` wrapper** | Unix: system-wide alias `go "prompt"` runs AI, `go build` → real Go |
| **Cross-platform** | Linux, macOS, **Windows 11** (amd64 + arm64) |

---

## Quick Install

### Windows 11 (PowerShell)

**Prerequisites:** [Go 1.22+](https://go.dev/dl/) and [Git](https://git-scm.com/download/win) installed.

```powershell
# From a cloned repo:
git clone https://github.com/yesk993-ops/go-terminal-agent.git
cd go-terminal-agent
powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1

# Or one-liner (clones + builds + installs):
irm https://raw.githubusercontent.com/yesk993-ops/go-terminal-agent/master/scripts/setup-windows.ps1 | iex
```

Then open a **new** PowerShell / Windows Terminal window and run:

```powershell
ai-agent                        # interactive TUI
ai-agent "what is docker"       # one-shot prompt
go-agent "explain goroutines"   # friendly alias
```

| Windows path | Purpose |
|--------------|---------|
| `%LOCALAPPDATA%\agent\bin\ai-agent.exe` | Binary |
| `%LOCALAPPDATA%\agent\bin\go-agent.cmd` | Launcher alias |
| `%APPDATA%\agent\config.yaml` | Config |
| `%APPDATA%\agent\agent.env` | Optional `KEY=value` API keys file |
| `%LOCALAPPDATA%\agent\sessions\` | Saved chat sessions |

Set an API key (pick one):

```powershell
# Current session only:
$env:NVIDIA_API_KEY = "nvapi-..."
$env:GROQ_API_KEY   = "gsk_..."

# Persist for all future sessions:
[System.Environment]::SetEnvironmentVariable("NVIDIA_API_KEY", "nvapi-...", "User")

# Or create %APPDATA%\agent\agent.env:
#   NVIDIA_API_KEY=nvapi-...
```

> **Note:** On Windows the assistant is `ai-agent` / `go-agent` so it never shadows the real `go` compiler.

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

# Linux / macOS
make setup          # Builds and installs to ~/.local/bin with config
# or: make build     # Just builds binary as ./agent

# Cross-compile a Windows .exe from Linux/macOS
make build-windows          # → agent-windows-amd64.exe
make dist-windows           # packages .exe + install.ps1 into dist/windows/
```

### Windows (pre-built binary, no Go on the PC)

1. On any machine with Go: `make dist-windows`
2. Copy the `dist/windows/` folder to the Windows 11 PC
3. In that folder run:
   ```powershell
   powershell -ExecutionPolicy Bypass -File .\install.ps1 -SkipBuild
   ```

---

## Configuration

### Config locations

| OS | Config file | Sessions |
|----|-------------|----------|
| **Windows** | `%APPDATA%\agent\config.yaml` | `%LOCALAPPDATA%\agent\sessions\` |
| **Linux/macOS** | `~/.config/agent/config.yaml` | `~/.local/share/agent/` |

### API Keys (choose at least one)

**Linux/macOS:**

```bash
export NVIDIA_API_KEY="nvapi-..."    # Free, reliable (default)
export GROQ_API_KEY="gsk_..."        # Free tier
export OPENAI_API_KEY="sk-..."       # Paid
export ANTHROPIC_API_KEY="sk-ant-..." # Paid
export GEMINI_API_KEY="AIza..."      # Free tier
export OPENROUTER_API_KEY="sk-or-..." # Paid (access to all models)
```

**Windows PowerShell:**

```powershell
$env:NVIDIA_API_KEY = "nvapi-..."
# Persist:
[System.Environment]::SetEnvironmentVariable("NVIDIA_API_KEY", "nvapi-...", "User")
```

Or edit the config file, or create `agent.env` next to it (`KEY=value` lines).

### Config File example

```yaml
provider:
  default: nvidia
  max_tokens: 8192
  temperature: 0.7

providers:
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "gpt-4o"
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
  # save_path is auto-detected per OS when omitted

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
# Linux/macOS (after setup with go-wrapper)
go

# Windows 11
ai-agent
# or
go-agent
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
# Linux/macOS
go "explain goroutines in Go"
go -provider groq -model llama-3.3-70b-versatile "write a http server in go"
go --list-providers

# Windows
ai-agent "explain goroutines in Go"
ai-agent -provider groq "write a http server in go"
ai-agent --list-providers
```

### Real Go Compiler Passthrough (Linux/macOS only)

On Unix the `go` wrapper forwards real Go subcommands:

```bash
go build ./...
go test ./...
go mod tidy
```

On Windows use the real `go` from [go.dev](https://go.dev/dl/) for builds, and `ai-agent` / `go-agent` for the AI assistant.

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
| `bash` | Execute shell commands (`sh` on Unix, `cmd.exe` / PowerShell on Windows; destructive commands blocked) |
| `web_search` | DuckDuckGo instant answer API search |
| `web_fetch` | Fetch and extract web page content |

> **Note:** Tools are only available in CLI mode (`ai-agent "prompt"` / `go "prompt"`), not in the interactive TUI.

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
│   ├── plugin/                # Plugin loader (.so on Unix; no-op on Windows)
│   │   ├── plugin.go
│   │   ├── plugin_unix.go
│   │   └── plugin_windows.go
│   ├── provider/              # LLM providers + fallback chain
│   │   ├── provider.go        # Registry, base provider, SSE streaming
│   │   ├── fallback.go        # Retry + fallback chain
│   │   ├── gemini.go          # Google Gemini
│   │   ├── groq.go            # Groq (OpenAI-compatible)
│   │   ├── nvidia.go          # NVIDIA (OpenAI-compatible)
│   │   ├── openai.go          # OpenAI
│   │   └── openrouter.go      # OpenRouter
│   ├── session/session.go     # JSON file persistence with auto-save
│   ├── tool/                  # 8 tool implementations
│   │   ├── bash.go            # Shell exec (sh / cmd.exe, destructive blocked)
│   │   ├── edit.go            # File edit tool
│   │   ├── glob.go            # Glob pattern search
│   │   ├── grep.go            # Regex content search
│   │   ├── read.go            # File read tool
│   │   ├── registry.go        # Tool registry + JSON schema
│   │   ├── safe_path.go       # Cross-platform path sandbox
│   │   ├── webfetch.go        # HTTP fetch tool
│   │   ├── websearch.go       # DuckDuckGo search tool
│   │   └── write.go           # File write tool
│   ├── config/
│   │   ├── config.go          # Viper config loader
│   │   ├── paths.go           # OS-specific config/session paths
│   │   └── envfile.go         # bashrc / PowerShell / agent.env key loading
│   └── tui/                   # Bubble Tea TUI
│       ├── tui.go             # Main TUI model & commands
│       ├── markdown.go        # Message styling
│       ├── render.go          # CLI frame writer (markdown, word-wrap)
│       └── styles.go          # Lip Gloss styles
├── config.yaml                # Example config
├── scripts/
│   ├── go-wrapper             # Smart `go` command wrapper (Unix)
│   ├── install.sh             # System-wide install (Unix, sudo)
│   ├── setup-global.sh        # One-command install (Unix, no sudo)
│   ├── install.ps1            # Windows 11 installer
│   └── setup-windows.ps1      # Windows 11 one-command setup
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

| Platform | Method | Command | Location |
|----------|--------|---------|----------|
| **Windows 11** | PowerShell install | `.\scripts\install.ps1` | `%LOCALAPPDATA%\agent\bin` |
| **Windows 11** | One-liner | `irm .../setup-windows.ps1 \| iex` | `%LOCALAPPDATA%\agent\bin` |
| **Windows 11** | Pre-built | `make dist-windows` then `install.ps1 -SkipBuild` | `%LOCALAPPDATA%\agent\bin` |
| Linux/macOS | One-command | `curl ... setup-global.sh \| bash` | `~/.local/bin` |
| Linux/macOS | Make target | `make setup` | `~/.local/bin` |
| Linux/macOS | System-wide | `make install` | `/usr/local/bin` |
| Any | Manual | `go build -o ai-agent(.exe) ./cmd/agent` | `./` |

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `"ai-agent" not recognized` (Windows) | Open a **new** terminal after install so PATH updates apply; or run `%LOCALAPPDATA%\agent\bin\ai-agent.exe` directly |
| "command not found: go" (Unix wrapper) | Add `export PATH="$HOME/.local/bin:$PATH"` to shell config, restart shell |
| "no API key configured" | Set an env var, edit config.yaml, or create `agent.env` (see Configuration) |
| "provider not found" | Run `ai-agent --list-providers` (or `go --list-providers`); check config.yaml spelling |
| TUI rendering issues | Use Windows Terminal (not legacy `conhost`); enable UTF-8 / true color |
| PowerShell execution policy | Run with `-ExecutionPolicy Bypass -File .\scripts\install.ps1` |
| Go wrapper conflicts (Unix) | Use `go build` (passthrough) vs `go "prompt"` (AI); wrapper detects subcommands |
| Want real `go` on Windows | Install from https://go.dev/dl/ — the agent uses `ai-agent`/`go-agent` and does not shadow `go` |

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