package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/dilitS/webox/providers/cpanel/uapi"
)

// _ keeps errors imported for the typed sentinel checks below; the
// runner itself uses errors.As/Is from the standard library.
var _ = errors.New

// nativeSSHCmdRunner satisfies [uapi.SSHRunner] by shelling out to
// the operator's installed `ssh` client, exactly like
// [sshExecRunner] does for `webox doctor preset --probe`. Auth,
// host-key validation, multiplexing, and the user's `~/.ssh/config`
// all stay on the operator's side; Webox owns zero new auth surface.
//
// Each invocation runs `ssh -p<port> user@host '<command>'` with
// BatchMode=yes (no password prompts), StrictHostKeyChecking=accept-new
// (pin first-seen keys without dropping into an interactive prompt),
// and ConnectTimeout=10s.
type nativeSSHCmdRunner struct {
	host    string
	user    string
	port    int
	timeout time.Duration
}

// newNativeSSHRunner is the factory the production cpanel doctor
// wires to. Tests bypass it via [cpanelOpts.sshFactory].
func newNativeSSHRunner(host, user string, port int, timeout time.Duration) (uapi.SSHRunner, error) {
	if host == "" || user == "" {
		return nil, errCpanelRunnerNeedsHostUser
	}
	return &nativeSSHCmdRunner{
		host:    host,
		user:    user,
		port:    port,
		timeout: timeout,
	}, nil
}

// Run satisfies [uapi.SSHRunner]. The command is passed as a
// single argv element to `ssh`, so no local shell expansion runs;
// only the remote shell on the panel parses it. This is the same
// model `webox doctor preset --probe` uses.
//
// See directadmin_runner.go for the design rationale (per-provider
// typed sentinel errors prevent a clean shared helper).
//
//nolint:dupl // Duplicates nativeDirectadminSSHCmdRunner.Run.
func (r *nativeSSHCmdRunner) Run(ctx context.Context, command string) (stdout, stderr []byte, exitCode int, err error) {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	}
	if r.port > 0 && r.port != defaultCpanelSSHPort {
		args = append(args, "-p", strconv.Itoa(r.port))
	}
	args = append(args, fmt.Sprintf("%s@%s", r.user, r.host), command)

	timeout := r.timeout
	if timeout == 0 {
		timeout = defaultCpanelTimeout
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
	return stdout, stderr, exitCode, fmt.Errorf("%w: %w", uapi.ErrTransportUnavailable, runErr)
}
