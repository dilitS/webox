package smallhost

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/dilitS/webox/providers"
)

// maxOutputSize caps each panel command's stdout/stderr at 1 MiB. Any
// command that exceeds this is almost certainly broken (panel error
// dump, infinite loop, log redirected) and parsing larger blobs would
// add risk without value. See SECURITY §3.3.
const maxOutputSize = 1 << 20

// ansiEscapeRegex matches the CSI subset of ANSI escape sequences
// observed in `devil` output (colour, bold, reset). The full ECMA-48
// set is unnecessary — we only need to strip what the panel emits.
var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// stripAndNormalize prepares raw command output for regex matching:
//
//  1. Cap at maxOutputSize (anything larger is fail-closed via
//     ErrOutputTooLarge).
//  2. Strip ANSI escape sequences.
//  3. Normalize CRLF / CR line endings to LF.
//  4. Reject any remaining non-printable bytes (NUL, BEL, …) as
//     ErrUnknownOutputFormat — they are a signal that the strip
//     missed something, which means our regex would be running over
//     unsafe input.
//
// The returned bytes are safe to pass to regex parsers and to include
// in operator logs after truncation.
func stripAndNormalize(raw []byte) ([]byte, error) {
	if len(raw) > maxOutputSize {
		return nil, fmt.Errorf("%w: %d bytes (cap %d)", providers.ErrOutputTooLarge, len(raw), maxOutputSize)
	}

	clean := ansiEscapeRegex.ReplaceAll(raw, nil)
	clean = bytes.ReplaceAll(clean, []byte("\r\n"), []byte("\n"))
	clean = bytes.ReplaceAll(clean, []byte("\r"), []byte("\n"))

	for i, b := range clean {
		if b == '\n' || b == '\t' {
			continue
		}
		if b < 0x20 || b == 0x7f || b >= 0x80 {
			return nil, fmt.Errorf("%w: non-printable byte 0x%02x at offset %d", providers.ErrUnknownOutputFormat, b, i)
		}
	}
	return clean, nil
}

// WwwAddResult is the typed outcome of `devil www add` parsing.
type WwwAddResult struct {
	Domain      string
	NodeVersion string
}

var wwwAddOKRegex = regexp.MustCompile(`^Added domain (?P<domain>\S+) with nodejs (?P<node>\S+)\s*$`)

// parseWwwAdd parses the response of `devil www add <domain> nodejs <ver>`.
// Maps the three known shapes onto sentinels:
//
//   - "Added domain ... with nodejs ..." → result, nil
//   - "exists: domain already exists"     → ErrSubdomainExists
//   - "invalid node version ..."          → ErrNodeVersionUnsupported
//
// Anything else returns ErrUnknownOutputFormat without echoing the raw
// bytes — operator logs include the byte length only.
func parseWwwAdd(raw []byte) (*WwwAddResult, error) {
	clean, err := stripAndNormalize(raw)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(clean))

	switch {
	case strings.HasPrefix(text, "exists:"):
		return nil, providers.ErrSubdomainExists
	case strings.HasPrefix(text, "invalid node version"):
		return nil, providers.ErrNodeVersionUnsupported
	}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if match := wwwAddOKRegex.FindStringSubmatch(line); match != nil {
			return &WwwAddResult{
				Domain:      match[wwwAddOKRegex.SubexpIndex("domain")],
				NodeVersion: match[wwwAddOKRegex.SubexpIndex("node")],
			}, nil
		}
	}
	return nil, fmt.Errorf("%w: parseWwwAdd over %d bytes", providers.ErrUnknownOutputFormat, len(raw))
}

// parseWwwRestart parses the response of `devil www restart <domain>`.
// Returns nil on success, ErrAppNotFound / ErrAppNotNode on the two
// well-known failure modes, ErrUnknownOutputFormat otherwise.
func parseWwwRestart(raw []byte) error {
	clean, err := stripAndNormalize(raw)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(clean))

	switch {
	case strings.HasPrefix(text, "Restarted "):
		return nil
	case strings.HasPrefix(text, "not a nodejs domain"):
		return providers.ErrAppNotNode
	case strings.HasPrefix(text, "not found"):
		return providers.ErrAppNotFound
	}
	return fmt.Errorf("%w: parseWwwRestart over %d bytes", providers.ErrUnknownOutputFormat, len(raw))
}

// wwwListLineRegex matches a single row of `devil www list` after
// whitespace-splitting columns. Named groups: domain, type,
// node_version. The node_version slot may be `-` for non-Node rows.
var wwwListLineRegex = regexp.MustCompile(`^(?P<domain>\S+)\s+(?P<type>nodejs|static|php)\s+(?P<node>\S+)\s*$`)

// parseWwwList parses the response of `devil www list`. The header
// row is recognised by prefix and skipped; empty output (headers only
// / nothing) returns an empty non-nil slice. Any line that fails the
// regex is fatal — better to surface a parse error than to drop a row
// silently and produce drift between config and panel.
func parseWwwList(raw []byte) ([]providers.Subdomain, error) {
	clean, err := stripAndNormalize(raw)
	if err != nil {
		return nil, err
	}

	out := make([]providers.Subdomain, 0)
	for i, line := range strings.Split(string(clean), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if i == 0 && strings.HasPrefix(line, "domain") {
			continue
		}
		match := wwwListLineRegex.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("%w: parseWwwList row %d", providers.ErrUnknownOutputFormat, i)
		}
		row := providers.Subdomain{
			Domain: match[wwwListLineRegex.SubexpIndex("domain")],
			Type:   match[wwwListLineRegex.SubexpIndex("type")],
		}
		if nv := match[wwwListLineRegex.SubexpIndex("node")]; nv != "-" {
			row.NodeVersion = nv
		}
		out = append(out, row)
	}
	return out, nil
}
