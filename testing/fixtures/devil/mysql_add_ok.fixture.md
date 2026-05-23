---
captured: inferred
account: webox-test (synthetic)
command: devil mysql add myapp_prod
sanitized: db name -> myapp_prod, password ->
  REDACTED-NEVER-A-REAL-SECRET-aBcD1234EfGh5678
notes: The "REDACTED-NEVER-A-REAL-SECRET-" prefix is a tripwire for
  the redactor regex (and any future secret scanner). The parser
  MUST return user/password but they belong to a memguard.LockedBuffer
  at the call site, and they MUST NOT appear in logs or errors.
---
