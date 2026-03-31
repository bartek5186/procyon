# Konfiguracja BusDoPolski - Coming Soon Page

## 🔐 Google reCAPTCHA v3 - Konfiguracja

### Krok 1: Utwórz klucze reCAPTCHA

1. Przejdź do: https://www.google.com/recaptcha/admin/create
2. Wybierz **reCAPTCHA v3**
3. Podaj domenę: `bus-do-polski.pl` (i ewentualnie `localhost` do testów)
4. Zaakceptuj warunki i kliknij "Submit"

### Krok 2: Skopiuj klucze

Otrzymasz dwa klucze:
- **Site Key (klucz publiczny)** - do użycia w frontendzie
- **Secret Key (klucz prywatny)** - do użycia w backendzie

### Krok 3: Zamień klucze w kodzie

W pliku `coming-soon.html` znajdź i zamień:

**Linia ~52 (w sekcji head):**
```html
<script src="https://www.google.com/recaptcha/api.js?render=TWÓJ_SITE_KEY"></script>
```

**Linia ~227 (w JavaScript):**
```javascript
grecaptcha.execute('TWÓJ_SITE_KEY', {action: 'submit_waitlist'})
```

Obecnie używany jest testowy klucz Google: `6LeIxAcTAAAAAJcZVRqyHh71UMIEGNQ_MXjiZKhI`

---

## 📊 Struktura danych wysyłanych z formularza

### JSON wysyłany do `/api/waitlist`:

```json
{
  "userType": "passenger" | "carrier",
  "email": "user@example.com",
  "phone": "+48123456789" | null,
  "consents": {
    "waitlist": true,
    "marketing": true | false,
    "cookies_analytics": true | false,
    "cookies_marketing": true | false
  },
  "recaptcha_token": "03AGdBq26...",
  "timestamp": "2026-03-30T12:34:56.789Z",
  "user_agent": "Mozilla/5.0...",
  "referrer": "https://google.com" | "direct"
}
```

### Opis pól:

| Pole | Typ | Opis |
|------|-----|------|
| `userType` | string | "passenger" lub "carrier" |
| `email` | string | Adres email (walidowany) |
| `phone` | string/null | Numer telefonu (opcjonalny) |
| `consents.waitlist` | boolean | Zgoda na listę oczekujących (zawsze true) |
| `consents.marketing` | boolean | Zgoda marketingowa z formularza |
| `consents.cookies_analytics` | boolean | Zgoda na cookies analityczne (z cookie bannera) |
| `consents.cookies_marketing` | boolean | Zgoda na cookies marketingowe (z cookie bannera) |
| `recaptcha_token` | string | Token reCAPTCHA v3 do weryfikacji |
| `timestamp` | string | ISO 8601 timestamp |
| `user_agent` | string | User agent przeglądarki |
| `referrer` | string | Źródło ruchu |

---

## 🔒 Weryfikacja reCAPTCHA na backendzie

### Przykład w Node.js:

```javascript
const axios = require('axios');

async function verifyRecaptcha(token, remoteIP) {
  const secretKey = 'TWÓJ_SECRET_KEY';
  
  try {
    const response = await axios.post(
      'https://www.google.com/recaptcha/api/siteverify',
      null,
      {
        params: {
          secret: secretKey,
          response: token,
          remoteip: remoteIP
        }
      }
    );
    
    const { success, score, action } = response.data;
    
    // reCAPTCHA v3 zwraca score od 0.0 do 1.0
    // 1.0 = bardzo prawdopodobnie człowiek
    // 0.0 = bardzo prawdopodobnie bot
    
    if (success && score >= 0.5 && action === 'submit_waitlist') {
      return { valid: true, score };
    }
    
    return { valid: false, score };
  } catch (error) {
    console.error('reCAPTCHA verification error:', error);
    return { valid: false, score: 0 };
  }
}

// Endpoint przykład:
app.post('/api/waitlist', async (req, res) => {
  const { recaptcha_token, email, userType, phone, consents } = req.body;
  const remoteIP = req.ip;
  
  // Weryfikuj reCAPTCHA
  const captchaResult = await verifyRecaptcha(recaptcha_token, remoteIP);
  
  if (!captchaResult.valid) {
    return res.status(400).json({ 
      error: 'reCAPTCHA verification failed',
      score: captchaResult.score 
    });
  }
  
  // Zapisz do bazy danych
  // ... twoja logika
  
  res.json({ success: true, message: 'Added to waitlist' });
});
```

### Przykład w PHP:

