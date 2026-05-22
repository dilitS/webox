#!/usr/bin/env bash
set -euo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=./_lib.sh
. "$HOOK_DIR/_lib.sh"
# shellcheck source=./_patterns.sh
. "$HOOK_DIR/_patterns.sh"

payload="$(cat)"
log_debug "secret-scan-file invoked"

file_path="$(printf '%s' "$payload" | (jq -r '.file_path // .path // .filePath // empty' 2>/dev/null || true))"
if [[ -z "$file_path" || ! -f "$file_path" ]]; then
  log_debug "no readable file path in payload — skipping"
  exit 0
fi

case "$file_path" in
  */.cursor/hooks/*|*/.cursor/skills/*|*/.cursor/rules/*|*/docs/SECURITY.md|*/AGENTS.md|*/CHANGELOG.md|*/docs/AUDIT.md|*/docs/IMPROVEMENT_PLAN.md)
    log_debug "allowlisted file path (security/docs): $file_path"
    exit 0
    ;;
esac

case "$file_path" in
  *.png|*.jpg|*.jpeg|*.gif|*.webp|*.pdf|*.zip|*.tar|*.gz|*.bin|*.exe|*.dll|*.so|*.dylib)
    exit 0
    ;;
esac

if [[ ! -r "$file_path" ]] || ! head -c 65536 "$file_path" 2>/dev/null | grep -qP '^[\x20-\x7e\t\n\r]*$' 2>/dev/null; then
  if file "$file_path" 2>/dev/null | grep -qi 'binary\|data'; then
    exit 0
  fi
fi

content="$(head -c 1048576 "$file_path" 2>/dev/null || true)"

if result="$(scan_for_secrets "$content" "strict")"; then
  pattern="${result#*|}"
  log_debug "secret in file $file_path matching: $pattern"
  printf '{"additional_context":%s}\n' \
    "$(json_escape "🚨 SECRET LEAK: file '$file_path' appears to contain a literal secret matching pattern '$pattern'. This file is now written to disk — rotate the secret immediately, scrub git history if committed, and replace with an env-var or keyring lookup.")"
fi

exit 0
