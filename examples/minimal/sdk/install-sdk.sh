#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${1:-}"

if [[ -z "${APP_DIR}" ]]; then
  echo "Usage: ./install-sdk.sh /path/to/vite-app"
  exit 1
fi

SDK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="${APP_DIR%/}/sdk"

rm -rf "${TARGET_DIR}"
mkdir -p "${TARGET_DIR}"
cp -R "${SDK_DIR}"/. "${TARGET_DIR}/"

ENV_FILE="${APP_DIR%/}/.env.example"
if [[ ! -f "${ENV_FILE}" ]]; then
  echo "VITE_API_URL=http://localhost:8080" > "${ENV_FILE}"
fi

echo "SDK installed to ${TARGET_DIR}"
