# ANG: рефакторинг Flow IR и направление развития

Дата: 2026-07-13. Документ описывает текущее состояние ветки `stage`. Это не опубликованный API contract, но раздел «Принятое стратегическое решение» является нормативным архитектурным roadmap для последующих реализаций.

## Что было отрефакторено

Главное изменение — генерация service flow постепенно перестаёт работать с неструктурированным `FlowStep.Args map[string]any`.

Теперь путь выглядит так:

```text
CUE flow → Flow IR registry/decode → TypedStep + конкретный Action → validator/checker → emitter
```

Для действий уже существуют отдельные Go-типы, а production emitter получает `flowir.TypedStep` и берёт значения из `Action`, `Children` и `Branches`. Legacy-представление пока остаётся адаптером только для ещё не перенесённых действий.

На текущем этапе на typed route переведены, в частности:

- core и data: mapping, repo, service/logic/flow calls, state, cache, storage, mail, config/secret, HTTP, JSON, строки, шаблоны, преобразования, время, UUID/ULID, hash;
- security: JWT, одноразовые токены, OAuth2 token/refresh, encrypt/decrypt/hash;
- delivery: `parallel.Run`, PDF, webhook, queue и DLQ;
- orchestration: notify, approval, outbox и event wait/subscribe/match/broadcast;
- policy: check/evaluate/require/decide;
- reliability: idempotency, dedupe, rate/quota/budget, mutex/concurrency, circuit/bulkhead, log/metric/trace/SLO;
- Google OAuth.

Вложенные действия и ветки (`Children`, `Branches`) для новых групп рендерятся из typed IR, а не вытягиваются обратно из `map[string]any`.

Добавлены регрессионные тесты: в typed step намеренно кладутся конфликтующие legacy `ScalarArgs`; тесты подтверждают, что emitter использует только typed action. Полный набор `go test ./compiler/...` проходит.

## Зачем это нужно

Раньше одно и то же действие описывалось и интерпретировалось в нескольких местах: CUE schema, decoder, validator и emitter. `map[string]any` не выражает обязательность поля, его тип и вложенную структуру. Поэтому ошибка могла проявиться лишь в сгенерированном Go или, хуже, во время выполнения приложения.

Typed Flow IR даёт практические преимущества:

- добавление action становится контролируемым: тип, decoder, checker и renderer должны согласоваться;
- обязательные поля и простые ошибки конфигурации ловятся раньше;
- меньше type-switch и неявных преобразований в emitter;
- renderer проще тестировать без CUE-парсинга;
- вложенные flow-блоки становятся явной частью модели;
- в перспективе можно проверять совместимость переменных и сигнатур до генерации Go.

Важно: это не требует переписывать существующие проекты ANG. Совместимость обеспечивается тем, что старый CUE flow декодируется в typed IR; миграция находится внутри компилятора.

## Что ещё не завершено

Это направленный рефакторинг, а не завершённая типовая система. В коде ещё есть legacy renderer families, которые вызываются через compatibility adapter. Их нужно переносить последовательно, не смешивая это с изменением поведения actions.

Также важно не создавать ложного ощущения полной compile-time безопасности: сейчас типы action и ранняя проверка уже заметно снижают риск, но полноценная проверка всех выражений, переменных и внешних сигнатур пока должна быть доведена отдельно.

## Сильные стороны ANG

1. CUE как слой намерения. Он хорошо подходит для декларативного описания домена, сервисов, политик и генерации нескольких артефактов из одной модели.

2. Широкий генераторный охват. ANG умеет связывать backend, API, SDK, схему БД, transport и инфраструктурные возможности. Это сильнее, чем просто CRUD-генератор.

3. Flow DSL. В нём уже выражены реальные прикладные сценарии: транзакции, вызовы, события, очереди, policy, reliability и интеграции.

4. Накопленная инженерная защита. В `docs/improvements.md` уже появились атомарная генерация, диагностики, doctor, проверки идемпотентности и другие механизмы доверия к генератору.

5. Хорошая точка для AI-assisted development. Декларативный, машиночитаемый intent плюс structured diagnostics позволяют агентам не «угадывать» структуру проекта, а работать по контракту.

## Слабые стороны и риски

1. Слишком большая поверхность DSL. Много actions и интеграций делают документацию, тесты и поддержку сложными. Без строгого registry-контракта action легко получает дрейф между schema, validation и emitter.

2. Legacy-переходный слой. Пока часть генерации опирается на совместимость со старым `FlowStep`, сложность остаётся выше желаемой. Его надо уменьшать метрикой и не расширять.

3. Неочевидный уровень абстракции. ANG одновременно пытается быть языком архитектуры, backend-платформой, workflow engine и генератором frontend/SDK. Если не выбрать первичный продуктовый фокус, новые возможности будут размывать ядро.

4. Риск «генерирует, но не объясняет». Большой объём generated code требует отличных трассировок: от строки CUE до конкретного Go-файла, DI wiring и runtime capability. Иначе debugging дорогой.

5. Валидация ещё не полностью семантическая. Особенно важны сигнатуры `service.Call`, `logic.Call`, `repo.*`, типы выражений и гарантии output variables.

6. Тестовая стоимость. Чем больше runtime-capabilities и templates, тем дороже матрица интеграционных тестов. Нужны contract tests для каждого action и несколько реальных fixture-проектов, а не только unit-тесты emitter.

## Вопросы, на которые нужно ответить до следующего большого этапа

### 1. Какой основной продукт ANG?

Вопрос: ANG в первую очередь internal platform для одной команды, open-source framework или коммерческий product?

Как решить: сформулировать один основной ICP (например, команда из 3–15 Go-разработчиков, строящая бизнес-backend) и три задачи, которые ANG решает для него лучше ручной разработки. Все новые actions сравнивать с этим фокусом.

### 2. Где граница DSL и обычного Go?

Вопрос: какие сценарии обязаны быть декларативными, а какие нужно сразу выносить в typed sidecar Go?

Как решить: оставить в DSL orchestration, domain contracts, policy и стандартные интеграции; сложные алгоритмы, нестандартные SDK и тяжёлую предметную логику делать sidecar-функциями с явными сигнатурами. Это удержит Flow DSL от превращения во второй Go.

