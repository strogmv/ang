# Интеграция Expert Runtime в генерацию ANG

Статус документа: implementation specification, а не описание уже готовой функции.

Цель документа — дать достаточно точный и проверяемый план, чтобы реализацию мог выполнить агент, который не знает историю проекта и склонен додумывать отсутствующие детали. Если фактический код расходится с этим документом, агент обязан сначала проверить код и обновить план, а не создавать совместимость «по памяти».

## 1. Цель

Нужно связать автономный генератор ANG и отдельный Expert Runtime так, чтобы каждая реальная генерация могла оставлять проверяемый результат:

```text
факты до генерации
    -> экспертный анализ
    -> обычная детерминированная генерация ANG
    -> локальная проверка результата
    -> outcome в Expert Runtime
    -> ограниченная история по конкретному проекту
```

Expert не должен генерировать Go-код, выполнять shell-команды, читать проект или изменять файлы. Он получает только нормализованные факты и возвращает findings/proposals. ANG остаётся владельцем:

- CUE intent;
- компиляции и генерации;
- файловой системы;
- проверки хешей;
- sandbox/dry-run;
- build/test verification;
- решения о применении proposal.

Накопление результата означает накопление проверенных outcomes, а не автоматическое обучение на исходниках и не самовольное изменение knowledge packs.

## 2. Репозитории и текущие точки расширения

### ANG

Репозиторий:

```text
/Users/m.ramanchak/Desktop/work/ang
```

Существующие важные файлы:

| Файл | Назначение |
|---|---|
| `cmd/ang/advise.go` | read-only команда `ang advise`; сейчас разрешает только goal `project.audit` |
| `cmd/ang/expert_provider.go` | HTTP/stdio клиент Expert Runtime и проверка ответа |
| `cmd/ang/pp.go` | команды `ang pp schema` и `ang pp vet` |
| `cmd/ang/build.go` | общий build; отдельно обнаруживает payment-provider проект |
| `cmd/ang/build_helpers.go` | flags и `OutputOptions` для build |
| `compiler/paymentprovider/load.go` | загрузка и Decode provider CUE в `ProviderSpec` |
| `compiler/paymentprovider/vet.go` | семантические PP diagnostics |
| `compiler/paymentprovider/schema.go` | bundled schema sync/check |
| `compiler/paymentprovider/build.go` | CUE -> TemplateData -> Emit |
| `compiler/paymentprovider/emit.go` | запись generated payment-provider файлов |
| `compiler/facts/` | общий контракт `ang/facts/v1`; он не описывает payment providers достаточно точно |
| `compiler/expert/` | локальная копия контракта `ang/expert-report/v1`, canonicalization и validation |

Текущий внешний клиент уже проверяет:

- `ang/expert-response/v1`;
- совпадение `request_id`;
- наличие `runtime_version`;
- совпадение goal;
- совпадение compiler version;
- совпадение facts hash;
- canonicalization и `ValidateReport` на стороне ANG.

Эти проверки нельзя удалять или обходить.

### Expert Runtime

Репозиторий:

```text
/Users/m.ramanchak/Desktop/work/deal/expert
```

Существующие важные файлы:

| Файл/пакет | Назначение |
|---|---|
| `api/v1/types.go` | `AnalyzeRequest`/`AnalyzeResponse` |
| `api/v1/report.go` | переносимый `ang/expert-report/v1` |
| `api/v1/validate.go` | transport validation |
| `runtime/runtime.go` | выбор packs, fixed-point inference, policy, report |
| `runtime/ang_facts.go` | адаптация `ang/facts/v1` в `engine.Environment` |
| `transport/http.go` | `POST /v1/analyze` |
| `memory/stability.go` | ограниченная value-object история; постоянного хранилища пока нет |
| `packs/registry.go` | versioned manifest/dependencies/goals/read-write predicates |
| `packcue/` | декларативные CUE rules, максимум четыре typed terms в факте |
| `cmd/expert-server` | HTTP server, по умолчанию loopback |
| `cmd/expert-runtime` | один analyze request через stdin/stdout |

## 3. Обязательные ограничения

### 3.1. Правила ANG repository

Перед любыми изменениями или тестами агент обязан выполнить MCP bootstrap, если соответствующие MCP tools доступны:

1. `ang_mcp_health`
2. `ang_schema`
3. `ang_validate`

Если tools недоступны, это надо явно написать в рабочем отчёте. Нельзя утверждать, что bootstrap прошёл.

По умолчанию ANG-проекты изменяются через CUE intent. Generated code нельзя править вручную. Любые существующие пользовательские или untracked изменения необходимо сохранить.

### 3.2. ANG остаётся самостоятельным

- Default mode всегда `off`.
- Без Expert Runtime существующие команды должны работать как раньше.
- Недоступность Expert в `shadow` не должна ломать build.
- ANG не импортирует Go module Expert Runtime.
- Wire structs при необходимости зеркалируются в ANG, как это уже сделано в `cmd/ang/expert_provider.go`.

### 3.3. Expert не получает полномочий

В Expert нельзя передавать:

- исходники Go/CUE;
- абсолютные пути;
- merchant secrets;
- PEM keys;
- токены;
- environment variables;
- raw test/build logs;
- filesystem handles;
- команды для выполнения.

Expert получает только canonical JSON facts, нормализованные diagnostic codes и hashes.

