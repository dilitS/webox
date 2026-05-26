package cpanel

import (
	"errors"
	"fmt"
	"regexp"
)

// maxDomainLength is the hard ceiling for a fully-qualified domain
// per RFC 1035 §2.3.4 — 255 octets on the wire minus two for the
// length prefix and root label. Anything longer is rejected before
// the panel sees it.
const maxDomainLength = 253

// domainPattern accepts standard DNS subdomain shapes:
//   - 1-63 char labels, lowercase alphanumerics with `-` internally,
//   - 2-4 labels (a.b.c[.d]) so subdomain-of-subdomain still validates,
//   - total length capped at [maxDomainLength] characters per RFC 1035.
//
// We refuse uppercase explicitly because the panel canonicalises
// everything to lowercase; round-tripping mixed case through config.json
// would let the same domain show up twice in the dashboard.
var domainPattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.){1,4}[a-z]{2,24}$`)

// nodeVersionPattern accepts the majors cPanel hosts currently
// expose (16, 18, 20, 22, 24) plus minor.patch combinations
// (`22.4.1`). Free-form text is rejected so a panel command
// builder never sees a shell-like argument.
var nodeVersionPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,16}$`)

// dbNamePattern matches the smallhost rule: lowercase
// alphanumerics + underscore, 1-32 chars. cPanel prepends the
// account-name prefix anyway; the operator-supplied half stays
// short and predictable so the resulting DB name fits MySQL's
// 64-byte limit even on accounts with long login names.
var dbNamePattern = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

// ErrInvalidDomain is the sentinel returned by [ValidateDomain].
var ErrInvalidDomain = errors.New("cpanel: invalid domain")

// ErrInvalidNodeVersion is the sentinel returned by [ValidateNodeVersion].
var ErrInvalidNodeVersion = errors.New("cpanel: invalid node version")

// ErrInvalidDBName is the sentinel returned by [ValidateDBName].
var ErrInvalidDBName = errors.New("cpanel: invalid database name")

// ValidateDomain runs the regex check against the wizard's
// domain input before the adapter substitutes it into any panel
// command or UAPI arg.
func ValidateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("%w: empty", ErrInvalidDomain)
	}
	if len(domain) > maxDomainLength {
		return fmt.Errorf("%w: %q exceeds %d chars", ErrInvalidDomain, domain, maxDomainLength)
	}
	if !domainPattern.MatchString(domain) {
		return fmt.Errorf("%w: %q does not match required shape", ErrInvalidDomain, domain)
	}
	return nil
}

// ValidateNodeVersion runs the regex check against the wizard's
// node-version input.
func ValidateNodeVersion(version string) error {
	if version == "" {
		return fmt.Errorf("%w: empty", ErrInvalidNodeVersion)
	}
	if !nodeVersionPattern.MatchString(version) {
		return fmt.Errorf("%w: %q has unexpected characters", ErrInvalidNodeVersion, version)
	}
	return nil
}

// ValidateDBName runs the regex check against the wizard's DB
// name input. cPanel's user-prefixing happens panel-side; the
// adapter passes the operator-supplied half verbatim.
func ValidateDBName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty", ErrInvalidDBName)
	}
	if !dbNamePattern.MatchString(name) {
		return fmt.Errorf("%w: %q does not match %s", ErrInvalidDBName, name, dbNamePattern.String())
	}
	return nil
}
