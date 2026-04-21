# AGENTS.md — Caddy Dashboard

Documento di contesto per assistenti AI che lavorano su questo repository.

## Scopo del progetto

**Caddy Dashboard** è un backend API in **Go** (framework **Gin**) per gestire nodi **Caddy** su **AWS**: registrazione manuale o discovery via EC2, operazioni su Caddy tramite **AWS Systems Manager (SSM) Run Command**, persistenza di nodi, regole di discovery, snapshot di configurazione e utenti in **MariaDB/MySQL** (driver GORM `mysql`), autenticazione **JWT** con ruoli `admin` e `user`.

Modulo Go: `github.com/nicholasricci/caddy-dashboard` · Go **1.26**.

## Stack tecnico

| Area | Tecnologia |
|------|------------|
| HTTP API | Gin, middleware CORS e logging |
| Auth | JWT (`github.com/golang-jwt/jwt/v5`), bcrypt per password utente |
| Database | GORM + `gorm.io/driver/mysql` (DSN TCP MariaDB/MySQL) |
| AWS | AWS SDK v2: EC2, SSM, Secrets Manager (config supporta ARN segreti in YAML) |
| Config | Viper (`configs/config.yaml`) + variabili d’ambiente; `godotenv` carica `.env` in locale |
| Log | Zap (`go.uber.org/zap`) |
| API docs | Swaggo: annotazioni su handler, generazione in `docs/`; UI su `/swagger/index.html`, JSON su `/swagger/doc.json` |
| Dev tooling | Server MCP opzionale in `tools/mcp-server/` (solo sviluppo, chiamate HTTP “sicure”) |

## Layout del repository

```
cmd/server/          # Entrypoint HTTP, wiring di config, DB, AWS, handler, router
configs/             # config.yaml (server, auth, aws, database, observability)
docs/                # Swagger generato (swagger.json, swagger.yaml, docs.go) — non editare a mano
internal/api/handlers/   # Handler Gin per auth, health, nodes, discovery, caddy, users
internal/api/middleware/ # Auth JWT, RequireAdmin, CORS, request logger
internal/api/routes/     # Registrazione route e gruppi protected/admin
internal/auth/       # Servizio JWT e validazione utenti
internal/aws/        # Client multi-regione, EC2, SSM, Secrets
internal/caddy/      # Esecuzione comandi Caddy via SSM, snapshot
internal/config/     # Load configurazione (Viper + env)
internal/database/   # Connessione GORM, AutoMigrate
internal/models/     # Entità GORM (CaddyNode, DiscoveryConfig, CaddySnapshot, User, …)
internal/repository/ # Accesso dati
internal/services/   # Logica di dominio (node, discovery, caddy, user)
pkg/logger/          # Costruzione logger Zap
tools/mcp-server/    # Server MCP Node/TS per Cursor (dev only)
```

## Configurazione

- File principale: [`configs/config.yaml`](configs/config.yaml) (porta, `gin_mode`, CORS, TTL JWT, regioni AWS, cache Caddy, DSN DB, log level).
- Variabili d’ambiente: vedi [`.env.example`](.env.example). Rilevanti: `SERVER_PORT` (binding), `DB_*`, `AWS_PROFILE`, `AWS_REGIONS` (lista separata da virgola), `JWT_SECRET` (**obbligatorio**), `CADDY_CACHE_TTL`, `LOG_LEVEL`, `GIN_MODE`.
- Viper usa prefisso `APP_` con sostituzione `.` → `_` per override (es. variabili annidate).
- CORS: con `cors_allowed_origins` vuoto si usa `*` senza credentials; per una SPA su altra origine impostare esplicitamente (es. `http://localhost:4200`) in YAML.

**Nota:** In `config.yaml` / `.env.example` la porta DB può comparire come `5432`; il driver è **MySQL/MariaDB** — in produzione usare tipicamente **3306** se il server è MariaDB standard.

## API HTTP

- Prefisso API: **`/api/v1`**.
- Swagger UI: `http://<host>:<port>/swagger/index.html`.
- Autenticazione: header `Authorization: Bearer <access_token>` per route protette.
- **Ruoli:**
  - Utente autenticato: lettura nodi/discovery, snapshot propri per nodo, ecc.
  - **`admin`**: mutazioni su nodi, discovery, utenti; sync/apply/reload Caddy; run discovery.

