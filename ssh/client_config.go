package ssh

import (
	"errors"
	"fmt"
	"net"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// dssWireName is the on-the-wire algorithm identifier for DSA host
// keys. Using the literal sidesteps the SA1019 deprecation warning on
// crypto/ssh's KeyAlgoDSA constant while still asserting at the test
// boundary that we never accept this banned algorithm.
const dssWireName = "ssh-dss"

// Options bundles the inputs [BuildClientConfig] needs to assemble a
// hardened *cryptossh.ClientConfig. The pool (Sprint 02 TASK-02.3)
// constructs these from a provider profile + the user's resolved
// AuthMethods; tests construct them directly with stub HostKeyDBs.
type Options struct {
	Target                Target
	Auth                  []cryptossh.AuthMethod
	HostKeyDB             HostKeyDB
	LegacyAlgorithmCompat bool
	Timeout               time.Duration
}

// BuildClientConfig produces a hardened *cryptossh.ClientConfig with
// the host-key algorithm whitelist from docs/SECURITY.md §5.5 and a
// HostKeyCallback that maps [knownhosts.KeyError] outcomes onto the
// distinguishable [ErrHostKeyUnknown] / [ErrHostKeyMismatch]
// sentinels.
//
// HostKeyDB is required — there is no auto-accept path. Returning
// nil-HostKeyDB as an error rather than falling back to
// [cryptossh.InsecureIgnoreHostKey] is the contract that makes the
// "strict block on mismatch" guardrail enforceable end-to-end.
func BuildClientConfig(opts Options) (*cryptossh.ClientConfig, error) {
	if opts.HostKeyDB == nil {
		return nil, ErrHostKeyDBRequired
	}

	return &cryptossh.ClientConfig{
		User:              opts.Target.User,
		Auth:              opts.Auth,
		HostKeyAlgorithms: hostKeyAlgorithms(opts.LegacyAlgorithmCompat),
		HostKeyCallback:   wrapHostKeyCallback(opts.HostKeyDB),
		Timeout:           opts.Timeout,
	}, nil
}

// hostKeyAlgorithms mirrors docs/SECURITY.md §5.5 verbatim:
//
//   - default: ed25519, rsa-sha2-512, rsa-sha2-256, ecdsa-sha2-nistp256
//   - +ssh-rsa (SHA-1) ONLY when LegacyAlgorithmCompat is set via
//     `properties.ssh_algorithms_legacy_compat=true`
//
// ssh-dss is never accepted. The order matters — crypto/ssh advertises
// algorithms in the order we list, so the preferred ed25519 sits first.
func hostKeyAlgorithms(legacy bool) []string {
	algorithms := []string{
		cryptossh.KeyAlgoED25519,
		cryptossh.KeyAlgoRSASHA512,
		cryptossh.KeyAlgoRSASHA256,
		cryptossh.KeyAlgoECDSA256,
	}
	if legacy {
		algorithms = append(algorithms, cryptossh.KeyAlgoRSA)
	}
	return algorithms
}

func wrapHostKeyCallback(db HostKeyDB) cryptossh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key cryptossh.PublicKey) error {
		err := db.Check(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			if len(keyErr.Want) == 0 {
				return fmt.Errorf("%w: host=%s", ErrHostKeyUnknown, hostname)
			}
			return fmt.Errorf("%w: host=%s", ErrHostKeyMismatch, hostname)
		}
		return err
	}
}
