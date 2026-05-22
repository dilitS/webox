#!/usr/bin/env bash
set -euo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=./_lib.sh
. "$HOOK_DIR/_lib.sh"

payload="$(cat)"
log_debug "scope-guard invoked"

file_path="$(printf '%s' "$payload" | (jq -r '.file_path // .path // .filePath // empty' 2>/dev/null || true))"
if [[ -z "$file_path" ]]; then
  exit 0
fi

STRETCH_PATHS=(
  'tui/views/topology'
  'tui/views/bento'
  'tui/views/logs_live'
  'tui/views/env_diff'
  'tui/views/database'
  'tui/keys/chord'
  'tui/animation/pulse'
  'tui/animation/border_pulse'
  'wizard/dag'
  'wizard/topological'
  'env/merger'
  'env/diff_engine'
  'sound/'
  'snd/'
  'topology/'
  'services/scan'
)

for stretch in "${STRETCH_PATHS[@]}"; do
  case "$file_path" in
    *"/$stretch"*|*"$stretch"*)
      log_debug "scope violation suspected: $file_path matches $stretch"
      printf '{"additional_context":%s}\n' \
        "$(json_escape "🛑 SCOPE GUARD: '$file_path' looks like a STRETCH-scope path matching '$stretch'. Per docs/AUDIT.md A6 and §8 folded IMP-* findings, these features are out of MVP (v0.1). Either: (a) move work to a v0.2+ milestone branch, or (b) open an ADR proposing scope promotion BEFORE writing code. See .cursor/skills/audit-trace/SKILL.md for the full traceability matrix.")"
      exit 0
      ;;
  esac
done

if [[ "$file_path" == *.go ]] && ! grep -qE '_test\.go$' <<< "$file_path"; then
  if [[ "$file_path" == *providers/* ]] || \
     [[ "$file_path" == *secrets/* ]] || \
     [[ "$file_path" == *wizard/* ]] || \
     [[ "$file_path" == *config/* ]] || \
     [[ "$file_path" == *ssh/* ]]; then
    if [[ -f "$file_path" ]]; then
      base="$(basename "$file_path" .go)"
      dir="$(dirname "$file_path")"
      test_file="${dir}/${base}_test.go"
      if [[ ! -f "$test_file" ]]; then
        log_debug "TDD reminder for $file_path (no $test_file)"
        printf '{"additional_context":%s}\n' \
          "$(json_escape "⚠️ TDD reminder: '$file_path' is in a TDD-mandated path but no '$test_file' exists. Per AGENTS.md §3 and .cursor/rules/50-tests.mdc, write failing test first. If you started with the test, ignore.")"
        exit 0
      fi
    fi
  fi
fi

exit 0
