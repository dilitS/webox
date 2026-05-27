package cpanel

import (
	"fmt"
	"strings"

	"github.com/dilitS/webox/wizard"
)

// WorkflowRestartCommand returns the shell command the GHA deploy
// workflow should execute over SSH to graceful-restart the
// Passenger-managed Node.js application living under appPath.
//
// We deliberately use Passenger's panel-agnostic restart mechanism
// (`touch <appPath>/tmp/restart.txt`) rather than the UAPI endpoint
// `PassengerApps::restart_application`. Reasoning:
//
//  1. The touch-restart hook works on EVERY cPanel host shipping
//     Passenger, including white-label hosters that disable the
//     UAPI module by default ("Phusion Passenger" feature flag).
//  2. It does not require the `uapi` CLI to be on PATH for the
//     deploy user (some hosters wrap it behind `cpapi2-wrapper`).
//  3. It avoids carrying a fresh API token into the GHA workflow
//     just to bounce the app — the deploy user already has SSH +
//     filesystem access to the app dir.
//
// The deploy workflow already wraps the value in single quotes
// when handing it to `ssh '<RestartCommand>'`, so embedding an
// extra layer of single quotes here would break the outer shell
// context. The adapter validates appPath via
// [Provider.renderTemplate] (operator-controlled Properties only)
// before this helper is called; any character that survives
// `ValidateDomain` + the template substitution is safe for
// unquoted shell use because the [domainPattern] regex already
// rejects shell metacharacters. Stripping single quotes here is
// defence-in-depth for the unlikely case a future Properties
// override slips one through.
func WorkflowRestartCommand(appPath string) string {
	clean := strings.TrimSpace(appPath)
	clean = strings.ReplaceAll(clean, "'", "")
	return fmt.Sprintf("touch %s/tmp/restart.txt", clean)
}

// WorkflowRsyncExcludes returns the rsync excludes the cPanel
// deploy workflow should honour on top of the wizard's defaults.
// The list mirrors the cPanel Passenger / Node.js Selector
// runtime layout so a `rsync --delete` from a built bundle cannot
// erase per-user runtime state managed by the panel.
//
// The wizard merges these into [WorkflowData.RsyncExcludes]
// alongside the operator's project-specific entries. Returning a
// fresh slice (no shared state) keeps the helper safe for parallel
// callers.
func WorkflowRsyncExcludes() []string {
	return []string{
		"tmp/",          // Passenger restart sentinel + lockfiles.
		".env",          // Per-AUDIT C6: never overwrite operator's .env.
		"node_modules/", // Selector / Application Manager manages this.
		"public_html/",  // Static asset dir for non-Node addon domains.
		".htaccess",     // Per-host hardening; never roll-back via deploy.
		"cgi-bin/",      // Legacy CGI bucket on shared hosting.
	}
}

// PrepareWorkflowData fills the cPanel-flavoured defaults the
// wizard hands to [assets.RenderDeployWorkflow] when the operator
// picked a cpanel preset. Returned [wizard.ErrInvalidPlan] when the
// domain or user fails the package's own validators.
//
// Three cPanel-specific overrides land here:
//
//  1. DeployPath  — `/home/<user>/nodejs/<app_root>/public` (default
//     CloudLinux Node.js Selector layout). When the operator
//     overrode the path templates via Properties, the adapter's
//     [Provider.GetDeployPath] is the source of truth.
//  2. RestartCommand — Passenger touch-restart at the app root.
//  3. RsyncExcludes — merge the cpanel defaults with the operator's
//     extras; deduplication is the caller's responsibility because
//     the wizard already deduplicates user-supplied paths.
//
// The helper does NOT touch `Domain`, `DeployHost`, `DeployUser`,
// `DistDir`, or `BuildCommand` — those are stack-level and the
// wizard fills them upstream.
func (p *Provider) PrepareWorkflowData(domain string, extraExcludes []string) (deployPath, restartCommand string, excludes []string, err error) {
	if vErr := ValidateDomain(domain); vErr != nil {
		return "", "", nil, fmt.Errorf("%w: %w", wizard.ErrInvalidPlan, vErr)
	}
	deployPath = p.GetDeployPath(domain)
	if deployPath == "" {
		return "", "", nil, fmt.Errorf("%w: cpanel deploy path unavailable", wizard.ErrInvalidPlan)
	}
	appPath := p.renderTemplate(p.props.AppRootTemplate, p.resolveAppRoot(domain))
	restartCommand = WorkflowRestartCommand(appPath)

	defaults := WorkflowRsyncExcludes()
	excludes = make([]string, 0, len(defaults)+len(extraExcludes))
	seen := make(map[string]bool, len(defaults)+len(extraExcludes))
	for _, slice := range [][]string{defaults, extraExcludes} {
		for _, e := range slice {
			e = strings.TrimSpace(e)
			if e == "" || seen[e] {
				continue
			}
			seen[e] = true
			excludes = append(excludes, e)
		}
	}
	return deployPath, restartCommand, excludes, nil
}
