# Devil CLI output fixtures

Golden files used by `providers/smallhost` parser tests. Each `.txt` is
the raw stdout/stderr Webox observes after running a `devil ...` command
on a small.pl account.

## Provenance policy

Every `<fixture>.txt` MUST have an accompanying `<fixture>.fixture.md`
file describing:

- `captured`: ISO date of the capture (or `inferred` when the fixture
  was drafted from `docs/providers/smallhost.md` ahead of a live
  capture).
- `account`: sanitised account identifier (NEVER the real login).
- `command`: the exact `devil ...` invocation.
- `sanitized`: list of replacements (login, domain, IP, etc.).
- `notes`: free-form gotchas (ANSI escapes? control bytes? trailing
  whitespace?).

Fixtures whose `captured: inferred` block out the parser's happy path
until a live capture replaces them. The CI guard `make
fixtures-provenance` (to be added in TASK-07.x) will fail if a `.txt`
exists without its `.md` companion.

## Sanitisation invariants

- Real user logins → `webox-test`.
- Real IP addresses → `203.0.113.10` (RFC 5737 documentation block).
- Real passwords → never committed. If a command output contains a
  password (e.g. `devil mysql add`), the fixture replaces it with
  `REDACTED-NEVER-A-REAL-SECRET-aBcD1234EfGh5678`. The redactor's
  regex matches this prefix; if it ever drops the prefix, repository
  scans will flag the leak.
- Real customer domains → `webox-test.smallhost.pl` and friends.

## Malicious fixtures

`*_malicious_*.txt` exists to exercise the parser's strict mode. It
embeds ANSI escape sequences, NUL bytes, command-injection-looking
substrings, and CR/LF mismatches. The parser MUST return
`providers.ErrUnknownOutputFormat` (or the relevant typed error) for
these — never a partially populated result and never the raw input
inside the error message.
