# Caddy Dashboard

[![CI](https://github.com/nicholasricci/caddy-dashboard-api/actions/workflows/ci.yml/badge.svg)](https://github.com/nicholasricci/caddy-dashboard-api/actions/workflows/ci.yml)
[![Docker Publish](https://github.com/nicholasricci/caddy-dashboard-api/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/nicholasricci/caddy-dashboard-api/actions/workflows/docker-publish.yml)

Backend API in Go (Gin) to manage Caddy nodes across AWS regions with:

- JWT local authentication
- Node discovery via EC2 tags
- Manual node registration by private IP or instance ID
- Caddy operations executed via AWS SSM Run Command
- Configuration snapshots persisted in MariaDB (RDS)
- Users persisted in MariaDB with roles (`admin`, `user`)
- JWT signing secret loaded from `JWT_SECRET` environment variable

## Prerequisites

- Go 1.24+
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

## Required environment variables

At minimum, set:

- `JWT_SECRET`
- `DB_HOST`
- `DB_USER`
- `DB_PASSWORD`
- `AWS_REGIONS`

See `.env.example` for the full list.

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
