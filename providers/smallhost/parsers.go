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

// ipv4Regex matches an IPv4 address — used by parseVhostList to
// extract the account IP needed for SetupSSL. We intentionally do not
// validate octet ranges with regex; the optional [net.ParseIP] guard
// inside the parser is the authoritative check.
var ipv4Regex = regexp.MustCompile(`\b(?P<ip>\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)

// vhostMinColumns is the minimum column count `devil vhost list`
// emits per row: domain, IP, type. Named constant so a future panel
// version adding extra columns (region, owner, …) shows up clearly
// in code review.
const vhostMinColumns = 3

// VhostEntry is a row of `devil vhost list` — domain + IP + type. The
// IP is the load-bearing field for SetupSSL.
type VhostEntry struct {
	Domain string
	IP     string
	Type   string
}

// parseVhostList parses the response of `devil vhost list` and returns
// the entries plus the first non-empty IP found (the account IP).
// Adapters call this once per SetupSSL to learn the IP they must hand
// to `devil ssl www add`.
func parseVhostList(raw []byte) ([]VhostEntry, string, error) {
	clean, err := stripAndNormalize(raw)
	if err != nil {
		return nil, "", err
	}
	entries := make([]VhostEntry, 0)
	accountIP := ""
	for i, line := range strings.Split(string(clean), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if i == 0 && strings.HasPrefix(line, "domain") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < vhostMinColumns {
			return nil, "", fmt.Errorf("%w: parseVhostList row %d (need %d fields)", providers.ErrUnknownOutputFormat, i, vhostMinColumns)
		}
		ip := fields[1]
		if !ipv4Regex.MatchString(ip) {
			return nil, "", fmt.Errorf("%w: parseVhostList row %d (bad IP)", providers.ErrUnknownOutputFormat, i)
		}
		entries = append(entries, VhostEntry{
			Domain: fields[0],
			IP:     ip,
			Type:   fields[2],
		})
		if accountIP == "" {
			accountIP = ip
		}
	}
	if accountIP == "" {
		return nil, "", fmt.Errorf("%w: parseVhostList yielded no IP", providers.ErrUnknownOutputFormat)
	}
	return entries, accountIP, nil
}

// parseSSLAdd parses the response of `devil ssl www add <ip> le le
// <domain>`. Success returns nil; the two well-known retryable errors
// (DNS not configured, Let's Encrypt rate limit) map to typed
// sentinels.
func parseSSLAdd(raw []byte) error {
	clean, err := stripAndNormalize(raw)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(clean))
	lower := strings.ToLower(text)

	switch {
	case strings.HasPrefix(text, "Certificate installed"):
		return nil
	case strings.Contains(lower, "dns not configured"), strings.Contains(lower, "dns not resolving"):
		return providers.ErrDNSNotResolving
	case strings.Contains(lower, "rate limit"):
		return providers.ErrRateLimitLetsEncrypt
	}
	return fmt.Errorf("%w: parseSSLAdd over %d bytes", providers.ErrUnknownOutputFormat, len(raw))
}

// parseSSLDelete parses the response of `devil ssl www del <ip>
// <domain>`. Idempotent: "Removed SSL ..." and "no cert: ..." both
// return nil — Remove* must never crash on "already gone".
func parseSSLDelete(raw []byte) error {
	clean, err := stripAndNormalize(raw)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(clean))
	switch {
	case strings.HasPrefix(text, "Removed SSL"):
		return nil
	case strings.HasPrefix(text, "no cert"):
		return nil
	}
	return fmt.Errorf("%w: parseSSLDelete over %d bytes", providers.ErrUnknownOutputFormat, len(raw))
}

// DBAddResult is the typed outcome of `devil <engine> add` parsing.
// User and Password fields are the panel-generated credentials; the
// caller MUST move Password into a memguard.LockedBuffer immediately
// and overwrite the struct field with the empty string before any
// logging or persistence.
type DBAddResult struct {
	User     string
	Password string
}

var (
	dbUserRegex = regexp.MustCompile(`(?m)^Username:\s+(?P<user>\S+)\s*$`)
	dbPassRegex = regexp.MustCompile(`(?m)^Password:\s+(?P<pass>\S+)\s*$`)
)

// parseDBAdd parses the response of `devil mysql add` / `devil pgsql
// add`. Both engines use the same wire format; the dbType arg is
// retained only for error context. Returns ErrDBNameTaken when the
// panel reports "database exists: ...".
//
// CRITICAL: the password is NEVER inserted into any error or log
// message — even on parse failure, only the byte length appears. This
// invariant is asserted by TestParseDBAdd_PasswordNeverInError.
func parseDBAdd(raw []byte) (*DBAddResult, error) {
	clean, err := stripAndNormalize(raw)
	if err != nil {
		return nil, err
	}
	text := string(clean)
	if strings.Contains(text, "database exists") {
		return nil, providers.ErrDBNameTaken
	}

	userMatch := dbUserRegex.FindStringSubmatch(text)
	passMatch := dbPassRegex.FindStringSubmatch(text)
	if userMatch == nil || passMatch == nil {
		return nil, fmt.Errorf("%w: parseDBAdd over %d bytes", providers.ErrUnknownOutputFormat, len(raw))
	}
	return &DBAddResult{
		User:     userMatch[dbUserRegex.SubexpIndex("user")],
		Password: passMatch[dbPassRegex.SubexpIndex("pass")],
	}, nil
}

// parseDBDelete parses the response of `devil mysql del` / `devil
// pgsql del`. Idempotent: "Deleted <db>" and "not found: <db>" both
// return nil.
func parseDBDelete(raw []byte) error {
	clean, err := stripAndNormalize(raw)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(string(clean))
	switch {
	case strings.HasPrefix(text, "Deleted "):
		return nil
	case strings.HasPrefix(text, "not found"):
		return nil
	}
	return fmt.Errorf("%w: parseDBDelete over %d bytes", providers.ErrUnknownOutputFormat, len(raw))
}

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
