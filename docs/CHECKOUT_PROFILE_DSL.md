# Checkout profile — границы DSL vs шаблоны vs Expert

Спецификация для **redirect checkout** интеграций (CentroBill MX-6, Apple Pay / Google Pay, hosted page).
Цель: зафиксировать, что описывается в CUE, что остаётся в `.ang/templates`, и где Expert выступает медиатором.

См. также: [INTEGRATION_WORKFLOW.md](./INTEGRATION_WORKFLOW.md).

---

## 1. Три слоя

```text
Expert knowledge/data/*.json + CUE intent
        │
        ▼
┌───────────────────────────────────────┐
│  CUE (DSL) — контракт поведения       │  ← Expert: vet / facts / advise / gate
│  provider.cue + Profile*              │
└───────────────────────────────────────┘
        │ TemplateData
        ▼
┌───────────────────────────────────────┐
│  Шаблоны — dialect Go проекта         │  ← Expert: shadow outcome (parity)
│  payment_providers/.ang/templates/    │
└───────────────────────────────────────┘
        │
        ▼
   Go provider (generated или hand-written parity target)
```

| Вопрос | Кто отвечает |
|--------|--------------|
| Какие операции есть у провайдера? | **CUE** (`has_payin`, `operations`, capabilities) |
| Куда ходить (path, method, auth)? | **CUE** (`endpoints`, `auth`, `request_signing`) |
| Как маппить статусы и ошибки? | **CUE** (`payin_statuses`, `error_codes`, `callback`) |
| Какие поля обязательны в запросе? | **CUE** (`payin_request.fields` + `source`) |
| Как выглядит `InitPay` в Go? | **Шаблон** (httpcli, tdsRedirector, описание tx) |
| Как форматировать телефон / redirect HTML? | **Шаблон** (или `api_compat`-ветка) |
| Согласован ли intent с кодом? | **Expert** (facts + packs + outcomes) |

---

## 2. Когда какой profile

| Profile | Flow | Пример | `auth_flow` |
|---------|------|--------|-------------|
| `ProfilePaytechGateway` | S2S card + optional redirect URL | paytech_mx724 | `h2h` |
| `ProfileFluxsgate` | Card H2H + 3DS POST redirect body | fluxsgate | `h2h` |
| **`ProfileRedirectCheckout`** *(новый)* | Hosted / wallet checkout, return URL | **mx6_centrobill** | **`redirect`** |
| `ProfileMacanP2P` | P2P requisites, не checkout | pumpp2p | `p2p` |

MX-6 — не Paytech: нет card PAN в InitPay, методы `applepay` / `googlepay`, пользователь уходит на hosted page.

---

## 3. Что обязано быть в CUE (DSL)

### 3.1 Identity (всегда per-provider)

```cue
provider: schema.#Provider & schema.ProfileRedirectCheckout & {
  package_name: "mx6_centrobill"
  sid:          "mx6"
  source:       "PPMX6Centrobill"
  label:        "MX-6"
  mid_prefix:   "MX"
  struct_name:      "PPMX6Centrobill"
  constructor_name: "NewPPMX6Centrobill"
}
```

### 3.2 Capabilities — что Expert проверяет через facts

| Поле | Checkout (MX-6) | Зачем |
|------|-----------------|-------|
| `auth_flow` | `"redirect"` | facts: flow type |
| `payment_source` | `"apm"` | не card-only |
| `has_payin` | `true` | init_payin declared |
| `has_payout` | `false` | |
| `has_p2p` | `false` | |
| `interfaces.tds_redirector` | `true` | redirect URL → TDS link |
| `supported_methods` | `["applepay", "googlepay"]` | wallet methods |
| `supported_currencies` | `["EUR"]` | из brief |

### 3.3 Transport — endpoints и auth

```cue
endpoints: {
  payin:        {path: "/api/v1/checkout/sessions", method: "POST"}   // from Expert knowledge/data
  payin_status: {path: "/api/v1/checkout/sessions", method: "GET"}  // или отдельный path
}
auth: {
  type:         "bearer"          // или header_token — из доки
  header:       "Authorization"
  secret_key:   "apiKey"
  content_type: "application/json"
}
secrets: {
  format:    "API Key:Signing Key"
  separator: ":"
  parts: [
    {name: "API Key", key: "apiKey"},
    {name: "Signing Key", key: "signingKey"},
  ]
}
```

**Правило:** path/method/secret keys — только CUE. Шаблон не хардкодит URL провайдера.

### 3.4 Request contract — `payin_request`

Декларативное описание тела InitPay. Источники — из `#CatalogFieldSource`:

