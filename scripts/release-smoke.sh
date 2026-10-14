#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "== ANG release smoke =="

echo "[1/3] Generator tests"
go test ./... -count=1

echo "[2/3] Blog sample: validate, build and test its module"
go run ./cmd/ang validate examples/blog
(cd examples/blog && go build ./... && go test ./... -count=1)

echo "[3/3] Deterministic generation (plan/apply/all on a scratch copy of the sample)"
go run ./scripts/determinism_ci

echo "Release smoke SUCCESSFUL"
