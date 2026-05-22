#!/usr/bin/env bash
set -euo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=./_lib.sh
. "$HOOK_DIR/_lib.sh"

payload="$(cat)"
log_debug "gofmt hook invoked"

file_path="$(printf '%s' "$payload" | (jq -r '.file_path // .path // .filePath // empty' 2>/dev/null || true))"
if [[ -z "$file_path" || ! -f "$file_path" ]]; then
  exit 0
fi

case "$file_path" in
  *.go) ;;
  *) exit 0 ;;
esac

case "$file_path" in
  */testdata/*|*_generated.go|*.pb.go)
    log_debug "skip generated/testdata file: $file_path"
    exit 0
    ;;
esac

if ! have_cmd gofmt; then
  printf '{"additional_context":%s}\n' \
    "$(json_escape "gofmt not on PATH; skipped auto-format of $file_path. Install Go or add gofmt to PATH so afterFileEdit hook can run.")"
  exit 0
fi

if have_cmd goimports; then
  if ! goimports -w "$file_path" 2>/dev/null; then
    log_debug "goimports failed on $file_path; falling back to gofmt"
    gofmt -s -w "$file_path" || true
  fi
else
  gofmt -s -w "$file_path" || true
fi

log_debug "formatted $file_path"
exit 0
