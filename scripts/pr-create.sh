#!/usr/bin/env bash
# Open a pull request with the body pre-filled from sprint + commits context.
#
# Usage:
#   scripts/pr-create.sh                # auto title from last commit
#   scripts/pr-create.sh "feat(config): atomic save"
#
# Requires: gh, jq, git.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

require_cmd gh "brew install gh"

ROOT="$(repo_root)"
BRANCH="$(git_current_branch)"

[[ "$BRANCH" != "main" && "$BRANCH" != "master" ]] \
  || die "refusing to create PR from $BRANCH — switch to a feature branch"

# Push first if needed
upstream="$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)"
if [[ -z "$upstream" ]]; then
  info "pushing branch with -u"
  git push -u origin "$BRANCH"
fi

# Derive task id from branch (feat/s01-03-name → TASK-01.3)
if [[ "$BRANCH" =~ ^[a-z]+/s([0-9]+)-([0-9]+)- ]]; then
  TASK_ID="TASK-${BASH_REMATCH[1]}.$(printf '%d' "${BASH_REMATCH[2]#0}")"
  SPRINT_DOC="docs/sprints/sprint-${BASH_REMATCH[1]}-*.md"
else
  TASK_ID="(unknown — please fill in)"
  SPRINT_DOC="docs/sprints/"
fi

# Default title from last commit
TITLE="${1:-$(git log -1 --pretty=%s)}"

# Build body
BASE="$(git symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's@^origin/@@' || echo main)"
COMMITS="$(git log --pretty='- %s' "origin/${BASE}..HEAD" 2>/dev/null || git log -10 --pretty='- %s')"
DIFFSTAT="$(git diff --stat "origin/${BASE}..HEAD" 2>/dev/null | tail -1 || true)"

BODY="$(cat <<EOF
## Summary

_Replace with 2-4 sentences describing what and why._

## Scope linkage

- **Task:** $TASK_ID
- **Sprint plan:** $SPRINT_DOC
- **Authoritative doc:** _link to PRD/DESIGN/SECURITY/ADR/AUDIT section_

## Commits in this PR

$COMMITS

## Diffstat

\`\`\`
$DIFFSTAT
\`\`\`

## Definition of Done

- [ ] \`make ci\` passes locally (lint, test, vulncheck, build)
- [ ] Tests added/updated; TDD order respected for critical logic
- [ ] Coverage target met
- [ ] No \`TODO\`/\`FIXME\` without linked issue
- [ ] CHANGELOG \`Unreleased\` entry added
- [ ] Docs updated where contract changed
- [ ] Conventional Commits in commit subjects
- [ ] No secrets in diff (hook-verified)

## Security checklist

- [ ] N/A — _or describe security impact_

## Reviewer notes

_Anything to focus on?_
EOF
)"

info "title:  ${C_BOLD}$TITLE${C_RESET}"
info "base:   ${C_BOLD}$BASE${C_RESET}"
info "branch: ${C_BOLD}$BRANCH${C_RESET}"

# Use --fill if available for commit-derived body, but prefer our template
gh pr create \
  --base "$BASE" \
  --head "$BRANCH" \
  --title "$TITLE" \
  --body "$BODY" \
  --draft

ok "draft PR opened. Mark Ready when DoD passes."