### 3.4. Никакого автоматического обучения правил

Outcome history может использоваться для:

- определения повторяющихся findings;
- оценки стабильности проекта;
- ранжирования предложений;
- обнаружения oscillation/thrashing;
- формирования кандидата на новое правило.

Outcome history не может автоматически:

- создавать или менять CUE knowledge pack;
- повышать hypothesis до known truth;
- снижать критичность security finding;
- включать auto-apply;
- переносить результат между организациями без явного scope mapping.

## 4. Почему нельзя сразу делать advise/apply для payment providers

Есть два фактических блокера.

### 4.1. Текущий report v1 не адресует `.cue/`

`compiler/expert.ValidateReport` разрешает `Change.File` только как относительный `.cue` файл внутри `cue/`.

Payment provider `incas_n692` хранит intent в:

```text
.cue/provider.cue
```

Нельзя просто расширить validator до любого `.cue` пути. Это ослабит существующую security boundary. До появления отдельного typed intent target payment-provider pack не должен возвращать proposals. Первая версия pack возвращает только findings.

Для будущего apply нужен новый major report contract или typed target, например:

```json
{
  "target": {
    "kind": "project_cue_root",
    "relative_path": "provider.cue"
  }
}
```

ANG должен сам разрешать `project_cue_root` через `ang.yaml`, проверять containment, `BeforeHash`, CUE path и approval. Expert не должен знать, равен ли фактический root `cue/`, `.cue/` или другому каталогу.

### 4.2. Payment-provider build сейчас неполностью соблюдает общие flags

Ветка payment provider в `cmd/ang/build.go` вызывает `paymentprovider.Build` с output по умолчанию и затем сразу возвращается.

Следствия:

- общий `--dry-run` не направляет payment-provider output во временный каталог;
- `--run-tests` не выполняется для payment-provider ветки;
- общий post-build verification обходится ранним return.

До исправления нельзя заявлять, что candidate payment-provider generation безопасно проверяется в sandbox. Shadow mode может наблюдать существующий build, но не должен изображать отсутствующий dry-run/test gate.

## 5. Режимы интеграции

Реализовывать строго по порядку.

| Режим | Поведение | Ошибка Expert | Может менять intent | Может блокировать build |
|---|---|---|---|---|
| `off` | текущий ANG | не применимо | нет | нет |
| `shadow` | анализ + обычный build + outcome | warning, build продолжается | нет | нет |
| `advise` | findings и review-required proposals | показывается пользователю | только после отдельного approval flow | только при invalid report/conflict, не автоматически |
| `gate` | trusted policy может остановить небезопасный build | fail-closed | только через approval | да |

Первая реализация vertical slice допускала только `off` и `shadow`. С шага 9+ также поддерживаются `advise`, `gate` и sandbox apply через `ang pp apply`.

Значения `advise`/`gate`/`apply` до реализации соответствующих gates должны завершаться понятной ошибкой, а не молча работать как shadow.

## 6. Итоговый lifecycle

```text
ANG CLI
  |
  | 1. Detect payment-provider project
  | 2. Load CUE ProviderSpec
  | 3. Extract canonical payment-provider facts (before)
  | 4. POST /v1/analyze, goal=payment_provider.audit
  | 5. Validate Expert response locally
  |
  | 6. Run the same deterministic paymentprovider.Build
  | 7. Record local verification status
  | 8. Extract canonical facts (after), when possible
  |
  | 9. POST /v1/outcomes
  v
Expert Runtime
  |
  | Store append-only outcome under one explicit scope
  | Derive bounded stability summary
  | Never mutate ANG files or packs
  v
Outcome history
```

В `shadow` шаги 3–5 и 9 являются best effort. Шаг 6 обязан сохранить прежнюю семантику build.

## 7. Контракт `ang/payment-provider-facts/v1`

### 7.1. Почему отдельная schema

`ang/facts/v1` содержит entities, repositories, service operations и HTTP endpoints приложения. Кодировать payment-provider semantics через фиктивные entities запрещено: это создаст ложную модель предметной области.

`AnalyzeRequest.Facts` уже является opaque JSON, поэтому Expert может выбирать адаптер по `facts.schema` без импорта ANG packages.

### 7.2. Wire model

Предлагаемое место в ANG:

```text
compiler/paymentprovider/facts/model.go
compiler/paymentprovider/facts/validate.go
compiler/paymentprovider/facts/canonical.go
compiler/paymentprovider/facts/extract.go
```

Имя Go package: `ppfacts`.

Модель:

```go
const SchemaV1 = "ang/payment-provider-facts/v1"

type Envelope struct {
    Schema       string       `json:"schema"`
    ScopeID      string       `json:"scope_id"`
    ProviderID   string       `json:"provider_id"`
    Facts        []Fact       `json:"facts"`
    Evidence     []Evidence   `json:"evidence"`
    Diagnostics  []Diagnostic `json:"diagnostics"`
}

type Term struct {
    Sort  string `json:"sort"`
    Value string `json:"value"`
}

type Fact struct {
    ID          string   `json:"id"`
    Predicate   string   `json:"predicate"`
    Terms       []Term   `json:"terms"`
    EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type Evidence struct {
    ID          string `json:"id"`
    Extractor   string `json:"extractor"`
    ContentHash string `json:"content_hash"`
}

type Diagnostic struct {
    Code     string `json:"code"`
    Severity string `json:"severity"`
}
```

