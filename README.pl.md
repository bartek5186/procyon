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
plugins/
plugins_local.go
plugins_gen.go
services/
static/
store/
main.go
```

## Co jest gotowe

- wersjonowana zależność `github.com/bartek5186/procyon-core` z konfiguracją,
  bazą, loggerem, telemetry, błędami API, walidacją, auth i middleware
- role i polityki Casbina należące do aplikacji w `internal/authz/`
- `internal/i18n/` z prostym loaderem tłumaczeń
- `models.HelloMessage` jako przykładowy model
- `HelloController`, `HelloService`, `HelloStore`
- `static/index.html`
- `config/config.example.json` dla MySQL
- `config/config.postgres.example.json` dla PostgreSQL
- `config/config.docker.json`
- `Dockerfile`, `compose.yaml`, `deploy.sh`, `prod.deploy.sh`
- domyślny GORM `AutoMigrate` i opcjonalne migracje SQL przez `goose` w `internal/migrations/`
- generowanie modułów aplikacji przez `procyon-cli module create`

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

## Aktualizacje Procyon

Samo CLI można zaktualizować z dowolnego katalogu:

```bash
procyon-cli self-update
```

Wspólna infrastruktura pochodzi z wersjonowanego modułu Go `procyon-core`.
W katalogu aplikacji uruchom:

```bash
procyon-cli core update
```

Polecenie Core aktualizuje zależność oraz uruchamia `go mod tidy` i
`go test ./...`.
Routing, kod domenowy, polityki i migracje aplikacji nie są nadpisywane.
Stare `procyon-cli update` pozostaje przestarzałym aliasem wyłącznie dla
aktualizacji Core.

## Typowane zdarzenia modułów

Core tworzy jeden typowany event bus i przekazuje go aplikacji oraz wszystkim
pluginom projektowym i zainstalowanym. Pluginy rejestrują handlery przez
`RegisterEvents`, a handlery należące do aplikacji są składane w `events.go` i
zwracane przez fabrykę aplikacji w `main.go`. Rejestracja zostaje zamknięta przed
uruchomieniem tras pluginów i zadań działających w tle.

Bus jest synchroniczny i celowo nie ma kolejki ani workera. Handlery muszą być
szybkie i idempotentne. Błąd handlera przerywa publikację, dzięki czemu trwałe
źródło, takie jak webhook płatności, może ponowić całe zdarzenie.

Runtime odpowiada za utworzenie busa, kolejność rejestracji i wywołanie `Seal`.
Dokładny lifecycle opisuje [dokumentacja
Core](https://github.com/bartek5186/procyon-core/tree/main/events).

## Pluginy Wewnętrzne Projektu

Prywatny plugin kompilowany razem z bieżącą aplikacją utworzysz przez:

```bash
procyon-cli plugin create leagues
```

Polecenie tworzy `plugins/leagues/` bez osobnego `go.mod` i manifestu modułu
publicznego oraz dopisuje fabrykę w `plugins_local.go`. Zewnętrzne pluginy nadal
są generowane w `plugins_gen.go`. Obie listy przechodzą przez jeden registry i
ten sam lifecycle konfiguracji, migracji, capabilities, eventów, polityk, tras,
workerów i shutdownu w odwrotnej kolejności.

Plugin deklaruje zależności przez `Requires`. Start odrzuca brakujące zależności,
cykle, powtórzone nazwy i nadpisanie istniejącej trasy. Synchroniczne porty są
udostępniane przez `plugins.Provide` i pobierane przez `plugins.Resolve`, a fakty,
które już wystąpiły, powinny korzystać z typowanego event busa.

Wpis `plugins.<nazwa>.enabled=false` w wybranym configu pomija zarejestrowany
plugin. Pluginy projektowe nie trafiają do `.procyon.json`, publicznego katalogu
modułów ani do `go.mod`.

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

Zainstalowane pluginy dostarczają konfigurację w osobnej warstwie
`config/plugins.generated.json`. Procyon Core składa ją z wybranym głównym
configiem. Jawny wpis `plugins.<nazwa>` w głównym configu ma pierwszeństwo przed
wygenerowaną wartością domyślną. Sekrety pluginu mogą nadal pochodzić ze
zmiennych środowiskowych zadeklarowanych w manifeście pluginu.

## Moduły Opcjonalne

Template pozwala wyłączyć moduły infrastrukturalne bez usuwania kodu:

```json
{
  "auth": { "enabled": false, "provider": "kratos", "domain": "" },
  "rbac": { "enabled": false, "default_role": "user", "admin_identity_ids": [] },
  "admin": { "enabled": false, "secret_key": "" }
}
```

Zasady:
- `auth.enabled=false` nie rejestruje tras wymagających sesji Kratos pod `/v1`
- `rbac.enabled=false` zostawia auth bez sprawdzania Casbina
- `admin.enabled=false` nie rejestruje tras `/admin/*` chronionych `X-Admin-Key`
- `rbac.enabled=true` wymaga `auth.enabled=true`
- Kratos daje do RBAC tylko identity ID; role i polityki użytkownika są trzymane w Casbin/app DB
- `rbac.default_role` jest nadawane nowej zalogowanej identity, jeśli nie ma jeszcze roli w Casbinie
- `rbac.admin_identity_ids` bootstrapuje początkowych adminów w Casbinie

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
procyon-cli module create invoice
```

Generator Procyon CLI tworzy:
- `models/invoice_*`
- `store/invoiceStore.go`
- `services/invoiceService.go`
- `controllers/invoiceController.go`
- migracje goose dla MySQL i PostgreSQL

Automatycznie podpina też `store.AppStore`, `services.AppService`, aplikację,
routing, automigrację i domyślne polityki. Po wygenerowaniu sprawdź logikę i
migracje, a następnie uruchom `go test ./...`.

## Kolekcja Postmana

Kolekcję aplikacji wygenerujesz przez:

```bash
procyon-cli postman generate
```

Wygenerowaną kolekcję można zsynchronizować ze wszystkimi targetami Postmana
zdefiniowanymi w `.env` projektu:

```bash
procyon-cli postman sync
```

Skrypt kompatybilności uruchamia tę samą synchronizację (a gdy nie ma targetów,
tylko generuje lokalny plik):

```bash
./gen-postman.sh
```

Jeden klucz API może obsłużyć dowolną liczbę kolekcji, a wybrany target może
mieć własny klucz:

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

CLI wykrywa każdy dodatni sufiks liczbowy, więc nie ma limitu dwóch ani ośmiu
targetów, a indeksy mogą mieć przerwy. Target numerowany korzysta ze wspólnego
`POSTMAN_API_KEY`, chyba że ma własny `POSTMAN_API_KEY_N`. Stara nazwa
`POSTMAN_API_COLLECTION_ID_N` nadal działa jako alias
`POSTMAN_COLLECTION_ID_N`. Niepełny target przerywa synchronizację z czytelnym
błędem. Zmienne procesu mają pierwszeństwo przed `.env`, flagi przed obiema
warstwami, a klucze API nie są wypisywane.

Generator jest wersjonowany razem z `procyon-cli`, dzięki czemu starsze projekty
dostają jego poprawki po aktualizacji CLI i nie potrzebują lokalnego katalogu
generatora. Skanuje routing aplikacji, pluginy projektowe w `plugins/`
oraz wszystkie zainstalowane pluginy Go zapisane w `.procyon.json`. Trasy
pluginów są odczytywane z ich metod
`RegisterRoutes`, umieszczane w głównym folderze nazwanym jak plugin i zachowują
właściwy tryb dostępu: publiczny, bearer albo admin.

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

W controllerach i middleware używaj `github.com/bartek5186/procyon-core/apierr` zamiast lokalnych odpowiedzi typu `{"error": "..."}`.

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
- RBAC używa Kratos identity ID oraz ról/polityk Casbina trzymanych w app DB; pola roli w Kratos traits albo metadata nie są traktowane jako zaufane

Zobacz [docs/METRICS.md](docs/METRICS.md), żeby sprawdzić konfigurację metryk i przykłady metryk biznesowych.
Zobacz [docs/ROLES.md](docs/ROLES.md), żeby sprawdzić konfigurację RBAC Kratos/Casbin i przykłady ról.
Zobacz [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), żeby sprawdzić reguły architektury projektu.

## Co podmienić w nowym projekcie

1. `app_name` i moduł w `go.mod`
2. modele domenowe
3. przykładowy `hello` feature
4. konfigurację DB i auth
5. statyczną stronę startową
