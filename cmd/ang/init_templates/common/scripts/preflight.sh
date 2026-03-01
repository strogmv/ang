#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ ! -f .env && -f .env.example ]]; then
  cp .env.example .env
  echo "Created .env from .env.example"
fi

if [[ -f .env && -f .env.example ]]; then
  while IFS= read -r line; do
    [[ -z "${line// }" ]] && continue
    [[ "${line:0:1}" == "#" ]] && continue
    key="${line%%=*}"
    val="${line#*=}"
    [[ -z "${key// }" ]] && continue

    if grep -Eq "^${key}=" .env; then
      current="$(grep -E "^${key}=" .env | tail -n 1 | cut -d'=' -f2-)"
      if [[ -z "${current// }" && -n "${val// }" ]]; then
        echo "${key}=${val}" >> .env
        echo "AUTOSET ENV: ${key} (from .env.example)"
      fi
    else
      if [[ -n "${val// }" ]]; then
        echo "${key}=${val}" >> .env
        echo "AUTOSET ENV: ${key} (from .env.example)"
      fi
    fi
  done < .env.example
fi

ang doctor start "$@"
