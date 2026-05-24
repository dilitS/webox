package tui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dilitS/webox/ssh"
	"github.com/dilitS/webox/tui/components"
	"github.com/dilitS/webox/tui/theme"
)

// classifyContextErr keeps the trace label distinct from "other" so
// operators can tell when a cancellation came from the cockpit shut-
// down vs a transient SSH error. Used by `classifyErrForTrace` and
// will be reused once more emit-call-sites land in TASK-14.6 follow-up.
//
//nolint:unused // wired in TASK-14.6 follow-up batch.
func classifyContextErr(err error) string {
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "context_deadline"
	}
	return ""
}

// hostKeyModal renders a blocking, informational modal whenever an
// SSH operation surfaces a host-key mismatch. It is the Sprint-14
// short-term fallback for the absence of a dedicated
// `webox doctor security --update-host-key` command (v0.2+). The
// modal is read-only on purpose: Webox MUST NEVER auto-accept an
// unknown or changed host key (`docs/SECURITY.md §5`), so the
// operator's only path forward is an out-of-band fingerprint check
// followed by a manual `ssh-keygen -R <host>`.
//
// Threat model
//
//   - Modal SHALL NOT include the offered key, its fingerprint, the
//     SHA-256, or any byte of cryptographic material. Showing it
//     would tempt a fatigued operator into "looks fine, accept"
//     even when the key has actually been replaced by an attacker.
//   - Modal SHALL include the hostname, the `known_hosts` path, and
//     the literal `ssh-keygen -R <host>` command so the operator
//     can act without leaving the cockpit.
//   - Modal SHALL NOT continue the connection on its own; closing
//     the modal returns to the previous state. The operator must
//     re-trigger the action that caused the failure after fixing
//     `known_hosts`.

// hostKeyModalForm is the per-render state captured by the cockpit
// when a host-key mismatch surfaces. The struct is intentionally
// minimal: the offending key bytes / fingerprint are *not* stored
// anywhere on the model, eliminating the risk of accidental
// rendering or logging downstream.
type hostKeyModalForm struct {
	Open     bool
	Host     string
	Port     int
	Username string
	// Kind distinguishes the two SECURITY §5 sentinel paths so the
	// remediation copy can stay accurate:
	//   - "mismatch" → key changed since the last successful
	//     connection (potential MITM).
	//   - "unknown"  → first connection ever, key not yet TOFU'd.
	Kind string
}

// hostKeyModalKind constants stay free-standing rather than typed so
// the form can be JSON-marshalled in `--debug-trace` without a
// custom UnmarshalJSON helper. Lower-case mirrors the verbs in the
// SECURITY doc.
const (
	hostKeyKindMismatch = "mismatch"
	hostKeyKindUnknown  = "unknown"
)

// classifyHostKeyErr inspects an arbitrary error and returns the
// sentinel verb the modal copy should reflect, or empty string if
// the error is unrelated. We deliberately use [errors.Is] (not type
// assertion / Error() string compare) so any future wrapper that
// preserves the sentinel chain keeps working.
func classifyHostKeyErr(err error) string {
	switch {
	case errors.Is(err, ssh.ErrHostKeyMismatch):
		return hostKeyKindMismatch
	case errors.Is(err, ssh.ErrHostKeyUnknown):
		return hostKeyKindUnknown
	default:
		return ""
	}
}

// classifyErrForTrace returns a short category label suitable for the
// `--debug-trace` JSONL stream. The label is intentionally coarse
// (e.g. `host_key_mismatch`, `pool_busy`, `context_canceled`) so the
// trace never embeds the underlying error message — which could
// contain hostnames, file paths, or even quoted secrets. The label
// "other" is the safe fallback for unknown error chains.
func classifyErrForTrace(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ssh.ErrHostKeyMismatch):
		return "host_key_mismatch"
	case errors.Is(err, ssh.ErrHostKeyUnknown):
		return "host_key_unknown"
	case errors.Is(err, ssh.ErrPoolBusy):
		return "pool_busy"
	case errors.Is(err, ssh.ErrReconnectExhausted):
		return "reconnect_exhausted"
	default:
		return "other"
	}
}

