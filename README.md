# Procyon

`Procyon` is a base backend template built on top of Echo.

Goals:
- keep the same layered structure,
- provide a minimal starter for new backends,
- include ready-to-use shared pieces: `config`, `logger`, `i18n`, `middleware`, and an example `controller/service/store`.

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

- `internal/config.go` with configuration loading and MySQL connection setup
- `internal/logger.go` with a JSON logger writing to both file and stdout
- `internal/validator.go`
- `internal/middleware/language.go`
- `internal/middleware/auth.go` with auth based on ORY Kratos
- `internal/i18n/` with a simple translation loader
- `models.HelloMessage` as an example model
- `HelloController`, `HelloService`, `HelloStore`
- `static/index.html`

## Running

```bash
cd base
go run . -migrate=true
```

By default, the application reads configuration from `config/config.json`.

## Endpoints

- `GET /health`
- `GET /hello`
- `GET /v1/hello` - endpoint protected by Kratos session auth

## What to replace in a new project

1. `app_name` and the module name in `go.mod`
2. domain models
3. the example `hello` feature
4. DB and auth configuration
5. the static landing page
