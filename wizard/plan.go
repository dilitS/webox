package wizard

import (
	"fmt"

	"github.com/dilitS/webox/providers"
	"github.com/dilitS/webox/providers/smallhost"
)

// Supported stacks for the MVP wizard. New stacks land in v0.2+; the
// list is kept short on purpose so the wizard stays scannable on
// `80x24` terminals. See PRD §6 F21.
const (
	// StackStatic is a no-runtime stack (built HTML/CSS/JS only).
	StackStatic = "static"

	// StackViteReact is Vite + React, served as static after build.
	StackViteReact = "vite-react"

	// StackNodeExpress is a long-running Node.js HTTP server (Express,
	// Fastify, etc.). This is the only MVP stack that REQUIRES a
	// node runtime on the panel and the only one where the wizard
	// asks about a database by default.
	StackNodeExpress = "node-express"
)

// SupportedStacks is the validation set for [ProvisionPlan.Stack].
// Tests assert that any new stack added here also gets a default DB
// requirement and a node-version default below.
var SupportedStacks = []string{StackStatic, StackViteReact, StackNodeExpress}

// SupportedDBKinds is the validation set for [ProvisionPlan.DBKind].
// Mirrors the [providers.DatabaseKind] constants — kept local so the
// wizard does not have to switch on every value the providers package
// might add in v0.2+.
var SupportedDBKinds = []string{providers.DatabaseMySQL, providers.DatabasePostgres}

// defaultNodeVersionForStack maps the stack to the node version the
// wizard pre-fills. Users can override before validation; "static" and
// "vite-react" stay empty because [providers.HostingProvider.CreateSubdomain]
// is only called when a node version is non-empty (the static wizard
// path uses a different provider call in v0.2+; MVP routes everything
// through node).
var defaultNodeVersionForStack = map[string]string{
	StackStatic:      "22",
	StackViteReact:   "22",
	StackNodeExpress: "22",
}

// DefaultNodeVersion returns the wizard's pre-filled node version for
// a given stack. Empty string is treated by the wizard as "do not
// pre-fill"; the user MUST type one before execution.
func DefaultNodeVersion(stack string) string {
	if v, ok := defaultNodeVersionForStack[stack]; ok {
		return v
	}
	return ""
}

// IsValidStack reports whether stack is in [SupportedStacks]. Pure
// function: no allocations, no I/O.
func IsValidStack(stack string) bool {
	for _, s := range SupportedStacks {
		if s == stack {
			return true
		}
	}
	return false
}

// IsValidDBKind reports whether kind is in [SupportedDBKinds]. The
// wizard uses this to reject free-form text in the DB step before
// any provider call is built.
func IsValidDBKind(kind string) bool {
	for _, k := range SupportedDBKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// IsDBRequiredForStack reports whether the wizard should ask about a
// database by default for stack. node-express needs persistence by
// convention; static / vite-react skip the step unless the user opts
// in explicitly (PRD §6 F3 "smart skip dla statycznych").
func IsDBRequiredForStack(stack string) bool {
	return stack == StackNodeExpress
}

// ValidatePlan runs the cross-field invariants before any provider
// call. Returns a wrapped [ErrInvalidPlan]. Tests cover every branch
// because this is the last line of defense before the LIFO stack
// starts pushing real provider state.
func ValidatePlan(plan ProvisionPlan) error {
	if plan.ProfileAlias == "" {
		return fmt.Errorf("%w: profile_alias is required", ErrInvalidPlan)
	}
	if !IsValidStack(plan.Stack) {
		return fmt.Errorf("%w: stack %q is not in %v", ErrInvalidPlan, plan.Stack, SupportedStacks)
	}
	if err := smallhost.ValidateDomain(plan.Domain); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPlan, err)
	}
	if plan.NodeVersion == "" {
		return fmt.Errorf("%w: node_version is required", ErrInvalidPlan)
	}
	if err := smallhost.ValidateNodeVersion(plan.NodeVersion); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPlan, err)
	}
	if plan.DBKind == "" && plan.DBName == "" {
		return nil
	}
	if plan.DBKind == "" || plan.DBName == "" {
		return fmt.Errorf("%w: db_kind and db_name must be set together", ErrInvalidPlan)
	}
	if !IsValidDBKind(plan.DBKind) {
		return fmt.Errorf("%w: db_kind %q is not in %v", ErrInvalidPlan, plan.DBKind, SupportedDBKinds)
	}
	if err := smallhost.ValidateDBName(plan.DBName); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPlan, err)
	}
	return nil
}
