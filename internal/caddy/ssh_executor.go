package caddy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/nicholasricci/caddy-dashboard/internal/secrets"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHExecutor runs Caddy admin operations over SSH using curl on the remote host.
type SSHExecutor struct {
	pool         *SSHPool
	resolver     *secrets.MultiResolver
	dialTimeout  time.Duration
	readDeadline time.Duration
}

// NewSSHExecutor constructs an SSH-backed executor.
func NewSSHExecutor(pool *SSHPool, resolver *secrets.MultiResolver, dialTimeout, readDeadline time.Duration) *SSHExecutor {
	if dialTimeout <= 0 {
		dialTimeout = 15 * time.Second
	}
	if readDeadline <= 0 {
		readDeadline = 2 * time.Minute
	}
	return &SSHExecutor{
		pool:         pool,
		resolver:     resolver,
		dialTimeout:  dialTimeout,
		readDeadline: readDeadline,
	}
}

func (e *SSHExecutor) ApplyConfig(ctx context.Context, t ExecTarget, payload []byte) (*ExecutionResult, error) {
	if e == nil || t.SSH == nil || e.resolver == nil {
		return nil, ErrTransportNotConfigured
	}
	cmd := `/bin/sh -c 'exec curl -sS -f -X POST http://127.0.0.1:2019/load -H "Content-Type: application/json" --data-binary @-'`
	return e.run(ctx, t, cmd, payload)
}

func (e *SSHExecutor) Reload(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	if e == nil || t.SSH == nil || e.resolver == nil {
		return nil, ErrTransportNotConfigured
	}
	cmd := `caddy reload --config /etc/caddy/Caddyfile`
	return e.run(ctx, t, cmd, nil)
}

func (e *SSHExecutor) FetchConfig(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	if e == nil || t.SSH == nil || e.resolver == nil {
		return nil, ErrTransportNotConfigured
	}
	cmd := `curl -sS -f http://127.0.0.1:2019/config/`
	return e.run(ctx, t, cmd, nil)
}

func (e *SSHExecutor) RunCommand(ctx context.Context, t ExecTarget, command string) (*ExecutionResult, error) {
	if e == nil || t.SSH == nil || e.resolver == nil {
		return nil, ErrTransportNotConfigured
	}
	return e.run(ctx, t, command, nil)
}

func (e *SSHExecutor) run(ctx context.Context, t ExecTarget, remoteCmd string, stdin []byte) (*ExecutionResult, error) {
	if e.readDeadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.readDeadline)
		defer cancel()
	}
	p := t.SSH
	h := sshConfigHash(t.Node.ID.String(), p)

	dial := func() (*ssh.Client, error) {
		return e.dial(ctx, p)
	}

	var client *ssh.Client
	var err error
	if e.pool != nil {
		client, err = e.pool.Acquire(t.Node.ID.String(), h, dial)
	} else {
		client, err = dial()
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransportUnreachable, err)
	}
	defer func() {
		if e.pool == nil && client != nil {
			_ = client.Close()
		}
	}()

	sess, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("%w: new session: %v", ErrTransportUnreachable, err)
	}
	defer sess.Close()

	if len(stdin) > 0 {
		sess.Stdin = bytes.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr

	if err := sess.Start(remoteCmd); err != nil {
		return nil, fmt.Errorf("%w: start: %v", ErrTransportUnreachable, err)
	}

	waitErr := make(chan error, 1)
	go func() { waitErr <- sess.Wait() }()

	select {
	case <-ctx.Done():
		_ = sess.Close()
		return nil, ctx.Err()
	case err := <-waitErr:
		res := &ExecutionResult{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
		}
		if err != nil {
			res.Status = ExecStatusFailed
			return res, nil
		}
		res.Status = ExecStatusSuccess
		return res, nil
	}
}

func (e *SSHExecutor) dial(ctx context.Context, p *SSHExecParams) (*ssh.Client, error) {
	keyPEM, err := e.resolver.Resolve(ctx, p.PrivateKeyRef)
	if err != nil {
		return nil, fmt.Errorf("private key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	var hostKeyCallback ssh.HostKeyCallback
	switch p.KnownHostsPolicy {
	case "insecure":
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // explicit opt-in per node transport_config
	default:
		khData, err := e.resolver.Resolve(ctx, p.KnownHostsRef)
		if err != nil {
			return nil, fmt.Errorf("known_hosts: %w", err)
		}
		f, err := os.CreateTemp("", "known_hosts_*")
		if err != nil {
			return nil, err
		}
		path := f.Name()
		if _, err := f.Write(khData); err != nil {
			_ = f.Close()
			_ = os.Remove(path)
			return nil, err
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(path)
			return nil, err
		}
		defer func() { _ = os.Remove(path) }()
		hostKeyCallback, err = knownhosts.New(path)
		if err != nil {
			return nil, fmt.Errorf("knownhosts: %w", err)
		}
	}

	cfg := &ssh.ClientConfig{
		User:            p.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         e.dialTimeout,
	}

	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
	d := net.Dialer{Timeout: e.dialTimeout}
	netConn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	c, chans, reqs, err := ssh.NewClientConn(netConn, p.Host, cfg)
	if err != nil {
		_ = netConn.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func sshConfigHash(nodeID string, p *SSHExecParams) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s",
		nodeID, p.Host, p.Port, p.User, p.PrivateKeyRef, p.KnownHostsRef, p.KnownHostsPolicy)))
	return hex.EncodeToString(sum[:])
}
