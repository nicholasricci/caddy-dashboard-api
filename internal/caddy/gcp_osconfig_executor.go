package caddy

import (
	"context"
	"strings"
	"time"

	gcpcloud "github.com/nicholasricci/caddy-dashboard/internal/cloud/gcp"
)

// GCPOsConfigExecutor runs Caddy guest operations via GCP OS Config / VM Manager (temporary OS policy assignment).
type GCPOsConfigExecutor struct {
	runner *gcpcloud.OSConfigShellRunner
}

func NewGCPOsConfigExecutor(runner *gcpcloud.OSConfigShellRunner) *GCPOsConfigExecutor {
	return &GCPOsConfigExecutor{runner: runner}
}

func executionResultFromGCP(cr *gcpcloud.ShellCommandResult) *ExecutionResult {
	if cr == nil {
		return &ExecutionResult{Status: ExecStatusFailed, Stderr: "nil gcp result"}
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

func (e *GCPOsConfigExecutor) ApplyConfig(ctx context.Context, t ExecTarget, payload []byte) (*ExecutionResult, error) {
	if e == nil || e.runner == nil || t.GCP == nil {
		return nil, ErrTransportNotConfigured
	}
	target := gcpcloud.RunShellTarget{
		ProjectID:        t.GCP.ProjectID,
		Zone:             t.GCP.Zone,
		InstanceName:     t.GCP.InstanceName,
		LabelKey:         t.GCP.LabelKey,
		LabelValue:       t.GCP.LabelValue,
		AssignmentPrefix: t.GCP.AssignmentPrefix,
		Timeout:          gcpTimeout(t.GCP.TimeoutSeconds),
	}
	cr, err := e.runner.RunShellCommand(ctx, target, caddyApplyConfigShell(payload))
	if err != nil {
		return nil, err
	}
	return executionResultFromGCP(cr), nil
}

func (e *GCPOsConfigExecutor) Reload(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	if e == nil || e.runner == nil || t.GCP == nil {
		return nil, ErrTransportNotConfigured
	}
	target := gcpcloud.RunShellTarget{
		ProjectID:        t.GCP.ProjectID,
		Zone:             t.GCP.Zone,
		InstanceName:     t.GCP.InstanceName,
		LabelKey:         t.GCP.LabelKey,
		LabelValue:       t.GCP.LabelValue,
		AssignmentPrefix: t.GCP.AssignmentPrefix,
		Timeout:          gcpTimeout(t.GCP.TimeoutSeconds),
	}
	cr, err := e.runner.RunShellCommand(ctx, target, caddyReloadShell())
	if err != nil {
		return nil, err
	}
	return executionResultFromGCP(cr), nil
}

func (e *GCPOsConfigExecutor) FetchConfig(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	if e == nil || e.runner == nil || t.GCP == nil {
		return nil, ErrTransportNotConfigured
	}
	target := gcpcloud.RunShellTarget{
		ProjectID:        t.GCP.ProjectID,
		Zone:             t.GCP.Zone,
		InstanceName:     t.GCP.InstanceName,
		LabelKey:         t.GCP.LabelKey,
		LabelValue:       t.GCP.LabelValue,
		AssignmentPrefix: t.GCP.AssignmentPrefix,
		Timeout:          gcpTimeout(t.GCP.TimeoutSeconds),
	}
	cr, err := e.runner.RunShellCommand(ctx, target, caddyFetchConfigShell())
	if err != nil {
		return nil, err
	}
	return executionResultFromGCP(cr), nil
}

func (e *GCPOsConfigExecutor) RunCommand(ctx context.Context, t ExecTarget, command string) (*ExecutionResult, error) {
	if e == nil || e.runner == nil || t.GCP == nil {
		return nil, ErrTransportNotConfigured
	}
	target := gcpcloud.RunShellTarget{
		ProjectID:        t.GCP.ProjectID,
		Zone:             t.GCP.Zone,
		InstanceName:     t.GCP.InstanceName,
		LabelKey:         t.GCP.LabelKey,
		LabelValue:       t.GCP.LabelValue,
		AssignmentPrefix: t.GCP.AssignmentPrefix,
		Timeout:          gcpTimeout(t.GCP.TimeoutSeconds),
	}
	cr, err := e.runner.RunShellCommand(ctx, target, command)
	if err != nil {
		return nil, err
	}
	return executionResultFromGCP(cr), nil
}

func gcpTimeout(sec int) time.Duration {
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}
