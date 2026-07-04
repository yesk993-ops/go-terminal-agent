#!/usr/bin/env bash
set -euo pipefail

REPO="https://github.com/yesk993-ops/go-terminal-agent.git"
CLONE_DIR="/tmp/ai-agent-install"
BIN_DIR="${HOME}/.local/bin"
CONFIG_DIR="${HOME}/.config/agent"
AGENT_BIN="${BIN_DIR}/ai-agent"
GO_WRAPPER="${BIN_DIR}/go"

info()  { printf "\033[36m==>\033[0m %s\n" "$1"; }
ok()    { printf "\033[32m  OK\033[0m  %s\n" "$1"; }
warn()  { printf "\033[33m WARN\033[0m  %s\n" "$1"; }
err()   { printf "\033[31mFAIL\033[0m  %s\n" "$1"; exit 1; }

check_go() {
  if command -v go &>/dev/null; then
    info "Found Go: $(go version)"
    return 0
  fi
  err "Go not found. Install Go 1.22+ from https://go.dev/dl/"
}

clone_repo() {
  info "Fetching AI agent..."
  rm -rf "$CLONE_DIR"
  git clone --depth 1 "$REPO" "$CLONE_DIR"
  ok "Source cloned to $CLONE_DIR"
}

build_agent() {
  info "Building AI agent (CGO_ENABLED=0)..."
  mkdir -p "$BIN_DIR"
  cd "$CLONE_DIR"
  CGO_ENABLED=0 go build -ldflags="-s -w" -o "$AGENT_BIN" ./cmd/agent
  ok "Binary installed at $AGENT_BIN"
}

install_wrapper() {
  info "Installing go wrapper..."
  local real_go
  real_go=$(command -v go || echo "/usr/bin/go")
  sed "s|REAL_GO=\"/usr/bin/go\"|REAL_GO=\"$real_go\"|" \
    "$CLONE_DIR/scripts/go-wrapper" > /tmp/go-wrapper-install
  install -m 755 /tmp/go-wrapper-install "$GO_WRAPPER"
  rm -f /tmp/go-wrapper-install
  ok "go wrapper installed at $GO_WRAPPER"
}

setup_config() {
  info "Setting up config..."
  mkdir -p "$CONFIG_DIR"
  if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
    cp "$CLONE_DIR/config.yaml" "$CONFIG_DIR/config.yaml"
    ok "Default config created at $CONFIG_DIR/config.yaml"
    warn "Edit $CONFIG_DIR/config.yaml to add API keys"
  else
    ok "Existing config found at $CONFIG_DIR/config.yaml"
  fi
}

cleanup() {
  rm -rf "$CLONE_DIR"
}

main() {
  printf "\033[35m========================================\033[0m\n"
  printf "\033[35m  Go Terminal AI Agent — Setup\033[0m\n"
  printf "\033[35m========================================\033[0m\n\n"

  check_go
  clone_repo
  build_agent
  install_wrapper
  setup_config
  cleanup

  printf "\n\033[32m=== Installation complete ===\033[0m\n"
  printf "\n"
  printf "  Add to your shell config (~/.bashrc / ~/.zshrc):\n"
  printf "    \033[33mexport PATH=\"\033[36m\$HOME/.local/bin\033[33m:\$PATH\"\033[0m\n"
  printf "\n"
  printf "  Then run:\n"
  printf "    \033[36mgo\033[0m                    Launch interactive TUI\n"
  printf "    \033[36mgo what is docker\033[0m      Ask a question\n"
  printf "    \033[36mgo build ./...\033[0m          Real Go compiler passthrough\n"
  printf "\n"
  printf "  Set at least one API key (choose one):\n"
  printf "    \033[33mexport NVIDIA_API_KEY=\"\033[36mnvapi-...\033[33m\"\033[0m    # free, reliable\n"
  printf "    \033[33mexport GROQ_API_KEY=\"\033[36mgsk_...\033[33m\"\033[0m          # free tier\n"
  printf "    \033[33mexport OPENAI_API_KEY=\"\033[36msk-...\033[33m\"\033[0m        # paid\n"
  printf "    \033[33mexport ANTHROPIC_API_KEY=\"\033[36msk-ant-...\033[33m\"\033[0m  # paid\n"
  printf "    \033[33mexport GEMINI_API_KEY=\"\033[36mAIza...\033[33m\"\033[0m        # free tier\n"
  printf "    \033[33mexport OPENROUTER_API_KEY=\"\033[36msk-or-...\033[33m\"\033[0m  # paid\n"
  printf "\n"
  printf "  Or edit config:\n"
  printf "    \033[33m%s/config.yaml\033[0m\n" "$CONFIG_DIR"
  printf "\n"
}

main