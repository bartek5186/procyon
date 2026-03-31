# Procyon

`Procyon` to bazowy template backendu w oparciu o Echo

Cel:
- zachować ten sam układ warstw,
- dać minimalny starter do nowych backendów,
- mieć gotowe elementy wspólne: `config`, `logger`, `i18n`, `middleware`, przykładowy `controller/service/store`.

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

- `internal/config.go` z ładowaniem konfiguracji i połączeniem MySQL
- `internal/logger.go` z loggerem JSON do pliku i stdout
- `internal/validator.go`
- `internal/middleware/language.go`
- `internal/middleware/auth.go` z auth opartym o ORY Kratos
- `internal/i18n/` z prostym loaderem tłumaczeń
- `models.HelloMessage` jako przykładowy model
- `HelloController`, `HelloService`, `HelloStore`
- `static/index.html`

## Uruchomienie

```bash
cd base
go run . -migrate=true
```

Domyślnie aplikacja czyta konfigurację z `config/config.json`.

## Endpointy

- `GET /health`
- `GET /hello`
- `GET /v1/hello` - endpoint zabezpieczony przez Kratos session auth

## Co podmienić w nowym projekcie

1. `app_name` i moduł w `go.mod`
2. modele domenowe
3. przykładowy `hello` feature
4. konfigurację DB i auth
5. statyczną stronę startową
