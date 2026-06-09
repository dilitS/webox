package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	daapi "github.com/dilitS/webox/providers/directadmin/api"
)

// nativeDirectadminSSHCmdRunner satisfies [daapi.SSHRunner] by
// shelling out to the operator's installed `ssh` client. Auth,
// host-key validation, multiplexing, and `~/.ssh/config` all
// stay on the operator's side; Webox owns zero new auth surface.
//
// Each invocation runs `ssh -p<port> user@host '<command>'` with
// BatchMode=yes, StrictHostKeyChecking=accept-new, ConnectTimeout=10s.
// Identical shape to the cpanel native runner; only the SSHRunner
// type differs (per-package interface).
type nativeDirectadminSSHCmdRunner struct {
	host    string
	user    string
	port    int
	timeout time.Duration
}

// newNativeDirectadminSSHRunner is the factory the production
// directadmin doctor wires to. Tests bypass it via
// [directadminOpts.sshFactory].
func newNativeDirectadminSSHRunner(host, user string, port int, timeout time.Duration) (daapi.SSHRunner, error) {
	if host == "" || user == "" {
		return nil, errDARunnerNeedsHostUser
	}
	return &nativeDirectadminSSHCmdRunner{
		host:    host,
		user:    user,
		port:    port,
		timeout: timeout,
	}, nil
}

// Run satisfies [daapi.SSHRunner]. The command is passed as a
// single argv element to `ssh`, so no local shell expansion
// runs; only the remote shell on the panel parses it.
//
// adapter package owns its own ErrTransportUnavailable sentinel
// (daapi.ErrTransportUnavailable vs uapi.ErrTransportUnavailable),
// and the wrap target must be the same package's typed error so
// the composite layer's errors.Is fall-over check works. A shared
// helper would invert the dependency direction (the helper would
// have to know about both packages' sentinels, or the wrap would
// have to be done by the caller). Revisit when a third provider
// lands (Sprint 24+) — an `internal/sshcmd` package taking a wrap
// func argument is the planned shape.
//
//nolint:dupl // Duplicates nativeSSHCmdRunner.Run by design: each
func (r *nativeDirectadminSSHCmdRunner) Run(ctx context.Context, command string) (stdout, stderr []byte, exitCode int, err error) {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	}
	if r.port > 0 && r.port != defaultDirectadminSSHPort {
		args = append(args, "-p", strconv.Itoa(r.port))
	}
	args = append(args, fmt.Sprintf("%s@%s", r.user, r.host), command)

	timeout := r.timeout
	if timeout == 0 {
		timeout = defaultDirectadminTimeout
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "ssh", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	stdout = outBuf.Bytes()
	stderr = errBuf.Bytes()
	exitCode = -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	if runErr == nil {
		return stdout, stderr, exitCode, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout, stderr, exitCode, nil
	}
	return stdout, stderr, exitCode, fmt.Errorf("%w: %w", daapi.ErrTransportUnavailable, runErr)
}
