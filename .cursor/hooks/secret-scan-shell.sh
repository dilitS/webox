#!/usr/bin/env bash
set -euo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=./_lib.sh
. "$HOOK_DIR/_lib.sh"
# shellcheck source=./_patterns.sh
. "$HOOK_DIR/_patterns.sh"

payload="$(cat)"
log_debug "secret-scan-shell invoked"

cmd="$(printf '%s' "$payload" | (jq -r '.command // empty' 2>/dev/null || true))"
if [[ -z "$cmd" ]]; then
  emit_allow
  exit 0
fi

if result="$(scan_for_secrets "$cmd" "strict")"; then
  pattern="${result#*|}"
  log_debug "secret pattern in shell command: $pattern"
  emit_deny \
    "Komenda powłoki zawiera literalny sekret (np. token/klucz). Webox blokuje. Użyj zmiennych środowiskowych z keyring lub interaktywnego promptu." \
    "Blocking shell command containing literal secret (pattern: $pattern). Refactor to read the secret from environment / keyring / stdin and re-issue the command. Never paste secrets in argv."
  exit 2
fi

emit_allow