Не добавлять `SourcePath`, `Snippet` или raw message.

### 7.3. Validation

`Validate(Envelope)` обязан проверять:

- schema точно равна `ang/payment-provider-facts/v1`;
- `scope_id` и `provider_id` не пусты;
- нет duplicate fact/evidence IDs;
- predicate входит в registry v1;
- каждый факт имеет от 1 до 4 terms;
- sort/value каждого term не пусты;
- value не содержит CR/LF и не превышает разумный лимит, например 2048 bytes;
- все `evidence_ids` существуют;
- content hash является lowercase SHA-256, то есть 64 hex characters;
- severity входит в `info|warning|error`;
- diagnostics не содержат path/message.

### 7.4. Canonicalization

Canonical JSON необходим до вычисления facts hash.

Правила:

- facts сортируются по `ID`;
- evidence сортируется по `ID`;
- diagnostics сортируются по `code + severity`;
- `EvidenceIDs` внутри факта сортируются и deduplicate;
- порядок terms сохраняется;
- nil slices нормализуются в пустые arrays, если контракт ожидает array;
- JSON создаётся через `encoding/json`, без indent.

Fact ID:

```text
"fact:" + sha256(predicate + NUL + sort1 + NUL + value1 + ...)
```

Evidence ID:

```text
"evidence:" + sha256(extractor + NUL + content_hash)
```

Нельзя включать absolute path или timestamp в hash input.

### 7.5. Predicate registry v1

Использовать отдельные predicates, чтобы pack manifests могли точно декларировать reads.

#### `pp_provider`

```text
(provider, providerID)
```

Пример:

```text
pp_provider(provider=n692)
```

#### `pp_capability`

```text
(provider, providerID), (capability, name), (enabled, true|false)
```

Capabilities берутся из `ProviderSpec`: `payin`, `payout`, `p2p`, `subscription`, `refund`, `cancel`.

Нужно эмитить факт для каждого известного capability, включая false. Engine пока не поддерживает negation-as-failure, поэтому отсутствие факта не может означать false.

#### `pp_operation`

```text
(provider, providerID),
(operation, operationName),
(declaration, declared|absent),
(implementation, implemented|stub|absent|unknown)
```

Нужно эмитить факт для каждого известного provider operation. Минимальный registry:

```text
init_pay
init_payout
check_status
init_refund
init_pay_p2p
cancel_pay
init_subscription
subscription_pay
parse_callback
validate_callback
finish_callback
```

`declared` определяется только из CUE `operations`.

`implemented` определяется консервативно по Go AST:

- `absent` — метода с receiver `ProviderSpec.StructName` нет;
- `stub` — тело является простым возвратом `ErrNotImplemented`;
- `implemented` — есть непустая реализация, не распознанная как stub;
- `unknown` — метод найден, но AST classifier не может безопасно классифицировать его.

Не считать существование интерфейсного stub полноценной реализацией.

#### `pp_schema_sync`

```text
(provider, providerID), (state, in_sync|drift|unknown)
```

Источник — `paymentprovider.CheckSchema`.

При drift можно дополнительно эмитить:

#### `pp_schema_drift`

```text
(provider, providerID), (schema_file, baseName)
```

Передавать только basename из разрешённого bundled schema registry. Не передавать полный путь.

#### `pp_vet_issue`

```text
(provider, providerID), (code, code), (severity, info|warning|error)
```

Передаётся code/severity из `VetProject`; message и hint остаются локально в ANG.

#### `pp_secret_part`

```text
(provider, providerID), (key, logicalKey), (optional, true|false), (type, string|bool)
```

Передаётся только форма секрета. Значение секрета никогда не читается.

#### `pp_auth`

```text
(provider, providerID), (kind, authKind), (header, headerName), (masked, true|false)
```

Не передавать token prefix или secret value. Если auth не декларирован, эмитить `kind=none`, а не пытаться угадать bearer auth из строки Go-кода.

#### `pp_endpoint`

```text
(provider, providerID), (operation, operationName), (method, HTTPMethod), (path, pathTemplate)
```

Разрешены только declarative endpoint templates из CUE. Query values и absolute secret-bearing URLs запрещены.

#### `pp_runtime_policy`

```text
(provider, providerID), (policy, policyName), (value, normalizedValue)
```

В v1 извлекать только `runtime_policy_config`, уже объявленный в CUE. Не угадывать retry policy по произвольным loops/constants.

#### `pp_behavior`

```text
(provider, providerID), (behavior, behaviorName)
```

В первой версии допустимы только точные AST observations с import resolution:

- вызов `crypto/rsa.EncryptOAEP` -> `rsa_oaep_card_encryption`;
- вызов `crypto/rsa.VerifyPKCS1v15` -> `rsa_pkcs1v15_callback_verification`.

Не искать эти строки простым regexp по исходнику. Нужно разобрать imports и `ast.SelectorExpr`, чтобы alias import не ломал extraction.

Не пытаться в v1 автоматически выводить:

- remote key discovery;
- бизнес-смысл произвольного HTTP GET;
- retry semantics из циклов;
- статусные mapping semantics из switch.

Если это требуется как устойчивый факт, позже добавить явный CUE intent или отдельный проверяемый extractor.

#### `pp_test_area`

```text
(provider, providerID), (area, unit|behavior|callback|live), (present, true|false)
```

