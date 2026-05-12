package caddy

import (
	"errors"
	"fmt"
)

var (
	// ErrNodeNoInstanceID is returned for aws_ssm nodes missing instance_id (legacy API compatibility).
	ErrNodeNoInstanceID = errors.New("node has no instance_id configured")

	// ErrTransportNotConfigured means required fields for the node's transport are missing.
	ErrTransportNotConfigured = errors.New("node transport is not fully configured")

	// ErrTransportUnsupportedOp means the operation is not available for this transport (e.g. reload on http_admin).
	ErrTransportUnsupportedOp = errors.New("operation not supported for this node transport")

	// ErrTransportUnreachable means a network-level failure talking to the node (SSH/HTTP).
	ErrTransportUnreachable = errors.New("remote node unreachable")

	// ErrUnknownTransport is returned when CaddyNode.Transport is not recognized.
	ErrUnknownTransport = errors.New("unknown node transport")
)

// TransportExecutionError wraps a failed remote invocation with stderr/stdout context.
type TransportExecutionError struct {
	Result *ExecutionResult
}

func (e *TransportExecutionError) Error() string {
	if e == nil || e.Result == nil {
		return "transport execution failed"
	}
	return fmt.Sprintf("remote execution failed: status=%s stderr=%s", e.Result.Status, e.Result.Stderr)
}
