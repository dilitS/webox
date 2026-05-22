---
name: release-check
description: Run the pre-release verification checklist for Webox before tagging a new version. Use when the user is about to cut a release (v0.1.0, v0.1.1, v0.2.0), or when verifying that a release candidate is shippable.
---

# Release Check — Webox

## When to run

- Before tagging `vX.Y.Z` in git.
- Before merging the `release/vX.Y.Z` branch.
- When verifying a release candidate ahead of human sign-off.

## Pre-release checklist

```
Task progress:
- [ ] 1. CI is green on the release commit
- [ ] 2. CHANGELOG promoted: [Unreleased] → [vX.Y.Z] - YYYY-MM-DD
- [ ] 3. Version bumped in cmd/webox/main.go (Version const)
- [ ] 4. go.mod has expected Go version
- [ ] 5. golangci-lint v2 clean
- [ ] 6. Coverage threshold met (≥70% MVP, ≥80% v0.2+)
- [ ] 7. govulncheck clean
- [ ] 8. teatest snapshots updated and reviewed
- [ ] 9. Manual checklist run (TESTING §8)
- [ ] 10. Cosign signature successful in release dry-run
- [ ] 11. SLSA provenance generated
- [ ] 12. No unresolved P0 / P1 items left in AUDIT.md
- [ ] 13. Docs cross-references valid (no dead anchors)
- [ ] 14. CODE_OF_CONDUCT.md present
- [ ] 15. LICENSE present (MIT)
- [ ] 16. README.md last-updated badge fresh
- [ ] 17. Homebrew tap PR drafted (post v0.1)
- [ ] 18. Retrospective entry drafted for release
```

## Step details

### 1. CI green

```bash
gh run list --workflow=ci.yml --limit=3
```

Latest run on the release commit must be green. If any job failed → stop, fix, don't bypass.

### 2. CHANGELOG promotion

In `CHANGELOG.md`:

```markdown
## [Unreleased]

## [v0.1.0] - 2026-08-15

### Added
- (everything previously under [Unreleased] / Added)
```

Add new empty `[Unreleased]` section above the new version. Update compare links at the bottom:

```markdown
[Unreleased]: https://github.com/webox/webox/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/webox/webox/releases/tag/v0.1.0
```

### 3. Version constant

```go
// cmd/webox/main.go (or internal/version/version.go)
const Version = "v0.1.0"
```

Tag must match: `git tag v0.1.0` (with `v` prefix).

### 4. go.mod sanity

```bash
head -3 go.mod
# module github.com/webox/webox
# 
# go 1.24
```

### 5. Linter

```bash
make lint
```

Zero issues. If any `//nolint` was added since last release, audit them.

### 6. Coverage

```bash
make cover-check
```

For v0.1: threshold 70%. For v0.2+: 80%.

### 7. Vulnerability scan

```bash
make vulncheck
```

Zero known CVE in current `go.mod` dependencies.

### 8. teatest snapshots

```bash
make test-tui
git diff tui/testdata/golden/
```

Review **every** golden file diff. Snapshots should change only when UI behavior intentionally changed.

### 9. Manual checklist

Run `docs/TESTING.md §8.1` items by hand on real account (small.pl test). Each `[ ]` must become `[x]` before release.

Critical items for v0.1:

- Fresh install on clean macOS + Ubuntu.
- Init wizard end-to-end.
- New project wizard (Vite+React, no DB) — success.
- New project wizard (Node+MySQL) — success.
- DNS-not-ready failure → rollback completes cleanly.
- Restart, logs, SSL renew on existing project.
- Stale detector triggers when subdomain manually deleted from panel.
- `webox doctor` zwraca exit 0 on healthy install.
- Fallback secrets work on WSL without keyring.

### 10. Cosign dry-run

```bash
make release-dry-run
```

Verify in dist/:

- Binaries for all OS/arch matrix.
- `*.sig` files present.
- `webox_v0.1.0_checksums.txt`.
- `webox_v0.1.0_SBOM.cdx.json` (v0.2+).

### 11. SLSA provenance

GoReleaser `release` workflow on tag push produces SLSA provenance. Verify in GH Release artifacts after publish.

### 12. AUDIT status

```bash
make audit-status
```

For v0.1: zero unresolved P0/P1 left in AUDIT.md. P2/P3 and folded `IMP-*` items are OK only if documented as fixed or explicitly deferred in `AUDIT.md §8`.

### 13. Doc cross-refs

```bash
make docs-links
```

Any 404 anchor must be fixed before release.

### 14, 15. Code of Conduct + License

```bash
ls CODE_OF_CONDUCT.md LICENSE
```

Both present, MIT license.

### 16. README freshness

Update "Last updated" line if README has one. Update status badge from "in-design" to "released".

### 17. Homebrew tap

Draft PR to `github.com/webox/homebrew-tap` updating `webox.rb` formula with new version + SHA256 of release tarball. Merge after GH Release published.

### 18. Release retro

Draft retro in `docs/retros/release-v0.1.0.md` (run `retro` skill).

## After release

1. `git tag v0.1.0 && git push origin v0.1.0` triggers `release.yml` workflow.
2. Monitor `gh run watch` until success.
3. Verify [GitHub Releases page](https://github.com/webox/webox/releases) shows v0.1.0 with all assets.
4. Verify `cosign verify` works:
   ```bash
   cosign verify-blob --certificate webox_v0.1.0_darwin_arm64.tar.gz.cosign.pem \
                      --signature webox_v0.1.0_darwin_arm64.tar.gz.sig \
                      --certificate-identity-regexp '^https://github.com/webox/webox' \
                      --certificate-oidc-issuer https://token.actions.githubusercontent.com \
                      webox_v0.1.0_darwin_arm64.tar.gz
   ```
5. Tweet / blog post (optional).
6. Open `vX.Y.(Z+1)` milestone for hot-fixes.

## What blocks a release

| Blocker | Decision |
|---|---|
| CI red | **Block.** Fix and re-run. |
| Lint issue | **Block.** Fix and re-run. |
| Coverage drop | **Block** unless justified by maintainer (write down justification). |
| New CVE in deps | **Block.** Upgrade deps and re-test. |
| AUDIT P0 / P1 unresolved | **Block.** Address before release. |
| Manual checklist `[ ]` missed | **Block.** Run it. |
| Cosign signature failure | **Block.** Fix pipeline. |
| Documentation 404 anchor | **Block.** Fix. |

Nothing on this list is bypassable without explicit maintainer override **with written reason** in the release retro.

## Done criteria

- [ ] All 18 checklist items passed.
- [ ] Tag pushed.
- [ ] GH Release page populated.
- [ ] Signatures verified.
- [ ] Homebrew tap PR opened.
- [ ] Release retro drafted.
- [ ] Next milestone created in GH.
