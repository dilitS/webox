package sshmetrics_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dilitS/webox/services/sshmetrics"
)

func TestParseUptimeLinuxFormats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		line string
		want sshmetrics.UptimeFacts
	}{
		{
			name: "linux days hours minutes",
			line: " 14:32:01 up 24 days,  3:42,  2 users,  load averages: 0.12, 0.28, 0.31",
			want: sshmetrics.UptimeFacts{Days: 24, Hours: 3, Minutes: 42, LoadAvg1: 0.12, LoadAvg5: 0.28, LoadAvg15: 0.31},
		},
		{
			name: "linux days minutes only",
			line: " 14:32:01 up 1 day, 5 min,  0 users,  load average: 1.00, 0.50, 0.25",
			want: sshmetrics.UptimeFacts{Days: 1, Minutes: 5, LoadAvg1: 1.00, LoadAvg5: 0.50, LoadAvg15: 0.25},
		},
		{
			name: "linux hour minute (sub-day)",
			line: " 14:32:01 up 3:14, 1 user, load average: 0.01, 0.02, 0.03",
			want: sshmetrics.UptimeFacts{Hours: 3, Minutes: 14, LoadAvg1: 0.01, LoadAvg5: 0.02, LoadAvg15: 0.03},
		},
		{
			name: "linux just minutes",
			line: " 14:32:01 up 47 min, 1 user, load average: 0.00, 0.00, 0.00",
			want: sshmetrics.UptimeFacts{Minutes: 47},
		},
		{
			name: "freebsd format with averages",
			line: " 7:09AM  up 24 days, 11:23, 2 users, load averages: 0.42, 0.31, 0.20",
			want: sshmetrics.UptimeFacts{Days: 24, Hours: 11, Minutes: 23, LoadAvg1: 0.42, LoadAvg5: 0.31, LoadAvg15: 0.20},
		},
		{
			name: "mac plus format",
			line: " 7:09AM up 24+11:23, 2 users, load averages: 1.10, 1.20, 1.30",
			want: sshmetrics.UptimeFacts{Days: 24, Hours: 11, Minutes: 23, LoadAvg1: 1.10, LoadAvg5: 1.20, LoadAvg15: 1.30},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := sshmetrics.ParseUptime(tc.line)
			if err != nil {
				t.Fatalf("ParseUptime: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ParseUptime = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestParseUptimeRejectsGarbage(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"not an uptime line at all",
		"   ",
	}
	for _, line := range cases {
		_, err := sshmetrics.ParseUptime(line)
		if !errors.Is(err, sshmetrics.ErrUptimeUnparseable) {
			t.Errorf("ParseUptime(%q) err = %v, want ErrUptimeUnparseable", line, err)
		}
	}
}

func TestParseFreeMemRow(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"             total       used       free     shared    buffers     cached",
		"Mem:          8192       3456       4736          0        128       1024",
		"-/+ buffers/cache:       2304       5888",
		"Swap:         2048          0       2048",
	}, "\n")

	got, err := sshmetrics.ParseFree(input)
	if err != nil {
		t.Fatalf("ParseFree: %v", err)
	}
	if got.TotalMB != 8192 || got.UsedMB != 3456 {
		t.Fatalf("ParseFree = %#v, want 8192/3456", got)
	}
	if pct := got.PercentUsed(); pct != 42 {
		t.Fatalf("PercentUsed = %d, want 42", pct)
	}
}

func TestParseFreeRejectsMissingMemRow(t *testing.T) {
	t.Parallel()

	_, err := sshmetrics.ParseFree("totally not the free output")
	if !errors.Is(err, sshmetrics.ErrFreeUnparseable) {
		t.Fatalf("ParseFree err = %v, want ErrFreeUnparseable", err)
	}
}

func TestFormatUptimeBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   sshmetrics.UptimeFacts
		want string
	}{
		{sshmetrics.UptimeFacts{Days: 24, Hours: 11}, "24d 11h"},
		{sshmetrics.UptimeFacts{Days: 5}, "5d"},
		{sshmetrics.UptimeFacts{Hours: 3, Minutes: 14}, "3h 14m"},
		{sshmetrics.UptimeFacts{Minutes: 47}, "47m"},
		{sshmetrics.UptimeFacts{}, "—"},
	}
	for _, tc := range cases {
		if got := sshmetrics.FormatUptime(tc.in); got != tc.want {
			t.Errorf("FormatUptime(%+v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatRAMHandlesZeroTotal(t *testing.T) {
	t.Parallel()

	if got := sshmetrics.FormatRAM(sshmetrics.MemoryFacts{}); got != "n/a" {
		t.Fatalf("FormatRAM(empty) = %q, want n/a", got)
	}
	if got := sshmetrics.FormatRAM(sshmetrics.MemoryFacts{TotalMB: 8192, UsedMB: 3456}); got != "3.4G/8.0G (42%)" {
		t.Fatalf("FormatRAM = %q, want 3.4G/8.0G (42%%)", got)
	}
}

func TestFormatLoadAvgPrecision(t *testing.T) {
	t.Parallel()

	got := sshmetrics.FormatLoadAvg(sshmetrics.UptimeFacts{LoadAvg1: 0.12, LoadAvg5: 0.28, LoadAvg15: 0.31})
	if got != "0.12, 0.28, 0.31" {
		t.Fatalf("FormatLoadAvg = %q, want 0.12, 0.28, 0.31", got)
	}
}

func TestFormatRTTHandlesZeroAndSubMillisecond(t *testing.T) {
	t.Parallel()

	if got := sshmetrics.FormatRTT(0); got != "n/a" {
		t.Fatalf("FormatRTT(0) = %q, want n/a", got)
	}
	if got := sshmetrics.FormatRTT(500 * time.Microsecond); got != "1ms" {
		t.Fatalf("FormatRTT(500us) = %q, want 1ms", got)
	}
	if got := sshmetrics.FormatRTT(18 * time.Millisecond); got != "18ms" {
		t.Fatalf("FormatRTT(18ms) = %q, want 18ms", got)
	}
}
