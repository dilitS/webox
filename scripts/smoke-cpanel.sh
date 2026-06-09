#!/usr/bin/env bash
# Live cPanel smoke-test — exercise the UAPI read paths against a
# real cPanel account so we know the typed envelope and the panel's
# actual response shape still line up.
#
# WHEN to run
# -----------
# - After provisioning a new cPanel host (e.g. vh.pl) and BEFORE any
#   manual provider integration.
# - Before tagging v0.2.0-rc1 — the cpanel adapter must answer to
#   real panel output, not just mocks.
# - When adding a new UAPI module/function — the smoke step proves
#   the envelope shape lines up with the package's typed responses.
#
# WHAT it does
# ------------
# 1. Reads the cPanel API TOKEN (NOT the account password) from the
#    operator's macOS Keychain via `security find-generic-password`
#    (or `secret-tool` on Linux).
# 2. Runs `webox doctor cpanel --json` against the live host — this
#    invokes the four read-only UAPI calls (DomainInfo, PassengerApps,
#    Mysql, SSL) via the package's transport.
# 3. With --capture, writes the redacted JSON report into
#    providers/cpanel/uapi/testdata/live/ for the operator to review
#    before any payload is promoted to a checked-in fixture.
# 4. NEVER touches a mutating endpoint. `WEBOX_CPANEL_MUTATIONS` is
#    explicitly unset before every call so a misconfigured shell
#    cannot create / delete anything.
#
# WHAT it does NOT do
# -------------------
# - Does NOT use the account password. Use an API token: cPanel UI →
#   "Manage API Tokens" → create, then store in keyring.
# - Does NOT write the token to disk, env vars that survive the
#   shell, or any log line.
# - Does NOT auto-promote captured payloads into testdata/. The
#   operator manually copies + redacts after review.
#
# Usage:
#   scripts/smoke-cpanel.sh --host=panel.vh.pl --user=alice
#   scripts/smoke-cpanel.sh --host=... --user=... --capture
#
# Seed the keyring once:
#   security add-generic-password -a "<user>" -s "webox-cpanel-smoke" -w
#   # ↑ paste the cPanel API TOKEN at the prompt.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/lib.sh"

HOST=""
USER_ACCT=""
CAPTURE=0
KEYCHAIN_SERVICE="webox-cpanel-smoke"

usage() {
  cat <<'USAGE'
scripts/smoke-cpanel.sh - read-only cPanel smoke-test

Required flags:
  --host=<panel.example.com>   cPanel host (DNS name or IP)
  --user=<accountuser>         cPanel account login

Optional flags:
  --capture                    write the redacted JSON report into
                               providers/cpanel/uapi/testdata/live/
                               (gitignored; review before promotion)
  --service=<keychain-service> override keyring service name
                               (default: webox-cpanel-smoke)
  -h, --help                   show this help and exit

Credential storage:
  This script reads the cPanel API TOKEN from the macOS Keychain.

    security add-generic-password -a "<user>" -s "webox-cpanel-smoke" -w
    # ↑ paste the API token at the prompt; -w keeps it out of shell history.

  Linux: use `secret-tool store --label='webox cpanel smoke' service webox-cpanel-smoke account "<user>"`.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host=*)    HOST="${1#*=}"; shift ;;
    --user=*)    USER_ACCT="${1#*=}"; shift ;;
    --service=*) KEYCHAIN_SERVICE="${1#*=}"; shift ;;
    --capture)   CAPTURE=1; shift ;;
    -h|--help)   usage; exit 0 ;;
    *)           err "unknown flag: $1"; usage; exit 2 ;;
  esac
done

[[ -z "${HOST}" || -z "${USER_ACCT}" ]] && { usage; exit 2; }
require_cmd jq "brew install jq"
require_cmd go "https://go.dev/dl/"

# Belt-and-braces: never let a mutating opt-in leak into the smoke run.
unset WEBOX_CPANEL_MUTATIONS

