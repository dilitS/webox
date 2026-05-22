// Package providers defines the HostingProvider abstraction that lets
// Webox orchestrate different shared-hosting panels through a single
// contract.
//
// The interface, ProviderConfig and sentinel errors are declared in
// provider.go and errors.go; the factory registry lives in registry.go.
// Concrete adapters (smallhost in MVP, cpanel/directadmin/cyberpanel
// in v0.2+) live in subpackages and register themselves via init().
// Business logic must never type-switch on a concrete provider — see
// docs/DESIGN.md §3 and docs/adr/0003-provider-pattern.md.
package providers
