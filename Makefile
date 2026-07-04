GO ?= go
GOPATH ?= $(shell $(GO) env GOPATH)
BIN_DIR ?= $(GOPATH)/bin

APP_NAME = agent
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_FLAGS = -ldflags="-s -w -X main.version=$(VERSION)"

.PHONY: all build clean test lint run install setup help

all: clean test build

build:
	$(GO) build $(BUILD_FLAGS) -o $(APP_NAME) ./cmd/agent

build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME)-linux-amd64 ./cmd/agent

build-darwin:
	GOOS=darwin GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME)-darwin-amd64 ./cmd/agent

build-windows:
	GOOS=windows GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -o $(APP_NAME)-windows-amd64.exe ./cmd/agent

build-all: build-linux build-darwin build-windows

test:
	$(GO) test ./... -v -count=1 -race

test-short:
	$(GO) test ./... -short -count=1

lint:
	$(GO) vet ./...

run: build
	./$(APP_NAME)

run-dev:
	$(GO) run ./cmd/agent

# Install the AI agent as the system `go` command
install:
	@sudo ./scripts/install.sh

# Quick setup without sudo (installs to ~/.local/bin)
setup:
	@echo "==> Building AI agent..."
	$(GO) build -ldflags="-s -w" -o $(APP_NAME) ./cmd/agent
	@mkdir -p "$(HOME)/.local/bin" "$(HOME)/.config/agent"
	@install -m 755 $(APP_NAME) "$(HOME)/.local/bin/ai-agent"
	@if [ ! -f "$(HOME)/.config/agent/config.yaml" ]; then \
		cp config.yaml "$(HOME)/.config/agent/config.yaml"; \
		echo "Created $$HOME/.config/agent/config.yaml"; \
	fi
	@echo ""
	@echo "Add to your ~/.bashrc or ~/.zshrc:"
	@echo "  export PATH=\"\$$HOME/.local/bin:\$$PATH\""
	@echo ""
	@echo "Then create an alias in ~/.bash_aliases or ~/.zshrc:"
	@echo "  go() {"
	@echo "    case \$$1 in"
@echo "      build|run|test|mod|get|install|clean|vet|fmt|doc|env|version|help|work|tool|list|generate|fix|cover)"
		@echo "        go \"\$$@\""  # forward to real Go
	@echo "        ;;"
	@echo "      *)"
	@echo '        ai-agent --config "$$HOME/.config/agent/config.yaml" "$$@"'  # AI mode
	@echo "        ;;"
	@echo "    esac"
	@echo "  }"
	@echo ""
	@echo "Reload your shell: source ~/.bashrc"

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
