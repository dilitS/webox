#!/usr/bin/env bash
# Common helpers for scripts/. Source, do not execute.
# Usage:  source "$(dirname "$0")/lib.sh"

set -euo pipefail

# ── Colors ──────────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
  C_RESET=$'\033[0m'
  C_BOLD=$'\033[1m'
  C_DIM=$'\033[2m'
  C_RED=$'\033[31m'
  C_GREEN=$'\033[32m'
  C_YELLOW=$'\033[33m'
  C_BLUE=$'\033[34m'
  C_CYAN=$'\033[36m'
else
  C_RESET="" C_BOLD="" C_DIM="" C_RED="" C_GREEN="" C_YELLOW="" C_BLUE="" C_CYAN=""
fi

# ── I/O ────────────────────────────────────────────────────────────────
info()  { printf '%b\n' "${C_CYAN}ℹ${C_RESET}  $*"; }
ok()    { printf '%b\n' "${C_GREEN}✓${C_RESET}  $*"; }
warn()  { printf '%b\n' "${C_YELLOW}⚠${C_RESET}  $*" >&2; }
err()   { printf '%b\n' "${C_RED}✗${C_RESET}  $*" >&2; }
die()   { err "$*"; exit 1; }

# ── Predicates ─────────────────────────────────────────────────────────
have_cmd() { command -v "$1" >/dev/null 2>&1; }

require_cmd() {
  have_cmd "$1" || die "missing dependency: $1${2:+ — install: $2}"
}

# ── Repo / paths ───────────────────────────────────────────────────────
repo_root() {
  git rev-parse --show-toplevel 2>/dev/null \
    || die "not in a git repository"
}

# ── Sprint discovery ───────────────────────────────────────────────────
# Returns the path to the LOWEST-numbered sprint file that still has
# uncompleted tasks (presence of unchecked "- [ ]" boxes, possibly indented).
# Falls back to the highest-numbered file overall.
sprint_current() {
  local root sprint first_open last_any
  root="$(repo_root)"
  first_open=""
  last_any=""
  for sprint in "$root"/docs/sprints/sprint-*.md; do
    [[ -f "$sprint" ]] || continue
    last_any="$sprint"
    if grep -qE '^[[:space:]]*-[[:space:]]*\[[[:space:]]\][[:space:]]' "$sprint" 2>/dev/null; then
      if [[ -z "$first_open" ]]; then first_open="$sprint"; fi
    fi
  done
  if [[ -n "$first_open" ]]; then
    printf '%s\n' "$first_open"
  elif [[ -n "$last_any" ]]; then
    printf '%s\n' "$last_any"
  else
    die "no sprint files found under docs/sprints/"
  fi
}

# Extract sprint number (e.g. "01") from filename like sprint-01-foundations.md
sprint_number() {
  local file="${1:-}"
  [[ -n "$file" ]] || die "sprint_number: missing argument"
  basename "$file" | sed -E 's/^sprint-([0-9]+).*/\1/'
}

# Pretty sprint name (e.g. "Sprint 01 — Foundations") from heading or filename
sprint_title() {
  local file="$1"
  awk '/^# /{ sub(/^# /,""); print; exit }' "$file" 2>/dev/null \
    || basename "$file" .md
}

# ── Branch helpers ─────────────────────────────────────────────────────
# Renamed from `current_branch` to avoid clobbering the zsh oh-my-zsh alias.
git_current_branch() { git rev-parse --abbrev-ref HEAD; }

# ── Confirmation prompt ────────────────────────────────────────────────
confirm() {
  local prompt="${1:-Continue?}"
  local reply
  read -r -p "$prompt [y/N] " reply
  [[ "$reply" =~ ^[yY]([eE][sS])?$ ]]
}
