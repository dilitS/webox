#!/usr/bin/env bash
# Suggest a Conventional Commit subject based on staged changes.
# Heuristics (best-effort, you still review):
#   - type    from file types and patterns (test → test, docs/ → docs, ...)
#   - scope   from the most-changed top-level directory or package
#   - subject placeholder for you to fill
#
# Usage:  scripts/commit-msg-suggest.sh
# Output: prints suggested subject to stdout. Pipe to commit:
#   git commit -em "$(scripts/commit-msg-suggest.sh)"

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

staged_files=()
while IFS= read -r f; do staged_files+=("$f"); done < <(git diff --cached --name-only)

if [[ ${#staged_files[@]} -eq 0 ]]; then
  die "no staged changes"
fi

# ── Type detection ─────────────────────────────────────────────────────
type="chore"
has_go_test=0; has_go_src=0; has_docs=0; has_workflow=0; has_security=0; has_makefile=0
for f in "${staged_files[@]}"; do
  case "$f" in
    *_test.go) has_go_test=1 ;;
    *.go) has_go_src=1 ;;
    docs/*|*.md|*/README.md) has_docs=1 ;;
    .github/workflows/*) has_workflow=1 ;;
    secrets/*|*/secrets/*|*SECURITY*|*security*) has_security=1 ;;
    Makefile|.golangci.yml|*.editorconfig|tools/*|.github/*) has_makefile=1 ;;
  esac
done

if   (( has_security == 1 )); then type="security"
elif (( has_go_src == 1 ));   then type="feat"
elif (( has_go_test == 1 ));  then type="test"
elif (( has_workflow == 1 )); then type="ci"
elif (( has_makefile == 1 )); then type="build"
elif (( has_docs == 1 ));     then type="docs"
fi

# Diff signals that override toward "fix"
if git diff --cached --unified=0 | grep -E '^\+.*\b(fix|bug|panic|nil pointer)\b' -q; then
  if [[ "$type" == "feat" || "$type" == "chore" ]]; then type="fix"; fi
fi

# ── Scope detection (bash 3.2-compatible: no associative arrays) ───────
scope=""
top_key=""
for f in "${staged_files[@]}"; do
  case "$f" in
    docs/*)     key="docs/${f#docs/}" ; key="${key%%/*}" ;;
    .github/*)  key="ci" ;;
    cmd/*)      key="${f#cmd/}" ; key="cmd-${key%%/*}" ;;
    internal/*) key="${f#internal/}" ; key="${key%%/*}" ;;
    *)          key="${f%%/*}" ;;
  esac
  printf '%s\n' "$key"
done | sort | uniq -c | sort -rn | {
  read -r _count top_key || true
  case "$top_key" in
    ""|"."|"docs"|"cmd") echo "" ;;
    *) echo "$top_key" ;;
  esac
} > /tmp/.webox_scope.$$ 2>/dev/null || true

scope="$(cat /tmp/.webox_scope.$$ 2>/dev/null | head -1)"
rm -f /tmp/.webox_scope.$$

# Sanitize scope (lowercase, no slashes, no spaces)
if [[ -n "$scope" ]]; then
  scope="$(echo "$scope" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9-]/-/g')"
fi

# ── Subject placeholder ────────────────────────────────────────────────
subject="<imperative present-tense summary>"

if [[ -n "$scope" ]]; then
  printf '%s(%s): %s\n' "$type" "$scope" "$subject"
else
  printf '%s: %s\n' "$type" "$subject"
fi
