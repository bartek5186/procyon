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
- migracje SQL przez `goose` w `internal/migrations/`
- `scripts/generate-feature.sh` jako generator szkieletu modułu

## Uruchomienie

```bash
cd procyon
go run . -migrate=true
```

Domyślnie aplikacja czyta konfigurację z `config/config.json`.
Ścieżkę można zmienić przez flagę albo env:

```bash
go run . -config=config/config.postgres.example.json -migrate=true
CONFIG_PATH=config/config.docker.json go run . -migrate=true
```

Punkt startowy dla nowego serwisu:
- `config/config.example.json` dla MySQL
- `config/config.postgres.example.json` dla PostgreSQL

Najważniejsze override'y env:
- `AUTH_ENABLED`, `AUTH_PROVIDER`, `AUTH_DOMAIN`
- `RBAC_ENABLED`
- `ADMIN_ENABLED`, `ADMIN_SECRET_KEY`
- `DB_DRIVER`, `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_PORT`
- `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME_SECONDS`, `DB_CONN_MAX_IDLE_TIME_SECONDS`
- `TRACE_EXPORTER`, `TRACE_OTLP_ENDPOINT`, `LOG_LEVEL`

## Moduły Opcjonalne

Template pozwala wyłączyć moduły infrastrukturalne bez usuwania kodu:

```json
{
  "auth": { "enabled": false, "provider": "kratos", "domain": "" },
  "rbac": { "enabled": false },
  "admin": { "enabled": false, "secret_key": "" }
}
```

Zasady:
- `auth.enabled=false` nie rejestruje tras wymagających sesji Kratos pod `/v1`
- `rbac.enabled=false` zostawia auth bez sprawdzania Casbina
- `admin.enabled=false` nie rejestruje tras `/admin/*` chronionych `X-Admin-Key`
- `rbac.enabled=true` wymaga `auth.enabled=true`

## Migracje

Domyślnie `-migrate=true` uruchamia wersjonowane migracje SQL przez `goose` z:
- `internal/migrations/mysql/`
- `internal/migrations/postgres/`

Wykonane migracje są zapisywane w tabeli `schema_migrations`. Nazwę tabeli można zmienić przez `database.migrations_table`, a katalog przez `database.migrations_dir`.
Dla szybkich prototypów można wrócić do GORM `AutoMigrate`:

```json
{
  "database": {
    "disable_versioned_migrations": true
  }
}
```

`AutoMigrate` nie jest "złe" dlatego, że kasuje dane. GORM zwykle nie usuwa kolumn ani tabel automatycznie. Problem jest inny: nie masz jawnej, wersjonowanej historii zmian schematu, rollbacków, statusu środowiska ani kontroli nad trudnymi zmianami typu rename kolumny, backfill, split tabeli, indeksy tworzone osobno, zmiany constraintów. Dlatego w produkcyjnej ścieżce template używa `goose`, a `AutoMigrate` zostaje jako szybki tryb prototypowy.

## Generator Modułu

```bash
scripts/generate-feature.sh invoice
```

Generator tworzy szkielety:
- `models/invoice_*`
- `store/invoiceStore.go`
- `services/invoiceService.go`
- `controllers/invoiceController.go`

Po wygenerowaniu trzeba ręcznie podpiąć moduł w `store.AppStore`, `services.AppService`, `main.go` i migracjach.

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