### 3. Нужна ли гарантия «не сгенерируем некорректный Go»?

Вопрос: это обязательное свойство компилятора или best effort?

Как решить: принять его как целевой инвариант для поддерживаемого DSL. Добавить этап semantic checking: symbol table для variables, типы input/output, сигнатуры calls, проверка repo entity и capability availability. После этого запускать compile-check сгенерированного проекта на fixture и реальных эталонных проектах.

### 4. Как управлять развитием actions?

Вопрос: кто и по каким критериям добавляет новый action?

Как решить: сохранить один канонический semantic contract в `ang-ir/flowsem` и сделать обязательными проверяемые bindings в `compiler/flowir`, emitter и публичном action catalog. Не следует складывать decoder, checker и renderer в один Go-пакет: это создаст циклы импортов. Для action обязательны: тип action, CUE schema, type-check rules, golden output, negative tests, documentation и версия/compatibility policy.

### 5. Как доказать доверие к генератору на реальных проектах?

Вопрос: что является release gate для ANG?

Как решить: поддерживать 2–3 representative fixture-проекта и минимум один реальный проект в CI (если это допустимо). Gates: `validate`, повторная генерация без diff, build generated project, unit/integration tests, проверка migration diff и doctor report.

### 6. На чём концентрировать runtime platform?

Вопрос: поддерживать широкий набор queues, AI, OAuth, files, mail, policy и т. д. одновременно или выбрать reference stack?

Как решить: определить opinionated default stack и capability interfaces. Всё остальное — plugins/providers с чётким статусом experimental/stable. Это уменьшит стоимость поддержки и DI-дрейф.

### 7. Какой DX должен быть у автора CUE?

Вопрос: может ли разработчик найти и исправить проблему без чтения сгенерированного Go?

Как решить: развивать LSP/diagnostics как продуктовую функцию: CUE path, действие, ожидаемый и фактический тип, suggested fix, ссылка на documentation. `ang doctor`, `ang validate --json` и code actions должны стать обычным ежедневным интерфейсом.

## Предлагаемое направление на ближайшие этапы

1. Завершить миграцию remaining legacy renderer families на `TypedStep` и запретить добавлять новые legacy actions.

2. Усилить Flow checker: таблица переменных, строгие input/output типы, проверка сигнатур `service.Call`, `logic.Call`, `repo.*`, затем понятные CUE diagnostics.

3. Сделать registry единым источником метаданных action: schema, validator, emitter dispatch, documentation и тестовые fixtures.

4. Ввести compatibility policy для CUE DSL: stable actions, deprecated actions, migration diagnostics и версия manifest.

5. Поставить доверие к генерации в release gate: atomic build, `generate × 2`, compile generated Go, golden diffs и проверка реальных fixtures.

6. После стабилизации ядра выбрать 1–2 вертикальных сценария, где ANG особенно силён (например, B2B backend с RBAC/policy, событиями и API/SDK), и довести их до эталонного качества вместо расширения DSL во все стороны.

## Моя позиция

У ANG есть шанс стать не «генератором всего», а надёжным compiler/platform для бизнес-систем, где архитектурное намерение важнее ручной сборки boilerplate. Его конкурентное преимущество — единая модель домена и поведения, из которой получаются согласованные контракты и реализации.

Для этого важнее всего сделать компилятор предсказуемым: меньше магии, строгий typed IR, прозрачная диагностика, стабильные capabilities и доказуемая повторяемость генерации. Новые интеграции стоит добавлять после того, как этот контур станет рутинно надёжным.

---

## Принятое стратегическое решение: ANG как ядро инженерной экспертной системы

Этот раздел является развёрнутым ответом на вопросы выше и техническим ориентиром для последующей реализации. Если краткое описание выше допускает разные трактовки, нужно следовать этому разделу.

### Что именно строим

ANG не должен становиться недетерминированным AI-агентом, который напрямую пишет произвольный Go-код. Целевая роль ANG — детерминированное, проверяемое ядро экспертной системы для проектирования и эволюции бизнес-приложений.

Экспертная система должна уметь:

1. Собрать факты о существующем проекте и CUE intent.
2. Сохранить происхождение каждого нетривиального факта.
3. Применить версионируемые инженерные правила и knowledge packs.
4. Найти проблемы, противоречия и недостающие сведения.
5. Объяснить каждое заключение через факты и применённые правила.
6. Предложить структурированный план изменений CUE.
7. Проверить план существующим ANG compiler pipeline.
8. Потребовать подтверждение человека для рискованных изменений.
9. Никогда не выдавать предположение за подтверждённый факт.

Целевой поток данных:

```text
Исходный код / OpenAPI / SQL / CUE intent
                    │
                    ▼
       детерминированное извлечение фактов
                    │
                    ▼
       FactSet + Evidence + Provenance
                    │
                    ▼
     versioned Knowledge Packs / Rules
                    │
                    ▼
    deterministic inference + conflict check
                    │
                    ▼
  Findings + Explanation + Typed Change Proposal
                    │
                    ▼
      применение в изолированной копии CUE
                    │
                    ▼
 validate → flowsem → Flow IR checker → build → tests
                    │
                    ▼
       Verified Decision Report + diff
                    │
                    ▼
          подтверждение и применение
```

### Что уже существует и должно быть переиспользовано

Нельзя создавать параллельные реализации уже существующих механизмов.

