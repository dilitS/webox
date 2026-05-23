package ssh

import (
	"errors"
	"testing"
)

func TestSentinels_AreDistinctAndStable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"ErrPoolBusy", ErrPoolBusy},
		{"ErrHostKeyUnknown", ErrHostKeyUnknown},
		{"ErrHostKeyMismatch", ErrHostKeyMismatch},
		{"ErrReconnectExhausted", ErrReconnectExhausted},
		{"ErrHostKeyDBRequired", ErrHostKeyDBRequired},
	}

	seen := make(map[error]string, len(cases))
	for _, tc := range cases {
		if tc.err == nil {
			t.Fatalf("%s is nil — sentinel must be a non-nil error value", tc.name)
		}
		if prev, dup := seen[tc.err]; dup {
			t.Fatalf("%s aliases %s — sentinels must be distinct values for errors.Is to work", tc.name, prev)
		}
		seen[tc.err] = tc.name

		if !errors.Is(tc.err, tc.err) {
			t.Fatalf("errors.Is(%s, %s) = false", tc.name, tc.name)
		}
		if tc.err.Error() == "" {
			t.Fatalf("%s.Error() is empty — callers and logs need a human label", tc.name)
		}
	}
}

func TestErrPoolBusy_MessageMentionsPool(t *testing.T) {
	t.Parallel()

	if got := ErrPoolBusy.Error(); !contains(got, "pool") {
		t.Fatalf("ErrPoolBusy.Error() = %q, want it to mention 'pool'", got)
	}
}

func TestHostKeyErrors_DistinctMessages(t *testing.T) {
	t.Parallel()

	unknown := ErrHostKeyUnknown.Error()
	mismatch := ErrHostKeyMismatch.Error()
	if unknown == mismatch {
		t.Fatalf("ErrHostKeyUnknown and ErrHostKeyMismatch share the message %q — operator UX needs to tell them apart", unknown)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
