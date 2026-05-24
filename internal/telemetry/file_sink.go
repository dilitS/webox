package telemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dilitS/webox/internal/log"
)

// FileSinkPolicy parameterises the local-only JSONL trace writer
// introduced by TASK-14.6. The defaults below — 8 KiB buffer flushed
// every 500 ms or whenever the writer is closed — keep the trace
// stream usable for live `tail -f` inspection while never blocking
// the cockpit's hot path: writes happen on a background goroutine
// and full-channel events are *dropped* rather than backpressuring
// the producer.
//
// Dropping is the correct trade-off because Webox MUST never let an
// observability sink slow down the operator surface. The dropped
// count is itself recorded so the operator can tell when the buffer
// was too small for the workload.
type FileSinkPolicy struct {
	// BufferSize is the size of the bufio writer that wraps the
	// underlying file. Larger values reduce syscall count; smaller
	// values reduce loss when the cockpit crashes before flush.
	BufferSize int

	// FlushInterval is the maximum time between background flushes.
	// Set to <=0 to disable the background flusher (callers must
	// then call Close to flush remaining events).
	FlushInterval time.Duration

	// Queue caps the in-memory channel feeding the background
	// goroutine. Events overflowing the queue are dropped; the
	// counter is exposed via DroppedEvents().
	Queue int

	// Clock is injectable for deterministic tests.
	Clock func() time.Time
}

// FileSink writes [Event] records as JSON Lines to a local file
// chosen by the operator via `--debug-trace=PATH`. The sink is
// strictly local — there is NO network code in this file, by design,
// and no fallback transport. If the file cannot be opened the sink
// degrades to the [Disabled] no-op so the cockpit still boots.
//
// # Concurrency
//
// Producers (cockpit Update, ssh ExecWithRetry, doctor) call Record
// from arbitrary goroutines. Record serialises the event into the
// queue channel and returns immediately; the queue is drained by a
// single background goroutine that writes through the bufio buffer.
// Buffered drain means there is exactly one writer touching the
// underlying file, so we avoid both per-write fsync (slow) and
// torn writes (corruption).
//
// # Redaction
//
// Every string-valued field in Event.Fields is passed through
// [log.Redact] before encoding so a careless caller cannot leak a
// secret into the trace file. Numeric / boolean fields are written
// verbatim. Tests pin the redaction guard
// (`TestFileSink_RedactsSecretsBeforeWrite`) so regressions surface
// immediately.
type FileSink struct {
	policy FileSinkPolicy

	file *os.File
	bw   *bufio.Writer

	queue chan Event
	wg    sync.WaitGroup

	closeOnce sync.Once
	closed    chan struct{}

	mu      sync.Mutex
	dropped uint64
}

// OpenFileSink creates (or appends to) the trace file at path with
// the supplied policy. The file is opened with `O_APPEND|O_CREATE`
// and mode 0600 — a TUI operator's working dir often sits in a
// non-XDG location and the file may end up under `~`. Mode 0600
// ensures that even if the path is shared we never widen access.
//
// Returns the canonical no-op [Disabled] sink instead of an error
// when the file cannot be opened; the cockpit MUST still boot. The
// returned error is informational so the CLI layer can warn the
// operator.
func OpenFileSink(path string, policy FileSinkPolicy) (Sink, error) {
	if path == "" {
		return Disabled, errors.New("telemetry: empty trace path")
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return Disabled, fmt.Errorf("telemetry: resolve path %q: %w", path, err)
	}
	if dir := filepath.Dir(resolved); dir != "" {
		if mkErr := os.MkdirAll(dir, traceDirMode); mkErr != nil {
			return Disabled, fmt.Errorf("telemetry: mkdir %q: %w", dir, mkErr)
		}
	}
	f, err := os.OpenFile(resolved, os.O_APPEND|os.O_CREATE|os.O_WRONLY, traceFileMode)
	if err != nil {
		return Disabled, fmt.Errorf("telemetry: open %q: %w", resolved, err)
	}
	sink := newFileSink(f, policy)
	return sink, nil
}

const (
	traceFileMode os.FileMode = 0o600
	traceDirMode  os.FileMode = 0o700

	defaultBufferSize    = 8 * 1024
	defaultFlushInterval = 500 * time.Millisecond
	defaultQueue         = 1024
)

func newFileSink(w *os.File, policy FileSinkPolicy) *FileSink {
	policy = normalizeFileSinkPolicy(policy)
	bw := bufio.NewWriterSize(w, policy.BufferSize)
	sink := &FileSink{
		policy: policy,
		file:   w,
		bw:     bw,
		queue:  make(chan Event, policy.Queue),
		closed: make(chan struct{}),
	}
	sink.wg.Add(1)
	go sink.drainLoop()
	return sink
}