- Текущий compiler pipeline описан в `docs/architecture.md` и остаётся авторитетным: `CUE -> Parser -> Normalizer -> FlowSem -> IR -> Emitters`.
- Существующий формат извлечённых данных — `ang/facts/v1`; его Go-модели сейчас находятся в `cmd/ang/extract.go` (`FactsEnvelope`, `FactEntity`, `FactOp`, `FactField` и другие).
- `FactField`, `FactOp` и `FactEndpoint` уже содержат часть provenance: `source`, `source_line`, `extractor`, `evidence`.
- Typed Flow IR находится в `compiler/flowir`. `ActionSpec`, `TypedStep`, `Action`, `TypeRef` и `checker` должны использоваться как семантическая граница flow, а не копироваться в экспертный слой.
- Семантика эффектов и допустимости flow уже реализована пакетами `github.com/strogmv/ang-ir/flowsem` и `github.com/strogmv/ang-ir/effects`. В текущем workspace `go.mod` заменяет `github.com/strogmv/ang-ir` на `../deal/ang-ir`. Документация иногда называет их `compiler/flowsem`/`compiler/effects`, но создавать такие новые локальные пакеты в ANG нельзя: это породит второй semantic catalog. Экспертные правила могут ссылаться на результаты `ang-ir`, но не должны заново определять эти правила.
- CUE pack schema уже начата в `cue/schema/packs.cue`; активные примеры находятся в `cue/policies/packs.cue`.
- Структурированные diagnostics и `SuggestedFix` уже создаются, например, в `compiler/pack_diagnostics.go`.
- Пользовательское объяснение diagnostics уже доступно через `ang explain` (`cmd/ang/explain.go`).
- `ang validate`, `ang lint`, `ang doctor`, atomic build, manifest compatibility и повторная генерация уже образуют verification foundation.
- `plan.BuildAutomata`, `plan.BuildMicroPlan` и последующие meta-actions существуют, но должны быть укреплены и не являются заменой общей модели экспертного решения.

### Существующие соседние механизмы: что использовать, а что не считать готовым экспертным ядром

Для слабой модели особенно важно не принять прототип или одноимённый тип за завершённую реализацию нужного слоя.

1. `ang-transform` — источник extractors, а не inference engine. `cmd/ang/extract.go` вызывает `github.com/strogmv/ang-transform/pkg/transform`. Нельзя переносить extraction logic обратно в ANG без отдельной архитектурной причины.

2. Java import уже содержит `high/medium/low confidence`, conflicts, TODO и verification report в `cmd/ang/import.go`. Это полезный опыт, но не единый экспертный confidence contract: оценки локальны для import pipeline и сейчас не прослеживаются до diagnostics/patch/artifact.

3. `compiler/plan/types.go` уже содержит `BuildPlan`, `Preconditions`, `PlanStep` и `FileChange`. Этот контракт нужно изучить перед созданием `expert.Proposal`, чтобы не дублировать применимые части. При этом текущий plan/apply pipeline ещё не является полноценным change planner: часть diff/change filling остаётся scaffold/placeholder, а apply в первую очередь проверяет preconditions/hashes.

4. `compiler/planner/contracts.go` содержит emitter-facing планы (`ScanPlan`, `RenderPlan`, `RoutePlan`, `ServicePlan` и другие). Это планы рендеринга, а не экспертные Goal/Decision/ChangePlan. Их нельзя расширять предметными inference rules только из-за слова `Plan` в названии.

5. `ang ops vet --proof --json` уже формирует `ang/proof/v1` и различает `proved`, `violated`, `unknown`. Существующие properties включают auth-before-write, validation-before-write, compensation и event atomicity. Экспертный report должен адаптировать эти proof results, а не заново вычислять их.

6. `ang ops context` уже объединяет operation schema, actions, diagnostics/explain и proof. Это хороший источник bounded context для модели. Перед использованием нужно устранить возможный drift: некоторые поверхности используют сырой `flowsem.ActionCatalog`, тогда как `ang actions` объединяет `flowsem` и `flowir` metadata.

7. `cmd/ang/ai_gen.go` уже отправляет `ang/facts/v1` в Claude и получает CUE. Это migration prototype, а не безопасная основа автономного expert planner: между ответом модели и записью отсутствует обязательная цепочка `hypothesis → evidence → typed proposal → sandbox verification → approval`.

8. MCP goal planner существует как прототип и распознаёт ограниченный набор keywords. Его hardcoded marketplace/Stripe/webhook/email эвристики нельзя объявлять общей базой знаний. Кроме того, старый `ang_do`/plan flow может иметь permissive `auto_apply` default; экспертный интерфейс обязан быть read-only по умолчанию.

9. Runtime policy engine в `compiler/emitter/infra_policy.go` сейчас документирует поведение, при котором неизвестный policy key возвращает `allow`. Для security-sensitive экспертных решений это опасный fail-open default. Его нельзя молча переиспользовать как inference authorization; изменение runtime default требует отдельного compatibility RFC и migration diagnostics.

10. `ActionSpec` сегодня содержит главным образом `Name`, `Description`, `Args` и `Decode`. Checker, renderer dispatch, effects, examples и proof semantics всё ещё находятся отдельно. Формулировка «единый registry» ниже является целевым состоянием, а не описанием уже завершённой реализации.

### Чёткая граница ответственности

Экспертный слой отвечает за анализ и предложение изменения. Compiler отвечает за истинность своих контрактов и генерацию.

| Компонент | Разрешённая ответственность | Запрещённая ответственность |
|---|---|---|
| Extractors | Наблюдать код/spec/schema и выдавать факты | Додумывать отсутствующие сущности |
| Knowledge packs | Описывать предметные правила и ожидаемые свойства | Генерировать Go-код |
| Inference engine | Сопоставлять факты и правила, строить findings | Повторять flowsem или emitter semantics |
| Planner | Строить typed change proposal для CUE | Мутировать generated directories |
| LLM adapter | Предлагать гипотезы или выбирать среди допустимых вариантов | Объявлять гипотезу фактом, обходить validation |
| ANG compiler | Валидировать intent и генерировать артефакты | Скрыто применять предметные эвристики |
| Human approval | Подтверждать рискованные или неоднозначные решения | Не требуется только для явно безопасного read-only анализа |

## Обязательные архитектурные инварианты

Слова MUST, MUST NOT, SHOULD и MAY ниже используются в нормативном смысле.

