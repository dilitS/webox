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

	"github.com/dilitS/webox/providers/cpanel/uapi"
)

// cpanelOpts captures every flag `webox doctor cpanel` understands.
// Defaults are applied by [runDoctorCpanel] (not the parser) so the
// CLI surface stays simple — `runDoctorCpanel` is the single
// authoritative place that documents which fields are required and
// which are optional.
type cpanelOpts struct {
	host           string
	user           string
	token          string
	apiPort        int
	sshPort        int
	timeout        time.Duration
	json           bool
	noSSH          bool
	noUAPI         bool
	httpsTransport http.RoundTripper
	sshFactory     func(host, user string, port int, timeout time.Duration) (uapi.SSHRunner, error)
}

// cpanelVerdict is the rolled-up status of a `doctor cpanel` run.
// The CLI exit code mirrors it: OK→0, DEGRADED→0 (warnings), BLOCKED→1.
type cpanelVerdict string

const (
	cpanelOK       cpanelVerdict = "OK"
	cpanelDegraded cpanelVerdict = "DEGRADED"
	cpanelBlocked  cpanelVerdict = "BLOCKED"

	defaultCpanelAPIPort = 2083
	defaultCpanelSSHPort = 22
	defaultCpanelTimeout = 30 * time.Second
	cpanelPreviewLineCap = 6
	cpanelPreviewWordCap = 80
)

// Validation sentinels for `runDoctorCpanel`. Wrapping them via
// typed errors lets callers branch on errors.Is without parsing
// strings, and satisfies the err113 lint policy that bans free-
// form errors.New strings inside the dispatch path.
var (
	errCpanelHostRequired        = errors.New("--host is required (e.g. --host=panel.example.com)")
	errCpanelUserRequired        = errors.New("--user is required (e.g. --user=operator)")
	errCpanelNoTransport         = errors.New("at least one transport must be enabled (drop --no-uapi or --no-ssh)")
	errCpanelTokenRequiredNoSSH  = errors.New("--token=TOKEN is required when --no-ssh is set (HTTPS is the only enabled transport)")
	errCpanelBuilderUnreachable  = errors.New("no transport configured (logic bug: validateCpanelOpts should have caught this)")
	errCpanelRunnerNeedsHostUser = errors.New("nativeSSHCmdRunner: host and user are required")
)

