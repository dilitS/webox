package ssh

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

const (
	defaultMaxPerHost     = 3
	defaultIdleTimeout    = 60 * time.Second
	defaultAcquireTimeout = 5 * time.Second
	defaultKeepalive      = 15 * time.Second
	cleanupDivisor        = 2
)

var errPoolClosed = errors.New("ssh: pool closed")

// ConfigFunc builds the SSH client config for target.
type ConfigFunc func(Target) (*cryptossh.ClientConfig, error)

// PoolOptions configures a connection pool.
type PoolOptions struct {
	MaxPerHost        int
	IdleTimeout       time.Duration
	AcquireTimeout    time.Duration
	CleanupInterval   time.Duration
	KeepaliveInterval time.Duration
	Dialer            Dialer
	Config            ConfigFunc
	Clock             Clock
}

// Pool reuses SSH clients per target key and enforces a per-host
// concurrency cap. It is safe for concurrent use.
type Pool struct {
	opts PoolOptions

	mu     sync.Mutex
	hosts  map[string]*hostPool
	closed bool
	done   chan struct{}
}

type hostPool struct {
	idle   []*pooledClient
	active map[*cryptossh.Client]struct{}
	total  int
	notify chan struct{}
}

type pooledClient struct {
	client   *cryptossh.Client
	lastUsed time.Time
}

// NewPool creates a pool and starts its idle cleanup loop.
func NewPool(opts PoolOptions) *Pool {
	opts = normalizePoolOptions(opts)
	pool := &Pool{
		opts:  opts,
		hosts: make(map[string]*hostPool),
		done:  make(chan struct{}),
	}
	go pool.cleanupLoop()
	return pool
}

// Acquire returns a reusable SSH client for target, dialing a new one if
// the per-host limit allows. If every slot is busy until the context or
// acquire timeout expires, it returns ErrPoolBusy wrapped with the
// context error.
func (p *Pool) Acquire(ctx context.Context, target Target) (*cryptossh.Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, ok := ctx.Deadline(); !ok && p.opts.AcquireTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.opts.AcquireTimeout)
		defer cancel()
	}

	for {
		client, wait, dial, err := p.reserveOrWait(target)
		if err != nil {
			return nil, err
		}
		if client != nil {
			return client, nil
		}
		if dial {
			return p.dialReserved(ctx, target)
		}

		select {
		case <-wait:
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %w", ErrPoolBusy, ctx.Err())
		case <-p.done:
			return nil, errPoolClosed
		}
	}
}

// Release returns client to the idle pool. Releasing nil or releasing
// the same client twice is a no-op.
func (p *Pool) Release(target Target, client *cryptossh.Client) {
	if client == nil {
		return
	}

	var closeClient bool
	key := target.Key()
	now := p.opts.Clock.Now()

	p.mu.Lock()
	host := p.hosts[key]
	if host == nil {
		p.mu.Unlock()
		_ = client.Close()
		return
	}
	if _, ok := host.active[client]; !ok {
		p.mu.Unlock()
		return
	}
	delete(host.active, client)
	if p.closed {
		host.total--
		closeClient = true
	} else {
		host.idle = append(host.idle, &pooledClient{client: client, lastUsed: now})
	}
	notify := host.notify
	host.notify = make(chan struct{})
	p.mu.Unlock()

	if closeClient {
		_ = client.Close()
	}
	close(notify)
}

// IdleCount returns the number of currently idle clients for target.
// It is exported for white-box tests and diagnostics only.
func (p *Pool) IdleCount(target Target) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	host := p.hosts[target.Key()]
	if host == nil {
		return 0
	}
	return len(host.idle)
}

// Close closes all idle and active clients and stops the cleanup loop.
func (p *Pool) Close() {
	var clients []*cryptossh.Client

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.done)
	for _, host := range p.hosts {
		for _, idle := range host.idle {
			clients = append(clients, idle.client)
		}
		for client := range host.active {
			clients = append(clients, client)
		}
		host.idle = nil
		host.active = make(map[*cryptossh.Client]struct{})
		host.total = 0
		notify := host.notify
		host.notify = make(chan struct{})
		close(notify)
	}
	p.mu.Unlock()

	for _, client := range clients {
		_ = client.Close()
	}
}