1. Экспертная система MUST сохранять CUE как единственный изменяемый source of truth ANG-проекта.
2. Она MUST NOT напрямую редактировать `internal/`, `api/`, `sdk/`, `db/schema/` и `db/queries/`.
3. Она MUST NOT добавлять бизнес-эвристики в emitter, templates или generated code.
4. Она MUST использовать существующие `flowsem`, `effects` и `flowir.Check` для проверки flow.
5. Каждое заключение MUST содержать `rule_id` и хотя бы одну ссылку на факт/evidence либо явно указывать, что это compiler invariant.
6. Отсутствие наблюдения MUST NOT интерпретироваться как ложь. Значения должны поддерживать состояния `known`, `unknown`, `conflict`.
7. План изменения MUST быть структурированным; произвольный текст или готовый Go-код не считается планом.
8. Любое изменение MUST сначала применяться в изолированной временной копии и проходить verification pipeline.
9. Результат MUST быть детерминирован для одинаковых facts, intent, knowledge pack versions и compiler version.
10. LLM output MUST считаться `hypothesis`, пока он не подтверждён facts, rule constraints и compiler verification.
11. Security, auth, data deletion, DB migration и breaking API changes MUST требовать human approval независимо от confidence.
12. Новая экспертная функциональность MUST иметь versioned JSON contract и golden tests.
13. Экспертный слой MUST NOT создавать второй catalog действий flow.
14. Экспертный слой MUST NOT выполнять произвольные Go/CUE expressions из rules как код процесса.
15. Любой автоматический fallback MUST порождать diagnostic; молчаливые fallback запрещены.

## Ответы на стратегические вопросы

### 1. Основной продукт ANG

Принятое направление: ANG — engineering expert compiler для команд, создающих бизнес-системы на Go и CUE.

Первичный пользователь:

- команда из 3–20 разработчиков;
- несколько сервисов или модульный backend;
- API, persistence, authorization, events и integrations;
- высокая цена несовпадения contracts, DI и generated artifacts;
- желание использовать AI без передачи ему права бесконтрольно менять код.

Три основные задачи продукта:

1. Компилировать архитектурное намерение в согласованные артефакты.
2. Находить архитектурные и семантические проблемы до runtime.
3. Предлагать проверяемые изменения проекта с объяснением и доказательствами.

На первом этапе ANG SHOULD развиваться как internal platform, проверяемая на реальных проектах. Публичный framework или коммерческий продукт — следующая продуктовая форма, но не отдельная архитектура.

### 2. Граница DSL и Go

В CUE/Flow DSL следует оставлять:

- domain entities, DTO и contracts;
- service operations и orchestration;
- authorization, policy, effects и capabilities;
- repository intent;
- events, queue, notification и reliability intent;
- декларативные integration bindings;
- knowledge packs и проверяемые architecture constraints.

В обычном Go следует оставлять:

- сложные алгоритмы;
- нестандартные SDK и protocol clients;
- CPU-heavy transformations;
- предметные вычисления, которым неудобно дать небольшой декларативный контракт;
- код, требующий обычного debugger/profiler workflow.

Связь должна идти через typed sidecar function с явной сигнатурой. Нельзя добавлять десятки узкоспециализированных actions только для переноса обычного программирования в DSL.

Критерий: если поведение можно кратко описать как orchestration/effect/policy и проверить декларативно — оно подходит DSL. Если основная сложность находится внутри алгоритма — нужен Go sidecar.

### 3. Гарантия корректного generated Go

Для stable DSL принимается обязательный инвариант: валидный поддерживаемый CUE intent не должен порождать синтаксически или типово некорректный Go.

Под type/compile-time checking в этом документе понимается семантическая проверка во время компиляции ANG до emitter, а не проверка Go generics компилятором `go`. Финальный `go build` остаётся дополнительным release gate.

Это не означает, что ANG гарантирует отсутствие всех runtime bugs. Гарантируется:

- schema-valid CUE;
- успешный Flow IR decode;
- отсутствие известных semantic/type issues;
- согласованные service/repo/logic signatures;
- наличие требуемых capabilities и DI wiring;
- синтаксически валидный generated Go;
- успешный `go build` эталонного generated проекта.

Если compiler не может доказать совместимость, он должен выдать `unknown/unsupported` diagnostic, а не продолжить с небезопасным предположением.

### 4. Управление actions

Под «единым registry» здесь понимается один semantic contract и полный набор доказуемых implementation bindings, а не один Go-пакет, импортирующий все слои. Иначе emitter, Flow IR и semantic catalog создадут import cycle.

Целевая ответственность:

```text
github.com/strogmv/ang-ir/flowsem.ActionCatalogEntry
    → каноническая семантика, args/outputs/errors/effects/nested rules

compiler/flowir.ActionSpec
    → typed decoder и checker binding

compiler/emitter typed renderer binding
    → генерация кода, не импортируемая обратно в flowir

cmd/ang/actions.go
    → публичная проверенная проекция semantic contract + bindings
```

`compiler/flowir.ActionSpec` не должен становиться вторым semantic catalog. Он должен связывать canonical action name с typed decode/check implementation. Добавление action считается завершённым только при наличии:

1. CUE schema.
2. Конкретного Go-типа, реализующего `flowir.Action`.
3. Decoder с deterministic defaults.
4. Arg/type metadata.
5. Checker rules.
6. Typed emitter renderer.
7. Positive unit test.
8. Negative decode/checker test.
9. Typed-dispatch test с конфликтующими legacy `ScalarArgs`.
10. Golden/generated compile fixture.
11. User-facing documentation.
12. Stability marker и migration note при изменении контракта.

Новый action MUST NOT добавляться только в `cue/schema/types.cue` и switch emitter. CI должен запрещать registry/schema/catalog drift.

Минимальные parity checks:

- каждый stable `flowsem.ActionCatalogEntry` имеет ровно один typed decoder binding;
- каждый stable action имеет ровно один typed renderer binding;
- emitter не поддерживает неизвестный semantic catalog action;
- typed args не противоречат canonical args;
- deprecated action указывает replacement или явное обоснование;
- stability transition в `stable` невозможен без positive, negative и generated compile tests.

### 5. Доверие к генератору

Release gate ANG должен включать:

1. `ang validate`.
2. Semantic lint/checker без error diagnostics.
3. `ang build` в чистой временной директории.
4. Повторный `ang build` без смыслового diff.
5. `go build` generated backend.
6. Unit tests ANG.
7. Golden fixtures минимум для небольшого CRUD, event-driven и policy/reliability проекта.
8. Build одного реального проекта, например dealingi, если это возможно в CI.
9. Проверку manifest/schema compatibility.
10. Отчёт о deprecated actions и unknown semantics.

