---
captured: inferred
account: webox-test (synthetic; db never created)
command: devil mysql del myapp_prod
sanitized: db name -> myapp_prod
notes: Maps to nil — Remove* is idempotent. Without this branch the
  LIFO rollback would crash on partial-success replay.
---
