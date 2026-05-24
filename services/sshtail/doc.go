// Package sshtail provides a context-cancellable `tail -f` stream over
// SSH for Webox's Live Log tile (Sprint 09, ADR-0007).
//
// Architecture: the package depends on a small [Executor] interface
// rather than on `ssh.Pool` directly. Production wiring composes an
// executor from `ssh.Pool` + `session.Start("tail -f <path>")`; tests
// inject a stub that emits canned bytes. This split keeps the streamer
// unit-testable without spinning up a mock SSH server for every line.
//
// Security contract (non-negotiable):
//
//   - Every byte read from the remote session is passed through
//     `internal/log.Redact` BEFORE being pushed onto the consumer
//     channel. A secret never lives in a webox buffer, even
//     transiently — see `docs/SECURITY.md §6` and
//     `docs/sprints/sprint-09-live-log-stream.md §TL;DR`.
//   - `ctx.Done()` closes the SSH session and the output channel in
//     both directions. The streamer goroutine returns within one
//     read cycle (≤500ms idle wakeup).
//   - Reconnect uses exponential backoff (2s/4s/8s) capped at 3
//     attempts; further failures surface as [ErrReconnectFailed].
//   - Log path is shell-escaped to defend against a hostile
//     `providers.HostingProvider.GetLogPath` implementation.
package sshtail
