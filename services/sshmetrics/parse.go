package sshmetrics

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrUptimeUnparseable is returned by [ParseUptime] when the input
// matches none of the supported formats. The caller MUST surface this
// to the tile so the badge can show "Uptime: n/a" instead of pretending
// the host is fresh.
var ErrUptimeUnparseable = errors.New("sshmetrics: cannot parse uptime")

// ErrFreeUnparseable is returned by [ParseFree] when the `free -m`
// output does not contain a "Mem:" line. The header tile shows
// "RAM: n/a" for this case.
var ErrFreeUnparseable = errors.New("sshmetrics: cannot parse free output")

// UptimeFacts captures the structured slice extracted from `uptime`.
// Days/Hours/Minutes are the human-friendly decomposition the tile
// renders; LoadAvg* are the three load averages.
type UptimeFacts struct {
	Days      int
	Hours     int
	Minutes   int
	LoadAvg1  float64
	LoadAvg5  float64
	LoadAvg15 float64
}

// MemoryFacts captures the parsed `free -m` projection (megabytes).
type MemoryFacts struct {
	TotalMB int
	UsedMB  int
}

// PercentUsed returns the used/total ratio as a 0..100 integer. Zero
// total returns zero (defensive: avoids division by zero in render).
func (m MemoryFacts) PercentUsed() int {
	if m.TotalMB <= 0 {
		return 0
	}
	return (m.UsedMB * percentMultiplier) / m.TotalMB
}

var (
	// loadAvgPattern matches the trailing "load averages: 0.12, 0.28, 0.31"
	// segment present in Linux, FreeBSD, and macOS uptime output.
	loadAvgPattern = regexp.MustCompile(`load averages?:\s*([0-9.]+)[,\s]+([0-9.]+)[,\s]+([0-9.]+)`)

	// upPattern is the lenient "X day(s), Y:ZZ" / "X days, HH min"
	// matcher. It also handles FreeBSD's "up 11:23" form (no day
	// component) via a separate fallback below.
	upPatternDaysHM    = regexp.MustCompile(`up\s+(\d+)\s+days?,\s+(\d+):(\d+)`)
	upPatternDaysMin   = regexp.MustCompile(`up\s+(\d+)\s+days?,\s+(\d+)\s+min`)
	upPatternDaysOnly  = regexp.MustCompile(`up\s+(\d+)\s+days?,\s+(\d+)\s+hr`)
	upPatternHourMin   = regexp.MustCompile(`up\s+(\d+):(\d+)`)
	upPatternMinOnly   = regexp.MustCompile(`up\s+(\d+)\s+min`)
	upPatternMacShort  = regexp.MustCompile(`up\s+(\d+)\+(\d+):(\d+)`)
	freeMemLinePattern = regexp.MustCompile(`(?m)^Mem:\s+(\d+)\s+(\d+)`)
)

const (
	percentMultiplier  = 100
	loadAvgFieldCount  = 4
	freeFieldCount     = 3
	bytesPerKilobyte   = 1024
	smallestRenderedGB = 0.1
)

