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
plugins/
plugins_local.go
plugins_gen.go
policies.go
services/
static/
store/
main.go
```

## What's included

- versioned `github.com/bartek5186/procyon-core` dependency with configuration,
  database, logging, telemetry, API errors, validation, auth and middleware
- application-owned Casbin policies in `policies.go`
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

## Procyon Updates

Update the CLI itself from any directory:

```bash
procyon-cli self-update
```

Shared infrastructure is supplied by the versioned `procyon-core` Go module.
Update it from an application directory with:

```bash
procyon-cli core update
```

The Core command updates the dependency and runs `go mod tidy` plus
`go test ./...`.
Application routes, domain code, policies and migrations are not overwritten.
The legacy `procyon-cli update` command remains a deprecated alias for the Core
update only.

## Typed Module Events

The application creates one typed event bus and shares it with every local and
installed plugin. Plugins register handlers through `RegisterEvents`;
application-owned
handlers belong in `events.go`. Registration is sealed before plugin routes and
background tasks start.

The bus is synchronous and intentionally has no queue or worker. Handlers must
be fast and idempotent. If a handler fails, the publishing operation fails so a
durable source such as a payment webhook can retry it.

Projects generated before typed events need a one-time wiring update in
`app.go` and `plugins.go`: create the bus, pass it as
`plugins.Dependencies.Events`, register application handlers and call `Seal`
before `registerPublicRoutes`. See the
[Core event guide](https://github.com/bartek5186/procyon-core/tree/main/events)
for the exact lifecycle.

## Project-owned Plugins

Create a private plugin compiled as part of the current application:

```bash
procyon-cli plugin create leagues
```

The command creates `plugins/leagues/` without a nested `go.mod` or public
module manifest and wires its factory into `plugins_local.go`. Installed shared
plugins remain generated in `plugins_gen.go`. Both lists are composed into one
registry and use the same configuration, migrations, capabilities, events,
policies, routes, workers and reverse-order shutdown lifecycle.

Plugins can declare dependencies with `Requires`. Startup rejects missing
dependencies, cycles, duplicate names and route overwrites. A plugin can expose
synchronous typed ports with `plugins.Provide` and consume them with
`plugins.Resolve`; facts that already happened should use the typed event bus.

Set `plugins.<name>.enabled=false` in the selected application config to omit a
registered plugin. Project-owned plugins are never added to `.procyon.json`,
the public module registry or `go.mod`.

## Project Init

Create a new project from this template with:

```bash
cd procyon-cli
go run . init
```

Or use flags for non-interactive setup:

```bash
cd procyon-cli
go run . init \
  --name demo-api \
  --module github.com/acme/demo-api \
  --db postgres \
  --auth kratos-casbin \
  --out ../demo-api
