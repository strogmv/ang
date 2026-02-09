#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "== ANG release smoke =="

echo "[1/7] Compiler tests"
go test ./compiler/... ./cmd/ang

echo "[2/7] Multi-target build (Go + Python)"
ANG_PY_SDK="${ANG_PY_SDK:-1}" go run ./cmd/ang build

echo "[3/7] Artifact presence checks"
test -f dist/release/go-service/cmd/server/main.go
test -f dist/release/python-service/app/main.py
test -f dist/release/go-service/sdk/python/pyproject.toml

echo "[4/7] OpenAPI compatibility report"
REPORT_DIR="dist/release/reports"
mkdir -p "$REPORT_DIR"
GO_OAS="dist/release/go-service/api/openapi.yaml"
PY_OAS="dist/release/python-service/api/openapi.yaml"
DIFF_FILE="$REPORT_DIR/openapi-go-vs-python.diff"
SUMMARY_FILE="$REPORT_DIR/openapi-go-vs-python.md"

GO_SHA="$(sha256sum "$GO_OAS" | awk '{print $1}')"
PY_SHA="$(sha256sum "$PY_OAS" | awk '{print $1}')"

if diff -u "$GO_OAS" "$PY_OAS" >"$DIFF_FILE"; then
  STATUS="identical"
else
  STATUS="different"
fi

cat >"$SUMMARY_FILE" <<EOF
# OpenAPI Compatibility Report

- Generated at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- Go spec: \`$GO_OAS\`
- Python spec: \`$PY_OAS\`
- Go sha256: \`$GO_SHA\`
- Python sha256: \`$PY_SHA\`
- Status: **$STATUS**

See diff: \`$DIFF_FILE\`
EOF

echo "[5/7] FastAPI scaffold syntax smoke"
python3 -m py_compile \
  dist/release/python-service/app/main.py \
  dist/release/python-service/app/routers/auth.py \
  dist/release/python-service/app/services/auth.py

echo "[6/7] PDF example build from CUE"
go run ./cmd/ang build examples/pdf-chart-python --target=python
test -f examples/pdf-chart-python/app/main.py
test -f examples/pdf-chart-python/app/services/report.py
python3 -m py_compile \
  examples/pdf-chart-python/app/main.py \
  examples/pdf-chart-python/app/services/report.py

echo "[7/7] Optional Python SDK install/import smoke"
if [[ "${SKIP_PIP_SMOKE:-0}" == "1" ]]; then
  echo "SKIP_PIP_SMOKE=1 -> skipping pip install/import smoke"
else
  python3 -m venv /tmp/ang-release-venv
  # shellcheck disable=SC1091
  . /tmp/ang-release-venv/bin/activate
  python -m pip install --upgrade pip
  python -m pip install -e dist/release/go-service/sdk/python
  python -c "from ang_sdk import AngClient; print('ok')"
fi

echo "Release smoke SUCCESSFUL"
