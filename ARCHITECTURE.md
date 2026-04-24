# ARCHITECTURE.md

Ten dokument opisuje faktyczny styl architektury używany w tym repo i ma służyć jako instrukcja dla AI podczas dodawania lub modyfikowania kodu.

To nie jest czysta architektura heksagonalna ani DDD. To praktyczna architektura warstwowa inspirowana heksagonalnym podziałem odpowiedzialności:

`HTTP/Echo -> controllers -> services -> store -> SQL/GORM`

Do tego dochodzą:
- `models` jako wspólne struktury domenowe, DTO i mapery,
- `internal` jako kod infrastrukturalny i cross-cutting,
- background jobs uruchamiane przy starcie aplikacji,
- integracje zewnętrzne trzymane w `services`.
- opcjonalne moduły infrastrukturalne sterowane konfiguracją: auth, RBAC i admin endpoints.

## 1. Główna zasada

Każda warstwa ma wąski zakres odpowiedzialności:

- `controllers/` obsługuje HTTP, bind/validate, parametry, sesję, statusy HTTP i odpowiedzi JSON.
- `services/` zawiera logikę aplikacyjną i orkiestrację use-case.
- `store/` wykonuje operacje DB przez GORM i transakcje.
- `models/` przechowuje encje GORM, DTO wejścia/wyjścia i mapowanie danych.
- `internal/` zawiera konfigurację, logger, walidator, middleware, i18n, migracje i inne elementy infrastrukturalne.

Najważniejsza reguła: nie mieszaj odpowiedzialności między warstwami.

## 2. Struktura katalogów

```text
config/        konfiguracja JSON i pliki credentials
controllers/   handlery Echo i mapowanie HTTP <-> service
internal/      config, logger, middleware, validator, i18n, migracje, helpers
models/        encje GORM, input/output DTO, mapery
services/      use-case, logika biznesowa, integracje zewnętrzne, cron/background jobs
store/         dostęp do danych przez GORM
static/        statyczne pliki HTTP
main.go        composition root: składanie aplikacji, routing, middleware, start
```

## 3. Composition Root

`main.go` jest jedynym centralnym miejscem składania aplikacji.

Przy starcie aplikacja robi w tej kolejności:

1. ładuje translacje i konfigurację,
2. nakłada override'y z env i waliduje konfigurację,
3. tworzy logger,
4. otwiera połączenie z DB i ustawia pool,
5. inicjalizuje opcjonalne moduły, np. Casbin tylko gdy `rbac.enabled=true`,
6. uruchamia migracje, jeśli proces dostał `-migrate=true`,
7. buduje `AppStore`,
8. inicjalizuje providerów płatności, jeśli są włączone,
9. buduje `AppService`,
10. uruchamia background jobs i consumerów,
11. tworzy kontrolery,
12. zakłada middleware,
13. rejestruje trasy zależnie od przełączników modułów.

W tym projekcie nie ma kontenera DI. Zależności są przekazywane ręcznie przez konstruktory albo przypisywane podczas tworzenia `AppService`.

## 4. Rola warstw

### 4.1 `controllers/`

Kontroler:
- pracuje na `echo.Context`,
- pobiera path params, query params, body i sesję ORY,
- uruchamia `Bind()` i opcjonalnie `Validate()`,
- wykonuje auth/ownership checks, jeśli są potrzebne na poziomie endpointu,
- wywołuje jedną lub kilka metod z `appService`,
- mapuje błędy na HTTP status i JSON response.

Kontroler nie powinien:
- wykonywać zapytań do DB,
- pisać SQL/GORM,
- implementować zasad biznesowych,
- znać detali zewnętrznych providerów poza capability checkiem.

Wzorzec z repo:

```go
func (c *SomeController) SomeAction(ec echo.Context) error {
    ctx := ec.Request().Context()

    var in models.SomeInput
    if err := ec.Bind(&in); err != nil {
        return ec.JSON(http.StatusBadRequest, map[string]string{"error": "invalid payload"})
    }

    out, err := c.appService.SomeService.DoSomething(ctx, in)
    if err != nil {
        return ec.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }

    return ec.JSON(http.StatusOK, out)
}
```

### 4.2 `services/`

Serwis:
- pracuje na `context.Context`,
- nie używa `echo.Context`,
- zawiera logikę use-case,
- scala wiele store'ów i integracji,
- może wykonywać walidację biznesową,
- może zarządzać cache lokalnym w pamięci,
- może uruchamiać crony, emitery i background workers.

