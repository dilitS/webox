package telemetry

import "context"

// Event is a future-proof local observability envelope. Sprint 01 keeps
// it intentionally small and write-only so packages can depend on a
// stable seam before any real implementation exists.
type Event struct {
	Name   string
	Fields map[string]any
}

// Sink records telemetry events. MVP ships only the disabled no-op
// implementation; remote telemetry is explicitly out of scope.
type Sink interface {
	Enabled() bool
	Record(ctx context.Context, event Event)
}

type disabledSink struct{}

// Enabled reports whether the sink performs any work.
func (disabledSink) Enabled() bool { return false }

// Record is a deliberate no-op.
func (disabledSink) Record(context.Context, Event) {}

// Disabled is the canonical no-op sink used throughout MVP code until a
// local-only metrics/log bundle implementation lands.
var Disabled Sink = disabledSink{}