### 6. Runtime platform

Нужно выбрать reference stack, который имеет статус `stable`. Остальные providers должны подключаться через capabilities/adapters.

Для каждого provider требуется статус:

- `experimental`: контракт может изменяться, обязательны diagnostics;
- `stable`: compatibility guarantees и integration fixture;
- `deprecated`: работает ограниченное время и имеет migration path;
- `removed`: decoder выдаёт понятную migration error.

Expert rules не должны зависеть от конкретного provider, если решение можно выразить capability, например `queue`, `mail`, `state`, `policy_engine`.

### 7. DX автора CUE

Разработчик должен иметь возможность исправить ошибку, не читая generated Go. Каждый diagnostic SHOULD содержать:

- стабильный `code`;
- severity;
- CUE file/path/line;
- action или operation;
- фактическое значение/тип;
- ожидаемое значение/тип;
- краткую причину;
- suggested fix;
- ссылку на rule/action documentation;
- признак, можно ли применить fix автоматически;
- evidence и confidence для экспертных conclusions.

## Целевая модель данных экспертного слоя

Ниже приведён рекомендуемый контракт. Имена могут корректироваться отдельным RFC, но реализации нельзя заменять неструктурированными `map[string]any` во внутренних слоях.

### 1. Facts и evidence

Сначала существующие types из `cmd/ang/extract.go` нужно сделать импортируемыми. Рекомендуемый безопасный путь:

1. Создать новый пакет `compiler/facts`.
2. Перенести туда `FactsEnvelope`, `FactEntity`, `FactField`, `FactOp` и остальные data-only types без изменения JSON tags.
3. В `cmd/ang` временно оставить type aliases для исходной совместимости CLI/tests.
4. Сохранить чтение и запись `ang/facts/v1` без изменения JSON.
5. Только после этого добавлять `ang/facts/v2` через явный migration adapter.

Для экспертных выводов поверх structural facts нужна нормализованная модель:

```go
type TruthState string

const (
    TruthKnown    TruthState = "known"
    TruthUnknown  TruthState = "unknown"
    TruthConflict TruthState = "conflict"
)

type Evidence struct {
    ID         string   `json:"id"`
    SourceType string   `json:"source_type"`
    SourcePath string   `json:"source_path"`
    Line       int      `json:"line,omitempty"`
    Extractor  string   `json:"extractor"`
    Snippets   []string `json:"snippets,omitempty"`
    ContentHash string  `json:"content_hash"`
}

type Fact struct {
    ID          string          `json:"id"`
    Kind        string          `json:"kind"`
    Subject     string          `json:"subject"`
    Predicate   string          `json:"predicate"`
    Value       json.RawMessage `json:"value"`
    State       TruthState      `json:"state"`
    Confidence  float64         `json:"confidence"`
    EvidenceIDs []string        `json:"evidence_ids"`
}
```

Требования:

- `ID` должен вычисляться из canonical content, а не через случайный UUID.
- Порядок facts/evidence в JSON должен быть стабильным.
- `confidence` находится в диапазоне `[0, 1]`.
- `unknown` не должен иметь выдуманное value.
- конфликтующие источники должны сохраняться как conflict, а не разрешаться молча.
- абсолютные machine-specific paths не должны участвовать в content ID.
- timestamps не должны влиять на deterministic output.

### 2. Knowledge pack и rule

Knowledge должна оставаться декларативной и версионируемой. Предпочтительный source of truth — CUE schema рядом с `cue/schema/packs.cue`, а не Go-switch.

Минимальная логическая модель rule:

```go
type Rule struct {
    ID             string
    Version        string
    Description    string
    Priority       int
    RequiredKinds  []string
    Conditions     []Condition
    Conclusions    []Conclusion
    ConflictKeys   []string
    BaseConfidence float64
    AutoApply      bool
    Risk           RiskLevel
}
```

Rule MUST иметь стабильный ID вида `domain.category.rule_name`, например `security.auth.endpoint_requires_actor`.

Rule MUST описывать:

- какие facts нужны;
- какое совпадение считается подтверждённым;
- когда результат `unknown`;
- какой finding создаётся;
- какое объяснение показывается;
- допустим ли proposal;
- уровень риска;
- conflict key для взаимоисключающих решений.

Conditions должны быть ограниченным typed DSL. Нельзя исполнять произвольный Go, shell или CUE expression, полученный из внешнего проекта.

### 3. Finding

```go
type Finding struct {
    ID          string   `json:"id"`
    Code        string   `json:"code"`
    Severity    string   `json:"severity"`
    Summary     string   `json:"summary"`
    RuleID      string   `json:"rule_id"`
    FactIDs     []string `json:"fact_ids"`
    EvidenceIDs []string `json:"evidence_ids"`
    Confidence  float64  `json:"confidence"`
    Status      string   `json:"status"` // confirmed|hypothesis|unknown|conflict
}
```

Finding без `RuleID` допускается только для существующего compiler diagnostic; тогда он должен иметь поле `origin: "compiler"` и stable diagnostic code.

### 4. Change proposal

Экспертная система не должна возвращать только текст «добавьте endpoint». Она должна возвращать применимый typed proposal.

```go
type Change struct {
    Op         string          `json:"op"` // insert|merge|replace|delete
    File       string          `json:"file"`
    CUEPath    string          `json:"cue_path"`
    Value      json.RawMessage `json:"value,omitempty"`
    BeforeHash string          `json:"before_hash,omitempty"`
    Rationale  string          `json:"rationale"`
}

type Proposal struct {
    ID             string      `json:"id"`
    Goal           string      `json:"goal"`
    RuleIDs        []string    `json:"rule_ids"`
    FindingIDs     []string    `json:"finding_ids"`
    Changes        []Change    `json:"changes"`
    Preconditions  []Assertion `json:"preconditions"`
    Postconditions []Assertion `json:"postconditions"`
    Risk           string      `json:"risk"`
    RequiresApproval bool      `json:"requires_approval"`
}
```

