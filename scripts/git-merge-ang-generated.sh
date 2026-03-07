#!/usr/bin/env bash
set -euo pipefail

# merge driver signature: %O %A %B
# We intentionally keep %A (current branch version) and try a best-effort regenerate.

if command -v git >/dev/null 2>&1 && command -v ang >/dev/null 2>&1; then
  repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
  if [[ -n "${repo_root}" ]]; then
    (cd "${repo_root}" && ang build >/dev/null 2>&1 || true)
  fi
fi

exit 0