Это только наличие test evidence, а не успешность тестов. Результаты запуска идут в outcome verification.

### 7.6. Scope

Для локального пилота:

```text
scope_id = "payment-provider:" + ProviderSpec.SID
provider_id = ProviderSpec.SID
```

Нельзя использовать absolute directory в scope.

Для hosted multi-tenant режима одного SID недостаточно. Там server обязан namespace scope authenticated tenant/org identifier. Это отдельная задача и не входит в локальный v1.

## 8. Извлечение payment-provider facts в ANG

### 8.1. Public API

```go
type ExtractOptions struct {
    ProjectPath string
    CueRoot     string
    SchemaDir   string
}

func Extract(opts ExtractOptions) (Envelope, error)
```

Порядок extraction:

1. `LoadProjectConfig`.
2. Разрешить cue root/schema dir теми же функциями, что использует `paymentprovider.Build`.
3. `paymentprovider.Load` -> `ProviderSpec`.
4. `paymentprovider.CheckSchema`.
5. `paymentprovider.VetProject`.
6. Parse Go files через `go/parser`, исключая vendor и скрытые временные каталоги.
7. Извлечь receiver methods и разрешённые crypto calls.
8. Проверить наличие test areas.
9. Canonicalize и Validate.

Нельзя повторно реализовывать YAML/CUE path resolution внутри extractor.

### 8.2. CLI

Добавить в `cmd/ang/pp.go`:

```text
ang pp facts [path] [--cue-root .cue] [--json]
```

Рекомендуемый файл:

```text
cmd/ang/pp_facts.go
```

Поведение:

- output по умолчанию — canonical JSON в stdout;
- `--json` можно оставить для симметрии, но stdout всё равно должен быть JSON;
- diagnostics не печатать вперемешку с JSON;
- ошибки идут в stderr и дают non-zero exit;
- команда read-only;
- команда не вызывает Expert и не запускает build.

## 9. Адаптер facts в Expert Runtime

### 9.1. Dispatch по schema

Текущий `runtime/ang_facts.go` должен быть отрефакторен без изменения поведения `ang/facts/v1`:

```go
type AdaptedFacts struct {
    Environment *engine.Environment
    ScopeID     string
    Schema      string
}

func adaptFacts(data []byte) (AdaptedFacts, error) {
    // decode only {schema: ...} first
    // switch schema
}
```

Dispatch:

```text
ang/facts/v1                  -> существующий adapter
ang/payment-provider-facts/v1 -> новый provider adapter
unknown schema               -> explicit error
```

Не импортировать ANG Go packages. В Expert создаётся зеркальная private wire struct и своя validation.

### 9.2. Mapping в engine facts

Каждый wire fact преобразуется непосредственно в `engine.Fact`:

- `Predicate` сохраняется;
- wire term order сохраняется;
- каждый term преобразуется в `engine.Term{Sort, Value}`;
- facts с arity > 4 отвергаются;
- duplicate semantic facts допустимо deduplicate через `Environment.AddBase`, но duplicate IDs должен отбраковать validator;
- неизвестный predicate в schema v1 отвергается.

### 9.3. Первый knowledge pack

Добавить CUE pack, например:

```text
examples/payment-provider-core.cue
```

Manifest:

```text
id:      "payment-provider.core"
version: "0.1.0"
goals:   ["payment_provider.audit"]
```

Первый набор правил должен быть небольшим и доказуемым.

#### Реализация есть, декларации нет

Input:

```text
pp_operation(provider=$provider, operation=$operation,
             declaration=absent, implementation=implemented)
```

Output finding:

```text
code: PP_OPERATION_INTENT_MISMATCH
severity: warning
summary: implemented provider operation is absent from CUE intent
```

#### Capability payout включён, init_payout отсутствует

Это правило должно использовать явный false/absent fact, а не отсутствие факта.

Output:

```text
code: PP_PAYOUT_OPERATION_MISSING
severity: warning
```

#### Schema drift

Input:

```text
pp_schema_sync(provider=$provider, state=drift)
```

Output:

```text
code: PP_SCHEMA_DRIFT
severity: warning
```

В первой версии pack не содержит `proposals`, потому что report v1 не умеет безопасно адресовать payment-provider cue root.

## 10. Контракт результата `ang/expert-outcome/v1`

### 10.1. Назначение

Outcome связывает один проверенный build с конкретными facts/report/knowledge versions. Он не содержит исходники или raw logs.

### 10.2. API types

Добавить в Expert:

```text
api/v1/outcome.go
api/v1/outcome_validate.go
api/v1/outcome_canonical.go
```

Предлагаемая модель:

```go
const (
    OutcomeSchema         = "ang/expert-outcome/v1"
    OutcomeResponseSchema = "ang/expert-outcome-response/v1"
)

type OutcomeRequest struct {
    Schema             string             `json:"schema"`
    RunID              string             `json:"run_id"`
    ScopeID            string             `json:"scope_id"`
    Goal               string             `json:"goal"`
    CompilerVersion    string             `json:"compiler_version"`
    FactsBeforeHash    string             `json:"facts_before_hash"`
    ReportHash         string             `json:"report_hash"`
    KnowledgeVersions  []string           `json:"knowledge_versions"`
    ProposalDecisions  []ProposalDecision `json:"proposal_decisions"`
    Verification       []Verification     `json:"verification"`
    FactsAfterHash     string             `json:"facts_after_hash,omitempty"`
    OutputManifestHash string             `json:"output_manifest_hash,omitempty"`
    FinalStatus        OutcomeStatus      `json:"final_status"`
}

type ProposalDecision struct {
    ProposalID string `json:"proposal_id"`
    Decision   string `json:"decision"`
    ReasonCode string `json:"reason_code,omitempty"`
}

type Verification struct {
    Check  string   `json:"check"`
    Status string   `json:"status"`
    Codes  []string `json:"codes,omitempty"`
}

type OutcomeResponse struct {
    Schema   string `json:"schema"`
    RunID    string `json:"run_id"`
    Accepted bool   `json:"accepted"`
}
```

