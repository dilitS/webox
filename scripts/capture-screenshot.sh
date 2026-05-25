#!/usr/bin/env bash
# capture-screenshot.sh — render static PNGs from the asciinema cast.
#
# README.md and landing/ need a static image fallback because:
#   - GitHub renders SVG/PNG inline but not asciinema embeds.
#   - SEO/OG previews scrape PNG, not iframes.
#   - Offline operators (airplane, archived snapshots) see something.
#
# Output:
#   assets/screenshots/dashboard.png   Frame from t=8s of demo.cast
#                                      (cockpit at rest, Bento Ultra).
#   assets/screenshots/wizard.png      Captured manually — operator runs
#                                      ./bin/webox, navigates to wizard
#                                      step 3 (subdomain + Node version)
#                                      and screenshots the terminal.
#   landing/og-image.png               1200x630 social card built from
#                                      dashboard.png + product wordmark
#                                      (handled by landing/ tooling, not
#                                      this script).
#
# Render strategy (in priority order):
#   1. `agg` (asciinema GIF renderer, since 2024) — best fidelity.
#      https://github.com/asciinema/agg
#   2. `asciinema-to-png` Python tool fallback.
#   3. Manual fallback: replay the cast in iTerm2 / Ghostty / Alacritty
#      with a 120x35 window, screenshot at t=8s.
#
# This script automates path 1, prints path-2 + path-3 hints if `agg`
# is missing.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

REPO_ROOT="$(repo_root)"
cd "$REPO_ROOT"

CAST="assets/demo/demo.cast"
OUT_DIR="assets/screenshots"
mkdir -p "$OUT_DIR"

if [[ ! -f "$CAST" ]]; then
  die "missing $CAST — run scripts/record-demo.sh first."
fi

DASHBOARD="$OUT_DIR/dashboard.png"

if have_cmd agg; then
  info "Rendering $DASHBOARD with agg (frame at t=8s)…"
  # agg renders the entire cast to GIF/PNG sequence; we extract the
  # frame closest to t=8s by setting --idle-time-limit very low and
  # then sampling. Easiest reproducible path: render full GIF, then
  # use ffmpeg to pull frame at t=8s.
  GIF_TMP="$(mktemp -t webox-demo.XXXXXX.gif)"
  trap 'rm -f "$GIF_TMP"' EXIT

  agg \
    --cols 120 \
    --rows 35 \
    --font-size 14 \
    --theme monokai \
    "$CAST" \
    "$GIF_TMP"

  require_cmd ffmpeg "macOS: brew install ffmpeg · Ubuntu: sudo apt install ffmpeg"
  ffmpeg -y -ss 00:00:08 -i "$GIF_TMP" -vframes 1 "$DASHBOARD" >/dev/null 2>&1
  ok "Wrote $DASHBOARD"
else
  warn "'agg' not installed — falling back to manual capture instructions."
  cat <<MANUAL

  Install one of the renderers and re-run:

    1. agg (preferred, native): https://github.com/asciinema/agg
       cargo install --git https://github.com/asciinema/agg
       brew install ffmpeg   # for the t=8s frame extraction

    2. asciinema-to-png (fallback):
       pip install asciinema-to-png

  Or capture manually:

    1. Resize your terminal to 120x35 (tput cols/lines must report 120 35).
    2. asciinema play assets/demo/demo.cast --speed 1 --idle-time-limit 1
    3. Pause at the 8-second mark (the cockpit "at rest" frame).
    4. Screenshot the terminal (macOS: ⌘⇧4 · Linux: gnome-screenshot -w).
    5. Save as $DASHBOARD (1280x800 or larger, PNG).

MANUAL
  exit 2
fi

WIZARD="$OUT_DIR/wizard.png"
if [[ ! -f "$WIZARD" ]]; then
  warn "Wizard screenshot ($WIZARD) is captured manually:"
  cat <<WIZARD
    1. Resize terminal to 120x35.
    2. Run: ./bin/webox --mock
    3. Press Ctrl+N to open the new-project wizard.
    4. Step through to "Subdomain + Node version" page.
    5. Screenshot the terminal.
    6. Save as $WIZARD.
WIZARD
fi

ok "Done. Commit any updated PNGs (Git LFS if > 1 MB)."
