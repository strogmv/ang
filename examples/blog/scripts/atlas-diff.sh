#!/usr/bin/env bash
set -euo pipefail

NAME="${1:-}"
if [[ -z "${NAME}" ]]; then
  echo "Usage: ./scripts/atlas-diff.sh <migration_name>"
  exit 1
fi

atlas migrate diff "${NAME}" --env local --to file://db/schema/schema.sql --dir file://db/migrations

LATEST="$(ls -t db/migrations/*.sql 2>/dev/null | head -n 1 || true)"
if [[ -z "${LATEST}" ]]; then
  exit 0
fi

if grep -nE "DROP TABLE|DROP COLUMN" "${LATEST}" >/dev/null; then
  echo "Detected destructive statements in ${LATEST}."
  echo "Review the migration. Re-run with ALLOW_DROP=1 to accept."
  if [[ "${ALLOW_DROP:-}" != "1" ]]; then
    exit 1
  fi
fi
