#!/usr/bin/env bash
# Open the current sprint plan in $EDITOR and create a feature branch
# for the next open task.
#
# Usage:
#   scripts/start-sprint.sh                       # next task, auto branch
#   scripts/start-sprint.sh TASK-01.3             # explicit task id
#   scripts/start-sprint.sh TASK-01.3 short-name  # custom branch suffix
#
# Branch naming: feat/s<NN>-<task-num>-<slug>
#   e.g. TASK-01.3 → feat/s01-03-config-save

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

SPRINT="$(sprint_current)"
SPRINT_NUM="$(sprint_number "$SPRINT")"

TASK_ID="${1:-}"
SLUG="${2:-}"

if [[ -z "$TASK_ID" ]]; then
  TASK_ID="$("$SCRIPT_DIR/next-task.sh" 2>/dev/null || true)"
  [[ -n "$TASK_ID" ]] || die "no open tasks. Plan a new sprint or finish current."
fi

if ! [[ "$TASK_ID" =~ ^TASK-([0-9]+)\.([0-9]+)$ ]]; then
  die "invalid task id: $TASK_ID (expected TASK-NN.M)"
fi
T_SPRINT="${BASH_REMATCH[1]}"
T_NUM="${BASH_REMATCH[2]}"

# Slug auto-generated from heading if not provided
if [[ -z "$SLUG" ]]; then
  SLUG="$(awk -v t="$TASK_ID" '
    $0 ~ "^### "t {
      sub("^### "t" — ?", "")
      sub(/\(.*$/, "")
      gsub(/`/, "")
      print
      exit
    }
  ' "$SPRINT" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g; s/^-+|-+$//g' \
    | cut -c1-40)"
fi
[[ -n "$SLUG" ]] || SLUG="task-${T_NUM}"

BRANCH="feat/s${T_SPRINT}-$(printf '%02d' "$T_NUM")-${SLUG}"

info "sprint:  $(sprint_title "$SPRINT")"
info "task:    ${C_BOLD}$TASK_ID${C_RESET}"
info "branch:  ${C_BOLD}$BRANCH${C_RESET}"

if git rev-parse --verify --quiet "refs/heads/$BRANCH" >/dev/null; then
  warn "branch already exists — switching"
  git switch "$BRANCH"
else
  base="$(git symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's@^origin/@@' || echo main)"
  if ! git diff --quiet HEAD 2>/dev/null; then
    die "working tree is dirty — commit or stash first"
  fi
  git switch "$base" >/dev/null 2>&1 || true
  git pull --ff-only origin "$base" >/dev/null 2>&1 || true
  git switch -c "$BRANCH"
  ok "created branch $BRANCH from $base"
fi

# Open sprint doc in $EDITOR if interactive
if [[ -t 0 ]] && [[ -n "${EDITOR:-}" ]] && have_cmd "${EDITOR%% *}"; then
  if confirm "Open sprint plan in \$EDITOR ($EDITOR)?"; then
    $EDITOR "$SPRINT"
  fi
fi

ok "ready. Next: ${C_BOLD}make dev PKG=./...${C_RESET} and start the TDD loop."
