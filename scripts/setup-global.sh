#!/usr/bin/env bash
set -euo pipefail

REPO="https://github.com/yesk993-ops/go-terminal-agent.git"
INSTALL_DIR="/opt/ai-agent"
AGENT_BIN="/usr/local/bin/ai-agent"
GO_WRAPPER="/usr/local/bin/go"
CONFIG_DIR="${HOME}/.config/agent"

info()  { printf "\033[36m==>\033[0m %s\n" "$1"; }
ok()    { printf "\033[32m  OK\033[0m  %s\n" "$1"; }
warn()  { printf "\033[33m WARN\033[0m  %s\n" "$1"; }
err()   { printf "\033[31mFAIL\033[0m  %s\n" "$1"; exit 1; }

if [ "$EUID" -eq 0 ]; then SUDO=""; else SUDO="sudo"; fi

install_deps() {
  info "Installing system dependencies..."
  if command -v apt-get &>/dev/null; then
    $SUDO apt-get update -qq && $SUDO apt-get install -y -qq git curl ca-certificates
  elif command -v yum &>/dev/null; then
    $SUDO yum install -y git curl ca-certificates
  elif command -v apk &>/dev/null; then
    $SUDO apk add --no-cache git curl ca-certificates
  else
    warn "Unknown package manager — ensure git and curl are installed."
  fi
  ok "System dependencies installed"
}

check_go() {
  if command -v go &>/dev/null; then
    info "Found Go: $(go version)"
    return 0
  fi
  err "Go not found. Install Go 1.22+ from https://go.dev/dl/"
}

clone_repo() {
  info "Fetching AI agent..."
  $SUDO rm -rf "$INSTALL_DIR"
  $SUDO git clone "$REPO" "$INSTALL_DIR"
  $SUDO chown -R "$(whoami):$(id -gn)" "$INSTALL_DIR"
  ok "Agent source at $INSTALL_DIR"
}

build_agent() {
  info "Building AI agent (CGO_ENABLED=0)..."
  cd "$INSTALL_DIR"
  CGO_ENABLED=0 go build -ldflags="-s -w" -o ai-agent ./cmd/agent
  $SUDO install -m 755 ai-agent "$AGENT_BIN"
  rm -f ai-agent
  ok "Binary installed at $AGENT_BIN"
}

install_wrapper() {
  info "Installing go wrapper..."
  local real_go
  real_go=$(command -v go || true)
  if [ -z "$real_go" ] || [ "$real_go" = "$GO_WRAPPER" ]; then
    for d in /usr/lib/go*/bin /usr/local/go/bin /usr/local/lib/go*/bin; do
      for f in "$d/go"; do
        if [ -x "$f" ] && [ "$f" != "$GO_WRAPPER" ]; then
          real_go="$f"
          break 2
        fi
      done
    done
  fi
  if [ -z "$real_go" ] || [ "$real_go" = "$GO_WRAPPER" ]; then
    real_go="/usr/bin/go"
  fi
  info "Real Go compiler: $real_go"
  sed "s|REAL_GO=\"/usr/bin/go\"|REAL_GO=\"$real_go\"|" \
    "$INSTALL_DIR/scripts/go-wrapper" > /tmp/go-wrapper-install
  $SUDO install -m 755 /tmp/go-wrapper-install "$GO_WRAPPER"
  rm -f /tmp/go-wrapper-install
  ok "go wrapper installed at $GO_WRAPPER"
}

setup_config() {
  info "Setting up config..."
  mkdir -p "$CONFIG_DIR"
  if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    cp "$INSTALL_DIR/config.yaml" "$CONFIG_DIR/config.yaml"
    ok "Default config created at $CONFIG_DIR/config.yaml"
    warn "Edit $CONFIG_DIR/config.yaml to add API keys, then run: go"
  else
    ok "Existing config found at $CONFIG_DIR/config.yaml"
  fi
}

main() {
  printf "\033[35m========================================\033[0m\n"
  printf "\033[35m  Go Terminal AI Agent — Setup\033[0m\n"
  printf "\033[35m========================================\033[0m\n\n"

  install_deps
  check_go
  clone_repo
  build_agent
  install_wrapper
  setup_config

  printf "\n\033[32m=== Installation complete ===\033[0m\n"
  printf "\n"
  printf "  \033[36mgo\033[0m                    Launch interactive TUI\n"
  printf "  \033[36mgo what is docker\033[0m      Ask a question\n"
  printf "  \033[36mgo build ./...\033[0m          Real Go compiler passthrough\n"
  printf "\n"
  printf "  Set API keys in: \033[33m%s/config.yaml\033[0m\n" "$CONFIG_DIR"
  printf "\n"
}

main