Serwis nie powinien:
- zwracać odpowiedzi HTTP,
- znać szczegółów routera Echo,
- robić bezpośrednio operacji DB, jeśli istnieje dla nich store,
- trzymać logiki SQL.

Akceptowalny styl w tym repo:
- serwis ma pola `Store store.Datastore` i `logger *zap.Logger`,
- logika jest pragmatyczna, bez przesadnego rozbijania na małe interfejsy,
- integracje zewnętrzne są również w `services/`, np. Stripe, Apple, Google, FCM, upload.

### 4.3 `store/`

Store:
- używa GORM,
- przyjmuje `context.Context`,
- realizuje odczyt i zapis do DB,
- używa transakcji przy wieloetapowych zapisach,
- preloaduje relacje potrzebne wyżej,
- dba o integralność danych w ramach DB.

Store nie powinien:
- znać HTTP,
- używać Echo,
- robić wywołań do Stripe/FCM/Google/Apple,
- implementować wysokopoziomowych reguł biznesowych.

W praktyce store może robić rzeczy takie jak:
- sprawdzanie istnienia rekordów,
- enforcing unikalności i relacji,
- soft delete / hard delete zależnie od przypadku,
- cursor pagination,
- `Preload`,
- `Association().Replace/Clear`,
- `Transaction(...)`.

### 4.4 `models/`

`models` nie oznacza tylko tabel DB. W tym repo ten katalog trzyma:
- encje GORM,
- input DTO,
- output DTO,
- mapery,
- czasem pomocniczą logikę transformacji.

Podział używany w repo:
- `models.go` dla encji domenowych i tabel,
- `inputs.go` dla payloadów wejściowych,
- `outputs.go` dla odpowiedzi,
- `mappers.go` dla mapowania model -> response,
- osobne pliki dla obszarów domenowych, np. ligi.

Jeśli nowy feature potrzebuje osobnych payloadów lub odpowiedzi, dodawaj je do `models/`, a nie do `controllers/`, chyba że jest to drobny, lokalny request używany tylko raz.

## 5. Agregatory: `AppService` i `Datastore`

Repo używa dwóch agregatorów:

- `store.Datastore` jako wspólny dostęp do store'ów,
- `services.AppService` jako wspólny dostęp do serwisów.

To jest świadomy kompromis:
- mniej boilerplate,
- prostsze ręczne DI,
- szybsze rozwijanie nowych modułów.

Jeśli dodajesz nowy moduł:

1. dodaj nowy store do `store/appStore.go`,
2. dodaj nowy serwis do `services/appService.go`,
3. wstrzyknij zależności w `main.go` albo w `NewAppService`,
4. udostępnij moduł z poziomu kontrolera przez `appService`.

Nie buduj osobnego kontenera ani kolejnej warstwy abstrakcji bez wyraźnej potrzeby.

Do wygenerowania początkowego szkieletu modułu można użyć:

```bash
scripts/generate-feature.sh invoice
```

Generator tworzy pliki modeli, store, service i controller, ale celowo nie edytuje automatycznie `AppStore`, `AppService`, `main.go` ani migracji. Te miejsca są composition rootem i powinny być świadomie podpięte podczas implementacji.

## 6. Typowy przepływ feature’a

Dla nowego endpointu trzymaj się poniższego flow:

1. Endpoint i routing w `main.go`.
2. Handler w `controllers/`.
3. Input DTO w `models/inputs.go` lub w dedykowanym pliku.
4. Use-case w `services/`.
5. Operacje DB w `store/`.
6. Output DTO i mapper w `models/outputs.go` i `models/mappers.go`, jeśli potrzebne.
7. Jeśli potrzebna jest tabela lub relacja, dopisz model i migrację w `internal/migrate.go`.

Minimalny kierunek zależności:

`controller -> appService.<module> -> store.<module>`

Nie odwracaj tego kierunku.

## 7. Zasady zachowania kodu

### 7.1 Kontekst i czas

- propaguj `context.Context` od requestu aż do store,
- wszystkie czasy zapisuj i licz w UTC,
- połączenie DB jest konfigurowane pod UTC,
- jeśli tworzysz `time.Now()`, używaj `time.Now().UTC()`.

### 7.2 Walidacja

Walidacja jest dwupoziomowa:

- walidacja transportowa w kontrolerze: bind, format, wymagane pola, path/query parsing,
- walidacja biznesowa w serwisie: reguły domenowe, spójność danych, capability, ownership flow.