// renderHostKeyModal builds the modal payload. When the form is not
// open the function returns the empty string and the cockpit composes
// no overlay. The renderer is pure — same inputs always produce the
// same string — which lets us snapshot-test the contents trivially.
func renderHostKeyModal(form hostKeyModalForm, screenWidth int, knownHostsPath string) string {
	if !form.Open {
		return ""
	}
	tokens := theme.Default()
	tone := components.ToneError
	verb := "Host key mismatch"
	severity := "potential man-in-the-middle attack"
	if form.Kind == hostKeyKindUnknown {
		tone = components.ToneWarning
		verb = "Unknown host key"
		severity = "first connection — verify out-of-band before accepting"
	}

	host := form.Host
	if host == "" {
		host = "<unknown host>"
	}
	target := host
	if form.Port > 0 && form.Port != defaultSSHListenPort {
		target = fmt.Sprintf("[%s]:%d", host, form.Port)
	}

	if knownHostsPath == "" {
		knownHostsPath = "~/.ssh/known_hosts"
	}

	var body strings.Builder
	fmt.Fprintf(&body, "Host: %s\n", host)
	if form.Username != "" {
		fmt.Fprintf(&body, "User: %s\n", form.Username)
	}
	fmt.Fprintf(&body, "Severity: %s\n\n", severity)
	body.WriteString("Webox refuses to auto-accept changed or unknown host keys (SECURITY §5).\n")
	body.WriteString("Steps to recover safely:\n\n")
	body.WriteString("  1. Verify the new fingerprint OUT OF BAND (provider panel / phone / chat\n")
	body.WriteString("     with the server admin). Do NOT trust the value shown by your SSH client.\n")
	body.WriteString(fmt.Sprintf("  2. If verified legit, remove the stale entry:\n     ssh-keygen -R %s -f %s\n", target, knownHostsPath))
	body.WriteString("  3. Re-run the action that triggered this modal — Webox will TOFU the\n     new key on first dial and store it in known_hosts.\n\n")
	body.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(tokens.TextDim)).
		Render("This modal does NOT continue the connection. Close it (Esc) and retry."))

	const (
		modalSidePadding = 4
		modalMinWidth    = 70
	)
	minWidth := screenWidth - modalSidePadding
	if minWidth < modalMinWidth {
		minWidth = modalMinWidth
	}

	return components.RenderModal(components.ModalOptions{
		Title:    fmt.Sprintf("%s · %s", verb, host),
		Body:     body.String(),
		Footer:   "Esc: close · then verify out-of-band, run ssh-keygen -R, retry",
		MinWidth: minWidth,
		Tone:     tone,
		Theme:    tokens,
	})
}

// defaultSSHListenPort is the IANA-assigned port. Surfacing it as a
// named constant rather than `22` lets us suppress it from the
// `[host]:22` label in the modal — most users do not need to see the
// default. The name avoids colliding with `defaultSSHPort` in
// `tui/wizard.go`, which represents the **wizard form default**
// rather than a network-protocol fact.
const defaultSSHListenPort = 22

// defaultKnownHostsPath returns a platform-portable default for the
// per-user `known_hosts` location. We resolve it lazily (rather than
// hard-coding `$HOME/.ssh/known_hosts`) so tests can override via the
// caller without touching the environment.
func defaultKnownHostsPath(home string) string {
	if home == "" {
		return "~/.ssh/known_hosts"
	}
	return filepath.Join(home, ".ssh", "known_hosts")
}

// tryRaiseHostKeyModal inspects an arbitrary error from a command
// result; when the error chain contains [ssh.ErrHostKeyMismatch] or
// [ssh.ErrHostKeyUnknown] the modal form is populated with the
// active profile's host / port / user and the function returns true.
// Callers should propagate the bool as a "handled?" signal so they
// can skip the legacy alert path and avoid duplicating the message
// in both the modal and the toast.
//
// The function is a method on *Model so it can read the currently
// selected profile; it never mutates anything except `hostKeyModal`.
func (m *Model) tryRaiseHostKeyModal(err error) bool {
	if m == nil || err == nil {
		return false
	}
	kind := classifyHostKeyErr(err)
	if kind == "" {
		return false
	}
	host, port, user := m.activeSSHTarget()
	m.hostKeyModal = hostKeyModalForm{
		Open:     true,
		Host:     host,
		Port:     port,
		Username: user,
		Kind:     kind,
	}
	return true
}

// dismissHostKeyModal closes the modal without modifying any other
// model state. Triggered by `Esc` while the modal is open.
func (m *Model) dismissHostKeyModal() {
	m.hostKeyModal = hostKeyModalForm{}
}

// activeSSHTarget returns the host / port / username that should be
// surfaced in the modal — typically the profile bound to the currently
// selected project. Falls back to the first profile when no project is
// selected and to placeholder values when the model has no config yet
// (so the modal can still render a generic recovery hint instead of
// crashing).
func (m Model) activeSSHTarget() (host string, port int, user string) {
	if m.cfg == nil {
		return "", 0, ""
	}
	if alias := m.activeProfileAlias(); alias != "" {
		for i := range m.cfg.Profiles {
			p := m.cfg.Profiles[i]
			if p.Alias == alias {
				return p.Host, p.Port, p.User
			}
		}
	}
	if len(m.cfg.Profiles) > 0 {
		p := m.cfg.Profiles[0]
		return p.Host, p.Port, p.User
	}
	return "", 0, ""
}
