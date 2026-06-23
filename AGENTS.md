# AGENTS.md — Caddy Dashboard

Documento di contesto per assistenti AI che lavorano su questo repository.

## Scopo del progetto

**Caddy Dashboard** è un backend API in **Go** (framework **Gin**) per gestire nodi **Caddy** su cloud (principalmente **AWS**, con estensioni **GCP** e **Azure**): registrazione manuale o discovery (EC2, SSM, IP statici, label GCP, tag Azure), operazioni su Caddy tramite **AWS SSM Run Command**, **GCP VM Manager / OS Config**, **Azure VM Run Command**, oltre a **SSH** e **admin HTTP** diretto, persistenza di nodi, regole di discovery, snapshot di configurazione e utenti in **MariaDB/MySQL** (driver GORM `mysql`), autenticazione **JWT** con ruoli `admin` e `user`, più **API key** scoped per chiamate M2M (es. registrazione upstream da EC2 al boot).

Modulo Go: `github.com/nicholasricci/caddy-dashboard` · Go **1.26**.

## Stack tecnico

| Area | Tecnologia |
|------|------------|
| HTTP API | Gin, middleware CORS e logging |
| Auth | JWT (`github.com/golang-jwt/jwt/v5`), bcrypt per password utente; API key M2M (`cdk_live_…`, SHA-256 in DB, scope `register_upstream` / `register_domain`) |
| Database | GORM + `gorm.io/driver/mysql` (DSN TCP MariaDB/MySQL) |
| AWS | AWS SDK v2: EC2, SSM, Secrets Manager (config supporta ARN segreti in YAML) |
| GCP / Azure | OS Config (guest policy) e Azure Run Command per esecuzione remota opzionale; credenziali via ADC / DefaultAzureCredential |
| Config | Viper (`configs/config.yaml`) + variabili d’ambiente; `godotenv` carica `.env` in locale |
| Log | Zap (`go.uber.org/zap`) |
| Scheduler | `github.com/robfig/cron/v3`: task schedulati via cron expression con lock MySQL per multi-replica, per-task re-entrancy guard, tabelle `scheduled_tasks` e `scheduled_task_logs`, hot-reload su mutazioni admin |
| API docs | Swaggo: annotazioni su handler, generazione in `docs/`; UI su `/swagger/index.html`, JSON su `/swagger/doc.json` |
| Dev tooling | Server MCP opzionale in `tools/mcp-server/` (solo sviluppo, chiamate HTTP “sicure”) |

## Layout del repository

```
cmd/server/          # Entrypoint HTTP, wiring di config, DB, AWS, GCP/Azure, handler, router
configs/             # config.yaml (server, auth, aws, gcp, azure, database, observability)
docs/                # Swagger generato (swagger.json, swagger.yaml, docs.go) — non editare a mano
internal/api/handlers/   # Handler Gin per auth, health, nodes, discovery, caddy, users, api-keys, upstream/domain-profiles, register-upstream/register-domain
internal/api/middleware/ # Auth JWT, API key M2M, RequireAdmin, CORS, request logger
internal/api/routes/     # Registrazione route e gruppi protected/admin
internal/auth/       # Servizio JWT, generazione/hash API key
internal/aws/        # Client multi-regione, EC2, SSM, Secrets
internal/caddy/      # Esecuzione comandi Caddy via dispatcher (SSM, SSH, HTTP admin, GCP OS Config, Azure Run Command), snapshot
internal/config/     # Load configurazione (Viper + env)
internal/database/   # Connessione GORM, AutoMigrate
internal/models/     # Entità GORM (CaddyNode, DiscoveryConfig, CaddySnapshot, User, APIKey, …)
internal/repository/ # Accesso dati
internal/scheduler/  # Scheduler engine cron, task runner generico, lock MySQL
internal/services/   # Logica di dominio (node, discovery, caddy, user, api_key, register_upstream, register_domain)
internal/tasks/      # Task runner per scheduled tasks (discovery_run, token_cleanup, node_healthcheck, upstream_healthcheck)
pkg/logger/          # Costruzione logger Zap
scripts/             # Script operativi (es. ec2-register-upstream.sh per user-data EC2)
tools/mcp-server/    # Server MCP Node/TS per Cursor (dev only)
```

