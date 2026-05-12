package caddy

import (
	"context"
	"encoding/base64"
	"fmt"

	awssvc "github.com/nicholasricci/caddy-dashboard/internal/aws"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

// SSMExecutor runs Caddy admin operations via AWS SSM RunShellScript.
type SSMExecutor struct {
	ssm *awssvc.SSMService
}

func NewSSMExecutor(ssm *awssvc.SSMService) *SSMExecutor {
	return &SSMExecutor{ssm: ssm}
}

func executionResultFromAWSSSM(cr *awssvc.CommandResult) *ExecutionResult {
	if cr == nil {
		return &ExecutionResult{Status: ExecStatusFailed, Stderr: "nil aws result"}
	}
	st := ExecStatusFailed
	if cr.Status == "Success" {
		st = ExecStatusSuccess
	}
	return &ExecutionResult{
		Status: st,
		Stdout: cr.Stdout,
		Stderr: cr.Stderr,
		Meta: map[string]string{
			"command_id": cr.CommandID,
			"aws_status": cr.Status,
		},
	}
}

func (e *SSMExecutor) ApplyConfig(ctx context.Context, t ExecTarget, payload []byte) (*ExecutionResult, error) {
	if e == nil || e.ssm == nil {
		return nil, ErrTransportNotConfigured
	}
	region := t.Node.RegionString()
	id := instanceIDString(t.Node)
	if region == "" || id == "" {
		return nil, ErrTransportNotConfigured
	}
	encoded := base64.StdEncoding.EncodeToString(payload)
	command := fmt.Sprintf(`base64 -d >/tmp/caddy_config.json <<'CADDY_CFG_B64_EOF'
%s
CADDY_CFG_B64_EOF
curl -sS -X POST http://localhost:2019/load -H 'Content-Type: application/json' --data-binary @/tmp/caddy_config.json`, encoded)
	cr, err := e.ssm.RunShellCommand(ctx, region, id, command)
	if err != nil {
		return nil, err
	}
	return executionResultFromAWSSSM(cr), nil
}

func (e *SSMExecutor) Reload(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	if e == nil || e.ssm == nil {
		return nil, ErrTransportNotConfigured
	}
	region := t.Node.RegionString()
	id := instanceIDString(t.Node)
	if region == "" || id == "" {
		return nil, ErrTransportNotConfigured
	}
	command := "caddy reload --config /etc/caddy/Caddyfile"
	cr, err := e.ssm.RunShellCommand(ctx, region, id, command)
	if err != nil {
		return nil, err
	}
	return executionResultFromAWSSSM(cr), nil
}

func (e *SSMExecutor) FetchConfig(ctx context.Context, t ExecTarget) (*ExecutionResult, error) {
	if e == nil || e.ssm == nil {
		return nil, ErrTransportNotConfigured
	}
	region := t.Node.RegionString()
	id := instanceIDString(t.Node)
	if region == "" || id == "" {
		return nil, ErrTransportNotConfigured
	}
	command := "curl -sS http://localhost:2019/config/"
	cr, err := e.ssm.RunShellCommand(ctx, region, id, command)
	if err != nil {
		return nil, err
	}
	return executionResultFromAWSSSM(cr), nil
}

func instanceIDString(n *models.CaddyNode) string {
	if n == nil || n.InstanceID == nil {
		return ""
	}
	return *n.InstanceID
}
