package uapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SSHMutator implements [Mutator] over the SSH `uapi --user=...`
// transport. The shape mirrors [SSHFallback] (read-only): same
// runner seam, same envelope parsing, same shell-quoting. Every
// method shares the env-var guard with [HTTPSMutator] so a single
// `WEBOX_CPANEL_MUTATIONS=1` flip controls both transports.
type SSHMutator struct {
	runner SSHRunner
	user   string
}

// NewSSHMutator validates user (required for the `uapi --user`
// flag) and the runner (production wiring uses [SSHPoolRunner];
// tests use an in-memory fake). The constructor does NOT check
// the env-var guard — that runs per method so live operator
// configuration changes (toggling the env on a long-running
// process) surface deterministically without restarting the
// adapter.
func NewSSHMutator(runner SSHRunner, user string) (*SSHMutator, error) {
	if user == "" {
		return nil, ErrMissingCredentials
	}
	if runner == nil {
		return nil, ErrSSHRunnerRequired
	}
	return &SSHMutator{runner: runner, user: user}, nil
}

// AddAddonDomain executes `uapi DomainInfo add_addon_domain` over SSH.
func (s *SSHMutator) AddAddonDomain(ctx context.Context, args CreateAddonDomainArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleDomainInfo, FunctionDomainInfoAddAddonDomain, argsForAddAddonDomain(args)))
}

// AddSubdomain executes `uapi SubDomain addsubdomain` over SSH.
func (s *SSHMutator) AddSubdomain(ctx context.Context, args CreateSubdomainArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleSubDomain, FunctionSubdomainAdd, argsForAddSubdomain(args)))
}

// DeleteDomain executes `uapi DomainInfo del_domain` over SSH.
func (s *SSHMutator) DeleteDomain(ctx context.Context, domain string) error {
	if err := guardAndValidateValue("domain", domain); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleDomainInfo, FunctionDomainInfoDelDomain, argsForDeleteDomain(domain)))
}

// DeleteSubdomain executes `uapi SubDomain delsubdomain` over SSH.
func (s *SSHMutator) DeleteSubdomain(ctx context.Context, fqSubdomain string) error {
	if err := guardAndValidateValue("subdomain", fqSubdomain); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleSubDomain, FunctionSubdomainDel, map[string]string{"domain": fqSubdomain}))
}

// CreatePassengerApp executes `uapi PassengerApps create_application` over SSH.
func (s *SSHMutator) CreatePassengerApp(ctx context.Context, args CreatePassengerAppArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModulePassengerApps, FunctionPassengerAppsCreate, argsForCreatePassengerApp(args)))
}

// EditPassengerApp executes `uapi PassengerApps edit_application` over SSH.
func (s *SSHMutator) EditPassengerApp(ctx context.Context, args EditPassengerAppArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModulePassengerApps, FunctionPassengerAppsEdit, argsForEditPassengerApp(args)))
}

// RestartPassengerApp executes `uapi PassengerApps restart_application` over SSH.
func (s *SSHMutator) RestartPassengerApp(ctx context.Context, appPath string) error {
	if err := guardAndValidateValue("path", appPath); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModulePassengerApps, FunctionPassengerAppsRestart, argsForRestartPassengerApp(appPath)))
}

// DeletePassengerApp executes `uapi PassengerApps delete_application` over SSH.
func (s *SSHMutator) DeletePassengerApp(ctx context.Context, appPath string) error {
	if err := guardAndValidateValue("path", appPath); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModulePassengerApps, FunctionPassengerAppsDelete, argsForDeletePassengerApp(appPath)))
}

// CreateMysqlDatabase executes `uapi Mysql create_database` over SSH.
func (s *SSHMutator) CreateMysqlDatabase(ctx context.Context, dbName string) error {
	if err := guardAndValidateValue("name", dbName); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleMysql, FunctionMysqlCreateDatabase, argsForCreateMysqlDatabase(dbName)))
}

// DeleteMysqlDatabase executes `uapi Mysql delete_database` over SSH.
func (s *SSHMutator) DeleteMysqlDatabase(ctx context.Context, dbName string) error {
	if err := guardAndValidateValue("name", dbName); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleMysql, FunctionMysqlDeleteDatabase, argsForDeleteMysqlDatabase(dbName)))
}