## Configurazione

- File principale: [`configs/config.yaml`](configs/config.yaml) (porta, `gin_mode`, CORS, TTL JWT, regioni AWS, cache Caddy, DSN DB, log level, scheduler).
- Variabili d’ambiente: vedi [`.env.example`](.env.example). Rilevanti: `SERVER_PORT`, `DB_*`, `AWS_PROFILE`, `AWS_REGIONS`, **`AWS_OPTIONAL`** (consente avvio senza regioni AWS), **`GCP_ENABLED`**, **`GCP_OSCONFIG_TIMEOUT`**, **`AZURE_ENABLED`**, **`AZURE_RUN_COMMAND_TIMEOUT`**, `CADDY_*` (cache, timeout SSH/HTTP admin, ecc.), `JWT_SECRET` (**obbligatorio**), `LOG_LEVEL`, `GIN_MODE`, `SCHEDULER_ENABLED`, `SCHEDULER_LOG_RETENTION_DAYS`, `SCHEDULER_GLOBAL_TIMEOUT`.
- Opzione sviluppo Loki/Grafana Cloud: con `docker compose --profile loki` e servizio Alloy (`docker/loki/alloy-config.alloy`) i log stdout JSON dell’API vengono spediti a Loki usando `LOKI_URL`, `LOKI_USER`, `LOKI_API_KEY`, `LOKI_TENANT_ID`, `LOKI_ENVIRONMENT`.
- Viper usa prefisso `APP_` con sostituzione `.` → `_` per override (es. variabili annidate).
- CORS: con `cors_allowed_origins` vuoto si usa `*` senza credentials; per una SPA su altra origine impostare esplicitamente (es. `http://localhost:4200`) in YAML.

**Nota:** In `config.yaml` / `.env.example` la porta DB può comparire come `5432`; il driver è **MySQL/MariaDB** — in produzione usare tipicamente **3306** se il server è MariaDB standard.

## API HTTP

- Prefisso API: **`/api/v1`**.
- Swagger UI: `http://<host>:<port>/swagger/index.html`.
- Autenticazione umana: header `Authorization: Bearer <access_token>` per route protette JWT.
- Autenticazione M2M: stesso header con API key `cdk_live_…` sulle route dedicate (non JWT).
- **Ruoli JWT:**
  - Utente autenticato: lettura nodi/discovery, snapshot propri per nodo, ecc.
  - **`admin`**: mutazioni su nodi, discovery, utenti; sync/apply/reload Caddy; run discovery; gestione API key.

Endpoint principali (dettaglio in [`internal/api/routes/routes.go`](internal/api/routes/routes.go)):

