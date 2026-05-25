// Package cpanel is the HostingProvider adapter for cPanel-based
// shared hosting (cPanel & WHM, CloudLinux Node.js Selector,
// hoster-specific variants documented through the preset registry).
//
// Sprint 21 ships the read-only layer only: the [uapi] sub-package
// exposes a typed UAPI client (`uapi.Client.Call`) for the four
// modules the Sprint 21 plan enumerates (DomainInfo, PassengerApps,
// Mysql, SSL). Mutating endpoints are deliberately not reachable
// from `uapi.Client`; the parallel [uapi.MutatingClient] interface
// returns [uapi.ErrSprintScopeNotMutable] until Sprint 22 wires the
// real implementation. That keeps the type system enforcing the
// "no destructive ops in v0.2-rc" guardrail.
//
// The full [providers.HostingProvider] implementation lands in
// Sprint 22 (TASK-22.x). Sprint 21 leaves `providers/cpanel/uapi/`
// as a stand-alone, importable transport layer that the future
// adapter will compose with the existing `ssh` package (UAPI first,
// SSH fallback per [TASK-21.2]).
//
// References:
//   - docs/sprints/sprint-21-cpanel-adapter-prep.md (Sprint plan).
//   - docs/contributing/PROVIDER.md (adapter walkthrough).
//   - docs/providers/preconfiguration-vision.md (preset vs adapter).
package cpanel
