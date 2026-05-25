#!/usr/bin/env bash
# create-all.sh — ship all 5 launch-day good-first-issues in one go.
#
# Usage:
#   bash .github/issue-drafts/create-all.sh                # ship all 5
#   bash .github/issue-drafts/create-all.sh --dry-run      # preview only
#   bash .github/issue-drafts/create-all.sh --continue-from 3
#
# Requirements:
#   - gh CLI authenticated (`gh auth status`).
#   - Repo labels created (see .github/issue-drafts/README.md).
#
# The script creates issues sequentially because gh label-attach is
# rate-limited under concurrent requests. Existing issues with the
# same title are detected and skipped.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Each entry: file|title|label1,label2,label3
declare -a ISSUES=(
  "01-cpanel-skeleton.md|Skeleton: cPanel UAPI provider adapter|good-first-issue,help wanted,provider"
  "02-directadmin-skeleton.md|Skeleton: DirectAdmin provider adapter|good-first-issue,help wanted,provider"
  "03-cyberpanel-research.md|Research: CyberPanel API for Phase 1 capabilities|help wanted,provider,research"
  "04-nextjs-scaffolding.md|Add scaffolding template: Next.js|good-first-issue,help wanted,documentation"
  "05-de-translation.md|Translate cockpit dialog labels to German (DE)|good-first-issue,help wanted,documentation"
)

DRY_RUN=0
START_AT=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    --continue-from)
      [[ -n "${2:-}" ]] || { echo "--continue-from requires an index 1-5" >&2; exit 1; }
      START_AT="$2"
      shift 2
      ;;
    -h|--help)
      sed -n '1,18p' "$0"
      exit 0
      ;;
    *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
done

command -v gh >/dev/null 2>&1 || {
  echo "gh CLI not installed. https://cli.github.com/" >&2
  exit 1
}

gh auth status >/dev/null 2>&1 || {
  echo "gh not authenticated. Run: gh auth login" >&2
  exit 1
}

cd "$REPO_ROOT"

INDEX=0
for entry in "${ISSUES[@]}"; do
  INDEX=$((INDEX + 1))
  if [[ "$INDEX" -lt "$START_AT" ]]; then
    echo "[$INDEX/5] skipped (--continue-from $START_AT)"
    continue
  fi

  IFS='|' read -r file title labels <<<"$entry"
  body_path=".github/issue-drafts/$file"

  if [[ ! -f "$body_path" ]]; then
    echo "[$INDEX/5] missing body file: $body_path" >&2
    exit 2
  fi

  # Skip if an open issue with the same title already exists.
  existing="$(gh issue list --search "in:title \"$title\"" --state open --json number,title --jq ".[] | select(.title == \"$title\") | .number" 2>/dev/null || true)"
  if [[ -n "$existing" ]]; then
    echo "[$INDEX/5] already open as #$existing: $title"
    continue
  fi

  if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "[$INDEX/5] DRY-RUN would create:"
    echo "         title:  $title"
    echo "         labels: $labels"
    echo "         body:   $body_path ($(wc -l < "$body_path") lines)"
    continue
  fi

  echo "[$INDEX/5] creating: $title"
  url="$(gh issue create \
    --title "$title" \
    --body-file "$body_path" \
    --label "$labels")"
  echo "         => $url"
done

echo
if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "Dry-run complete. Re-run without --dry-run to actually create issues."
else
  echo "All issues processed. Verify at:"
  echo "  gh issue list --label good-first-issue"
fi