// ParseUptime decomposes `uptime` output. The function is tolerant to
// trailing user counts and to the various per-OS formats.
func ParseUptime(raw string) (UptimeFacts, error) {
	out := UptimeFacts{}
	line := strings.TrimSpace(raw)
	if line == "" {
		return out, ErrUptimeUnparseable
	}

	matched := false

	switch {
	case upPatternMacShort.MatchString(line):
		m := upPatternMacShort.FindStringSubmatch(line)
		out.Days, _ = strconv.Atoi(m[1])
		out.Hours, _ = strconv.Atoi(m[2])
		out.Minutes, _ = strconv.Atoi(m[3])
		matched = true
	case upPatternDaysHM.MatchString(line):
		m := upPatternDaysHM.FindStringSubmatch(line)
		out.Days, _ = strconv.Atoi(m[1])
		out.Hours, _ = strconv.Atoi(m[2])
		out.Minutes, _ = strconv.Atoi(m[3])
		matched = true
	case upPatternDaysMin.MatchString(line):
		m := upPatternDaysMin.FindStringSubmatch(line)
		out.Days, _ = strconv.Atoi(m[1])
		out.Minutes, _ = strconv.Atoi(m[2])
		matched = true
	case upPatternDaysOnly.MatchString(line):
		m := upPatternDaysOnly.FindStringSubmatch(line)
		out.Days, _ = strconv.Atoi(m[1])
		out.Hours, _ = strconv.Atoi(m[2])
		matched = true
	case upPatternHourMin.MatchString(line):
		m := upPatternHourMin.FindStringSubmatch(line)
		out.Hours, _ = strconv.Atoi(m[1])
		out.Minutes, _ = strconv.Atoi(m[2])
		matched = true
	case upPatternMinOnly.MatchString(line):
		m := upPatternMinOnly.FindStringSubmatch(line)
		out.Minutes, _ = strconv.Atoi(m[1])
		matched = true
	}

	if loadMatches := loadAvgPattern.FindStringSubmatch(line); len(loadMatches) == loadAvgFieldCount {
		out.LoadAvg1, _ = strconv.ParseFloat(loadMatches[1], 64)
		out.LoadAvg5, _ = strconv.ParseFloat(loadMatches[2], 64)
		out.LoadAvg15, _ = strconv.ParseFloat(loadMatches[3], 64)
		matched = true
	}

	if !matched {
		return out, ErrUptimeUnparseable
	}
	return out, nil
}

// ParseFree extracts the `Mem:` row from `free -m` output. Other rows
// (`Swap:`, `-/+ buffers/cache:`) are intentionally ignored — the
// header tile cares about RAM only.
func ParseFree(raw string) (MemoryFacts, error) {
	m := freeMemLinePattern.FindStringSubmatch(raw)
	if len(m) != freeFieldCount {
		return MemoryFacts{}, ErrFreeUnparseable
	}
	total, _ := strconv.Atoi(m[1])
	used, _ := strconv.Atoi(m[2])
	return MemoryFacts{TotalMB: total, UsedMB: used}, nil
}

// FormatUptime renders the human-friendly badge string. We deliberately
// truncate at days+hours so the tile fits within the 12-column slot;
// the cockpit is not a forensic tool.
func FormatUptime(u UptimeFacts) string {
	switch {
	case u.Days > 0 && u.Hours > 0:
		return strconv.Itoa(u.Days) + "d " + strconv.Itoa(u.Hours) + "h"
	case u.Days > 0:
		return strconv.Itoa(u.Days) + "d"
	case u.Hours > 0:
		return strconv.Itoa(u.Hours) + "h " + strconv.Itoa(u.Minutes) + "m"
	case u.Minutes > 0:
		return strconv.Itoa(u.Minutes) + "m"
	default:
		return "—"
	}
}

// FormatRAM renders "<used>/<total>G (xx%)" for the tile. Inputs are
// MB; the conversion rounds to one decimal place.
func FormatRAM(m MemoryFacts) string {
	if m.TotalMB <= 0 {
		return "n/a"
	}
	usedG := float64(m.UsedMB) / bytesPerKilobyte
	totalG := float64(m.TotalMB) / bytesPerKilobyte
	return formatGB(usedG) + "/" + formatGB(totalG) + " (" + strconv.Itoa(m.PercentUsed()) + "%)"
}

func formatGB(g float64) string {
	if g < smallestRenderedGB {
		return "0.0G"
	}
	return strconv.FormatFloat(g, 'f', 1, 64) + "G"
}

// FormatLoadAvg returns "0.12, 0.28, 0.31". Float64 formatted with
// two decimal places matches the OS output for legibility.
func FormatLoadAvg(u UptimeFacts) string {
	return strconv.FormatFloat(u.LoadAvg1, 'f', 2, 64) + ", " +
		strconv.FormatFloat(u.LoadAvg5, 'f', 2, 64) + ", " +
		strconv.FormatFloat(u.LoadAvg15, 'f', 2, 64)
}

// FormatRTT returns "Nms" or "n/a" for a sub-millisecond unreadable
// reading. The tile colours RTT amber/red above thresholds, but the
// classification is done by the renderer; this function only formats.
func FormatRTT(d time.Duration) string {
	if d <= 0 {
		return "n/a"
	}
	ms := d.Milliseconds()
	if ms == 0 {
		ms = 1 // anything under 1ms still renders as 1ms — round-up
	}
	return strconv.FormatInt(ms, 10) + "ms"
}
