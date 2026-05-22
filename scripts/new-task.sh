#!/usr/bin/env bash
# Append a new task scaffold to the current sprint plan.
#
# Usage:
#   scripts/new-task.sh "Short name" [estimate]
#   scripts/new-task.sh "Add retry on SFTP" L
#
# The task gets the next free number in the sprint.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

NAME="${1:-}"
EST="${2:-M}"
[[ -n "$NAME" ]] || die "usage: $0 \"Task name\" [S|M|L|XL]"

SPRINT="$(sprint_current)"
SPRINT_NUM="$(sprint_number "$SPRINT")"

# Find the highest existing TASK number in this sprint
HIGHEST="$(grep -oE "^### TASK-${SPRINT_NUM}\.[0-9]+" "$SPRINT" 2>/dev/null \
  | sed -E 's/.*\.([0-9]+)/\1/' \
  | sort -n | tail -1 || echo 0)"
NEXT=$((HIGHEST + 1))
TID="TASK-${SPRINT_NUM}.${NEXT}"

# Find the "## Risk watch" section as anchor; insert before it.
if ! grep -q '^## Risk watch' "$SPRINT"; then
  die "sprint plan missing '## Risk watch' section — cannot find insertion point"
fi

BLOCK="$(cat <<EOF

### $TID — $NAME

- **Estymata:** $EST
- **Zależności:** —
- **Acceptance Criteria:**
  - [ ] _AC1 (testowalne)_
  - [ ] _AC2 (mierzalne)_
- **Pliki:**
  - \`path/to/file.go\` (new)
- **Docs:** _link to PRD/DESIGN/SECURITY/ADR/AUDIT_
- **Notatki:** _pułapki, edge cases_

EOF
)"

TMP="$(mktemp)"
awk -v block="$BLOCK" '
  /^## Risk watch/ && !done {
    print block
    done = 1
  }
  { print }
' "$SPRINT" > "$TMP"

mv "$TMP" "$SPRINT"

ok "appended ${C_BOLD}$TID${C_RESET} to $(basename "$SPRINT")"
info "open with: ${C_BOLD}\$EDITOR $SPRINT${C_RESET}"
