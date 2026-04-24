# Procyon

`Procyon` is a base backend template built on top of Echo.

Goals:
- keep the same layered structure,
- provide a minimal starter for new backends,
- include ready-to-use shared pieces: `config`, `logger`, `observability`, `i18n`, `middleware`, and an example `controller/service/store`.

## Structure

```text
config/
controllers/
internal/
models/
services/
static/
store/
main.go
```

## What's included

- `internal/config.go` with configuration loading and MySQL/PostgreSQL connection setup
- `internal/logger.go` with `zap` JSON logging to stdout and optional daily files
- `internal/telemetry/` with OpenTelemetry traces, OpenMetrics, health/readiness/info handlers and HTTP request logging
- `internal/authz/` with Casbin model, default policies and role helpers
- `internal/apierr/` with a shared API error envelope and Echo error handler
- `internal/validator.go`
- `internal/middleware/language.go`
- `internal/middleware/kratos_auth.go` with auth based on ORY Kratos session auth
- `internal/middleware/casbin_authz.go` with Casbin RBAC middleware
- `internal/middleware/admin_key_auth.go` for simple admin/internal endpoints
- `internal/i18n/` with a simple translation loader
- `models.HelloMessage` as an example model
- `HelloController`, `HelloService`, `HelloStore`
- `static/index.html`
- `config/config.example.json` for MySQL
- `config/config.postgres.example.json` for PostgreSQL
- `config/config.docker.json`
- `Dockerfile`, `compose.yaml`, `deploy.sh`, `prod.deploy.sh`
- SQL migrations through `goose` in `internal/migrations/`
- `scripts/generate-feature.sh` as a module skeleton generator

## Running

```bash
cd procyon
go run . -migrate=true
```

By default, the application reads configuration from `config/config.json`.
You can change the path through a flag or env:

```bash
go run . -config=config/config.postgres.example.json -migrate=true
CONFIG_PATH=config/config.docker.json go run . -migrate=true
```

Use:
- `config/config.example.json` for MySQL
- `config/config.postgres.example.json` for PostgreSQL

## Project Init

Create a new project from this template with:

```bash
go run ./cmd/procyon init
```

Or use flags for non-interactive setup:

```bash
go run ./cmd/procyon init \
  --name demo-api \
  --module github.com/acme/demo-api \
  --db postgres \
  --auth kratos-casbin \
  --out ../demo-api
```

Supported database values: `postgres`, `mysql`.
Supported auth values: `kratos-casbin`, `kratos`, `admin`, `none`.

Most important env overrides:
- `AUTH_ENABLED`, `AUTH_PROVIDER`, `AUTH_DOMAIN`
- `RBAC_ENABLED`
- `ADMIN_ENABLED`, `ADMIN_SECRET_KEY`
- `DB_DRIVER`, `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_PORT`
- `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME_SECONDS`, `DB_CONN_MAX_IDLE_TIME_SECONDS`
- `TRACE_EXPORTER`, `TRACE_OTLP_ENDPOINT`, `LOG_LEVEL`

## Optional Modules

The template can disable infrastructure modules without removing code:

```json
{
  "auth": { "enabled": false, "provider": "kratos", "domain": "" },
  "rbac": { "enabled": false },
  "admin": { "enabled": false, "secret_key": "" }
}
```

Rules:
- `auth.enabled=false` does not register Kratos-protected routes under `/v1`
- `rbac.enabled=false` keeps auth without Casbin checks
- `admin.enabled=false` does not register `/admin/*` routes protected by `X-Admin-Key`
- `rbac.enabled=true` requires `auth.enabled=true`

## Migrations

By default, `-migrate=true` runs versioned SQL migrations through `goose` from:
- `internal/migrations/mysql/`
- `internal/migrations/postgres/`

Applied migrations are stored in the `schema_migrations` table. You can change the table name with `database.migrations_table` and the directory with `database.migrations_dir`.
For fast prototypes, you can switch back to GORM `AutoMigrate`:

```json
{
  "database": {
    "disable_versioned_migrations": true
  }
}
```

`AutoMigrate` is not "bad" because it deletes data. GORM generally does not drop columns or tables automatically. The problem is different: you do not get an explicit, versioned schema history, rollbacks, environment status, or control over difficult changes such as column renames, backfills, table splits, separately created indexes, and constraint changes. That is why the production path uses `goose`, while `AutoMigrate` remains a quick prototype mode.

## Module Generator

```bash
scripts/generate-feature.sh invoice
scripts/generate-feature.sh invoice --with-wiring
```

The generator creates skeletons:
- `models/invoice_*`
- `store/invoiceStore.go`
- `services/invoiceService.go`
- `controllers/invoiceController.go`
- goose migrations for MySQL and PostgreSQL

With `--with-wiring`, the generator also wires `store.AppStore` and `services.AppService`.
After generation, manually register the controller and routes in `main.go`, then review the generated migrations.

## API Errors

Errors use a shared envelope:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "validation failed"
  },
  "request_id": "..."
}
```

Use `internal/apierr` in controllers and middleware instead of ad hoc `{"error": "..."}` responses.

## Docker

Local Docker runtime uses `config/config.docker.json`.
This example compose is the MySQL variant.

```bash
./deploy.sh
```

What it does:
- builds `build/procyon-server` locally
- starts `compose.yaml`
- runs MySQL in Docker and the API on `http://localhost:8081`

Notes:
- Docker MySQL is exposed on host port `3307`
- `auth_domain` in Docker config points to `http://host.docker.internal:4433`, so public endpoints work without Kratos, but protected endpoints require Kratos reachable from the container

## Production Deploy Example

Example:

```bash
cp .env.example .env
```

Then adjust `.env` and run:

```bash
set -a
source .env
set +a

./prod.deploy.sh --with-config
```

What it does:
- builds the Linux binary locally
- uploads the binary, `Dockerfile`, `compose.yaml`, `.dockerignore` and `static/`
- optionally uploads the selected runtime config
- restarts `docker compose` on the server

## Endpoints

- `GET /health` - demo endpoint from the example feature
- `GET /healthz`
- `GET /readyz`
- `GET /info`
- `GET /metrics`
- `GET /hello`
- `GET /v1/hello` - endpoint protected by Kratos session auth + Casbin `hello:read`
- `GET /v1/admin/hello` - endpoint protected by Kratos session auth + Casbin `hello:manage`
- `GET /admin/ping` - endpoint protected by `X-Admin-Key`

## Operational defaults

- logs are JSON and go to stdout by default, which is suitable for Docker, Alloy and Loki
- file logging is optional through `logging.file_enabled=true`
- HTTP requests are traced and logged with `request_id`, `trace_id` and `span_id`
- metrics expose a real latency histogram with exemplars
- OTLP trace export supports `log`, `none` and `otlp_grpc`
- RBAC role is resolved from Kratos `identity.metadata_public.role` or `identity.traits.role`, with `user` as default

## What to replace in a new project

1. `app_name` and the module name in `go.mod`
2. domain models
3. the example `hello` feature
4. DB and auth configuration
5. the static landing page