// cpanelSectionResult is one section of the doctor report: domains,
// passenger apps, mysql databases, or SSL keys. Each section
// independently records the transport that served it and any error
// surfaced; the rollup verdict aggregates the four.
type cpanelSectionResult struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	Status    string   `json:"status"`
	Count     int      `json:"count"`
	Sample    []string `json:"sample,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// cpanelReport is the machine-readable shape `--json` emits.
type cpanelReport struct {
	Host     string                `json:"host"`
	User     string                `json:"user"`
	APIPort  int                   `json:"api_port,omitempty"`
	SSHPort  int                   `json:"ssh_port,omitempty"`
	Verdict  cpanelVerdict         `json:"verdict"`
	Sections []cpanelSectionResult `json:"sections"`
	Notes    []string              `json:"notes,omitempty"`
}

// runDoctorCpanel is the production entry point for `webox doctor
// cpanel`. It validates the flags, builds the [uapi.Reader] chain
// (HTTPS primary, SSH fallback), invokes the four read-only methods,
// and prints either a text report or a JSON document.
//
// Required flags:
//   - --host=HOST and --user=USER are always required.
//   - --token=TOKEN is required when SSH fallback is disabled
//     (--no-ssh) because UAPI HTTPS needs a token.
//   - Either UAPI or SSH must be enabled; --no-uapi --no-ssh
//     simultaneously is a misuse.
//
// Exit codes:
//   - 0  : verdict OK or DEGRADED (DEGRADED still finishes with a
//     rendered report so the operator can see partial success).
//   - 1  : verdict BLOCKED (every section failed).
//   - 2  : flag validation error.
func runDoctorCpanel(opts cpanelOpts, stdout, stderr io.Writer) int {
	if err := validateCpanelOpts(&opts); err != nil {
		fmt.Fprintf(stderr, "webox: %v\n", err)
		return exitMisuse
	}
	reader, notes, err := buildCpanelReader(opts)
	if err != nil {
		fmt.Fprintf(stderr, "webox: cpanel doctor: %v\n", err)
		return exitGeneric
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()
	report := runCpanelChecks(ctx, opts, reader, notes)
	return emitCpanelReport(opts.json, report, stdout, stderr)
}

// validateCpanelOpts applies defaults and rejects impossible
// combinations. Exposed for unit tests via the runDoctorCpanel
// entry point.
func validateCpanelOpts(opts *cpanelOpts) error {
	switch {
	case opts.host == "":
		return errCpanelHostRequired
	case opts.user == "":
		return errCpanelUserRequired
	case opts.noSSH && opts.noUAPI:
		return errCpanelNoTransport
	case opts.token == "" && opts.noSSH:
		return errCpanelTokenRequiredNoSSH
	}
	if opts.apiPort == 0 {
		opts.apiPort = defaultCpanelAPIPort
	}
	if opts.sshPort == 0 {
		opts.sshPort = defaultCpanelSSHPort
	}
	if opts.timeout == 0 {
		opts.timeout = defaultCpanelTimeout
	}
	return nil
}

// buildCpanelReader composes the HTTPS client and SSH fallback per
// the flags, returning a single [uapi.Reader] the rest of the
// command speaks to. Notes are operator-facing messages explaining
// which transports were actually wired (e.g. "--token absent: HTTPS
// disabled, only SSH wired").
func buildCpanelReader(opts cpanelOpts) (uapi.Reader, []string, error) {
	var (
		primary   uapi.Reader
		secondary uapi.Reader
		notes     []string
	)
	switch {
	case !opts.noUAPI && opts.token != "":
		baseURL := fmt.Sprintf("https://%s:%d", opts.host, opts.apiPort)
		httpClient := &http.Client{Timeout: opts.timeout}
		if opts.httpsTransport != nil {
			httpClient.Transport = opts.httpsTransport
		}
		c, err := uapi.NewClient(baseURL, opts.user, opts.token, httpClient)
		if err != nil {
			return nil, nil, fmt.Errorf("HTTPS UAPI: %w", err)
		}
		primary = c
	case opts.noUAPI:
		notes = append(notes, "--no-uapi: HTTPS UAPI disabled, falling back to SSH")
	default:
		notes = append(notes, "--token absent: HTTPS UAPI disabled, falling back to SSH")
	}
	if !opts.noSSH {
		factory := opts.sshFactory
		if factory == nil {
			factory = newNativeSSHRunner
		}
		runner, err := factory(opts.host, opts.user, opts.sshPort, opts.timeout)
		if err != nil {
			return nil, nil, fmt.Errorf("SSH runner: %w", err)
		}
		fallback, fbErr := uapi.NewSSHFallback(runner, opts.user)
		if fbErr != nil {
			return nil, nil, fmt.Errorf("SSH fallback: %w", fbErr)
		}
		secondary = fallback
	} else {
		notes = append(notes, "--no-ssh: SSH fallback disabled")
	}
	if primary == nil && secondary == nil {
		return nil, notes, errCpanelBuilderUnreachable
	}
	return &uapi.Composite{Primary: primary, Secondary: secondary}, notes, nil
}

// runCpanelChecks executes the four read-only modules and rolls
// every outcome into a [cpanelReport]. The rollup verdict is OK if
// every section succeeded, BLOCKED if every section failed, and
// DEGRADED in any mixed case.
func runCpanelChecks(ctx context.Context, opts cpanelOpts, reader uapi.Reader, notes []string) cpanelReport {
	report := cpanelReport{
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
		{"Domains", func() (int, []string, string, error) {
			r, err := reader.ListDomains(ctx)
			if err != nil || r == nil {
				return 0, nil, "n/a", err
			}
			sample := []string{r.MainDomain}
			sample = appendCapped(sample, r.SubDomains)
			sample = appendCapped(sample, r.AddonDomains)
			sample = appendCapped(sample, r.ParkedDomains)
			return 1 + len(r.SubDomains) + len(r.AddonDomains) + len(r.ParkedDomains), sample, transportLabel(reader), nil
		}},
		{"PassengerApps", func() (int, []string, string, error) {
			r, err := reader.ListPassengerApps(ctx)
			if err != nil || r == nil {
				return 0, nil, "n/a", err
			}
			names := make([]string, 0, len(r.Applications))
			for _, app := range r.Applications {
				names = append(names, app.Name)
			}
			return len(r.Applications), capSample(names), transportLabel(reader), nil
		}},
		{"MysqlDatabases", func() (int, []string, string, error) {
			r, err := reader.ListMysqlDatabases(ctx)
			if err != nil || r == nil {
				return 0, nil, "n/a", err
			}
			names := make([]string, 0, len(r.Databases))
			for _, db := range r.Databases {
				names = append(names, db.Name)
			}
			return len(r.Databases), capSample(names), transportLabel(reader), nil
		}},
		{"SSLKeys", func() (int, []string, string, error) {
			r, err := reader.ListSSLKeys(ctx)
			if err != nil || r == nil {
				return 0, nil, "n/a", err
			}
			names := make([]string, 0, len(r.Keys))
			for _, k := range r.Keys {
				names = append(names, k.FriendlyName)
			}
			return len(r.Keys), capSample(names), transportLabel(reader), nil
		}},
	}
	for _, sec := range sections {
		count, sample, transport, err := sec.run()
		res := cpanelSectionResult{
			Name:      sec.name,
			Transport: transport,
			Count:     count,
			Sample:    sample,
		}
		switch {
		case err == nil:
			res.Status = "OK"
		case errors.Is(err, uapi.ErrModuleFunctionDenied):
			res.Status = "DISABLED"
			res.Error = err.Error()
		case errors.Is(err, uapi.ErrAuthenticationFailed):
			res.Status = "AUTH_FAILED"
			res.Error = err.Error()
		case errors.Is(err, uapi.ErrTransportUnavailable):
			res.Status = "UNREACHABLE"
			res.Error = err.Error()
		default:
			res.Status = "FAILED"
			res.Error = err.Error()
		}
		report.Sections = append(report.Sections, res)
	}
	report.Verdict = rollupCpanelVerdict(report.Sections)
	return report
}

// rollupCpanelVerdict computes the report's top-line status from
// the per-section results. Pure function — every test case is a
// straight table-row assertion.
func rollupCpanelVerdict(sections []cpanelSectionResult) cpanelVerdict {
	var okCount, failedCount int
	for _, sec := range sections {
		switch sec.Status {
		case "OK":
			okCount++
		case "DISABLED":
			// Disabled features are not failures; the cPanel
			// account simply doesn't have that capability.
			okCount++
		default:
			failedCount++
		}
	}
	switch {
	case okCount == len(sections):
		return cpanelOK
	case failedCount == len(sections):
		return cpanelBlocked
	default:
		return cpanelDegraded
	}
}

// transportLabel returns "HTTPS+SSH" for a fully wired composite,
// "HTTPS" or "SSH" for single-transport, and "?" otherwise.
// Surface-only; rendered next to every section.
func transportLabel(r uapi.Reader) string {
	c, ok := r.(*uapi.Composite)
	if !ok {
		return "?"
	}
	switch {
	case c.Primary != nil && c.Secondary != nil:
		return "HTTPS+SSH"
	case c.Primary != nil:
		return "HTTPS"
	case c.Secondary != nil:
		return "SSH"
	default:
		return "?"
	}
}

// appendCapped appends src to dst and returns the sample list,
// capped at cpanelPreviewLineCap entries total. Used to keep the
// text report from running off the screen when an account has 200
// addon domains.
func appendCapped(dst, src []string) []string {
	for _, s := range src {
		if len(dst) >= cpanelPreviewLineCap {
			return dst
		}
		dst = append(dst, s)
	}
	return dst
}

// capSample truncates to cpanelPreviewLineCap entries. The
// truncation marker is inlined as "(+N more)" so the text report
// always renders a complete final line.
func capSample(in []string) []string {
	if len(in) <= cpanelPreviewLineCap {
		return in
	}
	out := make([]string, cpanelPreviewLineCap, cpanelPreviewLineCap+1)
	copy(out, in[:cpanelPreviewLineCap])
	return append(out, fmt.Sprintf("(+%d more)", len(in)-cpanelPreviewLineCap))
}

// emitCpanelReport renders the report to stdout, returning the
// appropriate process exit code. Pulled out so both production and
// test callers can render through a single code path.
func emitCpanelReport(asJSON bool, report cpanelReport, stdout, stderr io.Writer) int {
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "webox: encode cpanel report: %v\n", err)
			return exitGeneric
		}
	} else {
		writeCpanelText(stdout, report)
	}
	switch report.Verdict {
	case cpanelOK, cpanelDegraded:
		return exitOK
	default:
		return exitGeneric
	}
}

// writeCpanelText renders the human-friendly report.
//
// See directadmin.go for the design rationale (per-provider
// report types share shape but not type).
//
//nolint:dupl // Duplicates writeDirectadminText (directadmin.go).
func writeCpanelText(stdout io.Writer, report cpanelReport) {
	fmt.Fprintf(stdout, "Webox doctor cpanel — %s\n", report.Host)
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
			fmt.Fprintf(stdout, "  error: %s\n", truncateWord(sec.Error, cpanelPreviewWordCap))
		}
		for _, s := range sec.Sample {
			fmt.Fprintf(stdout, "  - %s\n", s)
		}
		fmt.Fprintln(stdout)
	}
	fmt.Fprintf(stdout, "Verdict: %s\n", report.Verdict)
}

// truncateWord truncates s to at most n bytes; if truncated, an
// ellipsis (`...`) is appended so the report never silently lies
// about message length.
func truncateWord(s string, n int) string {
	if len(s) <= n {
		return s
	}
	trimmed := strings.TrimRight(s[:n], " \t")
	return trimmed + "..."
}