```

Supported database values: `postgres`, `mysql`.
Supported auth values: `kratos-casbin`, `kratos`, `admin`, `none`.

Runtime configuration lives in JSON files. The application does not apply environment variable overrides for app, auth, database, logging, or observability settings.

Installed plugins contribute namespaced defaults through
`config/plugins.generated.json`. Procyon Core composes that generated layer with
the selected base config. An explicit `plugins.<name>` entry in the base config
takes precedence over the generated default. Plugin-owned credentials may still
be read from environment variables declared by the plugin manifest.

## Optional Modules

The template can disable infrastructure modules without removing code:

```json
{
  "auth": { "enabled": false, "provider": "kratos", "domain": "" },
  "rbac": { "enabled": false, "default_role": "user", "admin_identity_ids": [] },
  "admin": { "enabled": false, "secret_key": "" }
}
```

Rules:
- `auth.enabled=false` does not register Kratos-protected routes under `/v1`
- `rbac.enabled=false` keeps auth without Casbin checks
- `admin.enabled=false` does not register `/admin/*` routes protected by `X-Admin-Key`
- `rbac.enabled=true` requires `auth.enabled=true`
- Kratos provides only the identity ID for RBAC; user roles and policies are stored in Casbin/app DB
- `rbac.default_role` is assigned to new authenticated identities when they have no Casbin role yet
- `rbac.admin_identity_ids` bootstraps initial admin identities in Casbin

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

## Postman Collection

Generate the application collection with:

```bash
procyon-cli postman generate
```

Synchronize the generated collection with all Postman targets configured in
the project `.env`:

```bash
procyon-cli postman sync
```

The compatibility script runs this synchronization command (and still only
generates locally when no remote targets are configured):

```bash
./gen-postman.sh
```

Use one shared API key for any number of collections, or override it per target:

```dotenv
POSTMAN_API_KEY=shared_postman_api_key
POSTMAN_COLLECTION_ID=primary_collection_id
POSTMAN_TARGET_NAME=Primary

POSTMAN_COLLECTION_ID_1=staging_collection_id
POSTMAN_TARGET_NAME_1=Staging

POSTMAN_API_KEY_8=separate_account_api_key
POSTMAN_COLLECTION_ID_8=production_collection_id
POSTMAN_TARGET_NAME_8=Production
```

The CLI discovers every positive numeric suffix, so there is no two-target (or
eight-target) limit and indexes may have gaps. A numbered target falls back to
the shared `POSTMAN_API_KEY`. `POSTMAN_API_COLLECTION_ID_N` is supported as a
legacy alias for `POSTMAN_COLLECTION_ID_N`. Incomplete targets stop the sync
with a clear error rather than being skipped. Process variables override `.env`
and explicit command flags override both; API keys are never logged.

The generator is versioned with `procyon-cli`, so existing projects receive
generator fixes by updating the CLI instead of copying a new `tools/postman-gen`
directory. It scans application routes, project-owned plugins under `plugins/`,
and installed Go plugins recorded in `.procyon.json`. Plugin routes are read
from their `RegisterRoutes` methods and
placed in a top-level folder named after the plugin, preserving public,
bearer-authenticated and admin access modes.

Go documentation comments above controller handlers become the request
description displayed in Postman's **Docs** tab. Keep these comments focused on
the endpoint contract: authentication, inputs, side effects, provider-specific
behavior and stable errors.

Plugins can provide multiple named request and response variants in
`docs/postman/*.json`. The generator matches each example by `METHOD /path` and
uses it instead of the generic inferred response:

```json
{
  "module": "payment-system",
  "version": 2,
  "examples": [
    {
      "key": "POST /v1/payments/checkout",
      "name": "Stripe one-time checkout",
      "default": true,
      "request": {
        "headers": {"Idempotency-Key": "{{$guid}}"},
        "body": {"provider": "stripe", "price_id": "price_example"}
      },
      "response": {
        "status": 201,
        "body": {"checkout_url": "https://checkout.stripe.com/example"}
      }
    }
  ]
}
```

Use `default: true` for the variant copied into the main request. For a route
with a parameter, a concrete key such as
`GET /v1/payments/prices/stripe` can be paired with
`"path": {"provider": "stripe"}`. Examples must use safe placeholders and
must never contain real credentials, webhook signatures or customer data.

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

Use `github.com/bartek5186/procyon-core/apierr` in controllers and middleware instead of ad hoc `{"error": "..."}` responses.

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
- metrics are recorded with OpenTelemetry instruments and exposed on the Prometheus-compatible `/metrics` endpoint
- business code can record domain events through `telemetry.BusinessMetrics`
- OTLP trace export supports `log`, `none` and `otlp_grpc`; OTLP metrics export supports `none` and `otlp_grpc`
- RBAC uses Kratos identity ID plus Casbin roles/policies stored in the app DB; role fields in Kratos traits or metadata are not trusted

See [docs/METRICS.md](docs/METRICS.md) for metrics configuration and business metric examples.
See [docs/ROLES.md](docs/ROLES.md) for Kratos/Casbin RBAC configuration and role examples.
See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for project architecture rules.

## What to replace in a new project

1. `app_name` and the module name in `go.mod`
2. domain models
3. the example `hello` feature
4. DB and auth configuration
5. the static landing page
