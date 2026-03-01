#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# shellcheck disable=SC1091
source "$ROOT_DIR/scripts/lib/ui.sh"

missing_tools=0
warnings=()

check_tool() {
  local tool="$1"
  if ! command -v "$tool" >/dev/null 2>&1; then
    ui::fail "required tool missing: $tool"
    missing_tools=1
  else
    ui::ok "tool available: $tool"
  fi
}

ui::section "Preflight: required tools"
check_tool ang
check_tool go
check_tool docker
check_tool atlas

if docker compose version >/dev/null 2>&1 || command -v docker-compose >/dev/null 2>&1; then
  ui::ok "tool available: docker compose"
else
  ui::fail "required tool missing: docker compose (plugin or docker-compose binary)"
  missing_tools=1
fi

if [[ "$missing_tools" -ne 0 ]]; then
  ui::summary "Preflight Result" 1 0
  ui::fail "preflight failed: missing required tools"
  exit 1
fi

ui::section "Preflight: environment"
if [[ ! -f .env && -f .env.example ]]; then
  cp .env.example .env
  ui::info "created .env from .env.example"
fi

if [[ -f .env && -f .env.example ]]; then
  while IFS= read -r line; do
    [[ -z "${line// }" ]] && continue
    [[ "${line:0:1}" == "#" ]] && continue
    key="${line%%=*}"
    val="${line#*=}"
    [[ -z "${key// }" ]] && continue

    if grep -Eq "^${key}=" .env; then
      current="$(grep -E "^${key}=" .env | tail -n 1 | cut -d'=' -f2- || true)"
      if [[ -z "${current// }" && -n "${val// }" ]]; then
        echo "${key}=${val}" >> .env
        ui::info "autofilled env: ${key} (from .env.example)"
      fi
    else
      if [[ -n "${val// }" ]]; then
        echo "${key}=${val}" >> .env
        ui::info "autofilled env: ${key} (from .env.example)"
      fi
    fi
  done < .env.example
  ui::ok "environment defaults synchronized"
else
  warnings+=("create .env from .env.example")
  ui::warn ".env or .env.example is missing"
fi

ui::summary "Preflight Result" 0 "${#warnings[@]}"
if ((${#warnings[@]} > 0)); then
  for item in "${warnings[@]}"; do
    ui::action "$item"
  done
fi

ui::ok "preflight completed"
