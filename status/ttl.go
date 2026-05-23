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
)

// Event names the operations that invalidate status-cache prefixes.
type Event string

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
		return []string{PrefixHTTP, PrefixGitHubLastDeploy}
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
