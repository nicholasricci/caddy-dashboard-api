package caddy

import (
	"context"
	"encoding/base64"
	"fmt"

	awssvc "github.com/nicholasricci/caddy-dashboard/internal/aws"
)

type SSMExecutor struct {
	ssm *awssvc.SSMService
}

func NewSSMExecutor(ssm *awssvc.SSMService) *SSMExecutor {
	return &SSMExecutor{ssm: ssm}
}

func (e *SSMExecutor) ApplyConfig(ctx context.Context, region, instanceID string, payload []byte) (*awssvc.CommandResult, error) {
	encoded := base64.StdEncoding.EncodeToString(payload)
	// Here-document avoids embedding the payload in shell quotes (defense in depth; no expansion with quoted delimiter).
	command := fmt.Sprintf(`base64 -d >/tmp/caddy_config.json <<'CADDY_CFG_B64_EOF'
%s
CADDY_CFG_B64_EOF
curl -sS -X POST http://localhost:2019/load -H 'Content-Type: application/json' --data-binary @/tmp/caddy_config.json`, encoded)
	return e.ssm.RunShellCommand(ctx, region, instanceID, command)
}

func (e *SSMExecutor) Reload(ctx context.Context, region, instanceID string) (*awssvc.CommandResult, error) {
	command := "caddy reload --config /etc/caddy/Caddyfile"
	return e.ssm.RunShellCommand(ctx, region, instanceID, command)
}

func (e *SSMExecutor) FetchConfig(ctx context.Context, region, instanceID string) (*awssvc.CommandResult, error) {
	command := "curl -sS http://localhost:2019/config/"
	return e.ssm.RunShellCommand(ctx, region, instanceID, command)
}
