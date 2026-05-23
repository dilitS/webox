package sshmock

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

// CommandResult describes the deterministic outcome returned when a
// client executes a command through a session channel.
type CommandResult struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	Delay      time.Duration
	Disconnect bool
}

// Option configures a test server.
type Option func(*Server)

// WithCommand registers the result returned for command.
func WithCommand(command string, result CommandResult) Option {
	return func(s *Server) {
		s.commands[command] = result
	}
}

// Server is an in-process SSH server bound to localhost on a random
// port. Tests should construct it with New so cleanup is registered on
// the parent testing.TB.
type Server struct {
	listener net.Listener

	hostSigner   cryptossh.Signer
	clientSigner cryptossh.Signer
	clientPub    cryptossh.PublicKey

	mu       sync.RWMutex
	commands map[string]CommandResult
	closed   chan struct{}
}

// New starts an SSH mock server. It fails the test immediately if the
// listener or ephemeral key generation fails.
func New(t testing.TB, opts ...Option) *Server {
	t.Helper()

	hostSigner := mustGenerateSigner(t)
	clientSigner := mustGenerateSigner(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen sshmock: %v", err)
	}

	server := &Server{
		listener:     listener,
		hostSigner:   hostSigner,
		clientSigner: clientSigner,
		clientPub:    clientSigner.PublicKey(),
		commands:     make(map[string]CommandResult),
		closed:       make(chan struct{}),
	}
	for _, opt := range opts {
		opt(server)
	}

	go server.serve()
	t.Cleanup(server.Close)
	return server
}

// Addr returns the listener address in host:port form.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// ClientConfig returns a public-key-auth config that can connect to the
// server. The mock intentionally uses InsecureIgnoreHostKey because
// TASK-02.2 tests server behavior; production host-key verification is
// covered in ssh/client_config_test.go and later pool tests can inject
// their own HostKeyCallback.
func (s *Server) ClientConfig() *cryptossh.ClientConfig {
	return &cryptossh.ClientConfig{
		User: "webox-test",
		Auth: []cryptossh.AuthMethod{
			cryptossh.PublicKeys(s.clientSigner),
		},
		HostKeyCallback: cryptossh.InsecureIgnoreHostKey(),
		Timeout:         2 * time.Second,
	}
}

// Dial connects to the mock server using [Server.ClientConfig].
func (s *Server) Dial(t testing.TB) *cryptossh.Client {
	t.Helper()

	client, err := cryptossh.Dial("tcp", s.Addr(), s.ClientConfig())
	if err != nil {
		t.Fatalf("dial sshmock %s: %v", s.Addr(), err)
	}
	return client
}

// Close stops accepting new connections. In-flight connections are
// closed when their client side closes or when a disconnect failure is
// injected.
func (s *Server) Close() {
	select {
	case <-s.closed:
		return
	default:
		close(s.closed)
		_ = s.listener.Close()
	}
}

func (s *Server) serve() {
	config := &cryptossh.ServerConfig{
		PublicKeyCallback: func(_ cryptossh.ConnMetadata, key cryptossh.PublicKey) (*cryptossh.Permissions, error) {
			if bytes.Equal(key.Marshal(), s.clientPub.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("sshmock: public key rejected")
		},
		ServerVersion: "SSH-2.0-webox-sshmock",
	}
	config.AddHostKey(s.hostSigner)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closed:
				return
			default:
				continue
			}
		}
		go s.handleConn(conn, config)
	}
}

func (s *Server) handleConn(conn net.Conn, config *cryptossh.ServerConfig) {
	serverConn, channels, requests, err := cryptossh.NewServerConn(conn, config)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer serverConn.Close()

	go cryptossh.DiscardRequests(requests)
	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(cryptossh.UnknownChannelType, "sshmock: only session channels are supported")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(serverConn, channel, requests)
	}
}

func (s *Server) handleSession(serverConn *cryptossh.ServerConn, channel cryptossh.Channel, requests <-chan *cryptossh.Request) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "exec":
			command, err := parseExecCommand(req.Payload)
			if err != nil {
				reply(req, false)
				return
			}
			reply(req, true)
			s.runCommand(serverConn, channel, command)
			return
		default:
			reply(req, false)
		}
	}
}

func (s *Server) runCommand(serverConn *cryptossh.ServerConn, channel cryptossh.Channel, command string) {
	result, ok := s.command(command)
	if !ok {
		result = CommandResult{
			Stderr:   fmt.Sprintf("sshmock: unknown command %q\n", command),
			ExitCode: 127,
		}
	}

	if result.Disconnect {
		_ = serverConn.Close()
		return
	}
	if result.Delay > 0 {
		time.Sleep(result.Delay)
	}
	if result.Stdout != "" {
		_, _ = io.WriteString(channel, result.Stdout)
	}
	if result.Stderr != "" {
		_, _ = io.WriteString(channel.Stderr(), result.Stderr)
	}
	status := result.ExitCode
	if status < 0 {
		status = 255
	}
	_, _ = channel.SendRequest("exit-status", false, cryptossh.Marshal(struct {
		Status uint32
	}{Status: uint32(status)}))
}

func (s *Server) command(command string) (CommandResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result, ok := s.commands[command]
	return result, ok
}

func parseExecCommand(payload []byte) (string, error) {
	var exec struct {
		Command string
	}
	if err := cryptossh.Unmarshal(payload, &exec); err != nil {
		return "", fmt.Errorf("sshmock: parse exec payload: %w", err)
	}
	return exec.Command, nil
}

func reply(req *cryptossh.Request, ok bool) {
	if req.WantReply {
		_ = req.Reply(ok, nil)
	}
}

func mustGenerateSigner(t testing.TB) cryptossh.Signer {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	signer, err := cryptossh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("new ssh signer: %v", err)
	}
	return signer
}
