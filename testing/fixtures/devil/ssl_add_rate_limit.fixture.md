---
captured: inferred
account: webox-test (synthetic)
command: devil ssl www add 203.0.113.10 le le webox-test.smallhost.pl
sanitized: domain -> webox-test.smallhost.pl, ip -> 203.0.113.10
notes: Maps to providers.ErrRateLimitLetsEncrypt. Status loop backs
  off until the next day rather than retrying on the regular ticker.
---
