#!/usr/bin/env bash

SECRET_PATTERNS=(
  'gh[ps]_[A-Za-z0-9]{36,255}'
  'github_pat_[A-Za-z0-9_]{82}'
  'glpat-[A-Za-z0-9_-]{20,}'
  'AKIA[0-9A-Z]{16}'
  'sk-[A-Za-z0-9]{32,}'
  'AIza[0-9A-Za-z_-]{35}'
  'xox[bpas]-[A-Za-z0-9-]{10,}'
  '-----BEGIN [A-Z ]*PRIVATE KEY-----'
  'eyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}'
)

SECRET_PATTERNS_PERMISSIVE=(
  '(password|passwd|secret|api[_-]?key|token)[[:space:]]*[:=][[:space:]]*["'\''][^"'\''[:space:]]{8,}["'\'']'
  'Authorization:[[:space:]]*Bearer[[:space:]]+[A-Za-z0-9._-]{16,}'
)

SECRET_ALLOWLIST_REGEX='(REDACTED|example|sample|placeholder|YOUR_TOKEN_HERE|XXXX|<token>|\$\{.*\}|TOBEVERIFIED)'

scan_for_secrets() {
  local text="$1"
  local mode="${2:-strict}"
  local pattern

  for pattern in "${SECRET_PATTERNS[@]}"; do
    if echo "$text" | grep -E -o "$pattern" 2>/dev/null | grep -E -v "$SECRET_ALLOWLIST_REGEX" >/dev/null 2>&1; then
      printf 'STRICT|%s\n' "$pattern"
      return 0
    fi
  done

  if [[ "$mode" == "all" ]]; then
    for pattern in "${SECRET_PATTERNS_PERMISSIVE[@]}"; do
      if echo "$text" | grep -E -o "$pattern" 2>/dev/null | grep -E -v "$SECRET_ALLOWLIST_REGEX" >/dev/null 2>&1; then
        printf 'PERMISSIVE|%s\n' "$pattern"
        return 0
      fi
    done
  fi

  return 1
}
