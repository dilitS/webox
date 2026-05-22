#!/usr/bin/env bash
set -euo pipefail

WEBOX_HOOK_DEBUG="${WEBOX_HOOK_DEBUG:-0}"
LOG_DIR="${WEBOX_HOOK_LOG_DIR:-${TMPDIR:-/tmp}/webox-hooks}"
mkdir -p "$LOG_DIR"

log_debug() {
  if [[ "$WEBOX_HOOK_DEBUG" == "1" ]]; then
    printf '[%s] %s\n' "$(date +%H:%M:%S)" "$*" >> "$LOG_DIR/debug.log"
  fi
}

have_cmd() {
  command -v "$1" >/dev/null 2>&1
}

emit_json() {
  printf '%s\n' "$1"
}

emit_allow() {
  emit_json '{"permission":"allow"}'
}

emit_ask() {
  local user_msg="$1"
  local agent_msg="$2"
  printf '{"permission":"ask","user_message":%s,"agent_message":%s}\n' \
    "$(json_escape "$user_msg")" "$(json_escape "$agent_msg")"
}

emit_deny() {
  local user_msg="$1"
  local agent_msg="$2"
  printf '{"permission":"deny","user_message":%s,"agent_message":%s}\n' \
    "$(json_escape "$user_msg")" "$(json_escape "$agent_msg")"
}

json_escape() {
  local s="$1"
  if have_cmd jq; then
    printf '%s' "$s" | jq -Rs .
    return
  fi
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"
  s="${s//$'\r'/\\r}"
  s="${s//$'\t'/\\t}"
  printf '"%s"' "$s"
}

read_field() {
  local field="$1"
  local payload="$2"
  if have_cmd jq; then
    printf '%s' "$payload" | jq -r ".${field} // empty" 2>/dev/null || true
    return
  fi
  printf '%s' "$payload" | sed -nE "s/.*\"${field}\"[[:space:]]*:[[:space:]]*\"([^\"]*)\".*/\1/p" | head -n1
}
