// Package e2e hosts end-to-end TUI scenarios that exercise the full
// cockpit through Bubble Tea's `teatest` harness rather than rendering
// individual surfaces in isolation. The goal is to catch the class of
// regressions that pure snapshot tests miss — keyboard flows that span
// multiple `Update` ticks, layout transitions across the bento
// thresholds, and CI/CD / live-log surfaces that depend on background
// commands.
//
// What lives here vs. unit / snapshot tests
//
//   - `tui/views/*_test.go` cover **per-surface body** rendering.
//   - `tui/*_test.go` cover **per-message** state transitions
//     (Update is a pure function so this stays unit-level).
//   - `tui/cockpit_snapshot_test.go` covers **single-frame snapshots**
//     of the cockpit for every layout tier.
//   - **This package** covers **multi-tick interaction flows**: open
//     the cockpit, send a sequence of keys, assert on what the
//     terminal looks like at each beat.
//
// # Driver
//
// Scenarios use `github.com/charmbracelet/x/exp/teatest` to wrap the
// `tui.Model`; the SSH-backed flows use `testing/sshmock` so the
// scenarios stay hermetic. Mock mode (`tui.MockOptions("")`) seeds
// the cockpit with deterministic fixtures so we can pin clock-based
// behaviour (status bar timestamp, last-deploy relative time, etc.).
//
// # Speed budget
//
// Each scenario is capped at 2 seconds wall-clock (`teatest`'s default
// `WithDuration` value). Slow scenarios go on a watch list — the CI
// budget for the whole e2e package is 10 seconds. If a scenario can't
// fit, split it or push it to an integration target gated on
// `WEBOX_E2E_LONG=1`.
package e2e
