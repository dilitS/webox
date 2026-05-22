# Security Policy

> Webox handles SSH keys, GitHub tokens, and other high-value secrets. Security reports are our highest priority.

## Supported versions

| Version | Supported |
|---------|-----------|
| `main` (pre-MVP) | ✅ (best-effort) |
| `v0.x` (when released) | ✅ |

Pre-MVP code is **not yet production-ready**. Once we ship `v0.1`, we will adopt a formal support window (current minor + previous minor).

## Reporting a vulnerability

**Please do not open public GitHub issues for security vulnerabilities.**

Instead:

1. Open a **private security advisory** via [GitHub Security Advisories](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability) on this repository, **or**
2. Email the maintainer at the address listed in the repository profile.

We aim to:

- **Acknowledge** within 72 hours.
- **Triage** within 7 days.
- **Patch** critical/high severity within 14 days (or coordinated disclosure window).

## Scope

In scope:

- Webox binary (`cmd/webox`) and library code.
- Cryptographic primitives in `secrets/`.
- SSH host key handling and TOFU flow.
- GitHub Actions deploy workflow templates embedded in the binary.
- Secret redactor (`internal/log/redact.go`).
- Config persistence (atomic write, file permissions).

Out of scope (we will route reports to the right place):

- Underlying hosting panels (small.pl, cPanel, DirectAdmin, etc.) — report to the vendor.
- Go standard library or third-party dependencies — we will forward upstream.
- Social engineering or physical attacks against maintainers.

## Detailed threat model and security architecture

See [`docs/SECURITY.md`](docs/SECURITY.md) for:

- Full threat model and attack surface (§2).
- Secret handling architecture (§3).
- Keyring + fallback design (§4).
- SSH TOFU policy (§5).
- GitHub token scopes (§6).
- Logging redactor design (§7).
- `webox doctor` security checks (§10).

## Acknowledgements

We will credit reporters in release notes and `SECURITY.md` unless anonymity is requested.

---

_Last reviewed: 2026-05-22._
