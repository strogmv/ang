#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
driver_script="${repo_root}/scripts/git-merge-ang-generated.sh"

git config merge.ang-generated.name "ANG generated artifacts merge driver"
git config merge.ang-generated.driver "bash ${driver_script} %O %A %B"

echo "Configured merge driver: ang-generated"
