#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "== ANG release smoke =="

echo "[1/3] Compiler tests"
go test ./compiler/... ./cmd/ang

echo "[2/3] Release build of the sample project"
go run ./cmd/ang build --mode=release

echo "[3/3] Artifact presence checks"
test -f dist/release/go-service/cmd/server/main.go
test -f dist/release/go-service/api/openapi.yaml

echo "Release smoke SUCCESSFUL"
