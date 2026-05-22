#!/usr/bin/env bash
# Show progress of the current sprint: tasks done / total, AC checks done / total.
#
# Usage:  scripts/sprint-status.sh
#
# A "task" is detected as `### TASK-XX.Y — Name`.
# An AC is detected as `- [ ]` / `- [x]` under the task heading.
# A task is "done" when all its AC are checked.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

SPRINT="$(sprint_current)"
TITLE="$(sprint_title "$SPRINT")"

printf '%b\n' "${C_BOLD}$TITLE${C_RESET}"
printf '%b%s%b\n\n' "$C_DIM" "$SPRINT" "$C_RESET"

awk -v R="$C_RESET" -v G="$C_GREEN" -v Y="$C_YELLOW" -v D="$C_DIM" -v B="$C_BOLD" '
  BEGIN {
    in_task = 0
    task_total = 0; task_done = 0
    ac_total = 0; ac_done = 0
    cur_task = ""
    cur_total = 0; cur_done = 0
  }
  /^### TASK-/ {
    if (in_task) emit()
    cur_task = $0
    sub(/^### /, "", cur_task)
    cur_total = 0; cur_done = 0
    in_task = 1
    task_total++
    next
  }
  /^### / || /^## / {
    if (in_task) { emit(); in_task = 0 }
  }
  in_task && /^[[:space:]]*-[[:space:]]+\[[[:space:]xX]\][[:space:]]/ {
    cur_total++; ac_total++
    if (/\[[xX]\]/) { cur_done++; ac_done++ }
  }
  END {
    if (in_task) emit()
    printf "\n%s%s\n", B, "Summary" R
    printf "  Tasks:  %s%d%s / %d done\n", G, task_done, R, task_total
    printf "  AC:     %s%d%s / %d checked\n", G, ac_done, R, ac_total
    if (task_total > 0) {
      pct = (task_done * 100) / task_total
      printf "  Done:   %.0f%%\n", pct
    }
  }
  function emit() {
    status = (cur_total > 0 && cur_done == cur_total) ? (G "✓" R) : (Y "·" R)
    if (cur_done == cur_total && cur_total > 0) task_done++
    printf "  %s %-60s %s%d/%d%s\n", status, cur_task, D, cur_done, cur_total, R
  }
' "$SPRINT"
