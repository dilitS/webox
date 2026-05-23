package smallhost

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Path constants mirror docs/providers/smallhost.md §3. They are
// fixed by the Devil panel layout; Webox never creates these
// directories — `devil www add` does — so we only describe the
// well-known locations to higher layers (rsync target, log tail,
// .env path).
const (
	domainsRoot      = "/usr/home"
	subdirNodeJS     = "public_nodejs"
	subdirPublic     = "public"
	subdirLogs       = "logs"
	envFilename      = ".env"
	subdirStorageImg = "uploads"

	// maxDomainLength is the RFC 1035 cap (255 bytes incl. the
	// trailing dot). We reject anything above this before regex
	// matching so a 64 KiB pathological input cannot defeat the
	// per-label check.
	maxDomainLength = 253
)

// ErrInvalidDomain is the sentinel returned by [ValidateDomain] when
// the input fails one of the per-character or per-label rules. It is
// adapter-local because the failure modes are too specific to fold
// into a generic providers-level sentinel; callers branch on the
// wrapped reason via the error message.
var ErrInvalidDomain = errors.New("smallhost: invalid domain")

// ErrInvalidUser is the sentinel for [ValidateUser] failures. Same
// rationale as [ErrInvalidDomain]: the failure is panel-specific and
// the wrapper text identifies the offending substring.
var ErrInvalidUser = errors.New("smallhost: invalid user")

// domainLabelPattern is the regex applied to each label of a domain
// segment. The label MUST:
//
//   - contain only lower-case letters, digits, and "-";
//   - be between 1 and 63 characters long;
//   - not start or end with "-".
//
// docs/providers/smallhost.md §5.1 reports the empirical 63-char
// limit; the broader RFC 1035 / DNS rules are stricter than the
// panel's accept list, so we apply them as our floor.
var domainLabelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// userPattern matches small.pl SSH usernames defensively. The panel
// uses short lowercase tokens; we accept the broader `_-` superset
// so existing accounts created off-panel keep working.
var userPattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// ValidateDomain checks that domain is safe to substitute into a
// shell-style path or panel command. It enforces:
//
//   - non-empty after TrimSpace;
//   - no whitespace, control bytes, or NUL anywhere;
//   - no path traversal segments ("..", "/", "\");
//   - every dot-separated label matches [domainLabelPattern].
//
// Returns nil on success; otherwise a wrapped [ErrInvalidDomain]
// whose message names the offending property (control byte, label
// mismatch, etc.) without echoing the entire payload, so operator
// logs stay readable.
func ValidateDomain(domain string) error {
	trimmed := strings.TrimSpace(domain)
	if trimmed == "" {
		return fmt.Errorf("%w: empty", ErrInvalidDomain)
	}
	if trimmed != domain {
		return fmt.Errorf("%w: surrounding whitespace", ErrInvalidDomain)
	}
	if len(domain) > maxDomainLength {
		return fmt.Errorf("%w: longer than %d characters", ErrInvalidDomain, maxDomainLength)
	}
	for i := 0; i < len(domain); i++ {
		c := domain[i]
		if c <= 0x20 || c == 0x7f {
			return fmt.Errorf("%w: control byte at offset %d", ErrInvalidDomain, i)
		}
		if c == '/' || c == '\\' {
			return fmt.Errorf("%w: path separator at offset %d", ErrInvalidDomain, i)
		}
	}
	for _, label := range strings.Split(domain, ".") {
		if label == ".." {
			return fmt.Errorf("%w: traversal label", ErrInvalidDomain)
		}
		if !domainLabelPattern.MatchString(label) {
			return fmt.Errorf("%w: label %q does not match %s", ErrInvalidDomain, label, domainLabelPattern.String())
		}
	}
	return nil
}

// ValidateUser validates the SSH user before it is substituted into a
// path. small.pl accounts are short alphanumeric strings; we apply a
// strict superset (`^[a-z0-9_-]{1,32}$`) so an unexpected character
// surfaces as a typed error rather than a shell injection vector.
func ValidateUser(user string) error {
	if user == "" {
		return fmt.Errorf("%w: empty", ErrInvalidUser)
	}
	if !userPattern.MatchString(user) {
		return fmt.Errorf("%w: %q does not match %s", ErrInvalidUser, user, userPattern.String())
	}
	return nil
}

// GetDeployPath returns the absolute path the deploy workflow rsyncs
// build artifacts into. It is the document root for static assets of
// a Node.js subdomain.
//
// Pure function: no I/O. domain is validated through ValidateDomain
// before substitution — invalid input returns "" so the caller's
// command builder fails closed instead of pointing rsync at "/".
func (p *Provider) GetDeployPath(domain string) string {
	if ValidateDomain(domain) != nil || ValidateUser(p.cfg.User) != nil {
		return ""
	}
	return joinPath(domainsRoot, p.cfg.User, "domains", domain, subdirNodeJS, subdirPublic)
}

// GetLogPath returns the absolute directory containing Node.js stdout /
// stderr logs for domain.
//
// Pure function: no I/O. Returns "" for invalid input (see GetDeployPath).
func (p *Provider) GetLogPath(domain string) string {
	if ValidateDomain(domain) != nil || ValidateUser(p.cfg.User) != nil {
		return ""
	}
	return joinPath(domainsRoot, p.cfg.User, "domains", domain, subdirLogs)
}

// EnvPath returns the absolute path to the .env file for domain. The
// file lives one level above the document root by design — see
// SECURITY §10.4. Not part of the HostingProvider interface today;
// exported for the env-merger wizard (STRETCH) and the post-deploy
// permission probe.
//
// Pure function: no I/O. Returns "" for invalid input.
func (p *Provider) EnvPath(domain string) string {
	if ValidateDomain(domain) != nil || ValidateUser(p.cfg.User) != nil {
		return ""
	}
	return joinPath(domainsRoot, p.cfg.User, "domains", domain, subdirNodeJS, envFilename)
}

// StoragePath returns the absolute directory used for user-uploaded
// assets — the canonical persistent dir excluded from rsync --delete.
// Not part of the HostingProvider interface today; exported for the
// workflow template generator (TASK-06.x) which needs to emit the
// per-project `--exclude` list.
//
// Pure function: no I/O. Returns "" for invalid input.
func (p *Provider) StoragePath(domain string) string {
	if ValidateDomain(domain) != nil || ValidateUser(p.cfg.User) != nil {
		return ""
	}
	return joinPath(domainsRoot, p.cfg.User, "domains", domain, subdirNodeJS, subdirPublic, subdirStorageImg)
}

// joinPath concatenates path segments with "/" separators. We do NOT
// use filepath.Join because the result is consumed by the remote
// Linux shell — on Windows builds, filepath.Join would emit "\"
// separators and break every command.
func joinPath(segments ...string) string {
	var sb strings.Builder
	for i, seg := range segments {
		if i > 0 && !strings.HasSuffix(sb.String(), "/") {
			sb.WriteByte('/')
		}
		sb.WriteString(seg)
	}
	return sb.String()
}
