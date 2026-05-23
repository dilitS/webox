package smallhost_test

import (
	"context"
	"errors"
	"sync"

	wssh "github.com/dilitS/webox/ssh"
)

// fakeExecutor is the deterministic stand-in for [smallhost.Executor]
// used by every method test in this package. It records the exact
// command strings the adapter emits and returns scripted
// stdout/stderr/exit results, so tests assert both behavior (parser
// output) and command shape (no shell injection, correct tokens).
//
// Each entry in `scripts` matches a command verbatim. A nil entry
// returns ErrCommandNotScripted, which makes "the adapter issued an
// unexpected command" trivially detectable.
type fakeExecutor struct {
	mu      sync.Mutex
	scripts map[string]fakeResponse
	calls   []string
}

type fakeResponse struct {
	stdout   []byte
	stderr   []byte
	exitCode int
	err      error
}

var errCommandNotScripted = errors.New("fakeExecutor: command not scripted")

func newFakeExecutor() *fakeExecutor {
	return &fakeExecutor{scripts: map[string]fakeResponse{}}
}

// On registers the response returned when command is exec'd. Multiple
// calls to the same command return the same response (no consume).
func (f *fakeExecutor) On(command string, resp fakeResponse) *fakeExecutor {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scripts[command] = resp
	return f
}

// Calls returns the ordered list of commands the adapter emitted.
// Tests assert on this to verify the command builder.
func (f *fakeExecutor) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeExecutor) Exec(_ context.Context, command string) (wssh.ExecResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, command)
	resp, ok := f.scripts[command]
	if !ok {
		return wssh.ExecResult{}, errCommandNotScripted
	}
	return wssh.ExecResult{
		Stdout:   resp.stdout,
		Stderr:   resp.stderr,
		ExitCode: resp.exitCode,
	}, resp.err
}
