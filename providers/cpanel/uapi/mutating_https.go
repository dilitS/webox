package uapi

import (
	"context"
	"net/http"
)

// HTTPSMutator implements [Mutator] over the same transport as the
// read-only [Client]. Construction validates the endpoint and
// credentials exactly like [NewClient]; every method short-circuits
// when [MutationsAllowed] returns false to keep the env-var guard
// authoritative even when the operator misconfigures the adapter.
//
// The mutator does NOT cache responses, does NOT retry past the
// transport's built-in transient-retry policy, and does NOT log the
// args map — the redactor in `internal/log` covers any accidental
// leak, but the transport never formats the args bag into an error
// string.
type HTTPSMutator struct {
	transport *transport
}

// NewHTTPSMutator returns a typed mutator over HTTPS. Same input
// validation as [NewClient]: HTTPS-only scheme, user + token both
// required.
func NewHTTPSMutator(baseURL, user, token string, httpClient *http.Client) (*HTTPSMutator, error) {
	tr, err := newTransport(baseURL, user, token, httpClient)
	if err != nil {
		return nil, err
	}
	return &HTTPSMutator{transport: tr}, nil
}

// AddAddonDomain provisions an addon domain via DomainInfo::add_addon_domain.
func (m *HTTPSMutator) AddAddonDomain(ctx context.Context, args CreateAddonDomainArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleDomainInfo, FunctionDomainInfoAddAddonDomain, argsForAddAddonDomain(args)))
}

// AddSubdomain provisions a subdomain via SubDomain::addsubdomain.
func (m *HTTPSMutator) AddSubdomain(ctx context.Context, args CreateSubdomainArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleSubDomain, FunctionSubdomainAdd, argsForAddSubdomain(args)))
}

// DeleteDomain removes an addon/parked domain via DomainInfo::del_domain.
// Idempotent: panel "not found" maps to [ErrResourceNotFound].
func (m *HTTPSMutator) DeleteDomain(ctx context.Context, domain string) error {
	if err := guardAndValidateValue("domain", domain); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleDomainInfo, FunctionDomainInfoDelDomain, argsForDeleteDomain(domain)))
}

// DeleteSubdomain removes a subdomain via SubDomain::delsubdomain.
func (m *HTTPSMutator) DeleteSubdomain(ctx context.Context, fqSubdomain string) error {
	if err := guardAndValidateValue("subdomain", fqSubdomain); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleSubDomain, FunctionSubdomainDel, map[string]string{"domain": fqSubdomain}))
}

// CreatePassengerApp registers a Passenger / Node.js application
// via PassengerApps::create_application.
func (m *HTTPSMutator) CreatePassengerApp(ctx context.Context, args CreatePassengerAppArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModulePassengerApps, FunctionPassengerAppsCreate, argsForCreatePassengerApp(args)))
}

// EditPassengerApp updates an existing application.
func (m *HTTPSMutator) EditPassengerApp(ctx context.Context, args EditPassengerAppArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModulePassengerApps, FunctionPassengerAppsEdit, argsForEditPassengerApp(args)))
}

// RestartPassengerApp triggers a Passenger graceful restart.
func (m *HTTPSMutator) RestartPassengerApp(ctx context.Context, appPath string) error {
	if err := guardAndValidateValue("path", appPath); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModulePassengerApps, FunctionPassengerAppsRestart, argsForRestartPassengerApp(appPath)))
}

// DeletePassengerApp deregisters the application at path.
func (m *HTTPSMutator) DeletePassengerApp(ctx context.Context, appPath string) error {
	if err := guardAndValidateValue("path", appPath); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModulePassengerApps, FunctionPassengerAppsDelete, argsForDeletePassengerApp(appPath)))
}

// CreateMysqlDatabase provisions a fresh database.
func (m *HTTPSMutator) CreateMysqlDatabase(ctx context.Context, dbName string) error {
	if err := guardAndValidateValue("name", dbName); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleMysql, FunctionMysqlCreateDatabase, argsForCreateMysqlDatabase(dbName)))
}

// DeleteMysqlDatabase drops a database.
func (m *HTTPSMutator) DeleteMysqlDatabase(ctx context.Context, dbName string) error {
	if err := guardAndValidateValue("name", dbName); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleMysql, FunctionMysqlDeleteDatabase, argsForDeleteMysqlDatabase(dbName)))
}

// CreateMysqlUser provisions a MySQL user with the supplied
// password. The password travels as a query param; the transport
// strips Authorization headers from debug renders, and the
// redactor in internal/log/redact.go masks `password=` patterns.
func (m *HTTPSMutator) CreateMysqlUser(ctx context.Context, user, password string) error {
	if err := guardAndValidateValue("user", user); err != nil {
		return err
	}
	if err := mustNonEmpty("password", password); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleMysql, FunctionMysqlCreateUser, argsForCreateMysqlUser(user, password)))
}

// DeleteMysqlUser drops a MySQL user.
func (m *HTTPSMutator) DeleteMysqlUser(ctx context.Context, user string) error {
	if err := guardAndValidateValue("user", user); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleMysql, FunctionMysqlDeleteUser, argsForDeleteMysqlUser(user)))
}

// SetMysqlPrivileges grants the supplied privilege list to the
// (user, database) pair.
func (m *HTTPSMutator) SetMysqlPrivileges(ctx context.Context, args MysqlPrivilegesArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleMysql, FunctionMysqlSetPrivileges, argsForSetMysqlPrivileges(args)))
}

// InstallSSL installs a supplied cert + key (byo-cert path).
func (m *HTTPSMutator) InstallSSL(ctx context.Context, args InstallSSLArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleSSL, FunctionSSLInstallSSL, argsForInstallSSL(args)))
}

// StartAutoSSL triggers cPanel's AutoSSL provisioning for the
// supplied host. cPanel runs the cert request asynchronously;
// callers poll [Reader.ListSSLKeys] to confirm completion.
func (m *HTTPSMutator) StartAutoSSL(ctx context.Context, domain string) error {
	if err := guardAndValidateValue("domain", domain); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleSSL, FunctionSSLStartAutoSSL, argsForStartAutoSSL(domain)))
}

// DeleteSSL revokes the installed cert for host.
func (m *HTTPSMutator) DeleteSSL(ctx context.Context, host string) error {
	if err := guardAndValidateValue("host", host); err != nil {
		return err
	}
	return classifyMutationError(m.exec(ctx, ModuleSSL, FunctionSSLDeleteSSL, argsForDeleteSSL(host)))
}

// exec is the single seam between every typed method and the
// underlying transport. Returning the envelope-less error keeps
// the typed methods focused on argument shape; the envelope's
// `data` payload is intentionally discarded because every Sprint-22
// mutating call cares only about success/failure (the typed
// follow-up is one of the read-only methods).
func (m *HTTPSMutator) exec(ctx context.Context, module Module, function Function, args map[string]string) error {
	_, err := m.transport.callWithArgs(ctx, module, function, args)
	return err
}

// guardAndValidate fuses the env-var guard with the args
// validator so every typed method follows the same fail-closed
// shape: guard first, validate second, transport last.
func guardAndValidate(validate func() error) error {
	if !MutationsAllowed() {
		return ErrMutationsDisabled
	}
	if validate != nil {
		return validate()
	}
	return nil
}

// guardAndValidateValue is the single-string flavour for methods
// that take one positional argument (e.g. DeleteDomain).
func guardAndValidateValue(field, value string) error {
	if !MutationsAllowed() {
		return ErrMutationsDisabled
	}
	if err := mustNonEmpty(field, value); err != nil {
		return err
	}
	return mustNotControl(field, value)
}
