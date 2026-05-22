#!/usr/bin/env bash
# Print the FIRST task in the current sprint that has at least one unchecked AC.
# Output is parseable: prints "TASK-XX.Y" on stdout. Optional --verbose prints
# the full task block.
#
# Usage:
#   scripts/next-task.sh
#   scripts/next-task.sh --verbose

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

VERBOSE=0
if [[ "${1:-}" == "--verbose" || "${1:-}" == "-v" ]]; then VERBOSE=1; fi

SPRINT="$(sprint_current)"

awk -v V="$VERBOSE" '
  function emit_and_exit() {
    if (emitted) return
    emitted = 1
    if (V == 1) printf "%s", buf
    else { match(tname, /TASK-[0-9]+\.[0-9]+/); print substr(tname, RSTART, RLENGTH) }
    exit 0
  }

  function close_task() {
    if (in_task && unchecked > 0) emit_and_exit()
    in_task = 0; buf = ""; tname = ""; unchecked = 0
  }

  /^### TASK-/ {
    if (in_task) close_task()
    tname = $0; sub(/^### /, "", tname)
    buf = $0 "\n"
    in_task = 1
    unchecked = 0
    next
  }

  /^## / || (/^### / && !/^### TASK-/) {
    close_task()
  }

  in_task {
    buf = buf $0 "\n"
    if (/^[[:space:]]*-[[:space:]]+\[[[:space:]]\][[:space:]]/) unchecked++
  }

  END {
    if (emitted) exit 0
    close_task()
    if (emitted) exit 0
    print "(no open tasks in sprint)" > "/dev/stderr"
    exit 1
  }
' "$SPRINT"
