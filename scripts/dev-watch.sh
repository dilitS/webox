#!/usr/bin/env bash
# TDD watch loop. Re-runs `go test` whenever a .go file changes.
#
# Usage:
#   scripts/dev-watch.sh [PKG]            # default: ./...
#   scripts/dev-watch.sh ./config/...
#
# Backends (auto-detected, in priority order):
#   1. gow              (github.com/mitranim/gow) — preferred, smart
#   2. fswatch + entr   (macOS / Linux)
#   3. inotifywait      (Linux only)
#   4. polling fallback (every 2s, last-resort)

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

PKG="${1:-./...}"
TEST_ARGS=(-race -timeout 60s -count=1 -failfast)

info "watching for *.go changes — runs: ${C_BOLD}go test ${TEST_ARGS[*]} $PKG${C_RESET}"

cleanup() { printf '\n%b\n' "${C_DIM}stopped.${C_RESET}"; }
trap cleanup EXIT INT TERM

# ── gow (best) ─────────────────────────────────────────────────────────
if have_cmd gow; then
  exec gow -c test "${TEST_ARGS[@]}" "$PKG"
fi

# ── fswatch + entr ─────────────────────────────────────────────────────
if have_cmd fswatch && have_cmd entr; then
  while true; do
    git ls-files '*.go' 2>/dev/null \
      | entr -d -c -p go test "${TEST_ARGS[@]}" "$PKG"
  done
fi

# ── inotifywait ────────────────────────────────────────────────────────
if have_cmd inotifywait; then
  while true; do
    clear
    go test "${TEST_ARGS[@]}" "$PKG" || true
    inotifywait -qr -e modify,create,delete,move \
      --include '\.go$' \
      "$(repo_root)" >/dev/null
  done
fi

# ── Polling fallback (cross-platform, no deps) ─────────────────────────
warn "no file-watcher found (gow / fswatch+entr / inotifywait). Polling every 2s."
warn "Install gow for best UX: ${C_BOLD}go install github.com/mitranim/gow@latest${C_RESET}"

# Cross-platform hash of all .go files we care about
hash_tree() {
  cd "$(repo_root)" && \
    git ls-files '*.go' 2>/dev/null \
    | xargs -I{} stat -f '%m %N' {} 2>/dev/null \
    || git ls-files '*.go' 2>/dev/null \
    | xargs -I{} stat -c '%Y %n' {} 2>/dev/null
}

LAST=""
while true; do
  CUR="$(hash_tree | shasum 2>/dev/null | awk '{print $1}' || echo "")"
  if [[ "$CUR" != "$LAST" ]]; then
    clear
    go test "${TEST_ARGS[@]}" "$PKG" || true
    LAST="$CUR"
  fi
  sleep 2
done