Walidacje wspólne trzymaj w `internal/validator.go`.

### 7.3 Błędy i statusy

Typowy mapping:

- `400` dla invalid payload / invalid param,
- `401` dla braku sesji,
- `403` dla braku ownership lub niedozwolonej akcji,
- `404` dla braku rekordu,
- `409` dla konfliktu stanu, jeśli ma sens biznesowy,
- `500` dla problemów infrastrukturalnych lub nieobsłużonych.

Nie przenoś logiki statusów HTTP do service/store.

### 7.4 Soft delete i dane historyczne

Encje używają `gorm.Model`, więc soft delete jest domyślny.

Jeśli flow biznesowy wymaga dostępu do historycznego rekordu, rób to świadomie przez opcję store, np. `WithDeleted()`. Przykład z repo: task może być soft-deleted, ale gameplay nadal musi móc go odczytać do historii stage.

### 7.5 Transakcje

Jeśli operacja:
- zapisuje kilka tabel,
- zmienia relacje many-to-many,
- wymaga spójnego odczytu i zapisu,

to transakcja powinna być w `store`, nie w kontrolerze.

### 7.6 Preload i mapping

Store ma zwracać dane potrzebne use-case’owi. Jeśli odpowiedź HTTP wymaga przekształcenia, użyj mappera z `models/mappers.go` albo zbuduj output DTO w serwisie.

Nie wypychaj do kontrolera skomplikowanego składania odpowiedzi z wielu relacji.

## 8. Middleware, auth i i18n

### Auth

Są dwa główne tryby:

- user auth przez ORY Kratos w `internal/middleware/kratos_auth.go`,
- authz przez Casbin w `internal/authz/` i `internal/middleware/casbin_authz.go`,
- admin/internal auth przez secret key w `internal/middleware/admin_key_auth.go`.

Te moduły są przełączane konfiguracją:

- `auth.enabled=false` nie rejestruje tras wymagających sesji Kratos,
- `rbac.enabled=false` zostawia auth bez middleware Casbina,
- `admin.enabled=false` nie rejestruje tras admin opartych o `X-Admin-Key`.

Nie zakładaj, że każdy projekt na tym template ma włączony Kratos, Casbin albo admin endpoints. Kod feature'a powinien jasno wskazywać wymagany poziom ochrony podczas rejestracji routingu.

Kontroler może odczytać sesję przez:

```go
sess, ok := middleware.SessionFromContext(c)
```

Rola do RBAC jest rozwiązywana z Kratos `identity.metadata_public.role` albo `identity.traits.role`.
Jeśli nie ma jawnej roli, template traktuje użytkownika jako `user`.

### Language

`LanguageMiddleware` wybiera język na podstawie `Accept-Language`.

Jeśli dana operacja wymaga language-aware odczytu z DB, kontroler przekazuje resolved language i candidates do `context.Context`, a store wybiera najlepsze tłumaczenie.

To ważny wzorzec dla feature’ów opartych o translacje. Nie rozwiązuj tego lokalnie ad hoc w kontrolerze.

## 9. Background jobs i procesy asynchroniczne

W tym repo background jobs są pełnoprawną częścią architektury.

Przykłady:
- `Notification.StartEmitter(...)`,
- `Notification.StartNotificationPrepareCron(...)`,
- `League.StartLeagueCron()`,
- `GameBots.StartCron()`,
- `GooglePaymentService.StartConsumer(...)`.

Zasady:
- startuj je podczas bootstrappingu aplikacji, a nie z requestu,
- logika joba należy do `services`,
- store wykonuje zapis/odczyt,
- kontrolery nie uruchamiają goroutines.

## 10. Integracje zewnętrzne

Integracje zewnętrzne trzymamy w `services/`, nie w `store/`.

Przykłady z repo:
- Stripe,
- Google Play + Pub/Sub,
- Apple StoreKit,
- Firebase Cloud Messaging,
- TUS upload.

Jeśli dodajesz nową integrację:
- klient i logika API umieść w `services/`,
- jeśli integracja zapisuje stan lokalny, użyj istniejącego store,
- nie wkładaj wywołań HTTP do `controllers` ani `store`.

## 11. Wzorzec provider/capability

Płatności pokazują istniejący wzorzec rozszerzalności:

- wspólny minimalny interfejs `PaymentProvider`,
- opcjonalne capability interfaces, np. `PaymentServiceInterface`, `SubscriptionServiceInterface`, `BillingPortalServiceInterface`,
- kontroler resolve’uje providera po nazwie,
- następnie sprawdza capability przez type assertion.