- Pubblici: `GET /api/v1/health`, `GET /api/v1/ready`, `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`.
- Protetti JWT: CRUD/list nodi e discovery in lettura.
- Protetti JWT: `POST /api/v1/auth/logout`.
- Solo admin JWT: creazione/aggiornamento/cancellazione nodi e discovery; `POST .../sync`, `/apply`, `/reload`; `GET /nodes/:id/snapshots`; `GET /discovery/:id/snapshots`; `POST /discovery/:id/run`; gestione utenti; `GET /audit`.
- Solo admin JWT: introspezione config Caddy live con `GET /nodes/:id/config/live/ids`, `GET /nodes/:id/config/live/ids/:configId`, `GET /nodes/:id/config/live/ids/:configId/upstreams`.
- Solo admin JWT: `POST /api/v1/nodes/:id/config/mutate/domains`, `POST /api/v1/nodes/:id/config/mutate/upstreams`, `POST /api/v1/nodes/:id/config/propagate`.
- Solo admin JWT: endpoint operativo `POST /api/v1/snapshots/backfill` per rilanciare on-demand il backfill `discovery_config_id` sugli snapshot legacy (idempotente, rate-limited).
- Solo admin JWT: `GET/POST /api/v1/api-keys`, `POST /api/v1/api-keys/:id/revoke`, `DELETE /api/v1/api-keys/:id`.
- Solo admin JWT: **scheduled tasks** — `GET/POST /api/v1/scheduled-tasks`, `GET/PUT/DELETE /api/v1/scheduled-tasks/:id`, `POST /api/v1/scheduled-tasks/:id/toggle`, `POST /api/v1/scheduled-tasks/:id/run-now`, `GET /api/v1/scheduled-tasks/:id/logs`. Creazione/modifica/cancellazione/toggle triggerano hot-reload dello scheduler engine. Task types supportati: `discovery_run`, `token_cleanup`, `node_healthcheck`, `upstream_healthcheck`.
- Solo admin JWT: CRUD **upstream profile** — `GET/POST /api/v1/discovery/:id/upstream-profiles`, `GET/PUT/DELETE /api/v1/upstream-profiles/:id`.
- Solo admin JWT: CRUD **domain profile** — `GET/POST /api/v1/discovery/:id/domain-profiles`, `GET/PUT/DELETE /api/v1/domain-profiles/:id`.
- **API key M2M** (scope `register_upstream`, discovery autorizzato): `POST /api/v1/discovery/:id/register-upstream` — aggiunge un dial upstream sul primo nodo Caddy raggiungibile del gruppo e propaga ai peer (lock per discovery group).
- **API key M2M** (scope `register_upstream`, profilo in `allowed_upstream_profile_ids`): `POST /api/v1/upstream-profiles/:id/register` — applica tutti i binding del profilo in una mutate+propagate atomica.
- **API key M2M** (scope `register_domain`, discovery autorizzato): `POST /api/v1/discovery/:id/register-domain` — aggiunge hostname su un `@id` Caddy sul leader del gruppo e propaga ai peer.
- **API key M2M** (scope `register_domain`, profilo in `allowed_domain_profile_ids`): `POST /api/v1/domain-profiles/:id/register` — applica i domini del body su tutti i binding del profilo in mutate+propagate atomica.

## Dominio funzionale

- **Nodo (`CaddyNode`)**: istanza/registrazione con IP privato, instance ID (opzionale a seconda del transport), `region` (obbligatoria per `aws_ssm`), campo **`transport`** (`aws_ssm`, `ssh`, `http_admin`, `gcp_osconfig`, `azure_run_command`, `inventory_only`) e **`transport_config`** (JSON: es. `base_url` per HTTP admin, `user`/`private_key_ref` per SSH, `project_id`/`zone`/`instance_name`/`label_key`/`label_value` per GCP OS Config, `subscription_id`/`resource_group`/`vm_name` per Azure). `ssm_enabled` è deprecato (derivato da `transport`). Operazioni Caddy usano un dispatcher.
- **Discovery (`DiscoveryConfig`)**: regole per trovare nodi (`aws_tag`, `aws_ssm`, `static_ip`, `gcp_labels`, `azure_tags`; `aws_cidr` non implementato). Metodi GCP/Azure richiedono credenziali cloud (ADC / DefaultAzureCredential) e `parameters` JSON documentati in Swagger.
- **Snapshot**: versioni di configurazione Caddy persistite dopo sync/apply, con scope configurabile per `DiscoveryConfig` (`node` o `group`).
- **Utenti**: username, ruolo, password hash; JWT emessi al login.
- **API key (`APIKey`)**: nome progetto, `key_prefix`, `key_hash`, `scopes` (es. `register_upstream`, `register_domain`), `allowed_discovery_config_ids`, `allowed_upstream_profile_ids`, `allowed_domain_profile_ids`, lifecycle (`expires_at`, `revoked_at`, `last_used_at`). Secret mostrato solo alla creazione.
- **Upstream profile (`UpstreamProfile`)**: configurazione standard per registrazione M2M multi-handler; `bindings` JSON (`config_id` + `port` per ogni `@id` Caddy), legato a un `discovery_config_id` (cluster proxy).
- **Domain profile (`DomainProfile`)**: configurazione standard per registrazione M2M hostname; `bindings` JSON (`config_id` + `match_indexes` per ogni `@id` Caddy), legato a un `discovery_config_id`. I domini arrivano nel body M2M.
- **Registrazione upstream EC2 (singolo handler)**: l'istanza app chiama `register-upstream` con `discovery_config_id` del **cluster Caddy proxy** (non del proprio ASG), più `config_id` (@id Caddy) e porta dal launch template. Script: [`scripts/ec2-register-upstream.sh`](scripts/ec2-register-upstream.sh).
- **Registrazione upstream EC2 (profilo)**: l'istanza passa `UPSTREAM_PROFILE_ID` + IP privata; l'API materializza tutti i dial definiti nel profilo. Script: [`scripts/ec2-register-upstream-profile.sh`](scripts/ec2-register-upstream-profile.sh).
- **Registrazione dominio M2M**: backend applicativi chiamano `register-domain` o `domain-profiles/.../register` con API key scope `register_domain`; supporta TLS automation come l'admin `mutate/domains`.