Enums:

```text
ProposalDecision.Decision:
  accepted | rejected | deferred | not_reviewed

Verification.Check:
  schema_check | pp_vet | build | go_test | post_facts

Verification.Status:
  passed | failed | skipped

OutcomeStatus:
  stable | advice | blocked | failed
```

В shadow mode все proposals, если они неожиданно присутствуют, записываются как `not_reviewed` с reason `shadow_mode`. Они не применяются.

### 10.3. Validation

Обязательные правила:

- schema exact match;
- `run_id`, `scope_id`, `goal`, compiler version не пусты;
- run ID не содержит path separators и ограничен по длине;
- facts/report/output hashes — lowercase SHA-256 при наличии;
- facts before hash и report hash обязательны;
- knowledge versions уникальны;
- proposal IDs уникальны;
- verification checks уникальны;
- verification содержит минимум `build`;
- codes нормализованы, без raw messages;
- если любой обязательный check failed, `final_status` должен быть `failed` или `blocked`;
- `stable` запрещён при failed verification;
- facts after hash допустимо не заполнять при failed build/extraction.

### 10.4. Canonicalization

- knowledge versions сортируются;
- proposal decisions сортируются по proposal ID;
- verification сортируется по check;
- codes внутри verification сортируются/deduplicate;
- client timestamps не входят в canonical outcome;
- server при хранении может добавить received timestamp как storage metadata, но rules не должны считать timestamp доказательством.

Run ID создаётся в ANG через `crypto/rand`, без внешней зависимости:

```text
"run-" + 16 random bytes encoded as lowercase hex
```

Нельзя использовать постоянный `ang.advise.v1` как run ID.

## 11. Outcome storage в Expert

### 11.1. Package

Добавить:

```text
outcomes/store.go
outcomes/memory.go
outcomes/jsonl.go
```

Interface:

```go
type Store interface {
    Append(context.Context, api.OutcomeRequest) error
    List(context.Context, string) ([]api.OutcomeRequest, error)
}
```

`List` принимает exact scope ID. Запрос без scope запрещён, чтобы случайно не смешать проекты.

### 11.2. In-memory implementation

Нужна для unit tests. Требования:

- mutex;
- canonical copy на входе и выходе;
- idempotency по `run_id`;
- повторная запись идентичного canonical outcome успешна;
- тот же run ID с другим payload возвращает conflict error;
- порядок List — порядок append.

### 11.3. JSONL implementation

Нужна для локального `expert-server`.

Требования:

- parent directory создаётся с безопасными permissions;
- файл открывается как append-only с `0600`;
- одна canonical JSON object на строку;
- append защищён mutex;
- после append выполняется `Sync`;
- при startup существующие строки валидируются и индексируются по run ID;
- последняя оборванная строка не должна молча считаться валидным outcome;
- corrupted middle line должна давать startup error;
- никаких исходников и raw logs в записи;
- одинаковый run ID obeys те же idempotency rules.

Не добавлять SQL/database dependency в первый vertical slice.

### 11.4. HTTP endpoint

Добавить:

```text
POST /v1/outcomes
```

Transport body limit: 1 MiB.

Runtime interfaces:

```go
type Analyzer interface {
    Analyze(context.Context, api.AnalyzeRequest) (api.AnalyzeResponse, error)
}

type OutcomeRecorder interface {
    RecordOutcome(context.Context, api.OutcomeRequest) (api.OutcomeResponse, error)
}
```

`runtime.Service` получает optional `OutcomeStore outcomes.Store`.

Если store nil:

- `/v1/analyze` продолжает работать;
- `/v1/outcomes` возвращает service unavailable;
- нельзя отвечать `accepted=true` без реального хранения.

`cmd/expert-server` получает flag:

```text
--outcome-store /path/to/outcomes.jsonl
```

Если flag не задан, storage выключен явно. README должен предупреждать, что накопление отключено.

`cmd/expert-runtime` через one-shot stdio в первой версии остаётся analyze-only. Нельзя изображать persistent memory в процессе, который завершается после одного запроса.

## 12. Shadow integration в ANG

### 12.1. Сначала отдельная команда

До изменения `ang build` добавить диагностическую команду:

```text
ang pp expert [path]
  --mode shadow
  --expert-base-url http://127.0.0.1:8787
  --expert-pack payment-provider.core
  --json
```

Рекомендуемые файлы:

```text
cmd/ang/pp_expert.go
cmd/ang/expert_client.go
```

Не копировать HTTP validation ещё раз. Вынести общий клиент из `expert_provider.go` так, чтобы существующий `ang advise` сохранил поведение.

Новый client API должен принимать canonical facts bytes напрямую, а не требовать временный facts file:

