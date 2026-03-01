#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# shellcheck disable=SC1091
source "$ROOT_DIR/scripts/lib/ui.sh"

ui::section "Doctor: preflight"
bash scripts/preflight.sh

ui::section "Doctor: ANG checks"
if ang doctor start "$@"; then
  ui::ok "ang doctor passed"
else
  ui::fail "ang doctor reported issues"
  ui::action "apply fixes from output above and rerun make doctor"
  exit 1
fi

ui::ok "doctor completed"
