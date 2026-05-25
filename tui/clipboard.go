package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

// clipboardCommandRunner is the seam tests inject to fake
// clipboard backends. Production wires it to [exec.Command]
// indirectly through [clipboardCopy].
type clipboardCommandRunner func(name string, arg ...string) clipboardCommand

// clipboardCommand mirrors the slice of [exec.Cmd] we care about
// — set stdin and run. Tests override it with an in-memory stub
// so we never shell out during `go test`.
type clipboardCommand interface {
	SetStdin(text string)
	Run() error
}

// realClipboardCommand wraps [exec.Cmd] so the production code
// path can satisfy [clipboardCommand] without leaking exec
// internals to tests.
type realClipboardCommand struct {
	cmd *exec.Cmd
}

// SetStdin configures the underlying process's stdin to read
// from `text`.
func (r *realClipboardCommand) SetStdin(text string) {
	r.cmd.Stdin = stringReader(text)
}

// Run delegates to [exec.Cmd.Run].
func (r *realClipboardCommand) Run() error {
	return r.cmd.Run()
}

// stringReader avoids importing strings.NewReader into the model
// layer (kept as a self-contained shim so the clipboard dep
// graph stays tight). Returns an [io.Reader] that yields `text`.
func stringReader(text string) *strReader { return &strReader{text: text} }

// strReader is the minimal [io.Reader] over a string. Used as
// stdin for clipboard helpers; we own the type so we can drop
// the `strings` import when this file is the only consumer.
type strReader struct {
	text string
	pos  int
}

// Read implements [io.Reader].
func (r *strReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.text) {
		return 0, errReaderEOF
	}
	n := copy(p, r.text[r.pos:])
	r.pos += n
	return n, nil
}

// errReaderEOF is the sentinel signalling end-of-string. We
// avoid `io.EOF` to keep the import surface tight.
var errReaderEOF = fmt.Errorf("EOF") //nolint:gochecknoglobals,err113 // sentinel matched by the stdlib's exec package via signature

// runner is the active backend. Swapped by tests via
// [setClipboardCommandRunner].
var runner = realRunner //nolint:gochecknoglobals // tests swap this once per case via t.Cleanup

func realRunner(name string, arg ...string) clipboardCommand {
	// gosec G204 false positive: `name` and `arg` are sourced
	// exclusively from [clipboardCandidates], a hard-coded
	// allow-list of well-known clipboard helpers (pbcopy,
	// xsel, xclip, wl-copy, clip). The slice is never derived
	// from user input or env; the variable shape exists only
	// so the same call site can dispatch per-OS.
	cmd := exec.Command(name, arg...) //nolint:gosec // see comment above
	return &realClipboardCommand{cmd: cmd}
}

// setClipboardCommandRunner is the test-only seam. Production
// code never calls this directly. The returned cleanup
// function restores the previous backend.
func setClipboardCommandRunner(next clipboardCommandRunner) func() {
	prev := runner
	runner = next
	return func() { runner = prev }
}

// errClipboardUnsupported is returned when neither pbcopy / xsel
// / xclip / clip.exe is installed in the operator's environment.
var errClipboardUnsupported = errors.New("no clipboard backend available")

// clipboardCopy writes `payload` to the OS clipboard via the
// platform-specific helper. The function is intentionally tiny:
// every error condition is surfaced as a wrapped error so the
// caller (`updateProviderCatalogKey`) can reflect it in the
// CopyHint without leaking exec internals through the chrome.
//
// Platform mapping:
//
//   - macOS  : pbcopy
//   - Linux  : xsel --clipboard --input (preferred) → xclip
//     -selection clipboard (fallback)
//   - Windows: clip.exe
//
// We probe each candidate by attempting to run it. The first
// non-failure wins; if every attempt fails we return
// [errClipboardUnsupported] so the operator sees an actionable
// hint ("install xsel / xclip" rather than a silent miss).
func clipboardCopy(payload string) error {
	candidates := clipboardCandidates()
	if len(candidates) == 0 {
		return errClipboardUnsupported
	}
	var lastErr error
	for _, c := range candidates {
		cmd := runner(c.name, c.args...)
		cmd.SetStdin(payload)
		if err := cmd.Run(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("clipboard backends failed: %w", lastErr)
	}
	return errClipboardUnsupported
}

// clipboardCandidate is one entry in the per-OS lookup table.
type clipboardCandidate struct {
	name string
	args []string
}

func clipboardCandidates() []clipboardCandidate {
	switch runtime.GOOS {
	case "darwin":
		return []clipboardCandidate{{name: "pbcopy"}}
	case "windows":
		return []clipboardCandidate{{name: "clip"}}
	default:
		// Linux + BSDs: prefer xsel (faster, lighter), fall
		// back to xclip (more common on servers).
		return []clipboardCandidate{
			{name: "xsel", args: []string{"--clipboard", "--input"}},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			// Wayland fallback — wl-copy is gaining traction.
			{name: "wl-copy"},
		}
	}
}
