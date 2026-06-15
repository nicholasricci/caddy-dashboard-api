package caddy

import (
	"context"
	"strings"
	"time"

	azurecloud "github.com/nicholasricci/caddy-dashboard/internal/cloud/azure"
)

// AzureRunCommandExecutor runs Caddy guest operations via Azure VM Run Command.
type AzureRunCommandExecutor struct {
	runner *azurecloud.RunCommandRunner
}

func NewAzureRunCommandExecutor(runner *azurecloud.RunCommandRunner) *AzureRunCommandExecutor {
	return &AzureRunCommandExecutor{runner: runner}
}

func executionResultFromAzure(cr *azurecloud.ShellCommandResult) *ExecutionResult {
	if cr == nil {
		return &ExecutionResult{Status: ExecStatusFailed, Stderr: "nil azure result"}
	}
	st := ExecStatusFailed
	if strings.EqualFold(cr.Status, "Success") {
		st = ExecStatusSuccess
	}
	meta := cr.Meta
	if meta == nil {
		meta = map[string]string{}
	}
	return &ExecutionResult{
		Status: st,
		Stdout: cr.Stdout,
		Stderr: cr.Stderr,
		Meta:   meta,
	}
}

func (e *AzureRunCommandExecutor) ApplyConfig(ctx context.Context, t ExecTarget, payload []byte) (*ExecutionResult, error) {
	if e == nil || e.runner == nil || t.Azure == nil {
		return nil, ErrTransportNotConfigured
	}
	target := azurecloud.RunShellTarget{
		SubscriptionID: t.Azure.SubscriptionID,
		ResourceGroup:  t.Azure.ResourceGroup,
		VMName:         t.Azure.VMName,
		Timeout:        azureTimeout(t.Azure.TimeoutSeconds),
	}
	cr, err := e.runner.RunShellCommand(ctx, target, caddyApplyConfigShell(payload))
	if err != nil {
		return nil, err
	}
	return executionResultFromAzure(cr), nil
}

func (e *AzureRunCommandExecutor) Reload(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	if e == nil || e.runner == nil || t.Azure == nil {
		return nil, ErrTransportNotConfigured
	}
	target := azurecloud.RunShellTarget{
		SubscriptionID: t.Azure.SubscriptionID,
		ResourceGroup:  t.Azure.ResourceGroup,
		VMName:         t.Azure.VMName,
		Timeout:        azureTimeout(t.Azure.TimeoutSeconds),
	}
	cr, err := e.runner.RunShellCommand(ctx, target, caddyReloadShell())
	if err != nil {
		return nil, err
	}
	return executionResultFromAzure(cr), nil
}

func (e *AzureRunCommandExecutor) FetchConfig(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	if e == nil || e.runner == nil || t.Azure == nil {
		return nil, ErrTransportNotConfigured
	}
	target := azurecloud.RunShellTarget{
		SubscriptionID: t.Azure.SubscriptionID,
		ResourceGroup:  t.Azure.ResourceGroup,
		VMName:         t.Azure.VMName,
		Timeout:        azureTimeout(t.Azure.TimeoutSeconds),
	}
	cr, err := e.runner.RunShellCommand(ctx, target, caddyFetchConfigShell())
	if err != nil {
		return nil, err
	}
	return executionResultFromAzure(cr), nil
}

func azureTimeout(sec int) time.Duration {
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}