```go
type ExpertClientConfig struct {
    BaseURL string
    Timeout time.Duration
}

func Analyze(ctx context.Context, cfg ExpertClientConfig, request AnalyzeRequest) (ValidatedReport, error)
func RecordOutcome(ctx context.Context, cfg ExpertClientConfig, outcome OutcomeRequest) error
```

Для нового API использовать base URL и явно добавлять `/v1/analyze` или `/v1/outcomes`. Существующий `--expert-url`, который принимает полный analyze endpoint, нельзя молча переопределять.

Security URL policy для первого релиза:

- разрешить `http://127.0.0.1`, `http://localhost`, `http://[::1]`;
- для non-loopback требовать `https`;
- remote auth/TLS configuration является отдельной задачей;
- redirects лучше запретить или повторно проверять destination.

Команда `ang pp expert`:

1. извлекает canonical provider facts;
2. отправляет goal `payment_provider.audit`;
3. локально валидирует report и hashes;
4. печатает findings;
5. ничего не пишет в provider project;
6. не вызывает outcome, потому что генерации не было.

### 12.2. Затем hook в `ang build`

Добавить в `OutputOptions`:

```go
ExpertMode    string
ExpertBaseURL string
ExpertPackIDs []string
```

Flags:

```text
--expert-mode off|shadow|advise|gate
--expert-base-url URL
--expert-pack ID   # repeatable
```

Default `off`.

Validation:

- `shadow`/`advise`/`gate` требуют base URL;
- `off` не делает network/process calls;
- watch mode: каждый build получает новый run ID; outcome не должен переиспользоваться;
- timeout shadow-вызова ограничен, например 10 seconds.

Payment-provider ветку `cmd/ang/build.go` лучше вынести в helper, возвращающий result вместо раннего печатания/return из нескольких мест:

```go
type paymentProviderBuildResult struct {
    BuildStatus string
    ErrorCode   string
}
```

Нельзя менять success/failure semantics существующего build в mode `off`.

Shadow orchestration:

1. Попытаться extract facts before.
2. Если получилось — Analyze.
3. Любая ошибка extraction/analyze/report validation записывается как local expert warning; build продолжается.
4. Выполнить обычный `paymentprovider.Build` ровно один раз.
5. Сформировать verification `build=passed|failed`.
6. Повторно извлечь facts after, если возможно.
7. Сформировать outcome.
8. Попытаться `POST /v1/outcomes`.
9. Ошибка записи outcome в shadow не меняет exit status build.

Важно: если build failed, outcome всё равно полезен и должен быть отправлен, если before facts/report были получены.

### 12.3. Report hash

ANG вычисляет report hash только через существующий:

```go
compiler/expert.Hash(report)
```

Не вычислять hash от pretty-printed или необработанного HTTP body.

### 12.4. Facts hashes

ANG вычисляет SHA-256 только от canonical facts bytes.

Expert Runtime продолжает возвращать hash именно request facts bytes. Поэтому ANG обязан отправлять canonical bytes, а не произвольную сериализацию equivalent object.

### 12.5. Final status в shadow

Минимальное правило:

```text
build failed                         -> failed
report blocked                      -> blocked
build passed, findings/proposals    -> advice
build passed, no findings/proposals -> stable
```

Недоступность outcome store не превращает успешный build в failed; выводится warning `EXPERT_OUTCOME_NOT_RECORDED`.

## 13. Исправление payment-provider dry-run и test verification

Это отдельный prerequisite перед advise/apply, но его можно сделать параллельно после working shadow loop.

### 13.1. Dry-run

При `--dry-run` payment-provider branch должна передавать:

```go
paymentprovider.BuildOptions{
    OutputDir: isolatedTempDir,
}
```

Нельзя удалять или очищать реальный provider root.

Dry-run comparison должен включать только generator-owned paths. Manual sidecars не должны считаться output, который генератор вправе удалить.

Если emitter пока не возвращает manifest, добавить новый API без поломки старого:

```go
func BuildWithResult(opts BuildOptions) (BuildResult, error)
func Build(opts BuildOptions) error // wrapper для compatibility
```

`BuildResult` может содержать relative generated paths и их SHA-256.

### 13.2. Tests

`--run-tests` для payment provider должен быть opt-in и выполняться после успешной генерации.

Outcome записывает:

```text
go_test=passed|failed|skipped
```

В outcome нельзя передавать raw stdout/stderr тестов. Разрешены стабильные diagnostic codes, например:

```text
GO_TEST_FAILED
GO_BUILD_FAILED
```

Тесты должны запускаться в корректном Go module root, а не предполагать, что provider package является отдельным module.

## 14. Использование накопленной истории

Первый milestone только записывает outcomes. Второй milestone начинает безопасно использовать агрегаты.

### 14.1. Связь с `memory.History`

Для каждого outcome строить `memory.Observation`:

```text
Scope          = outcome.ScopeID
Outcome        = mapping(outcome.FinalStatus)
BlockingKeys   = blocking finding codes
UnresolvedKeys = non-blocking finding codes
```

History bounded capacity задаётся deployment configuration, например 100 последних runs. Storage при этом остаётся append-only; bounded history является вычисляемым представлением, а не удалением evidence.

### 14.2. Что Expert может вывести