`BeforeHash` защищает от применения proposal к уже изменённому файлу. Paths должны указывать только на CUE intent. Операция `delete` всегда требует approval, пока не появится более строгая policy.

Существующий `normalizer.Fix`/`SuggestedFix` следует адаптировать к этому контракту или вынести общий changeset contract. Нельзя долго поддерживать две несовместимые модели патчей.

### 5. Explanation trace

Каждый запуск inference должен возвращать trace:

```go
type RuleTrace struct {
    RuleID        string   `json:"rule_id"`
    MatchedFacts  []string `json:"matched_facts"`
    MissingFacts  []string `json:"missing_facts,omitempty"`
    Result        string   `json:"result"` // matched|not_matched|unknown|conflict
    ProducedIDs   []string `json:"produced_ids,omitempty"`
    RejectedReason string  `json:"rejected_reason,omitempty"`
}
```

Trace нужен не только для debugging. Это основная защита от галлюцинаций: любой вывод можно разложить до наблюдений и правила.

### 6. Decision report

Публичный JSON-result должен иметь versioned envelope, например `ang/expert-report/v1`:

```text
schema
goal
status: no_change|advice|blocked|verified|failed
compiler_version
facts_hash
knowledge_versions[]
findings[]
proposals[]
trace[]
verification[]
diagnostics[]
```

Сериализация должна сортировать maps/slices по стабильным ключам до записи golden output.

## Детерминированный inference engine

Первую версию нельзя строить вокруг LLM. Она должна быть простым forward-chaining engine над facts и typed rules.

Рекомендуемый алгоритм:

1. Провалидировать FactsEnvelope и knowledge packs.
2. Нормализовать facts и вычислить stable IDs.
3. Отсортировать rules по `priority DESC`, затем `rule_id ASC`.
4. Для каждого rule проверить required fact kinds.
5. Вычислить conditions в трёхзначной логике: true, false, unknown.
6. При conflict создать conflict finding и не выбирать proposal автоматически.
7. При true создать conclusions/findings.
8. Добавить новые derived facts только с `derived_from` и rule ID.
9. Повторять до fixpoint либо до заданного лимита итераций.
10. Дедуплицировать результаты по canonical ID.
11. Разрешить взаимоисключающие proposals по явной conflict policy.
12. Вернуть trace для matched, unknown и conflict rules.

Для v1 установить жёсткий лимит, например 16 inference rounds. Достижение лимита должно завершаться diagnostic, а не частичным молчаливым успехом.

Простая формула confidence для первой версии:

```text
conclusion_confidence = min(rule.base_confidence, confidence всех обязательных facts)
```

Не нужно придумывать сложную вероятностную математику до появления реальных evaluation datasets. Conflict никогда не должен автоматически повышать confidence.

## Роль LLM

LLM является необязательным proposer, но не источником истины.

Допустимые задачи LLM:

- преобразовать пользовательскую цель в один из известных typed goal kinds;
- предложить candidate rule/pack;
- сформировать hypothesis о недостающей архитектуре;
- ранжировать несколько уже валидных proposals;
- написать human-readable explanation на основе готового trace;
- предложить CUE change, который затем проходит schema и compiler verification.

Запрещённые задачи LLM:

- придумывать field/service/action names без facts;
- изменять generated Go;
- самостоятельно объявлять proposal verified;
- обходить rule conflicts;
- выполнять write до dry-run и approval;
- создавать новые compiler semantics во время пользовательского запуска;
- использовать текст собственного предыдущего ответа как evidence.

LLM proposal должен маркироваться `origin: "llm"`, `status: "hypothesis"`. Он получает `verified` только после успешного применения в sandbox и всех обязательных checks.

## Первый полезный вертикальный сценарий: `ang advise`

Не следует начинать с команды «создай весь проект». Первый сценарий должен быть audit-only и собирать существующую экспертизу ANG в единый доказуемый отчёт.

Предлагаемый интерфейс v1:

```bash
ang advise --goal project.audit --json
ang advise --goal project.audit --facts facts.json --json
ang advise --goal security.audit --json
```

По умолчанию команда MUST быть read-only. В v1 не должно быть `--apply`.

`project.audit` должен переиспользовать:

- diagnostics семантики `github.com/strogmv/ang-ir/flowsem`, уже подключённые к compiler pipeline;
- Flow IR checker issues;
- effects prerequisite diagnostics;
- canonical pack diagnostics из `compiler/pack_diagnostics.go`;
- dead configuration diagnostics;
- DI/capability diagnostics;
- manifest/drift/doctor results, если они доступны без изменения проекта.

Результат должен преобразовывать существующие diagnostics в `Finding`, не переписывая их правила.

Acceptance criteria `ang advise` v1:

1. Команда ничего не записывает в проект.
2. Два запуска на одном input дают семантически идентичный JSON.
3. Каждый finding имеет source, stable code и explanation.
4. Compiler diagnostics сохраняют исходный code без переименования.
5. Unknown данные помечаются unknown, а не превращаются в warning/error без правила.
6. Exit code документирован: `0` без error findings, ненулевой при confirmed error/blocked execution.
7. Есть golden test JSON.
8. Есть тест проекта без findings.
9. Есть тест с конфликтующими evidence.
10. `go test ./cmd/ang ./compiler/...` проходит.

## План реализации по фазам

Каждая фаза должна быть отдельной серией небольших commits. Слабой модели запрещается реализовывать несколько фаз одним большим patch.

### Фаза 0. Завершить Typed Flow IR foundation

Цель: экспертная система не должна строиться поверх двух конкурирующих flow representations.

Работы:

1. Сначала зафиксировать baseline: список совпадений `renderLegacyStepDispatch`, `decodeCurrentActionAs`, `flowStepMetadata` и `ScalarArgs`, успешные compiler tests и сборку реального fixture-проекта.
2. Перенести оставшиеся production renderer families на `TypedStep`.
3. В каждом renderer получать action только через `typedActionAs[T]`.
4. Вложенные блоки брать только из `TypedStep.Children`/`Branches`; map keys сортировать.
5. Убрать production dependence на `flowStepMetadata` и legacy arg callbacks.
6. Сохранить legacy CUE compatibility только в decoder boundary.
7. Добавить метрику/тест, перечисляющий actions, всё ещё использующие legacy route.
8. Довести checker для service/repo/logic signatures и variables.

