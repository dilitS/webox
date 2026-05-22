// Package log is the Webox logging facade: a thin layer over log/slog
// that always pipes records through the secret redactor before they
// reach lumberjack-managed log files or stderr.
//
// Every component that emits diagnostic output must use this package
// instead of fmt.Print* or the bare slog.Default — that constraint is
// what makes the redactor effective. See docs/DESIGN.md §15.2 and
// docs/SECURITY.md §3.1 for the redactor's pattern catalog.
package log
