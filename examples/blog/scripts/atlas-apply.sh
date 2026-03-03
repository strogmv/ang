#!/usr/bin/env bash
set -euo pipefail

DB_URL="${DB_URL:-}"
if [[ -z "${DB_URL}" ]]; then
  echo "Set DB_URL, e.g. postgres://user:pass@localhost:5432/db?sslmode=disable"
  exit 1
fi

atlas migrate apply --dir file://db/migrations --url "${DB_URL}"
