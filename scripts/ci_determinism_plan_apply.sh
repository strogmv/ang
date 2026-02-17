#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

TMP_DIR="$(mktemp -d /tmp/ang-plan-apply-determinism-XXXXXX)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

hash_files_from_plan() {
  local plan_path="$1"
  local out_file="$2"

  python3 - "$plan_path" "$out_file" <<'PY'
import hashlib
import json
import os
import sys

plan_path = sys.argv[1]
out_file = sys.argv[2]

with open(plan_path, "r", encoding="utf-8") as f:
    plan = json.load(f)

paths = set()
for ch in (plan.get("changes") or []):
    op = (ch.get("op") or "").lower()
    path = (ch.get("path") or "").strip()
    if not path:
        continue
    if op in {"add", "update"}:
        paths.add(path)

rows = []
for path in sorted(paths):
    if not os.path.isfile(path):
        raise SystemExit(f"missing generated file from plan: {path}")
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1024 * 1024)
            if not chunk:
                break
            h.update(chunk)
    rows.append(f"{h.hexdigest()}  {path}")

with open(out_file, "w", encoding="utf-8") as f:
    for row in rows:
        f.write(row + "\n")
PY
}

run_cycle() {
  local cycle="$1"
  local plan_path="${TMP_DIR}/plan-${cycle}.json"
  local hash_path="${TMP_DIR}/hash-${cycle}.txt"

  rm -rf dist
  go run ./cmd/ang build --mode=release --phase=plan --out-plan "${plan_path}"
  go run ./cmd/ang build --mode=release --phase=apply --plan-file "${plan_path}"
  hash_files_from_plan "${plan_path}" "${hash_path}"
}

run_cycle "a"
run_cycle "b"

diff -u "${TMP_DIR}/hash-a.txt" "${TMP_DIR}/hash-b.txt"
echo "OK: plan/apply deterministic release output"
