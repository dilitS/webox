package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dilitS/webox/ssh"
)

// TestUpdate_StatusRefreshFailed_OpensHostKeyModal is the headline
// integration test for TASK-14.4: a host-key mismatch surfaced via
// the periodic status refresh MUST escalate into the strict-block
// modal rather than the dismissible alert toast. The toast text is
// asserted to remain empty so we never duplicate the message in two
// places (operator confusion + redundant render budget).
func TestUpdate_StatusRefreshFailed_OpensHostKeyModal(t *testing.T) {
	t.Parallel()

	m := New(Options{})
	m, _ = applyMsg(t, m, ConfigLoadedMsg{Config: fixtureConfig()})

	wrapped := fmt.Errorf("refresh statuses: %w", ssh.ErrHostKeyMismatch)
	m, _ = applyMsg(t, m, StatusRefreshFailedMsg{Err: wrapped})

	if !m.hostKeyModal.Open {
		t.Fatalf("expected modal open after mismatch, got %#v", m.hostKeyModal)
	}
	if m.hostKeyModal.Kind != hostKeyKindMismatch {
		t.Errorf("kind = %q, want %q", m.hostKeyModal.Kind, hostKeyKindMismatch)
	}
	if m.alert != "" {
		t.Errorf("alert toast leaked alongside modal: %q", m.alert)
	}
}

// TestUpdate_HostKeyModal_BlocksKeysAndDismissesOnEsc verifies the
// strict-block semantics. While the modal is open, navigation keys
// (Down / Tab / Enter on a tile) MUST be ignored so the operator
// cannot accidentally trigger another SSH command on top of the
// failing connection. Esc and Enter close the modal; nothing else
// does.
func TestUpdate_HostKeyModal_BlocksKeysAndDismissesOnEsc(t *testing.T) {
	t.Parallel()

	m := New(Options{})
	m, _ = applyMsg(t, m, ConfigLoadedMsg{Config: fixtureConfig()})
	m, _ = applyMsg(t, m, StatusRefreshFailedMsg{Err: ssh.ErrHostKeyUnknown})

	if !m.hostKeyModal.Open {
		t.Fatalf("setup failed: modal not open")
	}
	beforeSelect := m.selectedIndex

	for _, k := range []tea.KeyType{tea.KeyDown, tea.KeyTab, tea.KeyRight} {
		got, _ := applyMsg(t, m, key(k, ""))
		if got.selectedIndex != beforeSelect {
			t.Errorf("modal must block selection moves; key %v moved cursor", k)
		}
		if !got.hostKeyModal.Open {
			t.Errorf("key %v dismissed modal unexpectedly", k)
		}
	}

	m, _ = applyMsg(t, m, key(tea.KeyEsc, ""))
	if m.hostKeyModal.Open {
		t.Fatal("Esc should close the modal")
	}
}

// TestUpdate_StatusRefreshFailed_NonHostKeyKeepsLegacyAlert is the
// regression guard so we do not break the existing alert path when
// a *non-mismatch* error surfaces (e.g. transient DNS hiccup). The
// alert toast MUST keep working for those.
func TestUpdate_StatusRefreshFailed_NonHostKeyKeepsLegacyAlert(t *testing.T) {
	t.Parallel()

	m := New(Options{})
	m, _ = applyMsg(t, m, ConfigLoadedMsg{Config: fixtureConfig()})
	m, _ = applyMsg(t, m, StatusRefreshFailedMsg{Err: errors.New("dial: i/o timeout")})

	if m.hostKeyModal.Open {
		t.Fatal("non-host-key error should NOT open modal")
	}
	if !strings.Contains(m.alert, "status refresh failed") {
		t.Errorf("legacy alert missing, got %q", m.alert)
	}
}
