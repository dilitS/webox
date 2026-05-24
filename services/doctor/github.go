package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dilitS/webox/internal/log"
	"github.com/dilitS/webox/secrets"
)

// ErrRateLimitFetcherMissing is returned when [withGitHubDepsDefaults]
// is invoked without a configured RateLimit dep. Exported as a static
// sentinel so callers can [errors.Is] check rather than parsing the
// message string.
var ErrRateLimitFetcherMissing = errors.New("github: rate-limit fetcher not configured")

// GitHubRateLimit is the subset of the `/rate_limit` response the
// doctor surfaces. Reused by the dashboard via the GitHub fetcher
// without duplicating the parser.
type GitHubRateLimit struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	Reset     int64 `json:"reset"`
}

// GitHubDeps is the test seam for the GitHub doctor. Production wires
// it through [DefaultGitHubDeps]; tests substitute deterministic stubs
// so the doctor stays offline.
type GitHubDeps struct {
	// LookupGH returns the absolute path to `gh` on PATH, or an error
	// if the CLI is missing. Empty path with nil error is treated the
	// same as a missing binary.
	LookupGH func(ctx context.Context) (string, error)
	// AuthStatus returns the combined stdout+stderr of `gh auth
	// status`. The doctor never logs the raw output verbatim — it is
	// always passed through [log.Redact] before any operator
	// inspection.
	AuthStatus func(ctx context.Context) (string, error)
	// RateLimit fetches the GitHub /rate_limit core bucket. The
	// implementation chooses transport (REST or `gh api`) and must
	// time out within a few seconds.
	RateLimit func(ctx context.Context) (*GitHubRateLimit, error)
	// PATPresent reports whether the secrets backend has a GitHub PAT
	// stored under the default slot. The function MUST NOT return the
	// token value itself — only a boolean presence flag.
	PATPresent func() (bool, error)
}

const (
	// defaultGitHubPATAccount keeps doctor and the wizard aligned on
	// the same logical slot when a user has only one GitHub account.
	defaultGitHubPATAccount = "default"

	rateLimitWarnRatio   = 0.1
	rateLimitWarnPercent = "10%"
	githubAPITimeout     = 5 * time.Second
	authStatusOneLineCap = 240

	// authStatusParts is the number of fields produced by splitting
	// `gh auth status` "Logged in to ... account <name> ..." lines
	// on " account " — owner + remainder.
	authStatusParts = 2

	// authStatusAccountFields is how many tokens we slice the
	// remainder into so the account name is the first element.
	authStatusAccountFields = 2

	// minutesPerHour is used to render the rate-limit reset window.
	minutesPerHour = 60
)

// NewGitHub returns a [Doctor] that runs only the GitHub-integration
// checks. The check set is intentionally read-only: no `gh repo
// create`, no `secret set`, no PAT mutation — every probe is a
// `--version`-class or `GET`-only call. The doctor never writes the
// PAT value into the report; it only reports its presence.
func NewGitHub(deps GitHubDeps, opts Options) *Doctor {
	return New(githubChecks(deps), opts)
}

// NewGitHubDefault constructs the production GitHub doctor against
// the live process: `gh` on PATH, keyring-stored PAT, and the
// authenticated GitHub REST endpoint for `/rate_limit`.
func NewGitHubDefault() *Doctor {
	return NewGitHub(DefaultGitHubDeps(), Options{})
}

// DefaultGitHubDeps wires the production fetchers used by
// [NewGitHubDefault]. Exported so an integration harness can swap one
// fetcher at a time when reproducing a doctor finding.
func DefaultGitHubDeps() GitHubDeps {
	patReader := func() (bool, error) {
		backend, err := secrets.Detect()
		if err != nil {
			return false, err
		}
		token, err := secrets.GetGitHubPAT(backend, defaultGitHubPATAccount)
		if err != nil {
			if errors.Is(err, secrets.ErrSecretNotFound) {
				return false, nil
			}
			return false, err
		}
		return len(token) > 0, nil
	}
	return GitHubDeps{
		LookupGH:   lookupGH,
		AuthStatus: ghAuthStatus,
		RateLimit:  ghRateLimitCLI,
		PATPresent: patReader,
	}
}

func githubChecks(deps GitHubDeps) []Check {
	deps = withGitHubDepsDefaults(deps)
	return []Check{
		checkFunc(func(ctx context.Context) Result { return checkGHCLI(ctx, deps) }),
		checkFunc(func(ctx context.Context) Result { return checkGHAuthStatus(ctx, deps) }),
		checkFunc(func(ctx context.Context) Result { return checkRateLimit(ctx, deps) }),
		checkFunc(func(_ context.Context) Result { return checkPATSlot(deps) }),
	}
}

