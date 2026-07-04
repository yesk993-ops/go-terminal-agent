#!/usr/bin/env bash
# Install the AI agent as the system `go` command.
# Preserves access to the real Go compiler for build commands.
#
# Usage: sudo ./scripts/install.sh
#   or:  make install

set -e

PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
AGENT_BIN="/usr/local/bin/ai-agent"
GO_WRAPPER="/usr/local/bin/go"
CONFIG_DIR="${HOME}/.config/agent"
REAL_GO="${REAL_GO:-/usr/bin/go}"

echo "==> Building AI agent..."
cd "$PROJECT_DIR"
go build -ldflags="-s -w" -o ai-agent ./cmd/agent
echo "==> Installing ai-agent binary to $AGENT_BIN..."
install -m 755 ai-agent "$AGENT_BIN"
rm -f ai-agent

echo "==> Installing go wrapper to $GO_WRAPPER..."
sed "s|REAL_GO=\"/usr/lib/go-1.26/bin/go\"|REAL_GO=\"$REAL_GO\"|" \
    "$PROJECT_DIR/scripts/go-wrapper" > /tmp/go-wrapper-install
install -m 755 /tmp/go-wrapper-install "$GO_WRAPPER"
rm -f /tmp/go-wrapper-install

echo "==> Setting up config at $CONFIG_DIR..."
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    cp "$PROJECT_DIR/config.yaml" "$CONFIG_DIR/config.yaml"
    echo "Created default config (edit $CONFIG_DIR/config.yaml to set API keys)"
fi

echo ""
echo "=== Installation complete ==="
echo ""
echo "  Usage:"
echo "    go                    — Launch interactive TUI"
echo "    go what is docker      — Ask a question"
echo "    go build ./...         — Uses real Go compiler (passthrough)"
echo "    go run main.go         — Uses real Go compiler"
echo ""
echo "  Set your API keys in $CONFIG_DIR/config.yaml"
echo "  or via environment variables:"
echo "    export GROQ_API_KEY=\"gsk_...\""
echo "    export OPENAI_API_KEY=\"sk-...\""
echo ""
