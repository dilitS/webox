---
captured: inferred
account: webox-test (synthetic)
command: devil ssl www add 203.0.113.10 le le app.webox-test.smallhost.pl
sanitized: domain -> app.webox-test.smallhost.pl, ip -> 203.0.113.10
notes: Maps to nil (success). The status loop then probes the leaf
  cert via services/httpcheck.ProbeTLS to confirm visibility.
---