Operazioni Caddy lato macchina remota avvengono tramite il **dispatcher** (SSM, SSH, HTTP admin, GCP OS Config, Azure Run Command) a seconda del `transport` del nodo. Le letture live e i metadati derivati (`@id`, upstream) sono cache-ati in memoria per nodo con TTL configurabile (`caddy.cache_ttl` / `CADDY_CACHE_TTL`) e invalidazione su mutazioni (`apply`, `sync`, `reload`).

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

Sotto [`tools/mcp-server/`](tools/mcp-server/) c’è un server **MCP** per integrare Cursor con lo Swagger e chiamate HTTP **limitate** (GET su `/api/v1/*` + POST solo login/refresh/logout/backfill, denylist su path pericolosi incluso `register-upstream`, `register-domain`, `upstream-profiles/.../register`, `domain-profiles/.../register`, gate `CADDY_MCP_DEV=1`). **Non** espone mutazioni Caddy né registrazione M2M. **Non** è parte del deploy di produzione. Istruzioni: [`tools/mcp-server/README.md`](tools/mcp-server/README.md).

## Convenzioni per modifiche al codice

- Allinearsi a stile e strati esistenti: **handler** sottili, **services** con logica, **repository** per DB, **models** per GORM.
- Nuove route documentate con **commenti Swaggo** (`// @Summary`, `@Router`, `@Security`, ecc.) e rigenerare `make swagger` quando si espone l’API pubblicamente.
- Evitare binding diretto dei payload su `models.*` negli handler: usare DTO request/response dedicati.
- Non introdurre segreti nel codice: usare env / Secrets Manager come da configurazione.
- Test: `go test ./...`; per JWT/config esistono test in `internal/auth`, `internal/config`.

## Sicurezza e ambienti

- `JWT_SECRET` deve essere robusto in produzione.
- API key M2M: conservare il secret in Secrets Manager sull'EC2; ruotare revocando la chiave e creandone una nuova.
- Credenziali AWS tramite profile, variabili d’ambiente o ruolo IAM a seconda dell’ambiente.
- Le azioni admin e `register-upstream` / `register-domain` (mutate + propagate) hanno **impatto reale** sui nodi Caddy: trattare staging/prod con attenzione.
- **Dipendenze e vulnerabilità:** [`.github/dependabot.yml`](.github/dependabot.yml) monitora `go.mod` e GitHub Actions in modalità solo-sicurezza (`open-pull-requests-limit: 0` evita bump schedulati non critici). Abilitare una tantum su GitHub → **Settings → Code security and analysis**: **Dependabot alerts** e **Dependabot security updates** (le PR automatiche su CVE richiedono entrambi). La CI in [`.github/workflows/ci.yml`](.github/workflows/ci.yml) esegue anche `govulncheck` come controllo complementare su ogni push/PR.

## Frontend

Questo repository è principalmente **backend**. Un frontend (es. Angular) può vivere in un altro progetto; CORS va configurato di conseguenza. L’MCP in `tools/mcp-server` serve ad accelerare lo sviluppo UI contro API locale.
