#!/usr/bin/env bash
# record-demo.sh — record the 45-60 s Webox launch demo (asciinema cast).
#
# The cast file is the canonical "What does Webox look like?" artefact
# linked from README.md and landing/. It MUST stay scripted (same beats,
# same timing) so a viewer who watched it once recognises every action
# the next time. Manual ad-libbing while recording is not allowed —
# re-run this script instead and re-upload.
#
# Output:
#   assets/demo/demo.cast     The asciinema 2.x cast.
#   assets/demo/demo.sh.log   Companion: the literal keystroke script
#                             played, so reviewers can diff timing.
#
# Requirements:
#   - asciinema 2.x or 3.x in PATH (https://asciinema.org/docs/install)
#   - ./bin/webox built locally (`make build`)
#   - Terminal sized exactly 120x35 (Bento Ultra layout). The script
#     refuses to run otherwise to guarantee reproducible framing.
#   - `expect` (POSIX `tclsh expect`) so the demo is deterministic.
#     macOS: `brew install expect` · Ubuntu: `sudo apt install expect`.
#
# Scenario (timed by `expect send` + `sleep` blocks below):
#   00s  Start dashboard with --mock seed.
#   05s  Tab through cockpit tiles (5 ticks).
#   10s  Open project "shop-ease" detail panel (Enter).
#   18s  Open CI/CD pipeline modal (F8) and scroll steps (j/k).
#   28s  Open Live Log Stream tile (Tab 4) — three log lines roll.
#   38s  Esc back to Topology Map.
#   45s  Quit with q.
#
# Re-render frame 8 (used as static PNG fallback): set FRAME=8 below
# and re-run with FRAME=on to print the chosen frame index.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

REPO_ROOT="$(repo_root)"
cd "$REPO_ROOT"

require_cmd asciinema "https://asciinema.org/docs/install"
require_cmd expect "macOS: brew install expect · Ubuntu: sudo apt install expect"

BIN="./bin/webox"
if [[ ! -x "$BIN" ]]; then
  warn "$BIN missing — running 'make build' first."
  make build >/dev/null
fi

# Enforce 120x35 terminal size so framing is reproducible. Bento Ultra
# layout requires it; smaller terminals get the Standard Cockpit fallback
# which would produce a different cast.
COLS="$(tput cols)"
ROWS="$(tput lines)"
if [[ "$COLS" != "120" || "$ROWS" != "35" ]]; then
  die "terminal must be exactly 120x35 (currently ${COLS}x${ROWS}). Resize and retry."
fi

OUT_DIR="assets/demo"
mkdir -p "$OUT_DIR"

CAST="$OUT_DIR/demo.cast"
LOG="$OUT_DIR/demo.sh.log"
EXPECT_SCRIPT="$(mktemp -t webox-demo.XXXXXX.exp)"
trap 'rm -f "$EXPECT_SCRIPT"' EXIT

cat > "$EXPECT_SCRIPT" <<'EXPECT_EOF'
#!/usr/bin/env expect -f
# Deterministic Webox --mock keystroke script.
# Send delays are tuned so the cast lands in the 45-60 s target window.

set timeout 30
set send_human {0.05 0.10 1 0.05 0.20}

# 00s — Boot
spawn ./bin/webox --mock
expect -re "Topology|Bento|Dashboard"

# 05s — Tab through cockpit tiles (5 ticks).
for {set i 0} {$i < 5} {incr i} {
  send "\t"
  sleep 1
}

# 10s — Open "shop-ease" project detail.
#       Mock seed places "shop-ease" first → Enter on default focus.
send "\r"
sleep 5

# 18s — CI/CD modal (F8), scroll steps (j × 4).
send "\x1b\[19~"
sleep 2
for {set i 0} {$i < 4} {incr i} {
  send "j"
  sleep 0.4
}

# 28s — Live Log Stream tile (Tab 4 from cockpit context).
send "\x1b"
sleep 0.5
for {set i 0} {$i < 4} {incr i} {
  send "\t"
  sleep 0.6
}
sleep 8

# 38s — Esc back to Topology Map.
send "\x1b"
sleep 5

# 45s — Quit cleanly.
send "q"
expect eof
EXPECT_EOF

info "Recording demo to $CAST (target 45-60 s)…"
asciinema rec \
  --idle-time-limit=1.5 \
  --overwrite \
  --title="Webox v0.1 — 45-second mock cockpit tour" \
  --command="expect -f $EXPECT_SCRIPT" \
  "$CAST"

cp -f "$EXPECT_SCRIPT" "$LOG"
ok "Recorded: $CAST"
info "Companion script saved at: $LOG"
info ""
info "Next steps:"
info "  1. Play it back locally:    asciinema play $CAST"
info "  2. Upload to asciinema.org: asciinema upload $CAST"
info "  3. Update README.md badge URL with the new cast id."
info "  4. Capture frame ~8s as static PNG via scripts/capture-screenshot.sh."
