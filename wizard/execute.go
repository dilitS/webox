package wizard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dilitS/webox/providers"
)

// ExecutionFailedError wraps the per-step error so the wizard's
// failure UI can render the structured report and still surface the
// underlying provider sentinel via errors.Is.
//
// Returned by [Execute] when a single step fails — callers compare
// via errors.Is against the provider sentinels (ErrSubdomainExists,
// ErrDNSNotResolving, etc.) and read the embedded Report to drive
// the remediation menu.
type ExecutionFailedError struct {
	FailedStep string
	Report     *ProvisionReport
	Err        error
}

// Error renders the wrapped error so logging stays consistent.
func (e *ExecutionFailedError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("wizard: %s failed", e.FailedStep)
	}
	return fmt.Sprintf("wizard: %s failed: %v", e.FailedStep, e.Err)
}

// Unwrap lets errors.Is reach the provider sentinel through the
// wrapper.
func (e *ExecutionFailedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Preflight verifies the provider is reachable and the CLI is
// installed before any state-changing call. Failures are propagated
// verbatim — the wizard's remediation menu uses them to suggest
// `webox doctor`. Pure pass-through wrapper; kept here so the wizard
// only depends on [Execute] / [Preflight] from this package.
func Preflight(ctx context.Context, provider providers.HostingProvider) (*providers.ProviderStatus, error) {
	if provider == nil {
		return nil, fmt.Errorf("%w: provider is nil", ErrInvalidPlan)
	}
	status, err := provider.CheckStatus(ctx)
	if err != nil {
		return status, fmt.Errorf("wizard: preflight: %w", err)
	}
	if status == nil {
		return nil, ErrPreflightNilStatus
	}
	if !status.SSHConnected {
		return status, ErrPreflightSSHDisconnected
	}
	if !status.CLIInstalled {
		return status, fmt.Errorf("wizard: preflight: %w", providers.ErrCLINotFound)
	}
	return status, nil
}

// CheckSubdomainAvailable lists the panel subdomains and reports
// whether plan.Domain is already provisioned. Returns
// [providers.ErrSubdomainExists] when the domain collides so the
// wizard surfaces it as a recoverable validation error rather than
// running CreateSubdomain to discover the same fact.
func CheckSubdomainAvailable(ctx context.Context, provider providers.HostingProvider, domain string) error {
	if provider == nil {
		return fmt.Errorf("%w: provider is nil", ErrInvalidPlan)
	}
	if domain == "" {
		return fmt.Errorf("%w: domain is required", ErrInvalidPlan)
	}
	subs, err := provider.ListSubdomains(ctx)
	if err != nil {
		return fmt.Errorf("wizard: list subdomains: %w", err)
	}
	for _, sub := range subs {
		if sub.Domain == domain {
			return providers.ErrSubdomainExists
		}
	}
	return nil
}

// Execute runs the provider-side provisioning for plan, pushing the
// matching cleanup onto stack after every successful step. Failure
// stops execution immediately and returns an [ExecutionFailedError]
// — the wizard's failure UI prompts the operator to choose
// rollback-or-keep, then calls [Stack.Rollback] when rollback was
// chosen.
//
// Execute does NOT clear the snapshot on success; the wizard owns
// that lifecycle because successful provisioning still wants the
// snapshot around if the GitHub / deploy steps (Sprint 06) need to
// trigger a global rollback. The caller invokes [RemovePending]
// (or supplies an empty Steps slice) when it is sure no further
// rollback can fire.
func Execute(ctx context.Context, provider providers.HostingProvider, plan ProvisionPlan, stack *Stack) (*ProvisionReport, error) {
	if provider == nil {
		return nil, fmt.Errorf("%w: provider is nil", ErrInvalidPlan)
	}
	if stack == nil {
		return nil, fmt.Errorf("%w: stack is nil", ErrInvalidPlan)
	}
	validators, err := providers.PlanValidatorsFor(provider.Name())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPlan, err)
	}
	if err := ValidatePlan(plan, validators); err != nil {
		return nil, err
	}

	report := &ProvisionReport{}

	if err := provider.CreateSubdomain(ctx, plan.Domain, plan.NodeVersion); err != nil {
		report.Subdomain = ProvisionStatus{Step: "subdomain", Err: err}
		return report, &ExecutionFailedError{FailedStep: "subdomain", Report: report, Err: err}
	}
	report.Subdomain = ProvisionStatus{Step: "subdomain", OK: true}
	if err := stack.Push(ctx, CleanupStep{
		Name:      fmt.Sprintf("Remove subdomain %s", plan.Domain),
		Kind:      ResourceSubdomain,
		Params:    map[string]string{"domain": plan.Domain},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return report, &ExecutionFailedError{FailedStep: "subdomain-cleanup", Report: report, Err: err}
	}

	if err := provider.SetupSSL(ctx, plan.Domain); err != nil {
		report.SSL = ProvisionStatus{Step: "ssl", Err: err}
		return report, &ExecutionFailedError{FailedStep: "ssl", Report: report, Err: err}
	}
	report.SSL = ProvisionStatus{Step: "ssl", OK: true}
	if err := stack.Push(ctx, CleanupStep{
		Name:      fmt.Sprintf("Remove SSL %s", plan.Domain),
		Kind:      ResourceSSL,
		Params:    map[string]string{"domain": plan.Domain},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return report, &ExecutionFailedError{FailedStep: "ssl-cleanup", Report: report, Err: err}
	}

	if plan.DBKind == "" || plan.DBName == "" {
		return report, nil
	}

	user, password, err := provider.CreateDatabase(ctx, plan.DBKind, plan.DBName)
	if err != nil {
		report.Database = ProvisionStatus{Step: "database", Err: err}
		return report, &ExecutionFailedError{FailedStep: "database", Report: report, Err: err}
	}
	report.Database = ProvisionStatus{Step: "database", OK: true}
	report.Credentials = &DatabaseCredentials{Username: user, Password: password}
	if err := stack.Push(ctx, CleanupStep{
		Name:      fmt.Sprintf("Remove database %s (%s)", plan.DBName, plan.DBKind),
		Kind:      ResourceDatabase,
		Params:    map[string]string{"dbKind": plan.DBKind, "dbName": plan.DBName},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return report, &ExecutionFailedError{FailedStep: "database-cleanup", Report: report, Err: err}
	}

	return report, nil
}

// IsRecoverable reports whether err is one of the provider sentinels
// the failure UI should suggest the user fix rather than rollback —
// e.g. ErrSubdomainExists (operator can rename), ErrDNSNotResolving
// (operator can wait), ErrRateLimitLetsEncrypt (operator can wait).
//
// Returns false for nil, context cancellation, and unknown errors so
// the default path remains "stop and ask".
func IsRecoverable(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, providers.ErrSubdomainExists),
		errors.Is(err, providers.ErrDNSNotResolving),
		errors.Is(err, providers.ErrRateLimitLetsEncrypt),
		errors.Is(err, providers.ErrDBNameTaken),
		errors.Is(err, providers.ErrNodeVersionUnsupported):
		return true
	}
	return false
}
