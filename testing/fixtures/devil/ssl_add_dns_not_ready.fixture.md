---
captured: inferred
account: webox-test (synthetic; custom domain)
command: devil ssl www add 203.0.113.10 le le app.example.com
sanitized: domain -> app.example.com (illustrative)
notes: Maps to providers.ErrDNSNotResolving. For custom domains the
  status loop schedules a retry every 15 min for up to 48h before
  surfacing as SSL_FAILED (docs/providers/smallhost.md §5.4.b).
---
