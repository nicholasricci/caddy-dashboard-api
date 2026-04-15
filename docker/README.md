# Docker layout

La cartella `docker/` e il compose in root sono separati per ambiente:

- `docker/prod/Dockerfile`: immagine production-grade usata da CI/CD e deploy.
- `docker/dev/Dockerfile`: immagine sviluppo con hot reload (`air`).
- `docker/dev/.air.toml`: configurazione hot reload Go.
- `docker-compose.yml` (root): stack **solo sviluppo** (`api` + `mariadb`).

## Sviluppo locale con hot reload

Dalla root del repository:

```bash
export JWT_SECRET="sostituisci-con-un-segreto-lungo-almeno-32-caratteri"
docker compose up --build
```

`docker compose` legge automaticamente il file `.env` nella root del repository.

Con il container `api` avviato, ogni modifica ai file Go nel workspace viene ricompilata e riavviata automaticamente da `air`.

## Health check HTTP

Endpoint: `GET /api/v1/health`

```bash
curl -sS http://localhost:8080/api/v1/health
```

## Build immagine produzione

Dalla root del repository:

```bash
docker build -f docker/prod/Dockerfile -t caddy-dashboard-api:prod .
```

## CI/CD

- Test e vet: [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)
- Build/push GHCR: [`.github/workflows/docker-publish.yml`](../.github/workflows/docker-publish.yml) (usa `docker/prod/Dockerfile`, trigger su **Release published** con fallback `workflow_dispatch`)
