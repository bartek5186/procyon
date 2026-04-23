# Procyon

`Procyon` to bazowy template backendu w oparciu o Echo

Cel:
- zachować ten sam układ warstw,
- dać minimalny starter do nowych backendów,
- mieć gotowe elementy wspólne: `config`, `logger`, `observability`, `i18n`, `middleware`, przykładowy `controller/service/store`.

## Struktura

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

## Co jest gotowe

- `internal/config.go` z ładowaniem konfiguracji i połączeniem MySQL/PostgreSQL
- `internal/logger.go` z `zap`, JSON na stdout i opcjonalnym zapisem do pliku
- `internal/telemetry/` z OpenTelemetry, OpenMetrics, health/readiness/info i logami requestów HTTP
- `internal/authz/` z Casbinem, domyślnym modelem polityk i helperami ról
- `internal/validator.go`
- `internal/middleware/language.go`
- `internal/middleware/kratos_auth.go` z auth opartym o ORY Kratos
- `internal/middleware/casbin_authz.go` z middleware RBAC opartym o Casbin
- `internal/middleware/admin_key_auth.go` dla prostych endpointów admin/internal
- `internal/i18n/` z prostym loaderem tłumaczeń
- `models.HelloMessage` jako przykładowy model
- `HelloController`, `HelloService`, `HelloStore`
- `static/index.html`
- `config/config.example.json` dla MySQL
- `config/config.postgres.example.json` dla PostgreSQL
- `config/config.docker.json`
- `Dockerfile`, `compose.yaml`, `deploy.sh`, `prod.deploy.sh`

## Uruchomienie

```bash
cd procyon
go run . -migrate=true
```

Domyślnie aplikacja czyta konfigurację z `config/config.json`.
Punkt startowy dla nowego serwisu:
- `config/config.example.json` dla MySQL
- `config/config.postgres.example.json` dla PostgreSQL

## Docker

Lokalny runtime Dockerowy używa `config/config.docker.json`.
Ten przykładowy compose jest wariantem mysqlowym.

```bash
./deploy.sh
```

Co robi skrypt:
- buduje lokalnie `build/procyon-server`
- uruchamia `compose.yaml`
- stawia MySQL w Dockerze i API na `http://localhost:8081`

Uwagi:
- MySQL z Dockera jest wystawiony na porcie hosta `3307`
- `auth_domain` w configu dockerowym wskazuje na `http://host.docker.internal:4433`, więc publiczne endpointy działają bez Kratosa, ale zabezpieczone wymagają Kratosa osiągalnego z kontenera

## Przykładowy Deploy Produkcyjny

Przykład:

```bash
cp .env.example .env
```

Potem uzupełnij `.env` i uruchom:

```bash
set -a
source .env
set +a

./prod.deploy.sh --with-config
```

Co robi skrypt:
- buduje lokalnie binarkę Linuksową
- wysyła binarkę, `Dockerfile`, `compose.yaml`, `.dockerignore` i `static/`
- opcjonalnie wysyła wybrany runtime config
- restartuje `docker compose` na serwerze

## Endpointy

- `GET /health` - przykładowy endpoint z feature demo
- `GET /healthz`
- `GET /readyz`
- `GET /info`
- `GET /metrics`
- `GET /hello`
- `GET /v1/hello` - endpoint zabezpieczony przez Kratos session auth + Casbin `hello:read`
- `GET /v1/admin/hello` - endpoint zabezpieczony przez Kratos session auth + Casbin `hello:manage`
- `GET /admin/ping` - endpoint zabezpieczony przez `X-Admin-Key`

## Domyślne standardy operacyjne

- logi są w JSON i domyślnie lecą na stdout, więc nadają się pod Docker, Alloy i Loki
- zapis do plików jest opcjonalny przez `logging.file_enabled=true`
- requesty HTTP są logowane z `request_id`, `trace_id` i `span_id`
- metryki wystawiają prawdziwy histogram latency z exemplarami
- eksport trace wspiera `log`, `none` i `otlp_grpc`
- rola RBAC jest brana z Kratos `identity.metadata_public.role` albo `identity.traits.role`, a domyślnie ustawiany jest `user`

## Co podmienić w nowym projekcie

1. `app_name` i moduł w `go.mod`
2. modele domenowe
3. przykładowy `hello` feature
4. konfigurację DB i auth
5. statyczną stronę startową
