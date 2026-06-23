package caddy

import (
	"context"
	"fmt"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

// Dispatcher routes RemoteExecutor calls by node transport.
type Dispatcher struct {
	executors map[string]RemoteExecutor
}

// NewDispatcher builds a dispatcher. Keys must match models.Transport* constants.
func NewDispatcher(executors map[string]RemoteExecutor) *Dispatcher {
	cp := make(map[string]RemoteExecutor, len(executors))
	for k, v := range executors {
		if v == nil {
			continue
		}
		cp[k] = v
	}
	return &Dispatcher{executors: cp}
}

func (d *Dispatcher) ApplyConfig(ctx context.Context, t ExecTarget, payload []byte) (*ExecutionResult, error) {
	return d.delegate(ctx, t, func(ctx context.Context, ex RemoteExecutor) (*ExecutionResult, error) {
		return ex.ApplyConfig(ctx, t, payload)
	})
}

func (d *Dispatcher) Reload(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	return d.delegate(ctx, t, func(ctx context.Context, ex RemoteExecutor) (*ExecutionResult, error) {
		return ex.Reload(ctx, t)
	})
}

func (d *Dispatcher) FetchConfig(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	return d.delegate(ctx, t, func(ctx context.Context, ex RemoteExecutor) (*ExecutionResult, error) {
		return ex.FetchConfig(ctx, t)
	})
}

func (d *Dispatcher) RunCommand(ctx context.Context, t ExecTarget, command string) (*ExecutionResult, error) {
	return d.delegate(ctx, t, func(ctx context.Context, ex RemoteExecutor) (*ExecutionResult, error) {
		return ex.RunCommand(ctx, t, command)
	})
}

type execFn func(ctx context.Context, ex RemoteExecutor) (*ExecutionResult, error)

func (d *Dispatcher) delegate(ctx context.Context, t ExecTarget, fn execFn) (*ExecutionResult, error) {
	if d == nil {
		return nil, ErrTransportNotConfigured
	}
	if t.Transport == models.TransportInventoryOnly {
		return nil, ErrTransportUnsupportedOp
	}
	ex, ok := d.executors[t.Transport]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTransport, t.Transport)
	}
	return fn(ctx, ex)
}

// ErrRemoteExecutor always fails with the wrapped error (used when AWS SSM is not configured).
type ErrRemoteExecutor struct {
	Err error
}

func (e *ErrRemoteExecutor) ApplyConfig(ctx context.Context, _ ExecTarget, _ []byte) (*ExecutionResult, error) {
	return nil, e.err()
}
func (e *ErrRemoteExecutor) Reload(ctx context.Context, _ ExecTarget) (*ExecutionResult, error) {
	return nil, e.err()
}
func (e *ErrRemoteExecutor) FetchConfig(ctx context.Context, _ ExecTarget) (*ExecutionResult, error) {
	return nil, e.err()
}
func (e *ErrRemoteExecutor) RunCommand(ctx context.Context, _ ExecTarget, _ string) (*ExecutionResult, error) {
	return nil, e.err()
}
func (e *ErrRemoteExecutor) err() error {
	if e == nil || e.Err == nil {
		return ErrTransportNotConfigured
	}
	return e.Err
}
