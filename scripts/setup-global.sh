#!/usr/bin/env bash
set -euo pipefail

REPO="https://github.com/yesk993-ops/go-terminal-agent.git"
INSTALL_DIR="/opt/ai-agent"
AGENT_BIN="/usr/local/bin/ai-agent"
GO_WRAPPER="/usr/local/bin/go"
CONFIG_DIR="${HOME}/.config/agent"
GO_VERSION="1.26.2"

info()  { printf "\033[36m==>\033[0m %s\n" "$1"; }
ok()    { printf "\033[32m  OK\033[0m  %s\n" "$1"; }
warn()  { printf "\033[33m WARN\033[0m  %s\n" "$1"; }
err()   { printf "\033[31mFAIL\033[0m  %s\n" "$1"; exit 1; }

if [ "$EUID" -eq 0 ]; then
  SUDO=""
else
  SUDO="sudo"
fi

detect_go() {
  if command -v go &>/dev/null; then
    GO_EXEC=$(command -v go)
    info "Found Go at $GO_EXEC ($(go version | awk '{print $3}'))"
    return 0
  fi
  for candidate in /usr/lib/go-*/bin/go /usr/local/go/bin/go /opt/go/bin/go; do
    if [ -x "$candidate" ]; then
      info "Found Go at $candidate"
      export PATH="$PATH:$(dirname "$candidate")"
      return 0
    fi
  done
  return 1
}

install_go() {
  info "Go not found. Installing Go $GO_VERSION..."
  local arch
  arch=$(uname -m)
  case "$arch" in
    x86_64)  arch="amd64" ;;
    aarch64) arch="arm64" ;;
    *)       err "Unsupported architecture: $arch" ;;
  esac

  local tarball="go${GO_VERSION}.linux-${arch}.tar.gz"
  local url="https://go.dev/dl/${tarball}"

  if command -v wget &>/dev/null; then
    wget -q --show-progress "$url" -O "/tmp/$tarball"
  elif command -v curl &>/dev/null; then
    curl -sL "$url" -o "/tmp/$tarball"
  else
    err "neither wget nor curl found — install one of them first"
  fi

  $SUDO rm -rf /usr/local/go
  $SUDO tar -C /usr/local -xzf "/tmp/$tarball"
  rm -f "/tmp/$tarball"
  export PATH="/usr/local/go/bin:$PATH"
  ok "Go $GO_VERSION installed at /usr/local/go"
}

install_dependencies() {
  info "Installing system dependencies..."
  if command -v apt-get &>/dev/null; then
    $SUDO apt-get update -qq
    $SUDO apt-get install -y -qq git curl ca-certificates
  elif command -v yum &>/dev/null; then
    $SUDO yum install -y git curl ca-certificates
  elif command -v apk &>/dev/null; then
    $SUDO apk add --no-cache git curl ca-certificates
  else
    warn "Unknown package manager — skipping system deps. Ensure git is available."
  fi
  ok "System dependencies installed"
}

clone_repo() {
  info "Fetching AI agent from $REPO..."
  if [ -d "$INSTALL_DIR/.git" ]; then
    info "Updating existing installation..."
    cd "$INSTALL_DIR"
    git pull
  else
    $SUDO rm -rf "$INSTALL_DIR"
    git clone "$REPO" /tmp/ai-agent-clone
    $SUDO mkdir -p "$INSTALL_DIR"
    $SUDO cp -r /tmp/ai-agent-clone/* "$INSTALL_DIR/"
    $SUDO cp /tmp/ai-agent-clone/.* "$INSTALL_DIR/" 2>/dev/null || true
    rm -rf /tmp/ai-agent-clone
  fi
  ok "Agent source at $INSTALL_DIR"
}

build_agent() {
  info "Building AI agent..."
  cd "$INSTALL_DIR"
  go build -ldflags="-s -w" -o ai-agent ./cmd/agent
  $SUDO install -m 755 ai-agent "$AGENT_BIN"
  ok "Binary installed at $AGENT_BIN"
}

install_wrapper() {
  info "Installing system go wrapper..."
  local real_go
  real_go=$(command -v go)

  sed "s|REAL_GO=\"/usr/lib/go-1.26/bin/go\"|REAL_GO=\"$real_go\"|" \
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
    warn "Edit $CONFIG_DIR/config.yaml to add API keys"
  else
    ok "Existing config found at $CONFIG_DIR/config.yaml"
  fi
}

cleanup() {
  info "Cleaning up..."
  cd "$INSTALL_DIR"
  rm -f ai-agent
  ok "Done"
}

main() {
  printf "\033[35m========================================\033[0m\n"
  printf "\033[35m  Go Terminal AI Agent — Setup\033[0m\n"
  printf "\033[35m========================================\033[0m\n\n"

  install_dependencies
  if ! detect_go; then
    install_go
  fi
  clone_repo
  build_agent
  install_wrapper
  setup_config

  printf "\n\033[32m=== Installation complete ===\033[0m\n"
  printf "\n"
  printf "  \033[36mgo\033[0m                    Launch interactive TUI\n"
  printf "  \033[36mgo what is docker\033[0m      Ask a question\n"
  printf "  \033[36mgo build ./...\033[0m          Uses real Go compiler\n"
  printf "\n"
  printf "  Set API keys in: \033[33m%s/config.yaml\033[0m\n" "$CONFIG_DIR"
  printf "  or via env vars: \033[33mexport NVIDIA_API_KEY=\"nvapi-...\"\033[0m\n"
  printf "\n"
  printf "  Reload shell: \033[33msource ~/.bashrc\033[0m\n"
  printf "\n"
}

main
