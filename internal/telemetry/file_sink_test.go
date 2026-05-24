package telemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixedClock returns a fixed instant so the recorded `ts` field is
// deterministic per test.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestOpenFileSink_EmptyPathReturnsDisabled(t *testing.T) {
	t.Parallel()

	sink, err := OpenFileSink("", FileSinkPolicy{})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if sink != Disabled {
		t.Errorf("sink = %T, want Disabled no-op", sink)
	}
}

func TestOpenFileSink_FileModeIs0600(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")
	sink, err := OpenFileSink(path, FileSinkPolicy{FlushInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("OpenFileSink: %v", err)
	}
	closer, ok := sink.(*FileSink)
	if !ok {
		t.Fatalf("sink = %T, want *FileSink", sink)
	}
	t.Cleanup(func() { _ = closer.Close() })

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %v, want 0600 (no world / group read)", perm)
	}
}

// TestFileSink_RecordWriteRoundtrip is the headline behavioural
// test: events flow through the queue, the drain goroutine writes
// them as JSON Lines, and Close flushes the buffer so the test can
// read the file back without races.
func TestFileSink_RecordWriteRoundtrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")
	fixed := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	sink, err := OpenFileSink(path, FileSinkPolicy{
		FlushInterval: 5 * time.Millisecond,
		Clock:         fixedClock(fixed),
	})
	if err != nil {
		t.Fatalf("OpenFileSink: %v", err)
	}
	concrete, ok := sink.(*FileSink)
	if !ok {
		t.Fatalf("sink type = %T", sink)
	}

	ctx := context.Background()
	concrete.Record(ctx, Event{Name: "doctor.run", Fields: map[string]any{"exit_code": 0, "json": true}})
	concrete.Record(ctx, Event{Name: "ssh.acquire", Fields: map[string]any{"host": "s1.small.pl"}})

	if err := concrete.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2: %v", len(lines), lines)
	}

	var first traceLine
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal[0]: %v", err)
	}
	if first.Name != "doctor.run" {
		t.Errorf("Name = %q, want doctor.run", first.Name)
	}
	if first.TimestampUnixNano != fixed.UnixNano() {
		t.Errorf("ts = %d, want %d", first.TimestampUnixNano, fixed.UnixNano())
	}
	// json.Number / float64 round-trip: encoded as 0 → decoded as
	// float64 by encoding/json without Number mode.
	if got, ok := first.Fields["exit_code"].(float64); !ok || got != 0 {
		t.Errorf("exit_code field = %v (%T), want 0 as float64", first.Fields["exit_code"], first.Fields["exit_code"])
	}
}

// TestFileSink_RedactsSecretsBeforeWrite is THE headline security
// guard. Any future code path that records an event with secret-
// shaped content MUST find it redacted in the file.
func TestFileSink_RedactsSecretsBeforeWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")
	sink, err := OpenFileSink(path, FileSinkPolicy{FlushInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("OpenFileSink: %v", err)
	}
	concrete := sink.(*FileSink)

	concrete.Record(context.Background(), Event{
		Name: "credentials.leak",
		Fields: map[string]any{
			"token":           "ghp_1234567890abcdefghijklmnopqrstuvwxyz",
			"db_url":          "mysql://demo:supersecret@db.smallhost.pl/app",
			"password":        "p@ssw0rd",
			"non_secret_int":  42,
			"non_secret_bool": true,
		},
	})

	if err := concrete.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(raw)

	forbidden := []string{
		"ghp_1234567890abcdefghijklmnopqrstuvwxyz",
		"supersecret",
		"p@ssw0rd",
	}
	for _, leak := range forbidden {
		if strings.Contains(content, leak) {
			t.Errorf("trace leaks secret %q: %s", leak, content)
		}
	}
	if !strings.Contains(content, "[REDACTED]") {
		t.Errorf("expected [REDACTED] marker in trace: %s", content)
	}
	// Non-string fields must pass through unchanged.
	if !strings.Contains(content, "\"non_secret_int\":42") {
		t.Errorf("numeric field lost: %s", content)
	}
	if !strings.Contains(content, "\"non_secret_bool\":true") {
		t.Errorf("boolean field lost: %s", content)
	}
}

// TestFileSink_DropOnFullQueue verifies that producers are never
// blocked when the drain goroutine cannot keep up. The behaviour
// trade-off is deliberate: a slow trace MUST never freeze the
// cockpit.
func TestFileSink_DropOnFullQueue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")
	sink, err := OpenFileSink(path, FileSinkPolicy{
		FlushInterval: time.Hour,
		Queue:         1,
	})
	if err != nil {
		t.Fatalf("OpenFileSink: %v", err)
	}
	concrete := sink.(*FileSink)
	t.Cleanup(func() { _ = concrete.Close() })

	const flood = 1000
	var wg sync.WaitGroup
	for i := 0; i < flood; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			concrete.Record(context.Background(), Event{Name: "flood", Fields: map[string]any{"i": i}})
		}(i)
	}
	wg.Wait()

	if got := concrete.DroppedEvents(); got == 0 {
		t.Errorf("expected some drops with queue=1 and %d producers", flood)
	}
}

func TestFileSink_CloseIsIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "trace.jsonl")
	sink, err := OpenFileSink(path, FileSinkPolicy{FlushInterval: 5 * time.Millisecond})
	if err != nil {
		t.Fatalf("OpenFileSink: %v", err)
	}
	concrete := sink.(*FileSink)
	if err := concrete.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := concrete.Close(); err != nil {
		t.Errorf("second Close: %v (must be nil — idempotent)", err)
	}
}

func TestFileSink_NilReceiverIsNoop(t *testing.T) {
	t.Parallel()

	var s *FileSink
	if s.Enabled() {
		t.Error("nil *FileSink reports Enabled true")
	}
	s.Record(context.Background(), Event{Name: "x"})
	if got := s.DroppedEvents(); got != 0 {
		t.Errorf("dropped = %d on nil receiver", got)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close on nil receiver: %v", err)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open trace: %v", err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return lines
}
