#!/usr/bin/env bash
set -euo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=./_lib.sh
. "$HOOK_DIR/_lib.sh"

payload="$(cat)"
log_debug "commit-validator invoked"

cmd="$(printf '%s' "$payload" | (jq -r '.command // empty' 2>/dev/null || true))"
if [[ -z "$cmd" ]]; then
  emit_allow
  exit 0
fi

if ! grep -qE '(^|\s)git\s+commit($|\s)' <<< "$cmd"; then
  emit_allow
  exit 0
fi

if grep -qE '\-\-amend|\-\-no\-verify' <<< "$cmd"; then
  log_debug "amend or no-verify path; allow with note"
  emit_allow
  exit 0
fi

if grep -qE 'git\s+commit\s+.*-m\s+\$\(cat\s+<<' <<< "$cmd"; then
  msg_subject="$(grep -oE 'EOF\\$' <<< "$cmd" || true)"
  if msg_subject="$(printf '%s' "$cmd" | sed -n 's/.*<<'\''EOF'\''[[:space:]]*\(.*\)EOF.*/\1/p' | head -n1)"; then
    subject_line="$(printf '%s' "$msg_subject" | sed -E 's/^[[:space:]]+//' | head -n1)"
  else
    subject_line=""
  fi
else
  subject_line="$(printf '%s' "$cmd" | sed -n 's/.*-m[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  if [[ -z "$subject_line" ]]; then
    subject_line="$(printf '%s' "$cmd" | sed -n "s/.*-m[[:space:]]*'\([^']*\)'.*/\1/p" | head -n1)"
  fi
fi

if [[ -z "$subject_line" ]]; then
  log_debug "could not extract subject; allow"
  emit_allow
  exit 0
fi

cc_regex='^(feat|fix|refactor|perf|test|docs|chore|ci|build|revert|style)(\([a-z0-9._/-]+\))?!?: [a-z].{3,}$'

if [[ ! "$subject_line" =~ $cc_regex ]]; then
  log_debug "subject violates conventional commits: '$subject_line'"
  emit_ask \
    "Commit subject narusza Conventional Commits. Oczekiwany format: type(scope): subject (≤72 znaków, imperative, lowercase, brak emoji)." \
    "Commit subject '${subject_line}' violates Conventional Commits. Required: 'type(scope): subject' where type ∈ {feat,fix,refactor,perf,test,docs,chore,ci,build,revert}, subject is imperative mood and ≤72 chars and starts lowercase. See AGENTS.md §6 and .cursor/skills/commit-policy/SKILL.md. Rewrite the commit message and re-issue."
  exit 0
fi

if (( ${#subject_line} > 72 )); then
  log_debug "subject too long: ${#subject_line} chars"
  emit_ask \
    "Commit subject ma ${#subject_line} znaków, limit to 72." \
    "Commit subject is ${#subject_line} chars (limit 72). Trim it before committing."
  exit 0
fi

emit_allow