```cue
payin_request: {
  name: "createCheckoutSessionRequest"
  fields: [
    {name: "ReferenceID",   json: "referenceId",   source: "tx_id"},
    {name: "Amount",        json: "amount",        source: "tx_amount_float", type: "float64"},
    {name: "Currency",      json: "currency",      source: "tx_currency"},
    {name: "PaymentMethod", json: "paymentMethod", source: "tx_payment_method"}, // applepay/googlepay
    {name: "ReturnUrl",     json: "returnUrl",     source: "tx_result_url"},
    {name: "WebhookUrl",    json: "webhookUrl",    source: "tx_callback_url"},
    {name: "Email",         json: "email",         source: "owner_info", owner_key: "email", owner_from: "apm"},
    {name: "FirstName",     json: "firstName",     source: "owner_info", owner_key: "first_name", owner_from: "apm", omitempty: true},
    {name: "LastName",      json: "lastName",      source: "owner_info", owner_key: "last_name", owner_from: "apm", omitempty: true},
  ]
}
```

**Правило:** какие поля уходят в API — CUE. Как `GetParameter` / валидация пустого email — шаблон (`api_compat`).

### 3.5 Response / redirect hints

```cue
// Имена типов для generated datatypes.go
response_types: [{
  name: "checkoutSessionResponse"
  fields: [
    {name: "ID",          type: "string", json: "id"},
    {name: "RedirectUrl", type: "string", json: "redirectUrl"},
    {name: "State",       type: "string", json: "state"},
  ]
}]

// Операция (если используем operations DSL)
operations: [{
  kind: "init_pay"
  transport: {
    endpoint:       "payin"
    request_type:   "createCheckoutSessionRequest"
    response_type:  "checkoutSessionResponse"
    status_field:   "state"
    status_strategy: "direct"
  }
}]
```

Поле redirect в ответе (`redirectUrl` vs `paymentUrl`) — **CUE** (`response_types` + template data).
Логика `tdsRedirector.GetTDSLink(Location: …)` — **шаблон**.

### 3.6 Status / callback / errors

```cue
payin_statuses: [
  {code: "pending",   status: "pending", status_code: "SCodeOk"},
  {code: "completed", status: "success", status_code: "SCodeOk"},
  {code: "failed",    status: "declined", status_code: "SCodeDeclinedByBank"},
  {code: "expired",   status: "declined", status_code: "SCodeTimeouted"},
]

callback: {
  tx_id_field:      "ReferenceID"
  foreign_id_field: "SessionID"
  status_field:     "State"
  status_type:      "string"
  fields: [
    {name: "ReferenceID", type: "string", json: "referenceId"},
    {name: "SessionID",   type: "string", json: "sessionId"},
    {name: "State",       type: "string", json: "state"},
  ]
}

callback_signature: {
  algorithm:  "hmac-sha256"
  secret_key: "signingKey"
  format:     "hmac_body"
  compare:    "equal"
  fields:     [{json: "referenceId"}]
}
```

Expert сравнивает: declared callback ops ↔ реализованные `ParseCallback` / `FinishCallback`.

### 3.7 Constructor deps

```cue
constructor_deps: [
  {name: "tdsRedirector", type: "model.TDSRedirector", pkg: "gitlab.q-tech.host/transferty/backend/tnx_processor/model"},
  {name: "txPathLogger",  type: "model.TxPathLogger",  pkg: "gitlab.q-tech.host/transferty/backend/tnx_processor/model"},
]
```

---

## 4. Что остаётся в шаблонах

Шаблоны (`provider.go.tmpl`, `checkout/provider_checkout.go.tmpl`, …) — **проектные соглашения tnx**, не домен провайдера.

| Ответственность шаблона | Пример |
|-------------------------|--------|
| Imports, struct layout, `httpcli.NewHTTPTnxClient` | все providers |
| Формат description: `"Payment_" + tx.Id` | комментарий в `provider.go.tmpl` |
| `providers.GetParameter` / валидация mandatory customer | paytech-style helpers |
| Redirect: GET Location vs POST auto-submit HTML | `fluxsgate/redirect.go.tmpl` |
| Обработка 5xx/4xx → `PaymentResponse` | `handlePaymentError` |
| `CheckStatus` throttle, async 3DS | флаги из TemplateData |
| Специфичный `api_compat` branch | `paytech_gateway`, `fluxsgate`, **`redirect_checkout`** |

**Не в шаблоне:**

- SID, endpoints, status tables
- список supported methods / currencies
- структура callback payload (поля — из CUE → datatypes)

### 4.1 Новый `api_compat: "redirect_checkout"`

Рекомендуемая ветка для wallet/hosted checkout (отдельно от Paytech S2S card):

- `InitPay`: только APM (`ps.Card == nil`), метод из `tx.PaymentMethod`
- populate request из `payin_request` generated helpers
- pending + `RedirectUrl` → `tdsRedirector`
- без PAN/CVV/expiry

Paytech-шаблон **не переиспользовать** для MX-6 напрямую — другой payment source и request shape.

---

## 5. Expert как медиатор

Expert не генерирует Go. Он проверяет **согласованность** на границах:

