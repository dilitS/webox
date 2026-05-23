package sshmock

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

func TestServer_SmokeExecHello(t *testing.T) {
	t.Parallel()

	server := New(t, WithCommand("echo hello", CommandResult{Stdout: "hello\n"}))
	client := server.Dial(t)
	defer client.Close()

	stdout, stderr, exitCode, err := runCommand(client, "echo hello")
	if err != nil {
		t.Fatalf("runCommand(echo hello) = %v", err)
	}
	if stdout != "hello\n" {
		t.Fatalf("stdout = %q, want hello newline", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestServer_MapsCommandToStdoutStderrAndExitCode(t *testing.T) {
	t.Parallel()

	server := New(t, WithCommand("fail please", CommandResult{
		Stdout:   "partial\n",
		Stderr:   "boom\n",
		ExitCode: 42,
	}))
	client := server.Dial(t)
	defer client.Close()

	stdout, stderr, exitCode, err := runCommand(client, "fail please")
	if err == nil {
		t.Fatal("runCommand should return *ssh.ExitError for non-zero exit")
	}
	if stdout != "partial\n" {
		t.Fatalf("stdout = %q, want partial newline", stdout)
	}
	if stderr != "boom\n" {
		t.Fatalf("stderr = %q, want boom newline", stderr)
	}
	if exitCode != 42 {
		t.Fatalf("exitCode = %d, want 42", exitCode)
	}
}

func TestServer_RejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	server := New(t)
	client := server.Dial(t)
	defer client.Close()

	stdout, stderr, exitCode, err := runCommand(client, "missing")
	if err == nil {
		t.Fatal("runCommand(missing) returned nil error, want command failure")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Fatalf("stderr = %q, want unknown-command diagnostic", stderr)
	}
	if exitCode != 127 {
		t.Fatalf("exitCode = %d, want 127", exitCode)
	}
}

func TestServer_UsesEphemeralPublicKeyAuth(t *testing.T) {
	t.Parallel()

	server := New(t)

	cfg := server.ClientConfig()
	cfg.Auth = []cryptossh.AuthMethod{cryptossh.Password("wrong")}
	client, err := cryptossh.Dial("tcp", server.Addr(), cfg)
	if err == nil {
		_ = client.Close()
		t.Fatal("Dial with password auth succeeded, want public-key-only rejection")
	}

	client = server.Dial(t)
	if client == nil {
		t.Fatal("Dial with server-generated key returned nil client")
	}
	_ = client.Close()
}

func TestServer_DisconnectFailure(t *testing.T) {
	t.Parallel()

	server := New(t, WithCommand("disconnect", CommandResult{Disconnect: true}))
	client := server.Dial(t)
	defer client.Close()

	_, _, _, err := runCommand(client, "disconnect")
	if err == nil {
		t.Fatal("runCommand(disconnect) returned nil error, want transport/channel failure")
	}
}

func TestServer_DelayCanDriveClientTimeout(t *testing.T) {
	t.Parallel()

	server := New(t, WithCommand("slow", CommandResult{
		Stdout: "late\n",
		Delay:  200 * time.Millisecond,
	}))
	client := server.Dial(t)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		session, err := client.NewSession()
		if err != nil {
			errCh <- err
			return
		}
		defer session.Close()
		errCh <- session.Run("slow")
	}()

	select {
	case <-ctx.Done():
		_ = client.Close()
	case err := <-errCh:
		t.Fatalf("slow command returned before timeout with err=%v", err)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("slow command completed nil after client close, want cancellation/transport error")
		}
	case <-time.After(time.Second):
		t.Fatal("slow command did not unblock after client close")
	}
}

func runCommand(client *cryptossh.Client, command string) (stdout string, stderr string, exitCode int, err error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", -1, err
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	err = session.Run(command)
	if err != nil {
		var exitErr *cryptossh.ExitError
		if errors.As(err, &exitErr) {
			return stdoutBuf.String(), stderrBuf.String(), exitErr.ExitStatus(), err
		}
		if errors.Is(err, io.EOF) {
			return stdoutBuf.String(), stderrBuf.String(), -1, err
		}
		return stdoutBuf.String(), stderrBuf.String(), -1, err
	}
	return stdoutBuf.String(), stderrBuf.String(), 0, nil
}
