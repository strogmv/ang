#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# shellcheck disable=SC1091
source "$ROOT_DIR/scripts/lib/ui.sh"

MISSING=()

is_missing() {
  local cmd="$1"
  ! command -v "$cmd" >/dev/null 2>&1
}

detect_missing() {
  MISSING=()

  if is_missing ang; then MISSING+=(ang); fi
  if is_missing go; then MISSING+=(go); fi
  if is_missing docker; then MISSING+=(docker); fi
  if is_missing atlas; then MISSING+=(atlas); fi
  if ! docker compose version >/dev/null 2>&1 && ! command -v docker-compose >/dev/null 2>&1; then
    MISSING+=(docker-compose)
  fi
}

run_cmd() {
  ui::info "$*"
  "$@"
}

install_ubuntu() {
  local tool="$1"
  case "$tool" in
    go)
      run_cmd sudo apt-get update
      run_cmd sudo apt-get install -y golang-go
      ;;
    docker)
      run_cmd sudo apt-get update
      run_cmd sudo apt-get install -y docker.io docker-compose-plugin
      ;;
    docker-compose)
      run_cmd sudo apt-get update
      run_cmd sudo apt-get install -y docker-compose-plugin
      ;;
    atlas)
      run_cmd bash -lc 'curl -sSf https://atlasgo.sh | sh'
      ;;
    ang)
      if ! command -v go >/dev/null 2>&1; then
        ui::warn "go is required to install ang; installing go first"
        install_ubuntu go
      fi
      run_cmd go install github.com/strogmv/ang/cmd/ang@latest
      ;;
  esac
}

install_macos() {
  local tool="$1"
  if ! command -v brew >/dev/null 2>&1; then
    ui::fail "homebrew is required on macOS"
    ui::action "install Homebrew: https://brew.sh"
    exit 1
  fi

  case "$tool" in
    go)
      run_cmd brew install go
      ;;
    docker)
      run_cmd brew install --cask docker
      ;;
    docker-compose)
      run_cmd brew install docker-compose
      ;;
    atlas)
      run_cmd brew install ariga/tap/atlas
      ;;
    ang)
      if ! command -v go >/dev/null 2>&1; then
        ui::warn "go is required to install ang; installing go first"
        install_macos go
      fi
      run_cmd go install github.com/strogmv/ang/cmd/ang@latest
      ;;
  esac
}

OS="$(uname -s)"

ui::section "Bootstrap: scan"
detect_missing

if ((${#MISSING[@]} == 0)); then
  ui::ok "all required tools are already installed"
  exit 0
fi

ui::warn "missing tools: ${MISSING[*]}"

if [[ "${AUTO_CONFIRM:-0}" != "1" ]]; then
  read -r -p "Install missing tools automatically where possible? [y/N] " answer
  case "${answer:-}" in
    y|Y|yes|YES)
      ui::info "continuing with installation"
      ;;
    *)
      ui::warn "bootstrap cancelled by user"
      exit 1
      ;;
  esac
fi

ui::section "Bootstrap: install"
for tool in "${MISSING[@]}"; do
  ui::info "installing: $tool"
  case "$OS" in
    Linux)
      if [[ -f /etc/os-release ]] && grep -Eqi "ubuntu|debian" /etc/os-release; then
        install_ubuntu "$tool"
      else
        ui::fail "unsupported Linux distro for auto-install: $tool"
        ui::action "install $tool manually, then rerun make bootstrap"
        exit 1
      fi
      ;;
    Darwin)
      install_macos "$tool"
      ;;
    *)
      ui::fail "unsupported OS for auto-install: $OS"
      ui::action "install tools manually: ${MISSING[*]}"
      exit 1
      ;;
  esac
done

ui::section "Bootstrap: verify"
detect_missing
if ((${#MISSING[@]} > 0)); then
  ui::fail "some tools are still missing: ${MISSING[*]}"
  ui::action "install remaining tools manually and rerun make bootstrap"
  exit 1
fi

ui::ok "bootstrap complete"
ui::action "next step: make doctor"
