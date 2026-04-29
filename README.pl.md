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
- `internal/apierr/` ze wspólnym formatem błędów API i Echo error handlerem
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
- domyślny GORM `AutoMigrate` i opcjonalne migracje SQL przez `goose` w `internal/migrations/`
- `scripts/generate-feature.sh` jako generator szkieletu modułu

## Uruchomienie

```bash
cd procyon
go run . -migrate=true
```

Domyślnie aplikacja czyta konfigurację z `config/config.json`.
Ścieżkę można zmienić przez flagę:

```bash
go run . -config=config/config.postgres.example.json -migrate=true
```

Punkt startowy dla nowego serwisu:
- `config/config.example.json` dla MySQL
- `config/config.postgres.example.json` dla PostgreSQL

## Inicjalizacja Projektu

Nowy projekt z tego template'u utworzysz przez:

```bash
cd procyon-cli
go run . init
```

Albo przez flagi w trybie nieinteraktywnym:

```bash
cd procyon-cli
go run . init \
  --name demo-api \
  --module github.com/acme/demo-api \
  --db postgres \
  --auth kratos-casbin \
  --out ../demo-api
```

Obsługiwane bazy: `postgres`, `mysql`.
Obsługiwane tryby auth: `kratos-casbin`, `kratos`, `admin`, `none`.

Runtime config jest w plikach JSON. Aplikacja nie nakłada override'ów ze zmiennych środowiskowych dla ustawień app, auth, bazy, logowania ani obserwowalności.

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

Domyślnie `-migrate=true` uruchamia GORM `AutoMigrate`, bo `database.auto_migrate` ma wartość `true`.

Kiedy projekt ma już produkcyjne dane i potrzebuje jawnej historii schematu, przełącz się na wersjonowane migracje SQL:

```json
{
  "database": {
    "auto_migrate": false
  }
}
```

Wtedy `-migrate=true` uruchamia migracje `goose` embedowane z:
- `internal/migrations/mysql/`
- `internal/migrations/postgres/`

Wykonane migracje goose są zapisywane w tabeli `schema_migrations`. Nazwę tabeli można zmienić przez `database.migrations_table`, a katalog przez `database.migrations_dir`.

`AutoMigrate` jest dobrym defaultem na wczesnym etapie projektu. Nie jest "złe" dlatego, że kasuje dane; GORM zwykle nie usuwa kolumn ani tabel automatycznie. Ograniczenie jest inne: nie masz jawnej, wersjonowanej historii zmian schematu, rollbacków, statusu środowiska ani kontroli nad trudnymi zmianami typu rename kolumny, backfill, split tabeli, indeksy tworzone osobno, zmiany constraintów.

## Generator Modułu

```bash
scripts/generate-feature.sh invoice
scripts/generate-feature.sh invoice --with-wiring
```

Generator tworzy szkielety:
- `models/invoice_*`
- `store/invoiceStore.go`
- `services/invoiceService.go`
- `controllers/invoiceController.go`
- migracje goose dla MySQL i PostgreSQL

Z `--with-wiring` generator podpina też `store.AppStore` i `services.AppService`.
Po wygenerowaniu trzeba ręcznie zarejestrować kontroler i routing w `main.go`, a potem sprawdzić wygenerowane migracje.

## Błędy API

Błędy używają wspólnego formatu:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "validation failed"
  },
  "request_id": "..."
}
```

W controllerach i middleware używaj `internal/apierr` zamiast lokalnych odpowiedzi typu `{"error": "..."}`.

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

Utwórz produkcyjny JSON config poza gitem, np. `config/config.prod.json`, a potem wyślij go podczas deploya:

```bash
CONFIG_SOURCE=config/config.prod.json REMOTE=deploy@example.com REMOTE_DIR=/srv/apps/procyon ./prod.deploy.sh --with-config
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
- metryki są zapisywane przez instrumenty OpenTelemetry i wystawiane na endpointzie `/metrics` zgodnym z Prometheusem
- kod biznesowy może zapisywać zdarzenia domenowe przez `telemetry.BusinessMetrics`
- eksport trace wspiera `log`, `none` i `otlp_grpc`; eksport metryk OTLP wspiera `none` i `otlp_grpc`
- rola RBAC jest brana z Kratos `identity.metadata_public.role` albo `identity.traits.role`, a domyślnie ustawiany jest `user`

Zobacz [METRICS.md](METRICS.md), żeby sprawdzić konfigurację metryk i przykłady metryk biznesowych.

## Co podmienić w nowym projekcie

1. `app_name` i moduł w `go.mod`
2. modele domenowe
3. przykładowy `hello` feature
4. konfigurację DB i auth
5. statyczną stronę startową
