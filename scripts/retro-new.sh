#!/usr/bin/env bash
# Generate a fresh retrospective for the current sprint.
#
# Usage:
#   scripts/retro-new.sh                        # today, current sprint
#   scripts/retro-new.sh 01                     # explicit sprint number
#   scripts/retro-new.sh 01 2026-06-12          # explicit date too
#
# Output: docs/retros/YYYY-MM-DD-sprint-NN.md
# Idempotent: if the file already exists, it is opened without overwriting.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

SPRINT_NUM="${1:-}"
DATE="${2:-$(date -u +%Y-%m-%d)}"

if [[ -z "$SPRINT_NUM" ]]; then
  SPRINT_NUM="$(sprint_number "$(sprint_current)")"
fi

ROOT="$(repo_root)"
RETRO_DIR="$ROOT/docs/retros"
RETRO_FILE="$RETRO_DIR/${DATE}-sprint-${SPRINT_NUM}.md"

mkdir -p "$RETRO_DIR"

if [[ -f "$RETRO_FILE" ]]; then
  warn "retro already exists: $RETRO_FILE"
else
  cat > "$RETRO_FILE" <<EOF
# Retro — Sprint ${SPRINT_NUM}

> Date: ${DATE} · Owner: @maintainer · Sprint plan: [\`docs/sprints/sprint-${SPRINT_NUM}-*.md\`](../sprints/)

## TL;DR

_2-3 sentences. What was the sprint goal, did we hit it, what changed._

## Co poszło dobrze

- …

## Co poszło źle / było frustrujące

- …

## Niespodzianki (rzeczywistość ≠ docs)

- …

## Zmiany w procesie (apply next sprint)

- [ ] …

## Otwarte pytania

- …

## Metryki

| Metryka | Wartość |
|---------|---------|
| Tasks closed | _N_ / _M_ |
| Carry-over | _N_ |
| Coverage (delta) | _+X%_ |
| Real velocity vs estimate | _X.Xx_ |
| Burnout signal (0-5) | _N_ |

## Aktualizacja [RISKS.md](../RISKS.md)

_Czy któreś ryzyko zmienia score? Czy są nowe?_

- …

## Następny sprint

- Plan: \`docs/sprints/sprint-$(printf '%02d' $((10#${SPRINT_NUM} + 1)))-*.md\` _(do utworzenia w 30-min planning session)_
- Carry-over: _TASK-XX.Y → reason_
EOF
  ok "created $RETRO_FILE"
fi

# Open in $EDITOR if interactive
if [[ -t 0 ]] && [[ -n "${EDITOR:-}" ]] && have_cmd "${EDITOR%% *}"; then
  if confirm "Open retro in \$EDITOR ($EDITOR)?"; then
    $EDITOR "$RETRO_FILE"
  fi
fi
