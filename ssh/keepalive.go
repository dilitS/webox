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
			if _, _, err := client.SendRequest(keepaliveRequest, true, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
