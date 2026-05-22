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
