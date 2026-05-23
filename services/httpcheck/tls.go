package httpcheck

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

// TLSOptions configures ProbeTLS.
type TLSOptions struct {
	Dialer    net.Dialer
	TLSConfig *tls.Config
	Timeout   time.Duration
	Now       func() time.Time
}

// TLSResult captures certificate expiry data for the dashboard.
type TLSResult struct {
	Address  string
	NotAfter time.Time
	DaysLeft int
}

// ProbeTLS performs a TLS handshake and returns the leaf certificate
// expiry. The address must be host:port.
func ProbeTLS(ctx context.Context, address string, opts TLSOptions) (TLSResult, error) {
	if _, _, err := net.SplitHostPort(address); err != nil {
		return TLSResult{}, err
	}
	opts = normalizeTLSOptions(opts)
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	conn, err := tls.DialWithDialer(&opts.Dialer, "tcp", address, opts.TLSConfig)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return TLSResult{}, ctxErr
		}
		return TLSResult{}, err
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return TLSResult{}, errNoPeerCertificates
	}
	notAfter := state.PeerCertificates[0].NotAfter
	return TLSResult{
		Address:  address,
		NotAfter: notAfter,
		DaysLeft: int(notAfter.Sub(opts.Now()).Hours() / hoursPerDay),
	}, nil
}

func normalizeTLSOptions(opts TLSOptions) TLSOptions {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultProbeTimeout
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Dialer.Timeout <= 0 {
		opts.Dialer.Timeout = opts.Timeout
	}
	if opts.TLSConfig == nil {
		opts.TLSConfig = &tls.Config{}
	} else {
		opts.TLSConfig = opts.TLSConfig.Clone()
	}
	return opts
}
