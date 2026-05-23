---
captured: synthesized
account: n/a (parser corpus)
command: devil www add webox-test.smallhost.pl nodejs 24
sanitized: same as www_add_ok.txt with CRLF line endings
notes: Verifies the parser is tolerant of `\r\n` terminators (some
  SSH clients translate; some Devil panel versions emit them).
  Expected: same result as www_add_ok.
---
