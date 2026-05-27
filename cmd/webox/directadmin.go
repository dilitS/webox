package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	daapi "github.com/dilitS/webox/providers/directadmin/api"
)

// directadminOpts captures every flag `webox doctor directadmin`
// understands. Mirrors [cpanelOpts] surface so operators reading
// either subcommand's help see consistent shape.
type directadminOpts struct {
	host           string
	user           string
	loginKey       string
	apiPort        int
	sshPort        int
	timeout        time.Duration
	json           bool
	noSSH          bool
	noAPI          bool
	httpsTransport http.RoundTripper
	sshFactory     func(host, user string, port int, timeout time.Duration) (daapi.SSHRunner, error)
}

// directadminVerdict mirrors [cpanelVerdict]: OK→0, DEGRADED→0
// (warnings), BLOCKED→1.
type directadminVerdict string

const (
	directadminOK       directadminVerdict = "OK"
	directadminDegraded directadminVerdict = "DEGRADED"
	directadminBlocked  directadminVerdict = "BLOCKED"

	defaultDirectadminAPIPort = 2222
	defaultDirectadminSSHPort = 22
	defaultDirectadminTimeout = 30 * time.Second
	directadminPreviewLineCap = 6
	directadminPreviewWordCap = 80
)

// Validation sentinels mirror the cPanel set so callers can branch
// on errors.Is. Sprint 23 hard-renames `--token` to `--loginkey`
// for DA because the panel calls its bearer credential a "login
// key", not a token — preserving terminology avoids the friction
// of operators second-guessing which credential to paste.
var (
	errDAHostRequired          = errors.New("--host is required (e.g. --host=panel.example.com)")
	errDAUserRequired          = errors.New("--user is required (e.g. --user=operator)")
	errDANoTransport           = errors.New("at least one transport must be enabled (drop --no-api or --no-ssh)")
	errDALoginKeyRequiredNoSSH = errors.New("--loginkey=KEY is required when --no-ssh is set (HTTPS is the only enabled transport)")
	errDABuilderUnreachable    = errors.New("no transport configured (logic bug: validateDirectadminOpts should have caught this)")
	errDARunnerNeedsHostUser   = errors.New("nativeSSHCmdRunner: host and user are required")
)

