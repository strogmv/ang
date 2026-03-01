#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# shellcheck disable=SC1091
source "$ROOT_DIR/scripts/lib/ui.sh"

ui::section "Dev Up"
ui::info "running preflight checks"
bash scripts/preflight.sh

ui::info "starting local stack via ang up"
ang up --skip-doctor "$@"
ui::ok "local stack started"
