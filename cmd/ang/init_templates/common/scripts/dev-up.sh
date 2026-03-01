#!/usr/bin/env bash
set -euo pipefail

bash scripts/preflight.sh
ang up --skip-doctor "$@"
