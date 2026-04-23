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

## Running

```bash
cd procyon
go run . -migrate=true
```

By default, the application reads configuration from `config/config.json`.
Use:
- `config/config.example.json` for MySQL
- `config/config.postgres.example.json` for PostgreSQL

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
