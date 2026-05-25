#!/usr/bin/env bash
# lint-landing-license.sh — guard against MIT regressions in landing/.
#
# Why this script exists:
#   - On 2026-05-25 we relicensed from MIT to Apache-2.0 (explicit
#     patent grant for cPanel/DA/CyberPanel adapters).
#   - landing/ is gitignored (deployment is decoupled — Cloudflare
#     Pages / Vercel pulls from a separate location), so the main
#     repo CI cannot lint the landing assets directly.
#   - This script is the manual guard the maintainer runs before
#     deploying a new landing snapshot. It is also wired into the
#     CHANGELOG entry for TASK-15.5 / TASK-15.7 so re-running it
#     in CI on the landing-deploy pipeline is trivial.
#
# Exit codes:
#   0  no MIT references in landing/
#   1  landing/ directory missing (run this script after first build)
#   2  at least one MIT reference found — list printed to stderr
#
# Usage:
#   bash scripts/lint-landing-license.sh
#   bash scripts/lint-landing-license.sh --json   # machine-readable

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

REPO_ROOT="$(repo_root)"
cd "$REPO_ROOT"

if [[ ! -d landing ]]; then
  warn "landing/ directory is absent — nothing to lint."
  warn "(landing/ is gitignored — only present on the maintainer's"
  warn " machine. CI cannot run this check; deploy pipeline should.)"
  exit 1
fi

# Match \bMIT\b in HTML/JSON/MD landing assets. ripgrep is required
# because POSIX grep on macOS lacks proper word boundaries.
require_cmd rg "https://github.com/BurntSushi/ripgrep#installation"

JSON_MODE=0
if [[ "${1:-}" == "--json" ]]; then
  JSON_MODE=1
fi

PATTERN='\bMIT\b'
HITS_FILE="$(mktemp -t webox-mit-hits.XXXXXX)"
trap 'rm -f "$HITS_FILE"' EXIT

# Search HTML / JSON / MD / TXT under landing/. Skip vendor + node_modules.
rg -n --no-heading \
   --type-add 'web:*.{html,htm,json,md,txt,js,mjs,ts,svg,xml}' \
   --type web \
   --glob '!landing/node_modules/**' \
   --glob '!landing/dist/**' \
   --glob '!landing/vendor/**' \
   "$PATTERN" landing/ \
   > "$HITS_FILE" || true

HIT_COUNT="$(wc -l < "$HITS_FILE" | tr -d ' ')"

if [[ "$HIT_COUNT" -eq 0 ]]; then
  if [[ "$JSON_MODE" -eq 1 ]]; then
    printf '{"status":"ok","hits":0,"message":"no MIT references in landing/"}\n'
  else
    ok "no MIT references in landing/ — Apache-2.0 license enforced."
  fi
  exit 0
fi

if [[ "$JSON_MODE" -eq 1 ]]; then
  printf '{"status":"fail","hits":%d,"violations":[' "$HIT_COUNT"
  awk -F: '
    BEGIN { first = 1 }
    {
      file = $1; line = $2; $1 = ""; $2 = ""
      txt = substr($0, 3)
      gsub(/"/, "\\\"", txt)
      if (first) { first = 0 } else { printf "," }
      printf "{\"file\":\"%s\",\"line\":%s,\"text\":\"%s\"}", file, line, txt
    }
  ' "$HITS_FILE"
  printf ']}\n'
else
  err "Found $HIT_COUNT MIT reference(s) in landing/ — please change to Apache-2.0:"
  cat "$HITS_FILE" >&2
  echo
  err "Allowed exception: only when citing a third-party library that ships under MIT."
  err "In that case, qualify the mention (e.g. \"based on MIT-licensed foo\") so this"
  err "linter's intent — catching license-text regressions — stays unambiguous."
fi
exit 2