lookup_secret() {
  if have_cmd security; then
    security find-generic-password \
      -a "${USER_ACCT}" -s "${KEYCHAIN_SERVICE}" -w 2>/dev/null
  elif have_cmd secret-tool; then
    secret-tool lookup service "${KEYCHAIN_SERVICE}" account "${USER_ACCT}" 2>/dev/null
  else
    die "no supported keyring backend (need 'security' on macOS or 'secret-tool' on Linux)"
  fi
}

info "Looking up API token from keyring (service=${KEYCHAIN_SERVICE}, account=${USER_ACCT})..."
TOKEN="$(lookup_secret || true)"
if [[ -z "${TOKEN}" ]]; then
  err "keyring lookup returned empty — seed the token first (see --help)"
  exit 3
fi

# Pass the token through a non-export env var consumed by the CLI
# (the CLI prefers an env-var input over a flag specifically so the
# token never lands in /proc/<pid>/cmdline). Until the CLI grows
# that flag we use `--token=...` directly; this is the same pattern
# the existing test suite uses, and the password redactor scrubs it
# from any log line.
LIVE_DIR=""
if [[ "${CAPTURE}" -eq 1 ]]; then
  LIVE_DIR="$(repo_root)/providers/cpanel/uapi/testdata/live"
  mkdir -p "${LIVE_DIR}"
  chmod 700 "${LIVE_DIR}"
  info "Capture will land in providers/cpanel/uapi/testdata/live/ (gitignored)."
fi

OUT=""
if go run ./cmd/webox doctor cpanel \
  --host="${HOST}" --user="${USER_ACCT}" --token="${TOKEN}" \
  --json 2>/tmp/webox-cpanel-smoke.stderr 1>/tmp/webox-cpanel-smoke.stdout; then
  OUT="$(cat /tmp/webox-cpanel-smoke.stdout)"
else
  rc=$?
  err "webox doctor cpanel exited ${rc} — stderr:"
  cat /tmp/webox-cpanel-smoke.stderr >&2
  rm -f /tmp/webox-cpanel-smoke.stdout /tmp/webox-cpanel-smoke.stderr
  TOKEN=""
  unset TOKEN
  exit 4
fi

rm -f /tmp/webox-cpanel-smoke.stderr

# Wipe the token from the local scope; bash doesn't zero memory but
# unsetting closes the reference and lets the next allocation reclaim
# the slot.
TOKEN=""
unset TOKEN

VERDICT="$(echo "${OUT}" | jq -r '.verdict')"
case "${VERDICT}" in
  OK)
    ok "All four read-only sections returned status=1 (verdict=OK)."
    ;;
  DEGRADED)
    warn "Verdict=DEGRADED — some sections failed:"
    echo "${OUT}" | jq -r '.sections[] | select(.status != "ok") | "  • \(.name) → \(.status): \(.error)"' >&2
    ;;
  BLOCKED)
    err "Verdict=BLOCKED — every section failed:"
    echo "${OUT}" | jq -r '.sections[] | "  • \(.name) → \(.status): \(.error)"' >&2
    rm -f /tmp/webox-cpanel-smoke.stdout
    exit 4
    ;;
  *)
    err "unknown verdict: ${VERDICT}"
    rm -f /tmp/webox-cpanel-smoke.stdout
    exit 4
    ;;
esac

if [[ "${CAPTURE}" -eq 1 ]]; then
  STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
  OUT_FILE="${LIVE_DIR}/${STAMP}_doctor_cpanel_report.json"
  echo "${OUT}" >"${OUT_FILE}"
  chmod 600 "${OUT_FILE}"
  warn "Captured: ${OUT_FILE##*/}"
  warn "Review + redact (host, user IDs, IPs) BEFORE promoting any payload to providers/cpanel/uapi/testdata/."
fi

rm -f /tmp/webox-cpanel-smoke.stdout
