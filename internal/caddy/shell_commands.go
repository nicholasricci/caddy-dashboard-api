package caddy

import (
	"encoding/base64"
	"fmt"
)

// CaddyGuestShellCommands are executed on the guest (localhost admin API),
// matching the existing AWS SSM behavior.
func caddyApplyConfigShell(payload []byte) string {
	encoded := base64.StdEncoding.EncodeToString(payload)
	return fmt.Sprintf(`base64 -d >/tmp/caddy_config.json <<'CADDY_CFG_B64_EOF'
%s
CADDY_CFG_B64_EOF
curl -sS -X POST http://localhost:2019/load -H 'Content-Type: application/json' --data-binary @/tmp/caddy_config.json`, encoded)
}

func caddyReloadShell() string {
	return "caddy reload --config /etc/caddy/Caddyfile"
}

func caddyFetchConfigShell() string {
	return "curl -sS http://localhost:2019/config/"
}