- повторяющийся `PP_OPERATION_INTENT_MISMATCH`;
- несколько failed builds после одного и того же rule/proposal;
- stable run streak;
- blocked run streak;
- oscillation риска;
- предложение уже было отклонено или не reviewed.

### 14.3. Что Expert не должен делать

- скрывать finding только потому, что его раньше отклоняли;
- считать build pass доказательством корректности business logic;
- увеличивать logical truth confidence из статистики;
- переносить outcome одного provider SID в другой;
- считать live test skipped успешной проверкой.

History влияет на ranking/policy explanation, а не переписывает observed facts.

## 15. Pilot: `incas_n692`

Целевой provider:

```text
/Users/m.ramanchak/Desktop/transferty/tnx_processor/payment_providers/incas_n692
```

Известное baseline-состояние, которое необходимо повторно подтвердить перед реализацией:

- provider CUE использует `schema.ProfileIncasPayout`;
- `has_payout=true` получается из intent/profile;
- Go содержит настоящую реализацию `InitPayout`;
- CUE `operations` не содержит явный `init_payout`;
- `ang pp vet` сообщает warning `PP009`;
- schema check сообщает drift bundled schema;
- Go содержит `rsa.EncryptOAEP` для card payload;
- Go содержит `rsa.VerifyPKCS1v15` для callback signature;
- есть behavior/callback tests;
- уникальная криптография должна оставаться manual/protected до отдельного sidecar contract.

### 15.1. Ожидаемые факты

Минимально:

```text
pp_provider(provider=n692)
pp_capability(provider=n692, capability=payout, enabled=true)
pp_operation(provider=n692, operation=init_payout,
             declaration=absent, implementation=implemented)
pp_schema_sync(provider=n692, state=drift)
pp_vet_issue(provider=n692, code=PP009, severity=warning)
pp_behavior(provider=n692, behavior=rsa_oaep_card_encryption)
pp_behavior(provider=n692, behavior=rsa_pkcs1v15_callback_verification)
pp_test_area(provider=n692, area=behavior, present=true)
pp_test_area(provider=n692, area=callback, present=true)
```

Не фиксировать в golden test полный facts JSON, если он содержит большой список независимых facts. Лучше проверять canonical determinism и наличие/отсутствие конкретных семантических фактов.

### 15.2. Ожидаемые findings

```text
PP_OPERATION_INTENT_MISMATCH
PP_SCHEMA_DRIFT
```

Нельзя требовать proposal в первом shadow milestone.

### 15.3. Ожидаемый outcome

При успешном build и наличии findings:

```text
final_status=advice
verification.build=passed
proposal_decisions=[]
```

Повторный run создаёт другой run ID, но тот же facts hash при неизменном intent/implementation. Store должен сохранить оба runs и memory должна увидеть recurring unresolved keys.

## 16. Тестовый план

### 16.1. ANG unit tests

Добавить минимум:

```text
compiler/paymentprovider/facts/model_test.go
compiler/paymentprovider/facts/canonical_test.go
compiler/paymentprovider/facts/extract_test.go
cmd/ang/pp_facts_test.go
cmd/ang/pp_expert_test.go
cmd/ang/expert_client_test.go
```

Проверки:

1. canonical facts одинаковы при разном порядке input slices;
2. duplicate IDs rejected;
3. unknown predicates rejected;
4. absolute paths нигде не появляются;
5. secret values не могут попасть в model;
6. stub `ErrNotImplemented` не classified implemented;
7. receiver method другого struct не считается operation provider;
8. aliased `crypto/rsa` import распознаётся;
9. строка `"rsa.EncryptOAEP"` в comment/string не считается behavior;
10. schema drift передаёт только allowed basename;
11. invalid Expert facts hash rejected;
12. invalid report schema/request ID rejected;
13. shadow Expert failure не меняет build result;
14. mode off не вызывает HTTP;
15. non-loopback HTTP URL rejected;
16. outcome содержит только codes, без raw errors/logs.

### 16.2. Expert unit tests

Добавить минимум:

```text
api/v1/outcome_test.go
outcomes/memory_test.go
outcomes/jsonl_test.go
runtime/payment_provider_facts_test.go
runtime/outcomes_test.go
transport/outcomes_http_test.go
```

Проверки:

1. provider facts schema dispatch;
2. existing `ang/facts/v1` adapter не сломан;
3. arity > 4 rejected;
4. unknown predicate rejected;
5. malformed hash rejected;
6. outcome canonicalization deterministic;
7. duplicate run same payload idempotent;
8. duplicate run different payload conflict;
9. JSONL survives reopen;
10. corrupted JSONL detected;
11. nil store gives service unavailable;
12. body limit enforced;
13. GET on outcomes gives 405;
14. pack manifest reads exactly used predicates;
15. first PP rules produce expected findings;
16. PP pack emits no proposals in v1.

### 16.3. Integration tests

1. Запустить Expert server с `payment-provider.core` и temporary outcome store.
2. Получить facts через `ang pp facts`.
3. Вызвать `ang pp expert --mode shadow`.
4. Проверить report hash/facts hash.
5. Запустить build с shadow hook.
6. Проверить ровно одну outcome JSONL запись.
7. Повторить и проверить две записи с разными run IDs.
8. Остановить Expert server и убедиться, что shadow build сохраняет прежний exit status.
9. Сравнить generated manifest/hash в `off` и `shadow`: они должны совпадать.

### 16.4. Команды проверки

