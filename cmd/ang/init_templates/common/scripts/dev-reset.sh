#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# shellcheck disable=SC1091
source "$ROOT_DIR/scripts/lib/ui.sh"

ui::section "Dev Reset"
ui::info "dropping local state"
docker compose down -v

ui::info "restarting stack"
ang up --skip-doctor
ui::ok "local stack reset complete"