// directadminSectionResult is one section of the doctor report:
// whoami, domains, subdomains, databases, or SSL certificates.
type directadminSectionResult struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	Status    string   `json:"status"`
	Count     int      `json:"count"`
	Sample    []string `json:"sample,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// directadminReport is the JSON shape `--json` emits. Mirrors
// [cpanelReport] so external automations (Sprint 24 GHA template)
// can consume both reports with the same schema scaffold.
type directadminReport struct {
	Host     string                     `json:"host"`
	User     string                     `json:"user"`
	APIPort  int                        `json:"api_port,omitempty"`
	SSHPort  int                        `json:"ssh_port,omitempty"`
	Verdict  directadminVerdict         `json:"verdict"`
	Sections []directadminSectionResult `json:"sections"`
	Notes    []string                   `json:"notes,omitempty"`
}

// runDoctorDirectadmin is the production entry point for
// `webox doctor directadmin`. Validates flags, builds the
// [daapi.Reader] composite, invokes the five read-only methods,
// and emits a text or JSON report.
//
// Required flags:
//   - --host=HOST and --user=USER are always required.
//   - --loginkey=KEY is required when SSH fallback is disabled.
//   - Either /api/ or SSH must be enabled; --no-api --no-ssh
//     simultaneously is a misuse.
//
// Exit codes mirror `doctor cpanel`:
//   - 0 : verdict OK or DEGRADED.
//   - 1 : verdict BLOCKED.
//   - 2 : flag validation error.
func runDoctorDirectadmin(opts directadminOpts, stdout, stderr io.Writer) int {
	if err := validateDirectadminOpts(&opts); err != nil {
		fmt.Fprintf(stderr, "webox: %v\n", err)
		return exitMisuse
	}
	reader, notes, err := buildDirectadminReader(opts)
	if err != nil {
		fmt.Fprintf(stderr, "webox: directadmin doctor: %v\n", err)
		return exitGeneric
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()
	report := runDirectadminChecks(ctx, opts, reader, notes)
	return emitDirectadminReport(opts.json, report, stdout, stderr)
}

// validateDirectadminOpts applies defaults and rejects impossible
// combinations.
func validateDirectadminOpts(opts *directadminOpts) error {
	switch {
	case opts.host == "":
		return errDAHostRequired
	case opts.user == "":
		return errDAUserRequired
	case opts.noSSH && opts.noAPI:
		return errDANoTransport
	case opts.loginKey == "" && opts.noSSH:
		return errDALoginKeyRequiredNoSSH
	}
	if opts.apiPort == 0 {
		opts.apiPort = defaultDirectadminAPIPort
	}
	if opts.sshPort == 0 {
		opts.sshPort = defaultDirectadminSSHPort
	}
	if opts.timeout == 0 {
		opts.timeout = defaultDirectadminTimeout
	}
	return nil
}

// buildDirectadminReader composes the HTTPS client and SSH
// fallback per the flags. Notes are operator-facing.
func buildDirectadminReader(opts directadminOpts) (daapi.Reader, []string, error) {
	var (
		primary   daapi.Reader
		secondary daapi.Reader
		notes     []string
	)
	switch {
	case !opts.noAPI && opts.loginKey != "":
		baseURL := fmt.Sprintf("https://%s:%d", opts.host, opts.apiPort)
		httpClient := &http.Client{Timeout: opts.timeout}
		if opts.httpsTransport != nil {
			httpClient.Transport = opts.httpsTransport
		}
		c, err := daapi.NewClient(baseURL, opts.user, opts.loginKey, httpClient)
		if err != nil {
			return nil, nil, fmt.Errorf("HTTPS Live API: %w", err)
		}
		primary = c
	case opts.noAPI:
		notes = append(notes, "--no-api: HTTPS Live API disabled, falling back to SSH")
	default:
		notes = append(notes, "--loginkey absent: HTTPS Live API disabled, falling back to SSH")
	}
	if !opts.noSSH {
		factory := opts.sshFactory
		if factory == nil {
			factory = newNativeDirectadminSSHRunner
		}
		runner, err := factory(opts.host, opts.user, opts.sshPort, opts.timeout)
		if err != nil {
			return nil, nil, fmt.Errorf("SSH runner: %w", err)
		}
		// SSH fallback also needs the loginkey because curl
		// inside the box hits the same Live API surface. When
		// missing, drop the secondary entirely so the composite
		// surfaces ErrAPIDisabled / ErrAuthenticationFailed
		// directly from the primary instead of pretending SSH
		// can save us.
		if opts.loginKey == "" {
			notes = append(notes, "--loginkey absent: SSH fallback also cannot reach /api/ (loopback curl needs the same key)")
		} else {
			fallback, fbErr := daapi.NewSSHFallback(runner, opts.user, opts.loginKey, opts.apiPort)
			if fbErr != nil {
				return nil, nil, fmt.Errorf("SSH fallback: %w", fbErr)
			}
			secondary = fallback
		}
	} else {
		notes = append(notes, "--no-ssh: SSH fallback disabled")
	}
	if primary == nil && secondary == nil {
		return nil, notes, errDABuilderUnreachable
	}
	return makeDirectadminComposite(primary, secondary), notes, nil
}

// makeDirectadminComposite handles the three viable wirings:
// both readers (typical), HTTPS-only (no SSH), SSH-only (no
// loginkey). The composite type requires both Primary and
// Secondary to be non-nil, so single-transport wiring degrades
// to the bare reader.
func makeDirectadminComposite(primary, secondary daapi.Reader) daapi.Reader {
	switch {
	case primary != nil && secondary != nil:
		c, _ := daapi.NewComposite(primary, secondary)
		return c
	case primary != nil:
		return primary
	default:
		return secondary
	}
}

// runDirectadminChecks executes the five read-only endpoints and
// rolls every outcome into a [directadminReport].
func runDirectadminChecks(ctx context.Context, opts directadminOpts, reader daapi.Reader, notes []string) directadminReport {
	report := directadminReport{
		Host:    opts.host,
		User:    opts.user,
		APIPort: opts.apiPort,
		SSHPort: opts.sshPort,
		Notes:   notes,
	}
	sections := []struct {
		name string
		run  func() (count int, sample []string, transport string, err error)
	}{
		{"Whoami", func() (int, []string, string, error) {
			r, err := reader.Whoami(ctx)
			if err != nil || r == nil {
				return 0, nil, "n/a", err
			}
			return 1, []string{fmt.Sprintf("%s (%s)", r.Username, r.UserType)}, directadminTransportLabel(reader), nil
		}},
		{"Domains", func() (int, []string, string, error) {
			r, err := reader.ListDomains(ctx)
			if err != nil {
				return 0, nil, "n/a", err
			}
			names := make([]string, 0, len(r))
			for _, d := range r {
				names = append(names, d.Name)
			}
			return len(r), capDirectadminSample(names), directadminTransportLabel(reader), nil
		}},
		{"Subdomains", func() (int, []string, string, error) {
			r, err := reader.ListSubdomains(ctx)
			if err != nil {
				return 0, nil, "n/a", err
			}
			names := make([]string, 0, len(r))
			for _, s := range r {
				names = append(names, s.Name)
			}
			return len(r), capDirectadminSample(names), directadminTransportLabel(reader), nil
		}},
		{"Databases", func() (int, []string, string, error) {
			r, err := reader.ListDatabases(ctx)
			if err != nil {
				return 0, nil, "n/a", err
			}
			names := make([]string, 0, len(r))
			for _, db := range r {
				names = append(names, db.Name)
			}
			return len(r), capDirectadminSample(names), directadminTransportLabel(reader), nil
		}},
		{"SSLCertificates", func() (int, []string, string, error) {
			r, err := reader.ListSSLCertificates(ctx)
			if err != nil {
				return 0, nil, "n/a", err
			}
			names := make([]string, 0, len(r))
			for _, c := range r {
				names = append(names, c.Domain)
			}
			return len(r), capDirectadminSample(names), directadminTransportLabel(reader), nil
		}},
	}
	for _, sec := range sections {
		count, sample, transport, err := sec.run()
		res := directadminSectionResult{
			Name:      sec.name,
			Transport: transport,
			Count:     count,
			Sample:    sample,
		}
		switch {
		case err == nil:
			res.Status = "OK"
		case errors.Is(err, daapi.ErrAPIDisabled):
			res.Status = "DISABLED"
			res.Error = err.Error()
		case errors.Is(err, daapi.ErrAuthenticationFailed):
			res.Status = "AUTH_FAILED"
			res.Error = err.Error()
		case errors.Is(err, daapi.ErrTransportUnavailable):
			res.Status = "UNREACHABLE"
			res.Error = err.Error()
		default:
			res.Status = "FAILED"
			res.Error = err.Error()
		}
		report.Sections = append(report.Sections, res)
	}
	report.Verdict = rollupDirectadminVerdict(report.Sections)
	return report
}

// rollupDirectadminVerdict mirrors [rollupCpanelVerdict]: DISABLED
// counts as success because the panel's API surface being off is
// a configuration choice, not a Webox failure.
func rollupDirectadminVerdict(sections []directadminSectionResult) directadminVerdict {
	var okCount, failedCount int
	for _, sec := range sections {
		switch sec.Status {
		case "OK", "DISABLED":
			okCount++
		default:
			failedCount++
		}
	}
	switch {
	case okCount == len(sections):
		return directadminOK
	case failedCount == len(sections):
		return directadminBlocked
	default:
		return directadminDegraded
	}
}

// directadminTransportLabel renders the transport hint next to
// each section. Composite shows "HTTPS+SSH"; bare Reader shows
// "HTTPS" or "SSH" depending on the type assertion.
func directadminTransportLabel(r daapi.Reader) string {
	if _, ok := r.(*daapi.Composite); ok {
		return "HTTPS+SSH"
	}
	if _, ok := r.(*daapi.Client); ok {
		return "HTTPS"
	}
	if _, ok := r.(*daapi.SSHFallback); ok {
		return "SSH"
	}
	return "?"
}

// capDirectadminSample mirrors capSample; pulled out to keep this
// file self-contained.
func capDirectadminSample(in []string) []string {
	if len(in) <= directadminPreviewLineCap {
		return in
	}
	out := make([]string, directadminPreviewLineCap, directadminPreviewLineCap+1)
	copy(out, in[:directadminPreviewLineCap])
	return append(out, fmt.Sprintf("(+%d more)", len(in)-directadminPreviewLineCap))
}

// emitDirectadminReport renders the report and returns the exit
// code (0 for OK/DEGRADED, 1 for BLOCKED).
func emitDirectadminReport(asJSON bool, report directadminReport, stdout, stderr io.Writer) int {
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "webox: encode directadmin report: %v\n", err)
			return exitGeneric
		}
	} else {
		writeDirectadminText(stdout, report)
	}
	switch report.Verdict {
	case directadminOK, directadminDegraded:
		return exitOK
	default:
		return exitGeneric
	}
}

// writeDirectadminText renders the human-friendly report.
//
// both functions render the same shape but each takes a package-
// local struct type (cpanelReport / directadminReport) with
// different field types (cpanelVerdict / directadminVerdict). A
// shared helper would require either runtime reflection or a
// type-parameterised generic abstraction; both obscure the
// per-provider error classification table for marginal LOC
// savings. Revisit when a third provider lands (Sprint 24+).
//
//nolint:dupl // Duplicates writeCpanelText (cpanel.go) by design:
func writeDirectadminText(stdout io.Writer, report directadminReport) {
	fmt.Fprintf(stdout, "Webox doctor directadmin — %s\n", report.Host)
	fmt.Fprintf(stdout, "  user            %s\n", report.User)
	if report.APIPort > 0 {
		fmt.Fprintf(stdout, "  api_port        %d\n", report.APIPort)
	}
	if report.SSHPort > 0 {
		fmt.Fprintf(stdout, "  ssh_port        %d\n", report.SSHPort)
	}
	fmt.Fprintf(stdout, "  verdict         %s\n", report.Verdict)
	for _, note := range report.Notes {
		fmt.Fprintf(stdout, "  note            %s\n", note)
	}
	fmt.Fprintln(stdout)
	for _, sec := range report.Sections {
		fmt.Fprintf(stdout, "[%s] %s · transport=%s · count=%d\n",
			sec.Status, sec.Name, sec.Transport, sec.Count)
		if sec.Error != "" {
			fmt.Fprintf(stdout, "  error: %s\n", truncateDirectadminWord(sec.Error, directadminPreviewWordCap))
		}
		for _, s := range sec.Sample {
			fmt.Fprintf(stdout, "  - %s\n", s)
		}
		fmt.Fprintln(stdout)
	}
	fmt.Fprintf(stdout, "Verdict: %s\n", report.Verdict)
}

// truncateDirectadminWord mirrors truncateWord — pulled out so
// the directadmin command stays self-contained should the cpanel
// helper relocate.
func truncateDirectadminWord(s string, n int) string {
	if len(s) <= n {
		return s
	}
	trimmed := strings.TrimRight(s[:n], " \t")
	return trimmed + "..."
}
