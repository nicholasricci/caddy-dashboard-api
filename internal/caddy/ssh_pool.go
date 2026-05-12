package caddy

import (
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHPool keeps one SSH client per node id while transport_config is unchanged.
type SSHPool struct {
	idleTTL time.Duration
	mu      sync.Mutex
	entries map[string]*sshPooled // key: node UUID string
}

type sshPooled struct {
	client  *ssh.Client
	cfgHash string
	last    time.Time
}

// NewSSHPool returns a connection pool; idleTTL closes unused clients (e.g. 5m).
func NewSSHPool(idleTTL time.Duration) *SSHPool {
	if idleTTL <= 0 {
		idleTTL = 5 * time.Minute
	}
	return &SSHPool{
		idleTTL: idleTTL,
		entries: make(map[string]*sshPooled),
	}
}

// Evict closes and removes the pooled client for a node (e.g. after delete).
func (p *SSHPool) Evict(nodeID string) {
	if p == nil || nodeID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if ent, ok := p.entries[nodeID]; ok && ent.client != nil {
		_ = ent.client.Close()
	}
	delete(p.entries, nodeID)
}

// Acquire returns a client for the node, re-dialing when cfgHash changes or client is stale.
func (p *SSHPool) Acquire(nodeID, cfgHash string, dial func() (*ssh.Client, error)) (*ssh.Client, error) {
	if p == nil {
		return dial()
	}
	now := time.Now().UTC()
	p.mu.Lock()
	defer p.mu.Unlock()

	ent := p.entries[nodeID]
	if ent != nil && ent.client != nil {
		if ent.cfgHash == cfgHash && now.Sub(ent.last) < p.idleTTL {
			ent.last = now
			return ent.client, nil
		}
		_ = ent.client.Close()
		ent.client = nil
	}

	c, err := dial()
	if err != nil {
		return nil, err
	}
	p.entries[nodeID] = &sshPooled{client: c, cfgHash: cfgHash, last: now}
	return c, nil
}
