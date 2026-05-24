package log

import (
	"strings"
	"testing"
)

func BenchmarkRedact100KB(b *testing.B) {
	secret := githubToken("p")
	input := strings.Repeat("safe line with public data\n", 4_000) + secret

	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	for b.Loop() {
		_ = Redact(input)
	}
}

// BenchmarkRedactLogLinePAT measures the per-line redaction cost on a
// 200-char log line that ends with a GitHub PAT. Sprint 09 §TASK-09.7
// caps this at 50µs so the SSH tail pipeline can sustain 1000 lines/s
// without blocking the channel writer.
func BenchmarkRedactLogLinePAT(b *testing.B) {
	secret := githubToken("p")
	prefix := strings.Repeat("a", 200-len(secret)-len(" token="))
	line := prefix + " token=" + secret

	b.ReportAllocs()
	b.SetBytes(int64(len(line)))
	for b.Loop() {
		_ = Redact(line)
	}
}

// BenchmarkRedactLogLinePlain measures the no-secret hot path so we
// catch regressions where future patterns make innocent lines walk
// every regex without short-circuiting.
func BenchmarkRedactLogLinePlain(b *testing.B) {
	line := strings.Repeat("GET /healthz 200 12ms ", 8)
	b.ReportAllocs()
	b.SetBytes(int64(len(line)))
	for b.Loop() {
		_ = Redact(line)
	}
}
