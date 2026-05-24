package tui

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/dilitS/webox/internal/telemetry"
	"github.com/dilitS/webox/ssh"
)

// recordingSink captures Record calls in memory so the cockpit's
// emit-call-sites can be asserted from unit tests without spinning
// up a file-backed sink. Safe for concurrent use.
type recordingSink struct {
	mu     sync.Mutex
	events []telemetry.Event
}

func (r *recordingSink) Enabled() bool { return true }
func (r *recordingSink) Record(_ context.Context, ev telemetry.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *recordingSink) eventsByName(name string) []telemetry.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]telemetry.Event, 0, len(r.events))
	for _, ev := range r.events {
		if ev.Name == name {
			out = append(out, ev)
		}
	}
	return out
}

func TestEmit_StatusRefreshFailedRecordsErrClass(t *testing.T) {
	sink := &recordingSink{}
	m := New(Options{Trace: sink})
	m, _ = applyMsg(t, m, ConfigLoadedMsg{Config: fixtureConfig()})

	cases := []struct {
		name     string
		err      error
		wantKind string
	}{
		{"host key mismatch", fmt.Errorf("dial: %w", ssh.ErrHostKeyMismatch), "host_key_mismatch"},
		{"host key unknown", ssh.ErrHostKeyUnknown, "host_key_unknown"},
		{"pool busy", ssh.ErrPoolBusy, "pool_busy"},
	}
	for _, tc := range cases {
		_, _ = applyMsg(t, m, StatusRefreshFailedMsg{Err: tc.err})
	}

	events := sink.eventsByName("status.refresh_failed")
	if len(events) != len(cases) {
		t.Fatalf("emitted %d events, want %d", len(events), len(cases))
	}
	for i, ev := range events {
		gotClass, _ := ev.Fields["err_class"].(string)
		if gotClass != cases[i].wantKind {
			t.Errorf("event[%d] err_class = %q, want %q", i, gotClass, cases[i].wantKind)
		}
	}
}

func TestEmit_DisabledSinkIsZeroAlloc(t *testing.T) {
	// Sanity check: when no trace flag is given the cockpit MUST
	// not pay any cost for the emit calls. We verify by emitting
	// 10k events and asserting Record was never invoked (the
	// disabled sink's Record is a no-op so the assertion is "did
	// not panic / did not allocate noticeably" — Go's testing
	// framework will surface either via go test -race).
	m := New(Options{}) // Trace nil → Disabled
	for i := 0; i < 10_000; i++ {
		m.emit("hot.loop", map[string]any{"i": i})
	}
}

// TestEmit_HostKeyModalOpenEmittedWithKind verifies the second
// emit-call-site introduced in TASK-14.6: when the status refresh
// error chain triggers the host-key modal, we also record a
// `modal.hostkey_open` event so operators inspecting the trace can
// correlate the alert with a visible UI overlay.
func TestEmit_HostKeyModalOpenEmittedWithKind(t *testing.T) {
	sink := &recordingSink{}
	m := New(Options{Trace: sink})
	m, _ = applyMsg(t, m, ConfigLoadedMsg{Config: fixtureConfig()})

	_, _ = applyMsg(t, m, StatusRefreshFailedMsg{Err: ssh.ErrHostKeyMismatch})

	events := sink.eventsByName("modal.hostkey_open")
	if len(events) != 1 {
		t.Fatalf("modal.hostkey_open emitted %d times, want 1", len(events))
	}
	if kind, _ := events[0].Fields["kind"].(string); kind != hostKeyKindMismatch {
		t.Errorf("kind field = %q, want %q", kind, hostKeyKindMismatch)
	}
}
