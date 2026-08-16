GO ?= go
GOPATH ?= $(shell $(GO) env GOPATH)
BIN_DIR ?= $(GOPATH)/bin

APP_NAME = agent
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_FLAGS = -ldflags="-s -w -X main.version=$(VERSION)"

.PHONY: all build clean test lint run install reinstall setup help dist-windows build-windows build-windows-arm64 build-linux build-darwin build-all deps test-short run-dev

all: clean test build

build:
	CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME) ./cmd/agent

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME)-linux-amd64 ./cmd/agent

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME)-darwin-amd64 ./cmd/agent

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME)-windows-amd64.exe ./cmd/agent
	@echo "Windows binary: $(APP_NAME)-windows-amd64.exe"
	@echo "On Windows 11 run:  powershell -ExecutionPolicy Bypass -File .\\scripts\\install.ps1 -SkipBuild"
	@echo "  (place the .exe next to the script, or copy to %LOCALAPPDATA%\\agent\\bin\\ai-agent.exe)"

build-windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME)-windows-arm64.exe ./cmd/agent

build-all: build-linux build-darwin build-windows build-windows-arm64

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
	@echo ""
	@echo "Windows 11: use  powershell -File .\\scripts\\install.ps1"
	@echo "  or: make build-windows   then copy the .exe to the target PC"

# Cross-compile a Windows binary from Linux/macOS and package install scripts.
dist-windows: build-windows
	@mkdir -p dist/windows
	@cp $(APP_NAME)-windows-amd64.exe dist/windows/ai-agent.exe
	@cp scripts/install.ps1 scripts/setup-windows.ps1 config.yaml dist/windows/
	@echo "Packaged dist/windows/ — copy that folder to a Windows 11 machine and run install.ps1 -SkipBuild"

clean:
	rm -f $(APP_NAME) $(APP_NAME)-linux-amd64 $(APP_NAME)-darwin-amd64 $(APP_NAME)-windows-amd64.exe $(APP_NAME)-windows-arm64.exe
	rm -rf dist/

deps:
	$(GO) mod tidy
	$(GO) mod verify

help:
	@echo "Usage:"
	@echo "  make build          - Build for current platform"
	@echo "  make build-all      - Cross-compile for Linux, macOS, Windows (amd64+arm64)"
	@echo "  make build-windows  - Cross-compile Windows amd64 .exe"
	@echo "  make dist-windows   - Package .exe + install.ps1 into dist/windows/"
	@echo "  make test           - Run all tests with race detection"
	@echo "  make lint           - Run go vet"
	@echo "  make run            - Build and run"
	@echo "  make install        - Install system-wide as \`go\` command (requires sudo, Unix)"
	@echo "  make setup          - Install to ~/.local/bin (Unix)"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make deps           - Tidy and verify dependencies"
	@echo ""
	@echo "Quick start (Linux/macOS):"
	@echo "  1. make setup"
	@echo "  2. Add the alias to your shell config (shown above)"
	@echo "  3. export GROQ_API_KEY=\"gsk_...\""
	@echo "  4. go what is docker"
	@echo ""
	@echo "Quick start (Windows 11):"
	@echo "  1. Install Go 1.22+ from https://go.dev/dl/"
	@echo "  2. powershell -ExecutionPolicy Bypass -File .\\scripts\\install.ps1"
	@echo "  3. Set NVIDIA_API_KEY / GROQ_API_KEY (User env var or config.yaml)"
	@echo "  4. ai-agent   or   go-agent \"what is docker\""