func checkGHCLI(ctx context.Context, deps GitHubDeps) Result {
	path, err := deps.LookupGH(ctx)
	if err != nil || path == "" {
		return Result{
			ID:       "github.gh_cli_available",
			Category: "github",
			Severity: SeverityWarn,
			Status:   StatusWarn,
			Message:  "gh CLI not found on PATH.",
			Hint:     "Install with `brew install gh` or see https://cli.github.com/. Webox can fall back to a PAT stored in the keyring.",
		}
	}
	return Result{
		ID:       "github.gh_cli_available",
		Category: "github",
		Severity: SeverityInfo,
		Status:   StatusOK,
		Message:  "gh CLI present at " + path + ".",
	}
}

func checkGHAuthStatus(ctx context.Context, deps GitHubDeps) Result {
	raw, err := deps.AuthStatus(ctx)
	if err != nil {
		return Result{
			ID:       "github.gh_auth_status",
			Category: "github",
			Severity: SeverityWarn,
			Status:   StatusWarn,
			Message:  "gh auth status unavailable.",
			Hint:     "Run `gh auth login` or store a PAT under the `github:pat:default` keyring slot.",
		}
	}
	account := parseGHAccount(raw)
	redacted := log.Redact(raw)
	if account == "" {
		return Result{
			ID:       "github.gh_auth_status",
			Category: "github",
			Severity: SeverityWarn,
			Status:   StatusWarn,
			Message:  "gh CLI returned status but no active account was parsed.",
			Hint:     trimOneLine(redacted),
		}
	}
	return Result{
		ID:       "github.gh_auth_status",
		Category: "github",
		Severity: SeverityInfo,
		Status:   StatusOK,
		Message:  "gh CLI authenticated as " + account + ".",
	}
}

func checkRateLimit(ctx context.Context, deps GitHubDeps) Result {
	limit, err := deps.RateLimit(ctx)
	if err != nil {
		return Result{
			ID:       "github.api_rate_limit",
			Category: "github",
			Severity: SeverityWarn,
			Status:   StatusWarn,
			Message:  "GitHub /rate_limit probe failed: " + log.Redact(err.Error()),
			Hint:     "Verify network access to api.github.com or refresh the gh / PAT credentials.",
		}
	}
	if limit == nil || limit.Limit == 0 {
		return Result{
			ID:       "github.api_rate_limit",
			Category: "github",
			Severity: SeverityInfo,
			Status:   StatusSkipped,
			Message:  "GitHub /rate_limit returned no data.",
		}
	}
	ratio := float64(limit.Remaining) / float64(limit.Limit)
	msg := fmt.Sprintf("GitHub rate limit: %d / %d remaining (reset %s).",
		limit.Remaining, limit.Limit, formatResetWindow(limit.Reset))
	if ratio < rateLimitWarnRatio {
		return Result{
			ID:       "github.api_rate_limit",
			Category: "github",
			Severity: SeverityWarn,
			Status:   StatusWarn,
			Message:  msg,
			Hint:     "Below " + rateLimitWarnPercent + " of quota; back off or rotate the token until the reset window.",
		}
	}
	return Result{
		ID:       "github.api_rate_limit",
		Category: "github",
		Severity: SeverityInfo,
		Status:   StatusOK,
		Message:  msg,
	}
}

func checkPATSlot(deps GitHubDeps) Result {
	present, err := deps.PATPresent()
	switch {
	case err != nil && errors.Is(err, secrets.ErrKeyringUnavailable):
		return Result{
			ID:       "github.pat_keyring_slot",
			Category: "github",
			Severity: SeverityInfo,
			Status:   StatusSkipped,
			Message:  "Secrets backend unavailable; PAT slot not inspected.",
			Hint:     "Run `webox doctor` to confirm the secrets backend status (keyring vs encrypted fallback).",
		}
	case err != nil:
		return Result{
			ID:       "github.pat_keyring_slot",
			Category: "github",
			Severity: SeverityWarn,
			Status:   StatusWarn,
			Message:  "PAT slot probe failed: " + log.Redact(err.Error()),
			Hint:     "Inspect the secrets backend with `webox doctor` before re-running this check.",
		}
	case present:
		return Result{
			ID:       "github.pat_keyring_slot",
			Category: "github",
			Severity: SeverityInfo,
			Status:   StatusOK,
			Message:  "GitHub PAT found in the secrets backend.",
		}
	default:
		return Result{
			ID:       "github.pat_keyring_slot",
			Category: "github",
			Severity: SeverityInfo,
			Status:   StatusSkipped,
			Message:  "No GitHub PAT stored. Webox will rely on `gh auth` for API calls.",
		}
	}
}

