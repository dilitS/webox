#!/usr/bin/env bash
# Add a one-line entry to CHANGELOG.md under the appropriate "Unreleased" section.
#
# Usage:
#   scripts/changelog-add.sh added   "config.Save with flock + atomic rename"
#   scripts/changelog-add.sh fixed   "redactor missed GitHub fine-grained PATs"
#   scripts/changelog-add.sh changed "renamed Provider.Status to Health"
#   scripts/changelog-add.sh security "panic on CSPRNG failure (IMP-2)"
#
# Sections supported (Keep a Changelog 1.1.0): added, changed, deprecated,
# removed, fixed, security.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

KIND="${1:-}"
MSG="${2:-}"
[[ -n "$KIND" && -n "$MSG" ]] || die "usage: $0 added|changed|fixed|removed|deprecated|security \"message\""

case "$KIND" in
  added|changed|deprecated|removed|fixed|security) ;;
  *) die "invalid kind: $KIND (use Keep-a-Changelog sections)" ;;
esac

# Title-case the section
HEADER="### $(echo "${KIND:0:1}" | tr '[:lower:]' '[:upper:]')${KIND:1}"

ROOT="$(repo_root)"
FILE="$ROOT/CHANGELOG.md"
[[ -f "$FILE" ]] || die "CHANGELOG.md not found"

if ! grep -q '^## \[Unreleased\]' "$FILE"; then
  die "CHANGELOG.md missing '## [Unreleased]' section"
fi

TMP="$(mktemp)"
awk -v section="$HEADER" -v line="- $MSG" '
  BEGIN { in_unrel = 0; in_sect = 0; injected = 0 }
  /^## \[Unreleased\]/ { in_unrel = 1; print; next }
  in_unrel && $0 == section { in_sect = 1; print; next }
  in_unrel && /^## \[/ {
    if (!injected) {
      print ""
      print section
      print line
      injected = 1
    }
    in_unrel = 0
    in_sect = 0
    print
    next
  }
  in_unrel && in_sect && /^### / && !injected {
    print line
    injected = 1
    in_sect = 0
    print
    next
  }
  in_unrel && in_sect && /^$/ && !injected {
    print line
    injected = 1
    print
    next
  }
  { print }
  END {
    if (!injected && in_unrel) {
      print ""
      print section
      print line
    }
  }
' "$FILE" > "$TMP"

mv "$TMP" "$FILE"
ok "added to CHANGELOG.md → $HEADER"
printf '   %s\n' "- $MSG"
