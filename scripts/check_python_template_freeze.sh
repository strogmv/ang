#!/usr/bin/env bash
set -euo pipefail

if [[ "${ALLOW_PY_TEMPLATE_CHANGES:-0}" == "1" ]]; then
  echo "Python template freeze override enabled (ALLOW_PY_TEMPLATE_CHANGES=1)."
  exit 0
fi

BASE_REF="${1:-origin/main}"
if ! git rev-parse --verify "$BASE_REF" >/dev/null 2>&1; then
  echo "Base ref '$BASE_REF' not found; skipping freeze check."
  exit 0
fi

CHANGED="$(git diff --name-only "$BASE_REF"...HEAD -- templates/python || true)"
if [[ -n "$CHANGED" ]]; then
  echo "Python template freeze is active. Changed files:"
  echo "$CHANGED"
  echo
  echo "For critical fixes only, rerun with ALLOW_PY_TEMPLATE_CHANGES=1."
  exit 1
fi

echo "Python template freeze check passed."