// CreateMysqlUser executes `uapi Mysql create_user` over SSH.
func (s *SSHMutator) CreateMysqlUser(ctx context.Context, user, password string) error {
	if err := guardAndValidateValue("user", user); err != nil {
		return err
	}
	if err := mustNonEmpty("password", password); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleMysql, FunctionMysqlCreateUser, argsForCreateMysqlUser(user, password)))
}

// DeleteMysqlUser executes `uapi Mysql delete_user` over SSH.
func (s *SSHMutator) DeleteMysqlUser(ctx context.Context, user string) error {
	if err := guardAndValidateValue("user", user); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleMysql, FunctionMysqlDeleteUser, argsForDeleteMysqlUser(user)))
}

// SetMysqlPrivileges executes `uapi Mysql set_privileges_on_database` over SSH.
func (s *SSHMutator) SetMysqlPrivileges(ctx context.Context, args MysqlPrivilegesArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleMysql, FunctionMysqlSetPrivileges, argsForSetMysqlPrivileges(args)))
}

// InstallSSL executes `uapi SSL install_ssl` over SSH.
func (s *SSHMutator) InstallSSL(ctx context.Context, args InstallSSLArgs) error {
	if err := guardAndValidate(args.Validate); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleSSL, FunctionSSLInstallSSL, argsForInstallSSL(args)))
}

// StartAutoSSL executes `uapi SSL start_autossl_check` over SSH.
func (s *SSHMutator) StartAutoSSL(ctx context.Context, domain string) error {
	if err := guardAndValidateValue("domain", domain); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleSSL, FunctionSSLStartAutoSSL, argsForStartAutoSSL(domain)))
}

// DeleteSSL executes `uapi SSL delete_ssl` over SSH.
func (s *SSHMutator) DeleteSSL(ctx context.Context, host string) error {
	if err := guardAndValidateValue("host", host); err != nil {
		return err
	}
	return classifyMutationError(s.call(ctx, ModuleSSL, FunctionSSLDeleteSSL, argsForDeleteSSL(host)))
}

// call renders the typed args into the `uapi --user=... <Module>
// <function> key=value ...` invocation, runs it via the runner,
// and parses the JSON envelope. Every value is shell-quoted via
// [shellQuote] before substitution — even for typed Sprint-22
// inputs that the args validator already shape-checks. Defence
// in depth.
func (s *SSHMutator) call(ctx context.Context, module Module, function Function, args map[string]string) error {
	cmd := buildUAPICommandWithArgs(s.user, module, function, args)
	stdout, stderr, code, err := s.runner.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("uapi: ssh run: %w", err)
	}
	if code != 0 {
		if looksLikeModuleDenied(stderr) || looksLikeModuleDenied(stdout) {
			return ErrModuleFunctionDenied
		}
		return fmt.Errorf("%w: exit=%d stderr=%q", ErrAPIResultFailure, code, truncate(stderr))
	}
	env := &envelope{}
	if dErr := json.Unmarshal(stdout, env); dErr != nil {
		return fmt.Errorf("%w: %w", ErrMalformedResponse, dErr)
	}
	if env.Result.Status != 1 {
		if isModuleDenied(env.Result.Errors) {
			return ErrModuleFunctionDenied
		}
		return fmt.Errorf("%w: status=%d errors=%v", ErrAPIResultFailure, env.Result.Status, env.Result.Errors)
	}
	return nil
}

// buildUAPICommandWithArgs renders the SSH command for a mutating
// call. Arg ordering follows alphabetic sort so the command is
// stable across runs — important for test snapshots and audit
// logs. Every key + value is single-quote-escaped via shellQuote.
func buildUAPICommandWithArgs(user string, module Module, function Function, args map[string]string) string {
	base := fmt.Sprintf("uapi --user=%s --output=jsonpretty %s %s",
		shellQuote(user), shellQuote(string(module)), shellQuote(string(function)))
	if len(args) == 0 {
		return base
	}
	parts := []string{base}
	for _, k := range sortedKeys(args) {
		parts = append(parts, shellQuote(k)+"="+shellQuote(args[k]))
	}
	return strings.Join(parts, " ")
}
