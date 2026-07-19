GO ?= go
GOPATH ?= $(shell $(GO) env GOPATH)
BIN_DIR ?= $(GOPATH)/bin

APP_NAME = agent
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_FLAGS = -ldflags="-s -w -X main.version=$(VERSION)"

.PHONY: all build clean test lint run install reinstall setup help

all: clean test build

build:
	CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME) ./cmd/agent

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME)-linux-amd64 ./cmd/agent

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME)-darwin-amd64 ./cmd/agent

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME)-windows-amd64.exe ./cmd/agent

build-all: build-linux build-darwin build-windows

test:
	$(GO) test ./... -v -count=1 -race

test-short:
	$(GO) test ./... -short -count=1

lint:
	$(GO) vet ./...

run: build
	CGO_ENABLED=0 ./$(APP_NAME)

run-dev:
	CGO_ENABLED=0 $(GO) run ./cmd/agent

# Install the AI agent as the system `go` command
install:
	@sudo ./scripts/install.sh

# Rebuild and refresh the installed launcher binary in one step. Prevents the
# build and the /usr/local/bin/ai-agent launcher from drifting apart.
LAUNCHER ?= /usr/local/bin/ai-agent
reinstall: build
	@echo "==> Installing $(APP_NAME) to $(LAUNCHER)"
	@sudo install -m 755 $(APP_NAME) "$(LAUNCHER)"
	@echo "==> Done. 'go \"prompt\"' now uses the fresh build."

# Quick setup without sudo (installs to ~/.local/bin)
setup:
	@echo "==> Building AI agent..."
	CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o $(APP_NAME) ./cmd/agent
	@mkdir -p "$(HOME)/.local/bin" "$(HOME)/.config/agent"
	@install -m 755 $(APP_NAME) "$(HOME)/.local/bin/ai-agent"
	@if [ ! -f "$(HOME)/.config/agent/config.yaml" ]; then \
		cp config.yaml "$(HOME)/.config/agent/config.yaml"; \
		echo "Created $$HOME/.config/agent/config.yaml"; \
	fi
	@echo ""
	@echo "=== AI Agent installed ==="
	@echo ""
	@echo "Add to your ~/.bashrc or ~/.zshrc:"
	@echo "  export PATH=\"\$$HOME/.local/bin:\$$PATH\""
	@echo ""
	@echo "Set at least one API key (choose one):"
	@echo "  export NVIDIA_API_KEY=\"nvapi-...\"    # Free, recommended"
	@echo "  export GROQ_API_KEY=\"gsk_...\"         # Free tier"
	@echo "  export OPENROUTER_API_KEY=\"sk-or-...\" # Access to Claude, GPT-4o"
	@echo ""
	@echo "Usage:"
	@echo "  go what is docker        AI assistant query"
	@echo "  go build ./...           Real Go compiler (passthrough)"
	@echo "  go                       Launch interactive TUI"
	@echo "  go -h                    Show all options"

clean:
	rm -f $(APP_NAME) $(APP_NAME)-linux-amd64 $(APP_NAME)-darwin-amd64 $(APP_NAME)-windows-amd64.exe

deps:
	$(GO) mod tidy
	$(GO) mod verify

help:
	@echo "Usage:"
	@echo "  make build       - Build for current platform"
	@echo "  make build-all   - Cross-compile for Linux, macOS, Windows"
	@echo "  make test        - Run all tests with race detection"
	@echo "  make lint        - Run go vet"
	@echo "  make run         - Build and run"
	@echo "  make install     - Install system-wide as \`go\` command (requires sudo)"
	@echo "  make setup       - Install to ~/.local/bin with alias instructions"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make deps        - Tidy and verify dependencies"
	@echo ""
	@echo "Quick start:"
	@echo "  1. make setup"
	@echo "  2. Add the alias to your shell config (shown above)"
	@echo "  3. export GROQ_API_KEY=\"gsk_...\""
	@echo "  4. go what is docker"
