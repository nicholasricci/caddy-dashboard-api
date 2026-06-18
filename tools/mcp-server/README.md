# Caddy Dashboard MCP (development only)

Local MCP server for Cursor: read the Swagger spec and run **safe** HTTP calls against the Go API (`GET /api/v1/*` and `POST` only on `/api/v1/auth/login`, `/api/v1/auth/refresh`, `/api/v1/auth/logout`, `/api/v1/snapshots/backfill`). This includes read-only Caddy live-inspection endpoints such as `/api/v1/nodes/{id}/config/live/ids` and listing API keys (`GET /api/v1/api-keys` with admin JWT). Node create/update/delete, Caddy mutations (apply/reload/sync/mutate/propagate), discovery run, **`POST /api/v1/discovery/{id}/register-upstream`**, and **`POST /api/v1/upstream-profiles/{id}/register`** (M2M upstream registration) stay **out of scope** for `api_request` by design. It does not start unless `CADDY_MCP_DEV=1`.

The API models **CaddyNode** with `transport` (`aws_ssm`, `ssh`, `http_admin`, `gcp_osconfig`, `azure_run_command`, `inventory_only`), optional `transport_config` (JSON), and nullable `region` (required at runtime for `aws_ssm`). Discovery supports `gcp_labels` and `azure_tags` among other methods; optional discovery parameter `node_transport` can register `gcp_osconfig` / `azure_run_command` nodes. The MCP tools only reflect whatever is in the loaded Swagger (`docs/swagger.json` or live `/swagger/doc.json`). If you run the API without AWS regions, set `AWS_OPTIONAL=1` (or equivalent config) on the **API** process — the MCP server does not talk to AWS directly.

## Build

From repo root:

```bash
cd tools/mcp-server && npm install && npm run build
```

From repo root, `make mcp-run` sets `CADDY_MCP_DEV=1` for you. Inside `tools/mcp-server`, use `npm run start:dev` (plain `npm start` exits until you export `CADDY_MCP_DEV=1` — intentional so accidental runs stay off).

## Cursor configuration

Use **absolute paths** for `command`/`args`. Example (adjust `CADDY_DASHBOARD_ROOT` to your clone):

```json
{
  "mcpServers": {
    "caddy-dashboard-api": {
      "command": "node",
      "args": [
        "/home/you/Documents/github/nicholasricci/caddy-dashboard-api/tools/mcp-server/dist/index.js"
      ],
      "env": {
        "CADDY_MCP_DEV": "1",
        "CADDY_API_BASE_URL": "http://127.0.0.1:8080",
        "CADDY_DASHBOARD_ROOT": "/home/you/Documents/github/nicholasricci/caddy-dashboard-api",
        "CADDY_API_TOKEN": ""
      }
    }
  }
}
```

- **`CADDY_MCP_DEV`**: must be `1` or the process exits immediately.
- **`CADDY_API_BASE_URL`**: base URL of the Gin server (default `http://127.0.0.1:8080` if unset). Host must be `localhost` or `127.0.0.1` unless `MCP_ALLOW_NON_LOCALHOST=1` (then `host.docker.internal` / `host-gateway` are allowed).
- **`CADDY_DASHBOARD_ROOT`**: directory that contains `docs/swagger.json` when using `source=file` (defaults to the process working directory if unset).
- **`CADDY_API_TOKEN`**: optional JWT for protected `GET` requests (with or without `Bearer ` prefix). Use an **admin** token to list API keys; do not put M2M `cdk_live_…` keys here for `register-upstream` — that endpoint is blocked in MCP anyway.

Do **not** commit real tokens. Prefer user-level MCP settings or a local file ignored by git.

## Tools

| Tool | Purpose |
|------|---------|
| `get_openapi` | Full Swagger JSON from live `/swagger/doc.json` or `docs/swagger.json`. |
| `list_api_operations` | Compact list of operations; optional filter string. |
| `api_request` | Safe `GET`/`POST` as described above; blocks apply/reload/sync/mutate/propagate, discovery run, `register-upstream`, and `upstream-profiles/.../register`. Supports pagination query params like `?limit=20&offset=0`, including discovery snapshot reads such as `/api/v1/discovery/{id}/snapshots`, Caddy live config ID/upstream reads under `/api/v1/nodes/{id}/config/live/ids`, and `GET /api/v1/api-keys`. Also supports admin write backfill `POST /api/v1/snapshots/backfill` (rate-limited, admin-only). |

After backend changes, regenerate Swagger in the repo (`make swagger`) so `docs/swagger.json` matches the API; then `get_openapi` with `source=file` or `list_api_operations` reflect new fields (e.g. `transport`, `transport_config`, `APIKey`, `register-upstream`) and routes. `snapshot_scope` on discovery payloads and discovery-group snapshot routes are documented the same way.

**Upstream registration (EC2):** production automation uses M2M API keys (`cdk_live_…`), not the MCP. Single-handler: `POST /api/v1/discovery/{discovery_config_id}/register-upstream` ([`scripts/ec2-register-upstream.sh`](../../scripts/ec2-register-upstream.sh)). Multi-handler profile: `POST /api/v1/upstream-profiles/{profile_id}/register` ([`scripts/ec2-register-upstream-profile.sh`](../../scripts/ec2-register-upstream-profile.sh)). See root `README.md`.

## Security notes

- Intended **only** on a developer machine with a local API.
- Not part of production deploys; do not bake this into release images.
