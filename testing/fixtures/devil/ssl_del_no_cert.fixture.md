---
captured: inferred
account: webox-test (synthetic; cert never installed)
command: devil ssl www del 203.0.113.10 app.webox-test.smallhost.pl
sanitized: domain -> app.webox-test.smallhost.pl, ip -> 203.0.113.10
notes: Maps to nil — Remove* is idempotent. Without this branch the
  LIFO rollback would crash on partial-success replay.
---
