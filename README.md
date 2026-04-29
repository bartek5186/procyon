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
- GORM `AutoMigrate` by default, with optional SQL migrations through `goose` in `internal/migrations/`
- `scripts/generate-feature.sh` as a module skeleton generator

## Running

```bash
cd procyon
go run . -migrate=true
```

By default, the application reads configuration from `config/config.json`.
You can change the path through a flag:

```bash
go run . -config=config/config.postgres.example.json -migrate=true
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

Runtime configuration lives in JSON files. The application does not apply environment variable overrides for app, auth, database, logging, or observability settings.

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

By default, `-migrate=true` runs GORM `AutoMigrate`, because `database.auto_migrate` is `true`.

When the project has production data and needs explicit schema history, switch to versioned SQL migrations by setting:

```json
{
  "database": {
    "auto_migrate": false
  }
}
```

Then `-migrate=true` runs `goose` migrations embedded from:
- `internal/migrations/mysql/`
- `internal/migrations/postgres/`

Applied goose migrations are stored in the `schema_migrations` table. You can change the table name with `database.migrations_table` and the directory with `database.migrations_dir`.

`AutoMigrate` is a good default for early development. It is not "bad" because it deletes data; GORM generally does not drop columns or tables automatically. The limitation is that you do not get an explicit, versioned schema history, rollbacks, environment status, or control over difficult changes such as column renames, backfills, table splits, separately created indexes, and constraint changes.

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

Create a production JSON config outside git, for example `config/config.prod.json`, then upload it during deploy:

```bash
CONFIG_SOURCE=config/config.prod.json REMOTE=deploy@example.com REMOTE_DIR=/srv/apps/procyon ./prod.deploy.sh --with-config
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