```php
<?php
function verifyRecaptcha($token, $remoteIP) {
    $secretKey = 'TWÓJ_SECRET_KEY';
    
    $url = 'https://www.google.com/recaptcha/api/siteverify';
    $data = [
        'secret' => $secretKey,
        'response' => $token,
        'remoteip' => $remoteIP
    ];
    
    $options = [
        'http' => [
            'header' => "Content-type: application/x-www-form-urlencoded\r\n",
            'method' => 'POST',
            'content' => http_build_query($data)
        ]
    ];
    
    $context = stream_context_create($options);
    $result = file_get_contents($url, false, $context);
    $responseData = json_decode($result);
    
    if ($responseData->success && $responseData->score >= 0.5) {
        return ['valid' => true, 'score' => $responseData->score];
    }
    
    return ['valid' => false, 'score' => $responseData->score];
}

// Endpoint
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $data = json_decode(file_get_contents('php://input'), true);
    
    $captchaResult = verifyRecaptcha(
        $data['recaptcha_token'],
        $_SERVER['REMOTE_ADDR']
    );
    
    if (!$captchaResult['valid']) {
        http_response_code(400);
        echo json_encode(['error' => 'reCAPTCHA verification failed']);
        exit;
    }
    
    // Zapisz do bazy
    // ...
    
    echo json_encode(['success' => true]);
}
?>
```

---

## 📁 Struktura bazy danych (sugerowana)

### Tabela: `waitlist_subscribers`

```sql
CREATE TABLE waitlist_subscribers (
    id INT PRIMARY KEY AUTO_INCREMENT,
    user_type ENUM('passenger', 'carrier') NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    phone VARCHAR(50),
    
    -- Zgody
    consent_waitlist BOOLEAN DEFAULT TRUE,
    consent_marketing BOOLEAN DEFAULT FALSE,
    consent_cookies_analytics BOOLEAN DEFAULT FALSE,
    consent_cookies_marketing BOOLEAN DEFAULT FALSE,
    
    -- Metadane
    recaptcha_score DECIMAL(3,2),
    user_agent TEXT,
    referrer VARCHAR(500),
    ip_address VARCHAR(45),
    
    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    notified_at TIMESTAMP NULL,
    
    -- Indeksy
    INDEX idx_email (email),
    INDEX idx_user_type (user_type),
    INDEX idx_created_at (created_at)
);
```

---

## 🎯 Zalecenia dotyczące score reCAPTCHA

| Score | Interpretacja | Akcja |
|-------|---------------|-------|
| 0.9 - 1.0 | Bardzo prawdopodobnie człowiek | Akceptuj |
| 0.7 - 0.8 | Prawdopodobnie człowiek | Akceptuj |
| 0.5 - 0.6 | Neutralne | Akceptuj, ale monitoruj |
| 0.3 - 0.4 | Podejrzane | Odrzuć lub wymagaj dodatkowej weryfikacji |
| 0.0 - 0.2 | Bardzo prawdopodobnie bot | Odrzuć |

**Zalecany próg: 0.5** (możesz dostosować w zależności od poziomu false positives)

---

## 🔧 Google Analytics - Konfiguracja

W pliku `coming-soon.html` linia ~47:

```javascript
const GA_MEASUREMENT_ID = 'G-XXXXXXXXXX';
```

Zamień na swój Measurement ID z Google Analytics 4.

---

## ✅ Checklist wdrożenia

- [ ] Założ konto reCAPTCHA v3 i pobierz klucze
- [ ] Zamień Site Key w 2 miejscach w `coming-soon.html`
- [ ] Skonfiguruj backend do weryfikacji tokenów reCAPTCHA
- [ ] Utwórz endpoint `/api/waitlist` przyjmujący JSON
- [ ] Utwórz tabelę w bazie danych
- [ ] Zapisuj wszystkie zgody (waitlist, marketing, cookies)
- [ ] Zapisuj score reCAPTCHA do audytu
- [ ] Zamień Google Analytics ID
- [ ] Przetestuj formularz (wypełnij i sprawdź console.log)
- [ ] Przetestuj zapisywanie zgód cookies
- [ ] Włącz produkcyjny endpoint (usuń komentarze z fetch)

---

## 🧪 Testowanie

### Frontend (bez backendu):

1. Otwórz `coming-soon.html` w przeglądarce
2. Otwórz DevTools → Console
3. Wypełnij formularz
4. Sprawdź obiekt w console.log - powinien zawierać wszystkie dane

### Z backendem:

1. Odkomentuj blok `fetch()` w JavaScript
2. Wypełnij formularz
3. Sprawdź Network tab - powinno być POST do `/api/waitlist`
4. Sprawdź odpowiedź serwera

---

## 📝 Notatki

- reCAPTCHA v3 działa **w tle** - użytkownik nic nie klika
- Token jest ważny przez **2 minuty**
- reCAPTCHA v3 nie blokuje botów - tylko daje score (Ty decydujesz co z nim zrobić)
- Cookies consent jest **opcjonalny** w momencie wysyłania formularza (może być false)
- Wszystkie zgody są zapisywane do bazy dla compliance RODO