func withGitHubDepsDefaults(deps GitHubDeps) GitHubDeps {
	if deps.LookupGH == nil {
		deps.LookupGH = lookupGH
	}
	if deps.AuthStatus == nil {
		deps.AuthStatus = ghAuthStatus
	}
	if deps.RateLimit == nil {
		deps.RateLimit = func(context.Context) (*GitHubRateLimit, error) {
			return nil, ErrRateLimitFetcherMissing
		}
	}
	if deps.PATPresent == nil {
		deps.PATPresent = func() (bool, error) { return false, nil }
	}
	return deps
}

func lookupGH(_ context.Context) (string, error) {
	return exec.LookPath("gh")
}

func ghAuthStatus(ctx context.Context) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, githubAPITimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "gh", "auth", "status") //nolint:gosec // G204: constant program name "gh", no user-controlled args.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh auth status: %w: %s", err, log.Redact(stderr.String()))
	}
	return stderr.String() + stdout.String(), nil
}

func ghRateLimitCLI(ctx context.Context) (*GitHubRateLimit, error) {
	cctx, cancel := context.WithTimeout(ctx, githubAPITimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "gh", "api", "/rate_limit") //nolint:gosec // G204: constant program name "gh", no user-controlled args.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh api /rate_limit: %w: %s", err, log.Redact(stderr.String()))
	}
	return parseRateLimit(stdout.Bytes())
}

func parseRateLimit(raw []byte) (*GitHubRateLimit, error) {
	var resp struct {
		Resources struct {
			Core GitHubRateLimit `json:"core"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse rate-limit response: %w", err)
	}
	if resp.Resources.Core.Limit == 0 {
		return nil, nil //nolint:nilnil // documented "no data" sentinel
	}
	return &resp.Resources.Core, nil
}

// parseGHAccount extracts the active account from `gh auth status`
// output. Returns empty string when no account line was parsed.
//
// `gh` 2.40+ prefixes the status line with `✓ ` (or `x ` on failure),
// so we strip leading non-letter glyphs before pattern matching.
// The accepted line shapes are:
//
//	✓ Logged in to github.com account dilitS (keyring)
//	  Logged in to github.com as alice (oauth_token)  // legacy ≤2.39
//
// Anything else is ignored so a future format change degrades to the
// "raw status, no active account parsed" WARN rather than a panic.
func parseGHAccount(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		trimmed := stripStatusGlyph(strings.TrimSpace(line))
		if !strings.HasPrefix(trimmed, "Logged in to") {
			continue
		}
		if account := parseAccountClause(trimmed, " account "); account != "" {
			return account
		}
		if account := parseAccountClause(trimmed, " as "); account != "" {
			return account
		}
	}
	return ""
}

// stripStatusGlyph removes the leading status bullet `gh` adds in
// 2.40+ (`✓` or `x`) and any whitespace that follows so the
// downstream HasPrefix check works regardless of the gh version.
func stripStatusGlyph(line string) string {
	const (
		successGlyph = "✓"
		failureGlyph = "x"
	)
	for _, glyph := range []string{successGlyph, failureGlyph} {
		if strings.HasPrefix(line, glyph) {
			return strings.TrimSpace(strings.TrimPrefix(line, glyph))
		}
	}
	return line
}

// parseAccountClause is the shared splitter for the two known
// "Logged in to ..." line shapes. Returns "" when sep is absent so
// the caller can try the next pattern.
func parseAccountClause(line, sep string) string {
	parts := strings.Split(line, sep)
	if len(parts) != authStatusParts {
		return ""
	}
	return strings.SplitN(parts[1], " ", authStatusAccountFields)[0]
}

func trimOneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > authStatusOneLineCap {
		return s[:authStatusOneLineCap] + "..."
	}
	return s
}

func formatResetWindow(epoch int64) string {
	if epoch <= 0 {
		return "unknown"
	}
	reset := time.Unix(epoch, 0).UTC()
	delta := time.Until(reset)
	switch {
	case delta < 0:
		return "now"
	case delta < time.Minute:
		return "less than 1m"
	case delta < time.Hour:
		return fmt.Sprintf("in %dm", int(delta.Minutes()))
	default:
		return fmt.Sprintf("in %dh%02dm", int(delta.Hours()), int(delta.Minutes())%minutesPerHour)
	}
}
