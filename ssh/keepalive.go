package ssh

import (
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

const keepaliveRequest = "keepalive@openssh.com"

func keepaliveLoop(done <-chan struct{}, client *cryptossh.Client, interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Re-check the shutdown signal between the ticker firing
			// and the wire send. Pool.Close also tears down the
			// underlying client right after closing `done`, so the
			// SendRequest below will fail fast even if it slips
			// through this guard — but narrowing the window first
			// avoids a stray request arriving at the server when
			// Close raced an in-flight tick.
			select {
			case <-done:
				return
			default:
			}
			if _, _, err := client.SendRequest(keepaliveRequest, true, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
