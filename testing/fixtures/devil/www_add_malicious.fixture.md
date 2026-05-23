---
captured: synthesized
account: n/a (parser corpus)
command: n/a (crafted)
sanitized: not applicable (synthetic adversarial input)
notes: Embeds ANSI colour escapes, NUL bytes, BEL, and a
  command-injection-looking substring `$(rm -rf /)` inside the domain
  field. The parser MUST:

  1. Strip the ANSI escapes before regex matching;
  2. Reject the output if non-printable bytes remain after the strip
     (NUL, BEL, etc.);
  3. NEVER include the raw bytes in the returned error;
  4. NEVER substitute the resulting "domain" into any shell command.

  Expected: providers.ErrUnknownOutputFormat (typed).
---