Готово, когда:

- любой зарегистрированный stable action имеет typed renderer;
- emitter не декодирует action повторно;
- конфликтующие `ScalarArgs` не влияют на output;
- реальные fixture-проекты собираются.

Финальная проверка этой фазы:

```bash
rg "renderLegacyStepDispatch|decodeCurrentActionAs|flowStepMetadata|ScalarArgs" compiler
```

После полной миграции production-совпадений быть не должно. Допустимы только явно названные compatibility tests/migration documentation. Удалять compatibility helpers раньше миграции последней action family запрещено.

### Детализация semantic type checker внутри фазы 0

Текущее `TypeUnknown` нельзя бесконечно использовать как универсальное «всё допустимо». Целевая модель должна различать:

```text
invalid  — выражение или ссылка ошибочны;
unknown  — compiler не умеет доказать тип, migration-only состояние;
any      — динамический тип разрешён явно контрактом;
concrete — string/bool/int/bytes/entity/dto/list/map/pointer/time/etc.
```

Checker должен иметь scoped symbol table с именем, типом и source declaration. Минимальные правила ветвлений для первой версии:

- переменная, созданная только внутри одной branch, не доступна после блока;
- переменная, существовавшая до branch, остаётся доступной после branch;
- присваивание несовместимых типов в разных branches даёт `FLOW_BRANCH_TYPE_MISMATCH`;
- новый общий branch output разрешён только через явный typed control-flow output contract;
- duplicate declaration и использование undeclared variable дают отдельные stable codes.

Обязательные signature checks:

- `service.Call`: service, `uses`, method, request fields/types, output;
- `flow.Call`: operation, обязательные/лишние named args, output;
- `logic.Call`: только известная typed sidecar signature; неизвестная функция не считается доказанно корректной;
- `repo.*`: entity, finder/method, input arity/type и result kind;
- `event.Publish`: event name, required payload fields и их типы;
- conditions: только bool;
- list actions: совместимый element type;
- effects/capabilities: prerequisite и DI availability.

Полностью типизировать произвольные Go expressions на первом этапе не нужно. Поддерживается ограниченное подмножество: identifiers, selectors, literals, unary operations, простые comparisons/arithmetic и известные composite request values. Неразрешимое expression должно давать diagnostic и предложение вынести сложность в typed sidecar, а не молча получать подходящий тип.

### Фаза 1. Сделать facts импортируемым versioned contract

Цель: убрать зависимость экспертного слоя от package `main`.

Работы:

1. Создать `compiler/facts/model.go`.
2. Перенести data types, сохранив JSON tags.
3. Добавить aliases в `cmd/ang`, если они нужны существующим tests.
4. Создать `compiler/facts/validate.go`.
5. Создать canonical sorting/hash helpers.
6. Добавить round-trip и deterministic hash tests.

Не делать в этой фазе:

- не менять extractors;
- не вводить новый rule engine;
- не менять JSON schema с v1 на v2;
- не удалять старые CLI fields.

### Фаза 2. Ввести expert report model без inference

Цель: получить стабильный public contract раньше реализации правил.

Рекомендуемые новые файлы:

```text
compiler/expert/model.go
compiler/expert/report.go
compiler/expert/canonical.go
compiler/expert/model_test.go
```

Работы:

1. Определить Finding, Proposal, Change, RuleTrace, VerificationResult и Report.
2. Добавить schema constants.
3. Реализовать deterministic canonicalization.
4. Написать golden JSON contract.
5. Добавить migration policy document.

В этой фазе нельзя читать CUE, вызывать LLM или применять patches.

### Фаза 3. Реализовать `ang advise --goal project.audit`

Цель: первый end-to-end экспертный отчёт без новых предметных правил.

Рекомендуемые файлы:

```text
cmd/ang/advise.go
cmd/ang/advise_test.go
compiler/expert/audit.go
compiler/expert/adapters.go
```

Работы:

1. Собрать уже существующие diagnostics.
2. Адаптировать их в Findings.
3. Сохранить origin и diagnostic code.
4. Добавить trace с origin `compiler`.
5. Реализовать JSON и human-readable output.
6. Гарантировать read-only режим.

### Фаза 4. Knowledge schema и deterministic rule engine

Цель: добавлять новую экспертизу декларативно.

Работы:

1. Расширить CUE pack schema отдельными typed condition/conclusion definitions.
2. Добавить loader/normalizer knowledge packs.
3. Создать `compiler/expert/infer.go`.
4. Реализовать three-valued conditions.
5. Реализовать conflict detection.
6. Реализовать fixpoint limit и trace.
7. Добавить tests на порядок rules, unknown и conflict.

Первый новый rule pack должен быть небольшим. Хороший кандидат — security/auth completeness, потому что часть diagnostics уже существует и можно сравнить результаты.

### Фаза 5. Typed proposals и sandbox verification

Цель: система предлагает применимое изменение, но ещё не меняет пользовательский проект.

Работы:

1. Унифицировать Proposal.Change с существующим SuggestedFix contract.
2. Реализовать проверку path scope и BeforeHash.
3. Применять proposal только во временной копии.
4. Запускать `validate`, semantic checks и build.
5. Возвращать diff и VerificationResults.
6. Помечать proposal `verified` только при полном успехе.

В этой фазе CLI всё ещё не должен автоматически писать в рабочий проект.

### Фаза 6. Human-approved apply

Цель: безопасное применение verified proposal.

Работы:

1. Добавить explicit `--apply`.
2. Проверить BeforeHash повторно непосредственно перед записью.
3. Запретить paths вне CUE roots.
4. Создать atomic transaction/rollback.
5. Записать decision report рядом с build report или в `.ang/` cache/history.
6. После записи повторно запустить validation/build.

Auto-apply без подтверждения допустим позже только для allowlisted low-risk rules.

