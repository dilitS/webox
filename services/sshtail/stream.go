package sshtail

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	internalLog "github.com/dilitS/webox/internal/log"
	"github.com/dilitS/webox/tui/components"
)

// Line is the redacted, level-classified projection of a single remote
// log entry. Raw never contains the original secret payload — any
// regex match in [internal/log.Redact] flips Redacted=true.
type Line struct {
	Timestamp time.Time
	Raw       string
	Level     components.LogLevel
	Redacted  bool
}

// Profile is the minimal SSH target descriptor the streamer needs. The
// concrete `config.Profile` type is intentionally not imported — keeps
// `services/sshtail` free of the persistence layer cycle.
type Profile struct {
	Alias string
	Host  string
	User  string
	Port  int
}

// Options configures a [Streamer]. All durations are optional; zero
// values fall back to defensible defaults captured in package consts.
type Options struct {
	// Executor opens an SSH session for the given command.
	Executor Executor
	// Now produces timestamps for emitted lines. Defaults to time.Now.
	Now func() time.Time
	// Redact replaces secret-shaped substrings before emission.
	// Defaults to [internal/log.Redact] — overridable for tests that
	// want to assert on raw bytes.
	Redact func(string) string
	// ChannelBuffer caps the per-stream channel buffer. Defaults to 256.
	ChannelBuffer int
	// Backoff schedule between reconnect attempts. Defaults to
	// {2s, 4s, 8s}.
	Backoff []time.Duration
	// Sleep is the cancellable sleeper used between reconnects.
	// Defaults to a context-aware time.Timer.
	Sleep func(ctx context.Context, d time.Duration) error
}

const (
	defaultChannelBuffer = 256
	defaultMaxLineBytes  = 1 << 16 // 64KB; sanity cap for a single log line.
	initialScannerBuffer = 4096    // 4KB; matches bufio default for short lines.
)

// Streamer materialises live log streams from remote hosts. It is safe
// for concurrent use across distinct (profile, logPath) tuples; the
// caller is expected to spawn one stream per project and cancel the
// associated context when switching projects.
type Streamer struct {
	executor Executor
	now      func() time.Time
	redact   func(string) string
	bufSize  int
	backoff  []time.Duration
	sleep    func(ctx context.Context, d time.Duration) error
}

// New constructs a Streamer with the provided options. Executor MUST be
// non-nil; a nil executor returns a misuse panic at construction time
// because that is always a wiring bug, not a runtime condition.
func New(opts Options) *Streamer {
	if opts.Executor == nil {
		panic("sshtail.New: Executor is required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Redact == nil {
		opts.Redact = internalLog.Redact
	}
	if opts.ChannelBuffer <= 0 {
		opts.ChannelBuffer = defaultChannelBuffer
	}
	if len(opts.Backoff) == 0 {
		opts.Backoff = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
	}
	if opts.Sleep == nil {
		opts.Sleep = sleepContext
	}
	return &Streamer{
		executor: opts.Executor,
		now:      opts.Now,
		redact:   opts.Redact,
		bufSize:  opts.ChannelBuffer,
		backoff:  opts.Backoff,
		sleep:    opts.Sleep,
	}
}

// Stream opens a `tail -f` against logPath on profile and returns a
// channel that emits redacted lines until ctx is cancelled. The
// returned error reports preflight failures only (invalid path); once
// the channel is established, transport errors trigger reconnect.
//
// The channel is closed when ctx is cancelled or every reconnect
// attempt is exhausted.
func (s *Streamer) Stream(ctx context.Context, profile Profile, logPath string) (<-chan Line, error) {
	if err := validateLogPath(logPath); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrLogPathInvalid, logPath)
	}

	out := make(chan Line, s.bufSize)
	go s.runStream(ctx, profile, logPath, out)
	return out, nil
}

func (s *Streamer) runStream(ctx context.Context, profile Profile, logPath string, out chan<- Line) {
	defer close(out)

	command := fmt.Sprintf("tail -n 0 -f %s", shellEscape(logPath))
	_ = profile // reserved for production executor wiring (per-profile dial).

	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return
		}

		reader, err := s.executor.Open(ctx, command)
		if err != nil {
			if !s.handleOpenFailure(ctx, attempt, err) {
				return
			}
			continue
		}

		stopped := s.copyLines(ctx, reader, out)
		_ = reader.Close()
		if stopped {
			return
		}

		if !s.handleOpenFailure(ctx, attempt, ErrSessionClosed) {
			return
		}
	}
}

// copyLines reads reader line-by-line, redacts each line, and emits a
// [Line] on out. Returns true when the consumer cancelled ctx (no
// reconnect should follow), false on transport-level EOF/error so the
// caller can attempt a reconnect.
func (s *Streamer) copyLines(ctx context.Context, reader io.Reader, out chan<- Line) bool {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, initialScannerBuffer), defaultMaxLineBytes)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return true
		}

		text := scanner.Text()
		redacted := s.redact(text)
		line := Line{
			Timestamp: s.now(),
			Raw:       redacted,
			Level:     components.ParseLogLevel(redacted),
			Redacted:  redacted != text,
		}

		select {
		case out <- line:
		case <-ctx.Done():
			return true
		}
	}
	return ctx.Err() != nil
}

// handleOpenFailure sleeps according to the backoff schedule. It
// returns true when the caller should retry, false when retries are
// exhausted or ctx is cancelled.
func (s *Streamer) handleOpenFailure(ctx context.Context, attempt int, cause error) bool {
	if attempt >= len(s.backoff) {
		return false
	}
	delay := s.backoff[attempt]
	if err := s.sleep(ctx, delay); err != nil {
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	// Wrap the cause for future error-log piping; the boolean return
	// already signals "retry".
	_ = fmt.Errorf("sshtail: open attempt %d failed: %w", attempt+1, cause)
	return true
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsTransportError reports whether err looks like a recoverable
// transport-level failure. The streamer uses it internally; callers may
// reuse it to decide between alerting the operator and silently
// retrying in higher layers.
func IsTransportError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, io.EOF) ||
		errors.Is(err, ErrSessionClosed) ||
		strings.Contains(err.Error(), "use of closed network connection")
}
