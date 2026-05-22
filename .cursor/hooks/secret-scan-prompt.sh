#!/usr/bin/env bash
set -euo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=./_lib.sh
. "$HOOK_DIR/_lib.sh"
# shellcheck source=./_patterns.sh
. "$HOOK_DIR/_patterns.sh"

payload="$(cat)"
log_debug "secret-scan-prompt invoked"

prompt_text="$(printf '%s' "$payload" | (jq -r '.prompt // empty' 2>/dev/null || true))"
if [[ -z "$prompt_text" ]]; then
  emit_allow
  exit 0
fi

if result="$(scan_for_secrets "$prompt_text" "strict")"; then
  pattern="${result#*|}"
  log_debug "secret pattern matched in prompt: $pattern"
  emit_ask \
    "Twoja wiadomość wygląda jakby zawierała sekret (token/klucz). Webox blokuje taką wiadomość. Wyczyść sekret z wiadomości i prześlij ponownie." \
    "User prompt contains a probable secret matching pattern: $pattern. Refuse to act on the literal secret. Ask the user to remove it and resend."
  exit 0
fi

emit_allow
