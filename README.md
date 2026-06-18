# Caddy Dashboard

[![CI](https://github.com/nicholasricci/caddy-dashboard-api/actions/workflows/ci.yml/badge.svg)](https://github.com/nicholasricci/caddy-dashboard-api/actions/workflows/ci.yml)
[![Docker Publish](https://github.com/nicholasricci/caddy-dashboard-api/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/nicholasricci/caddy-dashboard-api/actions/workflows/docker-publish.yml)

Backend API in Go (Gin) to manage Caddy nodes across AWS regions with:

- JWT local authentication (access + revocable refresh)
- Scoped **API keys** for machine-to-machine calls (`register_upstream`)
- Node discovery via EC2 tags
- Manual node registration by private IP or instance ID
- Caddy operations executed via AWS SSM Run Command
- Configuration snapshots persisted in MariaDB (RDS), scoped by node or by discovery group (`snapshot_scope`)
- Users persisted in MariaDB with roles (`admin`, `user`)
- JWT signing secret loaded from `JWT_SECRET` environment variable (minimum 32 chars)

## Prerequisites

- Go 1.26+
- Docker and Docker Compose (optional but recommended for local DB)
- AWS credentials (profile or IAM role) with required EC2/SSM permissions
- MariaDB/MySQL instance

## Quick start (local)

1. Copy `.env.example` to `.env` and fill variables.
2. Ensure AWS credentials are available (profile or IAM role).
3. Run:

```bash
make run
```

API is served on `http://localhost:8080` by default.

## Quick start (Docker dependencies)

Use Docker Compose to run local dependencies, then start API from host:

```bash
docker compose up -d
make run
```

## Swagger

1. Install CLI:

```bash
make swag-install
```

2. Generate docs:

```bash
make swagger
```

3. Run the API and open:

`http://localhost:8080/swagger/index.html`

## API prefix

All APIs are under `/api/v1`.

Discovery snapshots are available via `GET /api/v1/discovery/:id/snapshots`; node snapshots (`GET /api/v1/nodes/:id/snapshots`) remain available and automatically resolve to group snapshots when the related discovery uses `snapshot_scope=group`.

Admin operators can re-run legacy snapshot backfill on demand with `POST /api/v1/snapshots/backfill` (rate-limited).

### M2M API keys and upstream registration

For EC2 (or other automation) that must add upstream dials to a Caddy proxy cluster without admin JWT:

1. **Create an API key** (admin JWT): `POST /api/v1/api-keys` with `scopes: ["register_upstream"]` and `allowed_discovery_config_ids` listing the target discovery group UUID(s). The plaintext secret (`cdk_live_…`) is returned **once**.
2. **Register upstream** (API key): `POST /api/v1/discovery/{discovery_config_id}/register-upstream` with `Authorization: Bearer <api_key>` and body:

```json
{
  "config_id": "myapp-prod-route",
  "port": 8080,
  "private_ip": "10.0.1.42"
}
```

The API picks the first reachable Caddy node in that discovery group, mutates upstreams, and propagates config to peers. See Swagger for the full response shape.

3. **EC2 user-data**: [`scripts/ec2-register-upstream.sh`](scripts/ec2-register-upstream.sh) reads the instance private IP from IMDS and calls the endpoint above. Store the API key in AWS Secrets Manager (`API_KEY_SECRET_ARN`); set `DISCOVERY_CONFIG_ID`, `CONFIG_ID`, `UPSTREAM_PORT`, and `CADDY_DASHBOARD_URL` in the launch template.

Admin key management: `GET /api/v1/api-keys`, `POST /api/v1/api-keys`, `POST /api/v1/api-keys/{id}/revoke`, `DELETE /api/v1/api-keys/{id}`.

## Required environment variables

At minimum, set:

- `JWT_SECRET`
- `DB_HOST`
- `DB_USER`
- `DB_PASSWORD`
- `AWS_REGIONS`
- `JWT_ISSUER`
- `JWT_AUDIENCE`

See `.env.example` for the full list.

## Health and readiness

- `GET /api/v1/health`: liveness probe
- `GET /api/v1/ready`: readiness probe (DB ping + AWS regions configured)

## Optional Loki shipping (Grafana Cloud)

The API already emits structured JSON logs to stdout via Zap. You can optionally ship them to Grafana Cloud Loki using the Docker Compose `alloy` service.

1. Set Loki credentials in `.env` (see `.env.example` keys: `LOKI_URL`, `LOKI_USER`, `LOKI_API_KEY`, `LOKI_TENANT_ID`, `LOKI_ENVIRONMENT`).
2. Start Compose with the Loki profile:

```bash
docker compose --profile loki up --build
```

3. Generate API traffic and query Loki in Grafana Explore, e.g.:

```logql
{service="caddy-dashboard-api",environment="local"}
```

This integration is optional: if you do not run the `loki` profile, logging behavior remains unchanged (stdout only).

## Database migrations

- Use `make migrate` to run schema migrations from `cmd/migrate`.
- API startup does not auto-migrate unless launched with `--auto-migrate`.

## Rate limits and payload limits

- Login/refresh endpoints are rate-limited by IP.
- Caddy apply endpoint is rate-limited and has a larger request body limit.
- `POST /api/v1/discovery/:id/register-upstream` is rate-limited by IP (M2M).
- Global request body limit applies to all routes by default.

## CI and release

- CI workflow validates format, vet, build, race tests, static analysis, and vulnerability checks.
- Docker image release publishes to GHCR via GitHub release workflow.

## Security

- Report vulnerabilities privately following `SECURITY.md`.
- Do not commit `.env`, `.env.prod`, or any secret material.

## Contributing

Contributions are welcome. Please read `CONTRIBUTING.md` and follow the PR template.

## Development-only tooling

`tools/mcp-server` is development tooling for local workflows and is not part of the production deployment path.
