package caddy

import (
	"context"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

// ExecutionResult is a transport-neutral outcome of a remote command or HTTP call.
type ExecutionResult struct {
	Status string            `json:"status"`
	Stdout string            `json:"stdout"`
	Stderr string            `json:"stderr"`
	Meta   map[string]string `json:"meta,omitempty"`
}

const (
	ExecStatusSuccess = "success"
	ExecStatusFailed  = "failed"
)

// ExecTarget carries everything a RemoteExecutor needs for one node invocation.
type ExecTarget struct {
	Node *models.CaddyNode

	Transport string
	SSH       *SSHExecParams
	HTTP      *HTTPAdminParams
}

// SSHExecParams is parsed from transport_config for transport "ssh".
type SSHExecParams struct {
	Host             string
	User             string
	Port             int
	PrivateKeyRef    string
	KnownHostsRef    string
	KnownHostsPolicy string // "secure" (default) or "insecure"
}

// HTTPAdminParams is parsed from transport_config for transport "http_admin".
type HTTPAdminParams struct {
	BaseURL        string
	BearerTokenRef string
	TLSSkipVerify  bool
	ClientCertRef  string
	ClientKeyRef   string
}

// RemoteExecutor applies Caddy admin operations on a remote node.
type RemoteExecutor interface {
	ApplyConfig(ctx context.Context, t ExecTarget, payload []byte) (*ExecutionResult, error)
	Reload(ctx context.Context, t ExecTarget) (*ExecutionResult, error)
	FetchConfig(ctx context.Context, t ExecTarget) (*ExecutionResult, error)
}

func executionFailed(stderr string) *ExecutionResult {
	return &ExecutionResult{Status: ExecStatusFailed, Stderr: stderr}
}

func executionSuccess(stdout string, meta map[string]string) *ExecutionResult {
	if meta == nil {
		meta = map[string]string{}
	}
	return &ExecutionResult{Status: ExecStatusSuccess, Stdout: stdout, Meta: meta}
}
