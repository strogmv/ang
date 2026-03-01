#!/usr/bin/env bash

# Lightweight terminal UI helpers for project scripts.
# Colors are enabled only for TTY output and can be disabled with NO_COLOR=1.

if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
  UI_BOLD='\033[1m'
  UI_DIM='\033[2m'
  UI_RED='\033[31m'
  UI_YELLOW='\033[33m'
  UI_GREEN='\033[32m'
  UI_BLUE='\033[34m'
  UI_RESET='\033[0m'
else
  UI_BOLD=''
  UI_DIM=''
  UI_RED=''
  UI_YELLOW=''
  UI_GREEN=''
  UI_BLUE=''
  UI_RESET=''
fi

ui::section() {
  printf "\n%s==> %s%s\n" "$UI_BOLD" "$1" "$UI_RESET"
}

ui::info() {
  printf "%s[INFO]%s %s\n" "$UI_BLUE" "$UI_RESET" "$1"
}

ui::ok() {
  printf "%s[ OK ]%s %s\n" "$UI_GREEN" "$UI_RESET" "$1"
}

ui::warn() {
  printf "%s[WARN]%s %s\n" "$UI_YELLOW" "$UI_RESET" "$1"
}

ui::fail() {
  printf "%s[FAIL]%s %s\n" "$UI_RED" "$UI_RESET" "$1"
}

ui::action() {
  printf "      %sfix:%s %s\n" "$UI_DIM" "$UI_RESET" "$1"
}

ui::summary() {
  local label="$1"
  local fails="$2"
  local warns="$3"

  printf "\n%s%s%s\n" "$UI_BOLD" "$label" "$UI_RESET"
  printf "  failures: %s\n" "$fails"
  printf "  warnings: %s\n" "$warns"
}