```text
Expert knowledge/data ──► CUE intent ──► facts JSON ──► Expert packs
                    │                              │
                    │         findings/proposals   │
                    ▼                              ▼
              ang build ◄──── gate/shadow ──── outcome store
                    │
                    ▼
                 Go code ──► facts (implementation) ──► Expert parity
```

### 5.1 Shared pack: `payment-provider.core`

- operation declared vs implemented
- schema drift
- payout capability vs init_payout

### 5.2 Integration pack: `mx6.integration`

Примеры правил (в `integration/expert.pack.cue`):

| Rule | Условие |
|------|---------|
| `mx6.wallet_methods_without_payin` | brief methods ⊃ {applepay,googlepay}, `init_pay` absent in CUE |
| `mx6.redirect_without_return_url` | `auth_flow=redirect`, `payin_request` без `tx_result_url` |
| `mx6.checkout_missing_tds` | `auth_flow=redirect`, `interfaces.tds_redirector=false` |
| `mx6.currency_mismatch` | brief EUR, CUE supported_currencies без EUR |

### 5.3 Режимы

| Этап | Expert mode | Действие |
|------|-------------|----------|
| Investigation | `advise` | findings, без блокировки |
| Перед первым build | `gate` | блок если critical findings |
| CI / regression | `shadow` | outcome без блокировки |

---

## 6. `ProfileRedirectCheckout` (реализован)

Профиль в `compiler/paymentprovider/schema/profiles.cue` и tnx `.ang/schema/profiles.cue`.

Шаблоны: `payment_providers/.ang/templates/redirect_checkout/`

Эталонный scaffold: `payment_providers/mx6_centrobill/` (`implementation: investigation`, endpoints `/TODO/...`).

```cue
provider: schema.#Provider & schema.ProfileRedirectCheckout & { ... }
```

Per-provider delta (endpoints, statuses, payin_request) — только в `.cue/provider.cue`.

---

## 7. MX-6 — минимальный provider.cue (investigation)

```cue
package provider

import "transferty.local/mx6_centrobill/schema"

provider: schema.#Provider & schema.ProfileRedirectCheckout & {
  package_name: "mx6_centrobill"
  sid:          "mx6"
  source:       "PPMX6Centrobill"
  label:        "MX-6"
  mid_prefix:   "MX"

  struct_name:      "PPMX6Centrobill"
  constructor_name: "NewPPMX6Centrobill"

  supported_currencies: ["EUR"]
  supported_methods:    ["applepay", "googlepay"]

  currency: {code: "EUR", iso_num: 978, country: "EU"}

  // TODO: endpoints from deal/expert/knowledge/data after PM sign-off
  endpoints: payin: {path: "...", method: "POST"}

  secrets: { /* из PM / creds format */ }

  payin_statuses: [ /* из доки */ ]
  callback: { /* из webhook spec */ }

  constructor_deps: [
    {name: "tdsRedirector", type: "model.TDSRedirector", pkg: "gitlab.q-tech.host/transferty/backend/tnx_processor/model"},
    {name: "txPathLogger",  type: "model.TxPathLogger",  pkg: "gitlab.q-tech.host/transferty/backend/tnx_processor/model"},
  ]
}
```

Expert `knowledge/data/*.json`: `implementation: investigation` → **без `ang build`**, только vet/facts/expert.

Когда intent стабилен → `implementation: generated` → добавить `ProfileRedirectCheckout` в schema + шаблон `redirect_checkout` → `ang build`.

---

## 8. Когда переносить логику из шаблона в DSL

| Сигнал | Действие |
|--------|----------|
| Одно и то же поле/request mapping у 2+ checkout провайдеров | → `payin_request` + generic template |
| Новый формат redirect (POST form) второй раз | → флаг в CUE (`redirect_mode: "post_form"`) + shared redirect helper в templates |
| Одноразовая quirk Centrobill | → остаётся в `api_compat` branch или hand-written |
| Expert finding «intent mismatch» повторяется | → правило в core pack + поле в CUE |

**Анти-паттерн:** if/else по `sid` в шаблоне. Правильно: `api_compat` или data-driven TemplateData.

---

## 9. Чеклист перед `ang build` (generated)

- [ ] Expert knowledge: `implementation: generated`
- [ ] `ang pp vet` — 0 errors
- [ ] `ang pp facts` — operations/endpoints/callback declared
- [ ] Expert advise/gate — no blocking findings
- [ ] `ProfileRedirectCheckout` в ang bundle + `ang pp schema sync`
- [ ] Шаблон `redirect_checkout` покрывает InitPay / CheckStatus / Callback
- [ ] `go test ./payment_providers/mx6_centrobill/...`
- [ ] Expert shadow outcome → `stable`

---

## 10. Связь с incas (hand-written)

Hand-written провайдеры **тот же CUE pipeline**:

- CUE = source of truth для Expert
- `ang build` **запрещён** (`implementation: hand_written`)
- Expert проверяет parity: facts declaration vs AST implementation

Checkout-generated и payout-hand-written — один медиатор (Expert), разные режимы генерации.
