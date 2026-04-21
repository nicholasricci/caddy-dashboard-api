# Caddy Dashboard MCP (development only)

Local MCP server for Cursor: read the Swagger spec and run **safe** HTTP calls against the Go API (`GET /api/v1/*` and `POST` only on `/api/v1/auth/login`, `/api/v1/auth/refresh`, `/api/v1/auth/logout`, `/api/v1/snapshots/backfill`). This includes read-only Caddy live-inspection endpoints such as `/api/v1/nodes/{id}/config/live/ids`. It does not start unless `CADDY_MCP_DEV=1`.

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
        "/home/you/Documents/github/nicholasricci/caddy-dashboard/tools/mcp-server/dist/index.js"
      ],
      "env": {
        "CADDY_MCP_DEV": "1",
        "CADDY_API_BASE_URL": "http://127.0.0.1:8080",
        "CADDY_DASHBOARD_ROOT": "/home/you/Documents/github/nicholasricci/caddy-dashboard",
        "CADDY_API_TOKEN": ""
      }
    }
  }
}
```

- **`CADDY_MCP_DEV`**: must be `1` or the process exits immediately.
- **`CADDY_API_BASE_URL`**: base URL of the Gin server (default `http://127.0.0.1:8080` if unset). Host must be `localhost` or `127.0.0.1` unless `MCP_ALLOW_NON_LOCALHOST=1` (then `host.docker.internal` / `host-gateway` are allowed).
- **`CADDY_DASHBOARD_ROOT`**: directory that contains `docs/swagger.json` when using `source=file` (defaults to the process working directory if unset).
- **`CADDY_API_TOKEN`**: optional JWT for protected `GET` requests (with or without `Bearer ` prefix).

Do **not** commit real tokens. Prefer user-level MCP settings or a local file ignored by git.

## Tools

| Tool | Purpose |
|------|---------|
| `get_openapi` | Full Swagger JSON from live `/swagger/doc.json` or `docs/swagger.json`. |
| `list_api_operations` | Compact list of operations; optional filter string. |
| `api_request` | Safe `GET`/`POST` as described above; blocks apply/reload/sync/discovery run paths. Supports pagination query params like `?limit=20&offset=0`, including discovery snapshot reads such as `/api/v1/discovery/{id}/snapshots` and Caddy live config ID/upstream reads under `/api/v1/nodes/{id}/config/live/ids`. Also supports admin write backfill `POST /api/v1/snapshots/backfill` (rate-limited, admin-only). |

When you refresh the OpenAPI spec after backend changes, `snapshot_scope` on discovery payloads and discovery-group snapshot routes are available through `get_openapi` and `list_api_operations`.

## Security notes

- Intended **only** on a developer machine with a local API.
- Not part of production deploys; do not bake this into release images.
