package telemetry

import (
	"context"
	"testing"
)

func TestDisabledSink(t *testing.T) {
	t.Parallel()

	if Disabled.Enabled() {
		t.Fatal("Disabled.Enabled() = true, want false")
	}

	Disabled.Record(context.Background(), Event{
		Name: "doctor.run",
		Fields: map[string]any{
			"exit_code": 0,
		},
	})
}
