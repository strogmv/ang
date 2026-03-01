#!/usr/bin/env bash
set -euo pipefail

docker compose down -v
ang up --skip-doctor
