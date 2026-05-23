package ssh

import (
	"context"
	"net"
	"strconv"

	cryptossh "golang.org/x/crypto/ssh"
)

// NetDialer is the production Dialer backed by net.Dialer and
// golang.org/x/crypto/ssh. It exists as a value type so tests can wrap
// it with counters while still exercising the real SSH transport.
type NetDialer struct {
	Net net.Dialer
}

// Dial establishes a TCP connection to target and upgrades it to an SSH
// client using config. The context controls the TCP dial; subsequent SSH
// handshake deadlines are governed by config.Timeout.
func (d NetDialer) Dial(ctx context.Context, target Target, config *cryptossh.ClientConfig) (*cryptossh.Client, error) {
	conn, err := d.Net.DialContext(ctx, "tcp", target.Addr())
	if err != nil {
		return nil, err
	}

	clientConn, chans, reqs, err := cryptossh.NewClientConn(conn, target.Addr(), config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return cryptossh.NewClient(clientConn, chans, reqs), nil
}

func splitHostPort(addr string) (host string, port int, err error) {
	host, portRaw, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err = strconv.Atoi(portRaw)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}