Перед командами соблюдать MCP bootstrap из `AGENTS.md`.

ANG:

```bash
cd /Users/m.ramanchak/Desktop/work/ang
go test ./compiler/paymentprovider/... ./cmd/ang -count=1
go test ./... -count=1 -timeout=240s
```

Expert:

```bash
cd /Users/m.ramanchak/Desktop/work/deal/expert
go test ./... -count=1 -timeout=120s
```

Target provider baseline, из Go module root:

```bash
cd /Users/m.ramanchak/Desktop/transferty/tnx_processor
go test ./payment_providers/incas_n692 -count=1
```

Не заменять target test запуском тестов только самого ANG generator.

## 17. Пошаговый порядок pull requests/commits

Не смешивать все изменения в один commit.

### Шаг 1. Expert outcome contract

- API types;
- validation/canonicalization;
- tests;
- без HTTP/storage.

Критерий: API tests проходят, runtime не изменён.

### Шаг 2. Expert stores и endpoint

- memory store;
- JSONL store;
- `runtime.OutcomeRecorder`;
- `/v1/outcomes`;
- server flag;
- tests/readme.

Критерий: outcomes реально переживают restart JSONL store.

### Шаг 3. ANG provider facts

- model/validation/canonicalization;
- extractor;
- `ang pp facts`;
- tests на synthetic fixture и Incas-like fixture.

Критерий: два запуска на неизменном дереве дают byte-identical canonical JSON.

### Шаг 4. Expert provider adapter и pack

- schema dispatch;
- provider facts adapter;
- `payment-provider.core`;
- findings-only rules;
- tests.

Критерий: existing security audit и новый provider audit проходят вместе.

### Шаг 5. ANG read-only `ang pp expert`

- reusable client;
- base URL validation;
- goal `payment_provider.audit`;
- local report validation;
- no writes/outcomes.

Критерий: Incas produces expected findings; repository diff пустой.

### Шаг 6. Shadow hook и outcomes

- build flags;
- before/report/build/after/outcome lifecycle;
- fail-open shadow behavior;
- tests.

Критерий: `off` и `shadow` производят идентичные generated outputs; outcome stored.

### Шаг 7. Payment-provider dry-run/test parity

- isolated OutputDir;
- generated manifest;
- opt-in test result;
- outcome verification.

Критерий: dry-run не меняет target tree; `--run-tests` действительно запускает provider package tests.

### Шаг 8. History-aware ranking

- derive bounded history from outcome store;
- expose explanation in report diagnostics/trace;
- не менять truth автоматически.

Критерий: recurring issue виден, но finding не исчезает и pack не изменяется.

### Шаг 9. Typed proposal target/report v2

- отдельный design review;
- typed `project_cue_root` target;
- containment + BeforeHash + approval + sandbox;
- только после этого `advise` для payment providers.

Критерий: proposal не может адресовать Go/generated/outside-root файл.

## 18. Definition of Done первого vertical slice

Первый vertical slice завершён только если одновременно истинно:

- `ang pp facts` создаёт canonical `ang/payment-provider-facts/v1`;
- Expert понимает обе facts schemas;
- payment-provider pack выдаёт findings без proposals;
- Expert server сохраняет valid outcomes;
- `ang build --expert-mode shadow` не меняет результат генерации;
- недоступный Expert не ломает shadow build;
- invalid Expert response не принимается как valid advice;
- Incas build создаёт outcome;
- повторный Incas build виден как отдельное наблюдение того же scope;
- в facts/outcome нет source code, secrets, absolute paths и raw logs;
- все новые и существующие тесты проходят;
- unrelated пользовательские изменения не попали в commit.

## 19. Антипаттерны

Запрещено:

- вызывать LLM из template renderer;
- позволять Expert возвращать готовый Go source;
- кодировать provider facts как фиктивные domain entities;
- считать отсутствие engine fact значением false;
- анализировать crypto/retry простым grep и объявлять это known truth;
- отправлять raw diagnostics или test logs на remote Expert;
- хранить outcomes в ANG repository;
- автоматически коммитить outcomes;
- автоматически менять knowledge pack после успешного build;
- считать успешную компиляцию доказательством корректности платёжного протокола;
- расширять V1 path validator до произвольных `.cue` файлов;
- применять proposals в shadow;
- делать Expert обязательным dependency для обычного `ang build`;
- проглатывать invalid report и записывать его как accepted outcome;
- утверждать, что payment-provider dry-run/tests работают, пока ранний return не исправлен.

## 20. Следующее продуктовое развитие после vertical slice

После накопления outcomes минимум по нескольким реальным providers можно принимать решения на данных:

1. какие повторяющиеся manual patterns заслуживают CUE abstraction;
2. какие operations чаще всего расходятся между intent и Go;
3. какие generator changes дают regression;
4. какие proposals стабильно проходят build/tests;
5. какие provider-specific особенности должны остаться sidecar;
6. какие knowledge packs полезны для конкретной организации.

Платная hosted-система может добавить:

- authenticated organizations/scopes;
- PostgreSQL outcome store;
- signed/versioned corporate packs;
- aggregate dashboards;
- candidate rule mining;
- policy/ranking profiles;
- audit retention.

Но wire boundary остаётся прежней: ANG отправляет факты и outcomes, Expert возвращает объяснимые findings/proposals, а право изменять и проверять проект остаётся у локального ANG.