Tak samo warto projektować kolejne rozszerzalne integracje:
- minimum wspólnego kontraktu,
- opcjonalne capability,
- bez przeciążania jednego grubego interfejsu.

## 12. Cache i invalidacja

Serwis może mieć lokalny cache w pamięci, jeśli:
- cache jest wyraźnie związany z use-case,
- ma prostą invalidację,
- nie jest ukrytą globalną zależnością.

Przykład z repo:
- `GameService` trzyma cache,
- `TaskService` po zmianie tasków wywołuje invalidację przez mały interfejs `GameCacheInvalidator`.

To jest dobry wzorzec dla lekkich zależności między serwisami.

## 13. Jak AI ma dodawać nowy moduł

Jeśli tworzysz nowy feature, wykonuj go w tej kolejności:

1. Zdefiniuj model/DTO w `models/`.
2. Dodaj metody do odpowiedniego store lub nowy store.
3. Dodaj serwis use-case.
4. Jeśli trzeba, podłącz serwis w `AppService`.
5. Dodaj handler w istniejącym kontrolerze albo nowy kontroler.
6. Podłącz routing w `main.go`.
7. Dodaj middleware/auth tam, gdzie wymagane.
8. Dodaj mapper outputu, jeśli endpoint nie powinien zwracać surowej encji.
9. Jeśli zmieniasz schemat danych, dopisz model i migrację.
10. Jeśli feature ma efekt uboczny async, umieść go w `services`, nie w kontrolerze.

Jeśli feature dodaje tabelę, preferuj wersjonowaną migrację SQL w `internal/migrations/<driver>/`. `AutoMigrate` jest zostawione tylko jako lekki tryb prototypowy przez `database.disable_versioned_migrations=true`.

## 14. Czego AI nie powinno robić

Nie rób tych rzeczy:

- nie wkładaj logiki biznesowej do kontrolera,
- nie wkładaj klienta zewnętrznego API do `store`,
- nie odwołuj się do `echo.Context` w `services`,
- nie rozbijaj małego feature’a na nadmiar interfejsów i pakietów,
- nie buduj drugiego systemu DI obok `main.go`,
- nie twórz “repository per entity” poza stylem już użytym w `store/`,
- nie implementuj odpowiedzi HTTP w `services`,
- nie mieszaj modeli DB z tymczasowym JSON-em bez jawnego DTO lub mappera,
- nie pomijaj `context.Context`,
- nie pomijaj ownership checków tam, gdzie endpoint działa na zasobach użytkownika.

## 15. Praktyczny szablon nowego feature’a

Nowy feature powinien wyglądać mniej więcej tak:

```text
models/<feature>_inputs.go
models/<feature>_outputs.go
models/<feature>_mappers.go
store/<feature>Store.go
services/<feature>Service.go
controllers/<feature>Controller.go
```

Jeśli feature jest mały, nie mnoż plików sztucznie. W tym repo dopuszczalne jest dopisywanie kolejnych DTO do istniejących plików `models/inputs.go` i `models/outputs.go`.

## 16. Decyzje stylistyczne specyficzne dla tego repo

To repo jest pragmatyczne, więc AI powinno zachować obecny styl:

- prostota ponad akademicką czystość,
- ręczne DI ponad frameworki,
- warstwy są ważne, ale bez przesadnego ceremony,
- use-case i integracje siedzą razem w `services`,
- `AppService` i `Datastore` są akceptowanym service locator-like kompromisem,
- niektóre serwisy zwracają `interface{}`; nie trzeba tego na siłę przerabiać przy każdym change’u,
- background jobs są częścią aplikacji, nie osobnym systemem.

## 17. Krótka checklista dla AI

Przed zakończeniem zmiany sprawdź:

- Czy kontroler robi tylko HTTP?
- Czy serwis nie zna Echo?
- Czy store robi tylko DB?
- Czy DTO i mapery są w `models/`?
- Czy `context.Context` przechodzi przez cały flow?
- Czy auth i ownership są zachowane?
- Czy nowa relacja/tabela jest dodana do migracji?
- Czy odpowiedź HTTP nie ujawnia niepotrzebnie surowych danych?
- Czy logika async nie została uruchomiona z handlera?
- Czy rozwiązanie pasuje do istniejącego stylu, a nie do nowej wymyślonej architektury?

Jeśli odpowiedź na któreś z tych pytań brzmi "nie", popraw implementację przed dalszą rozbudową.
