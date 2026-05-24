// Package sshmetrics polls remote server health for Webox's Bento
// Ultra header tile (Sprint 09, ADR-0007).
//
// Architecture: mirrors `services/sshtail` — the package depends on a
// thin [CommandRunner] interface, not on `ssh.Pool`. Production wiring
// implements the runner against the SSH connection pool; tests inject
// canned-output stubs and exercise the parsers directly without
// network I/O.
//
// What it polls:
//
//   - `uptime` for boot time and 1/5/15-minute load averages. Parser
//     supports Linux, FreeBSD (small.pl's OS), and macOS output formats.
//   - `free -m` for total/used RAM (Linux). FreeBSD/macOS hosts that
//     return an error here are reported with zeroed RAM fields rather
//     than failing the whole poll — the header tile degrades to "RAM:
//     n/a" rather than blocking the entire metrics row.
//   - Round-trip time of an `echo` SSH round-trip. Cheaper than `ping`
//     and works through bastions/firewalls that block ICMP.
package sshmetrics
