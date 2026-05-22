#!/usr/bin/env bash
# One-shot local environment setup for a fresh clone.
# Idempotent; safe to re-run anytime.
#
# Usage:  scripts/bootstrap.sh

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

ROOT="$(repo_root)"
cd "$ROOT"

printf '%b\n\n' "${C_BOLD}Webox local environment bootstrap${C_RESET}"

# ── Required tools ─────────────────────────────────────────────────────
info "checking required tools..."
require_cmd go         "https://go.dev/dl/  (need 1.24+)"
require_cmd git
require_cmd make
ok "go $(go version | awk '{print $3}')"

# ── Optional but recommended ───────────────────────────────────────────
info "checking optional tools..."
for cmd in gh jq fswatch entr gow; do
  if have_cmd "$cmd"; then
    ok "$cmd"
  else
    warn "missing optional: $cmd"
  fi
done

# ── Go module deps ─────────────────────────────────────────────────────
if [[ -f go.mod ]]; then
  info "syncing Go module deps..."
  go mod download
  ok "go modules ready"
else
  warn "no go.mod yet (pre-Sprint-00) — skipping module deps"
fi

# ── Git hooks ──────────────────────────────────────────────────────────
info "installing git hooks..."
"$SCRIPT_DIR/install-git-hooks.sh"

# ── Cursor hooks executable bit ────────────────────────────────────────
if [[ -d .cursor/hooks ]]; then
  chmod +x .cursor/hooks/*.sh 2>/dev/null || true
  ok "Cursor hooks marked executable"
fi

# ── Dev tools (best-effort) ────────────────────────────────────────────
if [[ -f tools/tools.go ]]; then
  info "installing dev tools (gofumpt, goimports, golangci-lint, govulncheck)..."
  go install mvdan.cc/gofumpt@latest                       || warn "gofumpt install failed"
  go install golang.org/x/tools/cmd/goimports@latest       || warn "goimports install failed"
  go install golang.org/x/vuln/cmd/govulncheck@latest      || warn "govulncheck install failed"
  if ! have_cmd golangci-lint; then
    warn "golangci-lint not on PATH — install:  https://golangci-lint.run/usage/install/"
  fi
fi

# ── Summary ────────────────────────────────────────────────────────────
echo
ok "${C_BOLD}bootstrap complete${C_RESET}"
echo
echo "Next steps:"
echo "  • ${C_CYAN}make sprint-status${C_RESET}    — see current sprint progress"
echo "  • ${C_CYAN}make next-task${C_RESET}        — pick the next open task"
echo "  • ${C_CYAN}make sprint-start${C_RESET}     — branch + open sprint plan"
echo "  • ${C_CYAN}make dev${C_RESET}              — TDD watch mode"
echo "  • ${C_CYAN}make ci${C_RESET}               — full local CI bundle"
