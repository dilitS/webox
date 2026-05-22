#!/usr/bin/env bash
# Wire up versioned git hooks from .githooks/ into this clone.
#
# Sets `git config core.hooksPath .githooks` (local to this clone only).
# Idempotent; safe to re-run.
#
# Usage:  scripts/install-git-hooks.sh

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

ROOT="$(repo_root)"
HOOKS_DIR="$ROOT/.githooks"

[[ -d "$HOOKS_DIR" ]] || die "missing $HOOKS_DIR — pull latest main"

# Mark all hook files executable
chmod +x "$HOOKS_DIR"/* 2>/dev/null || true

# Point git at our hooks
git -C "$ROOT" config core.hooksPath .githooks

ok "git hooks installed (core.hooksPath = .githooks)"

ls -1 "$HOOKS_DIR" | while read -r h; do
  [[ -f "$HOOKS_DIR/$h" ]] || continue
  case "$h" in
    *.sh|README*) continue ;;
  esac
  printf '  • %s\n' "$h"
done

info "to disable temporarily:  ${C_BOLD}git -c core.hooksPath= ...${C_RESET}"
info "to uninstall:            ${C_BOLD}git config --unset core.hooksPath${C_RESET}"