func normalizeFileSinkPolicy(p FileSinkPolicy) FileSinkPolicy {
	if p.BufferSize <= 0 {
		p.BufferSize = defaultBufferSize
	}
	if p.FlushInterval <= 0 {
		p.FlushInterval = defaultFlushInterval
	}
	if p.Queue <= 0 {
		p.Queue = defaultQueue
	}
	if p.Clock == nil {
		p.Clock = time.Now
	}
	return p
}

// Enabled reports that the sink performs work. Always true once
// OpenFileSink succeeds (even if every subsequent write is dropped).
func (s *FileSink) Enabled() bool { return s != nil }

// Record enqueues the event for asynchronous JSON serialisation.
// Drops the event when the queue is full so the cockpit's hot path
// is never blocked. The dropped count is recoverable via
// DroppedEvents().
func (s *FileSink) Record(ctx context.Context, event Event) {
	if s == nil {
		return
	}
	select {
	case <-s.closed:
		return
	default:
	}
	if event.Fields == nil {
		event.Fields = map[string]any{}
	}
	select {
	case s.queue <- event:
	case <-ctx.Done():
	default:
		s.mu.Lock()
		s.dropped++
		s.mu.Unlock()
	}
}

// DroppedEvents returns how many events have been silently dropped
// because the queue was full at Record time. Exposed via doctor.
func (s *FileSink) DroppedEvents() uint64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

// Close stops accepting events, flushes the buffer, and closes the
// underlying file. Safe to call multiple times.
func (s *FileSink) Close() error {
	if s == nil {
		return nil
	}
	var closeErr error
	s.closeOnce.Do(func() {
		close(s.closed)
		close(s.queue)
		s.wg.Wait()
		if flushErr := s.bw.Flush(); flushErr != nil {
			closeErr = flushErr
		}
		if fErr := s.file.Close(); fErr != nil && closeErr == nil {
			closeErr = fErr
		}
	})
	return closeErr
}

func (s *FileSink) drainLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.policy.FlushInterval)
	defer ticker.Stop()
	for {
		select {
		case ev, ok := <-s.queue:
			if !ok {
				s.flushQueueRemainder()
				return
			}
			s.writeEvent(ev)
		case <-ticker.C:
			_ = s.bw.Flush()
		}
	}
}

func (s *FileSink) flushQueueRemainder() {
	// queue has been closed; drain any leftover events that arrived
	// between close() and the select returning ok=false. Because
	// Go's channel close is atomic this loop terminates as soon as
	// the receive side observes the close signal with an empty
	// queue.
	for ev := range s.queue {
		s.writeEvent(ev)
	}
	_ = s.bw.Flush()
}

func (s *FileSink) writeEvent(ev Event) {
	envelope := traceLine{
		TimestampUnixNano: s.policy.Clock().UnixNano(),
		Name:              ev.Name,
		Fields:            ev.Fields,
	}
	// Two-stage redaction: encode first so [log.Redact]'s JSON-aware
	// patterns (`"password":"..."`, Authorization headers, database
	// URLs with embedded creds) can fire on the canonical wire form
	// — operating on individual map values does not, because the
	// regex needs the surrounding JSON quoting / colons to anchor.
	// Marshal-then-redact is O(n) over the line and runs on the
	// background drain goroutine, so the hot path is unaffected.
	raw, err := json.Marshal(envelope)
	if err != nil {
		s.bumpDropped()
		return
	}
	clean := log.Redact(string(raw))
	if _, err := s.bw.WriteString(clean); err != nil {
		s.bumpDropped()
		return
	}
	if _, err := s.bw.WriteString("\n"); err != nil {
		s.bumpDropped()
	}
}

func (s *FileSink) bumpDropped() {
	s.mu.Lock()
	s.dropped++
	s.mu.Unlock()
}

// traceLine is the wire format. Field names are short to keep the
// JSONL grep-friendly; ordering is alphabetic so `sort -u` produces
// stable diffs across runs (useful when comparing two trace files).
type traceLine struct {
	Fields            map[string]any `json:"fields,omitempty"`
	Name              string         `json:"name"`
	TimestampUnixNano int64          `json:"ts"`
}

// Compile-time guard that FileSink satisfies the Sink interface.
var _ Sink = (*FileSink)(nil)

// Compile-time guard that the sink also satisfies io.Closer so
// callers can use a deferred Close without an additional type
// assertion.
var _ io.Closer = (*FileSink)(nil)
