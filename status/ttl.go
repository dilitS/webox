package status

import "time"

const (
	// PrefixHTTP keys store HTTPS reachability / status-code checks.
	PrefixHTTP = "http:"
	// PrefixSSHNode keys store remote node --version checks.
	PrefixSSHNode = "ssh:node:"
	// PrefixSSL keys store TLS certificate expiry checks.
	PrefixSSL = "ssl:"
	// PrefixGitHubLastDeploy keys store GitHub Actions last-run status.
	PrefixGitHubLastDeploy = "gh:lastDeploy:"
	// PrefixGitHubSteps keys store live CI/CD pipeline step snapshots
	// for the Sprint 10 dashboard tile. Format: `gh:steps:<owner>/<repo>:<workflow>`.
	PrefixGitHubSteps = "gh:steps:"
)

const (
	// HTTPStatusTTL follows ADR-0005: HTTP can change during deploys.
	HTTPStatusTTL = 30 * time.Second
	// SSHNodeTTL follows ADR-0005: node version changes rarely.
	SSHNodeTTL = time.Minute
	// SSLCertTTL follows ADR-0005: certificates change infrequently.
	SSLCertTTL = 5 * time.Minute
	// GitHubLastDeployTTL follows ADR-0005: workflow status updates within minutes.
	GitHubLastDeployTTL = time.Minute
	// GitHubStepsTTL caps how often the CI/CD pipeline tile polls
	// GitHub Actions for per-step state. Sprint 10 plan §TASK-10.2
	// fixes this at 10s — `gh` CLI auth quota is 5000/h so even
	// 5 projects × 6/min stay well below the limit.
	GitHubStepsTTL = 10 * time.Second
)

// Event names the operations that invalidate status-cache prefixes.
type Event string

// Event values map domain operations to deterministic cache-prefix
// invalidations.
const (
	EventRestart           Event = "Restart"
	EventDeploy            Event = "Deploy"
	EventRemoveSubdomain   Event = "RemoveSubdomain"
	EventChangeNodeVersion Event = "ChangeNodeVersion"
	EventSetupSSL          Event = "SetupSSL"
	EventRenewSSL          Event = "RenewSSL"
	EventRemoveSSL         Event = "RemoveSSL"
)

// PrefixesForEvent returns the cache prefixes invalidated by event.
func PrefixesForEvent(event Event) []string {
	switch event {
	case EventRestart:
		return []string{PrefixHTTP}
	case EventDeploy:
		return []string{PrefixHTTP, PrefixGitHubLastDeploy, PrefixGitHubSteps}
	case EventRemoveSubdomain:
		return []string{PrefixHTTP}
	case EventChangeNodeVersion:
		return []string{PrefixSSHNode}
	case EventSetupSSL, EventRenewSSL, EventRemoveSSL:
		return []string{PrefixSSL}
	default:
		return nil
	}
}