Endpoint principali (dettaglio in [`internal/api/routes/routes.go`](internal/api/routes/routes.go)):

- Pubblici: `GET /api/v1/health`, `GET /api/v1/ready`, `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`.
- Protetti: CRUD/list nodi e discovery in lettura.
- Protetti: `POST /api/v1/auth/logout`.
- Solo admin: creazione/aggiornamento/cancellazione nodi e discovery; `POST .../sync`, `/apply`, `/reload`; `GET /nodes/:id/snapshots`; `GET /discovery/:id/snapshots`; `POST /discovery/:id/run`; gestione utenti; `GET /audit`.
- Solo admin: introspezione config Caddy live con `GET /nodes/:id/config/live/ids`, `GET /nodes/:id/config/live/ids/:configId`, `GET /nodes/:id/config/live/ids/:configId/upstreams`.
- Solo admin: endpoint operativo `POST /api/v1/snapshots/backfill` per rilanciare on-demand il backfill `discovery_config_id` sugli snapshot legacy (idempotente, rate-limited).

## Dominio funzionale

- **Nodo (`CaddyNode`)**: istanza/registrazione con IP privato, instance ID, regione, stato, flag SSM, ecc.
- **Discovery (`DiscoveryConfig`)**: regole per trovare nodi (metodi documentati in Swagger; alcuni possono essere non implementati — vedere errori `501` / messaggi servizio).
- **Snapshot**: versioni di configurazione Caddy persistite dopo sync/apply, con scope configurabile per `DiscoveryConfig` (`node` o `group`).
- **Utenti**: username, ruolo, password hash; JWT emessi al login.

Operazioni Caddy lato macchina remota avvengono tramite **SSM** (non SSH diretto dall’API). Le letture live e i metadati derivati (`@id`, upstream) sono cache-ati in memoria per nodo con TTL configurabile (`caddy.cache_ttl` / `CADDY_CACHE_TTL`) e invalidazione su mutazioni (`apply`, `sync`, `reload`).

## Comandi utili

```bash
make run          # go run ./cmd/server
make build        # go build ./...
make test         # go test ./...
make lint         # go vet ./...
make fmt          # go fmt ./...
make swagger      # rigenera docs Swagger da annotazioni (richiede swag CLI)
make swag-install # installa swag CLI
make mcp-build    # build server MCP in tools/mcp-server
```

## Server MCP (solo sviluppo)

Sotto [`tools/mcp-server/`](tools/mcp-server/) c’è un server **MCP** per integrare Cursor con lo Swagger e chiamate HTTP **limitate** (GET su `/api/v1/*` + POST solo login/refresh, denylist su path pericolosi, gate `CADDY_MCP_DEV=1`). **Non** è parte del deploy di produzione. Istruzioni: [`tools/mcp-server/README.md`](tools/mcp-server/README.md).

## Convenzioni per modifiche al codice

- Allinearsi a stile e strati esistenti: **handler** sottili, **services** con logica, **repository** per DB, **models** per GORM.
- Nuove route documentate con **commenti Swaggo** (`// @Summary`, `@Router`, `@Security`, ecc.) e rigenerare `make swagger` quando si espone l’API pubblicamente.
- Evitare binding diretto dei payload su `models.*` negli handler: usare DTO request/response dedicati.
- Non introdurre segreti nel codice: usare env / Secrets Manager come da configurazione.
- Test: `go test ./...`; per JWT/config esistono test in `internal/auth`, `internal/config`.

## Sicurezza e ambienti

- `JWT_SECRET` deve essere robusto in produzione.
- Credenziali AWS tramite profile, variabili d’ambiente o ruolo IAM a seconda dell’ambiente.
- Le azioni admin (apply/reload/sync, run discovery) hanno **impatto reale** su AWS e sui nodi: trattare staging/prod con attenzione.

## Frontend

Questo repository è principalmente **backend**. Un frontend (es. Angular) può vivere in un altro progetto; CORS va configurato di conseguenza. L’MCP in `tools/mcp-server` serve ad accelerare lo sviluppo UI contro API locale.