func (p *Pool) reserveOrWait(target Target) (client *cryptossh.Client, wait <-chan struct{}, shouldDial bool, err error) {
	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()
		return nil, nil, false, errPoolClosed
	}
	host := p.getHostLocked(target.Key())
	expired := p.reapHostLocked(host, p.opts.Clock.Now())

	if n := len(host.idle); n > 0 {
		idle := host.idle[n-1]
		host.idle = host.idle[:n-1]
		host.active[idle.client] = struct{}{}
		p.mu.Unlock()
		closeClients(expired)
		return idle.client, nil, false, nil
	}
	if host.total < p.opts.MaxPerHost {
		host.total++
		p.mu.Unlock()
		closeClients(expired)
		return nil, nil, true, nil
	}
	wait = host.notify
	p.mu.Unlock()
	closeClients(expired)
	return nil, wait, false, nil
}

func (p *Pool) dialReserved(ctx context.Context, target Target) (*cryptossh.Client, error) {
	config, err := p.opts.Config(target)
	if err != nil {
		p.releaseReservation(target.Key())
		return nil, err
	}
	client, err := p.opts.Dialer.Dial(ctx, target, config)
	if err != nil {
		p.releaseReservation(target.Key())
		return nil, err
	}

	p.mu.Lock()
	host := p.getHostLocked(target.Key())
	if p.closed {
		host.total--
		p.mu.Unlock()
		_ = client.Close()
		return nil, errPoolClosed
	}
	host.active[client] = struct{}{}
	p.mu.Unlock()
	p.startKeepalive(client)
	return client, nil
}

func (p *Pool) releaseReservation(key string) {
	p.mu.Lock()
	host := p.hosts[key]
	if host != nil && host.total > 0 {
		host.total--
		notify := host.notify
		host.notify = make(chan struct{})
		close(notify)
	}
	p.mu.Unlock()
}

func (p *Pool) getHostLocked(key string) *hostPool {
	host := p.hosts[key]
	if host != nil {
		return host
	}
	host = &hostPool{
		active: make(map[*cryptossh.Client]struct{}),
		notify: make(chan struct{}),
	}
	p.hosts[key] = host
	return host
}

func (p *Pool) cleanupLoop() {
	ticker := time.NewTicker(p.opts.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.ReapIdle()
		case <-p.done:
			return
		}
	}
}

// ReapIdle closes idle clients older than IdleTimeout. Tests may call
// it directly; production normally relies on the background cleanup
// loop.
func (p *Pool) ReapIdle() {
	p.mu.Lock()
	expired := make([]*cryptossh.Client, 0, len(p.hosts))
	now := p.opts.Clock.Now()
	for _, host := range p.hosts {
		expired = append(expired, p.reapHostLocked(host, now)...)
	}
	p.mu.Unlock()

	for _, client := range expired {
		closeClient(client)
	}
}

func (p *Pool) reapHostLocked(host *hostPool, now time.Time) []*cryptossh.Client {
	if len(host.idle) == 0 {
		return nil
	}
	kept := host.idle[:0]
	var expired []*cryptossh.Client
	for _, idle := range host.idle {
		if now.Sub(idle.lastUsed) > p.opts.IdleTimeout {
			expired = append(expired, idle.client)
			host.total--
			continue
		}
		kept = append(kept, idle)
	}
	host.idle = kept
	if len(expired) > 0 {
		notify := host.notify
		host.notify = make(chan struct{})
		close(notify)
	}
	return expired
}

func normalizePoolOptions(opts PoolOptions) PoolOptions {
	if opts.MaxPerHost <= 0 {
		opts.MaxPerHost = defaultMaxPerHost
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = defaultIdleTimeout
	}
	if opts.AcquireTimeout < 0 {
		opts.AcquireTimeout = 0
	}
	if opts.AcquireTimeout == 0 {
		opts.AcquireTimeout = defaultAcquireTimeout
	}
	if opts.CleanupInterval <= 0 {
		opts.CleanupInterval = opts.IdleTimeout / cleanupDivisor
		if opts.CleanupInterval <= 0 {
			opts.CleanupInterval = time.Second
		}
	}
	if opts.KeepaliveInterval == 0 {
		opts.KeepaliveInterval = defaultKeepalive
	}
	if opts.Dialer == nil {
		opts.Dialer = NetDialer{}
	}
	if opts.Clock == nil {
		opts.Clock = SystemClock{}
	}
	return opts
}

func (p *Pool) startKeepalive(client *cryptossh.Client) {
	if p.opts.KeepaliveInterval < 0 {
		return
	}
	go keepaliveLoop(p.done, client, p.opts.KeepaliveInterval)
}

func closeClients(clients []*cryptossh.Client) {
	for _, client := range clients {
		closeClient(client)
	}
}

func closeClient(client *cryptossh.Client) {
	if client != nil {
		_ = client.Close()
	}
}