### Фаза 7. Подключить LLM proposer

Цель: использовать модель для задач, которые нельзя выразить простыми rules, не отдавая ей финальную власть.

Работы:

1. Ввести provider-neutral interface.
2. Передавать модели только bounded goal, relevant facts, schemas и constraints.
3. Требовать structured output Proposal/Hypothesis.
4. Валидировать JSON/schema до любой дальнейшей обработки.
5. Запускать обычный sandbox verification.
6. Сохранять prompt/input hashes и model metadata в report для audit.
7. Добавить offline fake provider для tests.

Тесты не должны зависеть от сети или реальной модели.

### Фаза 8. Evaluation и controlled learning

Цель: измерять качество, а не «обучаться» на каждом ответе автоматически.

Нужен набор versioned scenarios:

- input facts/intent;
- ожидаемые findings;
- допустимые proposals;
- запрещённые изменения;
- expected verification result;
- human verdict.

Knowledge pack не должен изменяться автоматически после одного пользовательского решения. Новое правило проходит review, version bump и regression evaluation.

## Явные non-goals до завершения roadmap

Следующие задачи не входят в ближайшую реализацию и не должны добавляться «заодно»:

- универсальная экспертная система для любых языков и предметных областей;
- автономное изменение CUE без verification и подтверждения;
- самоизменяющиеся knowledge rules в production;
- runtime inference engine внутри сгенерированного приложения;
- полный type checker произвольного Go expression;
- перенос сложной бизнес-логики из Go в Flow DSL;
- второй semantic action catalog рядом с `ang-ir/flowsem`;
- объединение expert ChangePlan и generator BuildPlan без явного adapter/контракта;
- добавление новых AI/providers/actions только ради размера каталога;
- замена compiler diagnostics ответом LLM;
- breaking rewrite существующего CUE DSL;
- исправление compiler defects прямым редактированием generated files.

Если задача требует один из этих пунктов, модель должна остановиться и запросить отдельное архитектурное решение.

## Минимальный набор тестов экспертной системы

1. Determinism: одинаковый input даёт одинаковый canonical report.
2. Evidence completeness: confirmed finding без evidence отклоняется.
3. Unknown safety: неполный fact scope не создаёт ложное нарушение.
4. Conflict safety: два несовместимых rules не дают auto-selected proposal.
5. Path safety: proposal не может изменить generated directory.
6. Hash safety: stale BeforeHash блокирует apply.
7. Compiler authority: proposal, не прошедший flowsem/Flow IR checker, не получает verified.
8. Build authority: proposal с некорректным generated Go не получает verified.
9. Approval policy: security/DB/delete/breaking API требуют approval.
10. No-network tests: основной suite работает без LLM и интернета.
11. Backward compatibility: `ang/facts/v1` продолжает читаться.
12. Version rejection: неизвестная major schema version завершается явной ошибкой.
13. Trace completeness: каждый derived fact ссылается на rule и parent facts.
14. Stable ordering: rules/findings/proposals сериализуются в фиксированном порядке.
15. Real fixture: verified proposal собирает минимум один реальный или representative проект.

## Инструкция для AI-модели, реализующей этот roadmap

Каждая задача, выдаваемая слабой модели, должна иметь явную рамку:

```text
Phase: <одна фаза и один подпункт>
Goal: <один проверяемый результат>
Allowed files: <точный список или каталоги>
Forbidden files: internal/, api/, sdk/, db/schema/, db/queries/
Behavior change: forbidden|описан явно
DSL change: forbidden|описан явно
Public schema change: forbidden|version bump указан
Required tests: <точные команды>
Stop if: требуется изменение ang-ir, внешнего проекта или другого контракта
Commit: forbidden, если пользователь не разрешил явно
```

Если `go.mod` указывает `replace` на внешний `ang-ir`, модель должна считать его отдельной областью изменений. Наличие локального checkout не является разрешением редактировать его.

Перед началом каждого изменения модель должна:

1. Прочитать `AGENTS.md`.
2. Проверить текущую ветку и dirty worktree.
3. Не перезаписывать несвязанные изменения пользователя.
4. Если доступны MCP tools, выполнить обязательный bootstrap.
5. Прочитать файлы, названные в конкретной фазе.
6. Найти существующие типы и helpers через `rg`, прежде чем создавать новые.
7. Выбрать ровно одну фазу или один небольшой пункт фазы.
8. Сначала добавить/обновить contract test, затем минимальную реализацию.
9. Проверить `git diff --check`.
10. Запустить узкие tests и затем `go test ./compiler/... ./cmd/ang` в зависимости от scope.

Модель не должна:

- создавать package с именем, не проверив существующую структуру;
- менять public JSON contract без schema version и golden fixture;
- использовать `map[string]any` как внутреннюю модель rules/findings/proposals;
- добавлять inference logic в emitter/templates;
- редактировать generated output для исправления compiler bug;
- одновременно переносить facts, вводить rules и добавлять LLM integration;
- объявлять задачу завершённой только потому, что unit test одного helper прошёл;
- коммитить или пушить без прямого разрешения пользователя.

Если информации недостаточно, правильный результат — `unknown` diagnostic или запрос уточнения, а не догадка.

## Ближайший практический порядок работ

С учётом текущего рефакторинга рекомендуется следующий порядок:

1. Закончить migration оставшихся emitter families на Typed Flow IR.
2. Удалить production legacy dispatch только после тестов реальных проектов.
3. Усилить type/signature checker.
4. Зафиксировать registry completeness gates.
5. Вынести facts model из `cmd/ang` в импортируемый package без изменения v1 JSON.
6. Ввести expert report types и golden contract.
7. Реализовать read-only `ang advise --goal project.audit` как агрегацию существующих доказуемых diagnostics.
8. Только после dogfooding отчёта проектировать новые knowledge rules.
9. Typed proposals и sandbox verification делать раньше LLM integration.
10. Подключать LLM последним как заменяемый hypothesis proposer.

Такой порядок сохраняет главное преимущество ANG: экспертная система сможет быть умной на верхнем уровне, но её решения останутся проверяемыми, воспроизводимыми и ограниченными строгим compiler kernel.
