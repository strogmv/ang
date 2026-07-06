# ANG: рекомендации по улучшению генератора

## Статус реализации (2026-07-06)

Реализованы §1–§11 и §13: staging/transactional emit, DTO selector
validation, self-healing writes, template hash, idempotency and OpenAPI gates,
Go sidecars, structured warnings, dead-CUE/DDL checks, queue delivery defaults,
capability DI validation, JSON diagnostics, and aggregated project doctor.

Из приложения реализованы нетестовые пункты: строгие flow-декодеры и
compile-time проверки, parser/formatter gate для Go emission, dependency
preflight, frontend startup, runbook generation, diagnostic documentation and
links, performance benchmark/parallel emission/cache checks, synchronized CLI
docs, recipes, and migration notes.

§12 и пункты приложения про генерацию/golden-тесты исключены из текущего
объёма по явному решению пользователя.

Составлено 2026-07-04 по итогам недели интенсивной эксплуатации ANG на боевом
проекте dealingi-back (Telegram sales channels): ~15 циклов «CUE → generate →
build → deploy → prod», несколько инцидентов в проде, причиной или
усилителем которых был генератор. Каждая рекомендация привязана к реально
прожитому случаю, а не к теории. Приоритет — сверху вниз внутри секций;
итоговый топ-3 в конце. (Прежний общий бэклог сохранён в приложении внизу.)

---

## A. Доверие к генератору (главный дефицит сегодня)

### 1. Атомарная генерация: всё или ничего

**Инцидент.** Ошибка в одном опе (`GetCustomer`, см. §2) уронила пайплайн
посередине эмита: `channel__block_customer.gen.go` и `internal/port/channel.go`
уже записаны, моки — нет. Репозиторий остался в несобираемом промежуточном
состоянии, диагностика увела в ложный след («генератор моков не догоняет
интерфейс») на ~40 минут.

**Сейчас.** `compiler/emitter/atomic_write.go` атомарен только пофайлово
(temp + rename). Межфайловой транзакции нет.

**Предложение.** Эмитить весь билд во временную директорию → прогнать
`go/parser` + типовую проверку по всему пакету → атомарно свапнуть в рабочее
дерево. Любая ошибка = ноль изменённых файлов в репо. Это одно изменение
устраняет целый класс «полусгенерённых» состояний.

### 2. Семантическая валидация `logic.Call` ДО эмита, с маппингом на CUE

**Инцидент.** Опечатка `out.TelegramUserId` (вместо `TelegramUserID`) в
теле лямбды дала:

```
Build FAILED: generated go is invalid (internal/service/channel__get_customer.gen.go): 85:8: expected selector or type assertion, found '='
```

Позиция — в сгенерённом файле; текст — про синтаксис; причина — несуществующее
поле DTO. Связь «ошибка → строка CUE → поле» пришлось восстанавливать вручную
через реконструкцию файла и `gofmt -e`.

**Предложение.** IR уже знает полную схему всех DTO. Перед эмитом:
распарсить каждую лямбду отдельно (`go/parser`), разрешить селекторы
`out.*` / `req.*` / известных локалов против IR, при промахе — ошибка вида:

```
cue/api/impl_customer.cue:73: поле "TelegramUserId" не существует в
GetCustomerResponse — возможно, вы имели в виду "TelegramUserID"?
```

Подсказка — ближайшее поле по Левенштейну. Это самый частый тип ошибки
при написании имплов, и он полностью ловится статически.

### 3. Самовосстанавливающийся инкрементальный кэш

**Инцидент.** Удалённые `.gen.go` (моки) не пересоздаются: манифест считает
артефакт актуальным по входному хэшу и пропускает эмит. Точечное удаление
записей из `manifest.json` тоже не помогло. Рабочий обходной путь у
пользователей — «снеси `.ang/cache` целиком», т.е. инкрементальность
фактически не заслуживает доверия.

**Предложение.** При решении «пропустить артефакт» сверять не только
входной хэш, но и **наличие файла на диске + его контент-хэш** (хэши уже
лежат в `artifacts[]`). Файла нет или контент отличается → пере-эмитить
именно его. Кэш становится самовосстанавливающимся, ритуал `rm -rf
.ang/cache` умирает.

### 4. Баг: `templateHash` в манифесте — SHA-256 пустой строки

**Факт.** В боевом `manifest.json`:

```
templateHash = e3b0c44298fc1c149afbf4c8996fb924...
```

— это SHA-256 от пустого ввода. В компиляторе нет кода, который его
вычисляет и записывает (grep по `TemplateHash` в `compiler/` пуст). То есть
отпечаток шаблонов заявлен схемой манифеста, но не реализован: правка
шаблона (auth-фикс в `http_common.tmpl`) не инвалидирует кэш честным
путём — спасает только пересборка бинаря.

**Предложение.** Вычислять сводный хэш по всем embed-шаблонам при сборке
бинаря, писать в манифест, учитывать при инкрементальных решениях. Полчаса
работы, закрывает невидимый skew «бинарь новый — кэш думает, что шаблоны
старые».

### 5. Гейт идемпотентности: `generate × 2 = no diff`

**Инцидент.** У прикладной команды закрепилось знание «`make generate` не
может воспроизвести бэк: падает и переписывает API-контракт». Следствие
фатальное: люди начинают **хэндэдитить генерат** — на этом проекте ручная
правка `common.go` дважды стиралась последующим регеном и дважды ломала
прод-логин (401 на сессии), пока фикс не переехал в шаблон.

**Предложение.**
- CI-инвариант самого ANG и шаблонных изменений: двойная генерация подряд →
  пустой дифф; генерация на чистом чекауте эталонного проекта → побайтово
  воспроизводит закоммиченное дерево.
- Семантический дифф контракта при каждом билде: печатать
  `openapi: +4 paths, 0 breaking`; на неожиданных breaking-изменениях —
  фейл без явного `--accept-contract`.

Доверие «реген ничего не сломает» — это фундамент, на котором держится
запрет хэндэдитов.

---

## B. Авторинг логики (самая большая боль DX)

### 6. Sidecar-`.go` вместо Go-кода в triple-quoted CUE-строках

**Боль.** Тело `logic.Call.func` — это Go внутри CUE-строки: нет gofmt, нет
gopls/подсветки, табозависимый сплайсинг (ошибки «invalid whitespace» при
машинных правках), двойное экранирование в строковых литералах (`\\n`,
`\\"` — реальный кейс с HTML-ссылкой «Купить» в подписи Telegram-альбома).
WARN `LAMBDA_INLINE_ESCAPE` — фактически признание проблемы самим
компилятором.

**Предложение.** Дать первоклассный escape-hatch:

```cue
{ action: "logic.Call", funcRef: "impl_customer_logic.go#getCustomerBody" }
```

или соглашение — рядом с CUE лежит настоящий Go-файл с маркерами:

```go
//ang:impl GetCustomer
func getCustomerBody(ctx context.Context, req port.GetCustomerRequest) (port.GetCustomerResponse, error) { ... }
```

Компилятор сплайсит тело при эмите. IDE и агенты сразу получают полный
тулинг (формат, автодополнение, go vet), класс ошибок из §2 в основном
исчезает ещё до билда.

### 7. Предупреждения: адресные, дедуплицированные, подавляемые

**Боль.** `REPO_FIND_WITHOUT_ERROR` и `LAMBDA_INLINE_ESCAPE` печатаются на
каждый билд, без позиции в CUE, одними и теми же строками. Шум приучает
грепать только `ERROR` — так на проекте едва не был пропущен реальный
`Build FAILED` (он шёл после стены WARN).

**Предложение.** Каждому warning — позиция источника (`file:line`),
дедупликация по месту, `//ang:nolint <CODE>` per-site, сводка в конце
(`3 warnings, 2 suppressed`). Реально опасные случаи — промоутить в ошибки.

### 8. Детектор мёртвой конфигурации и дрейфа схемы

**Инциденты.**
- `cue/schedules/` — мёртвый каталог: не входит ни в один пакет, компилятор
  молча игнорирует; рабочие расписания живут в `cue/api/schedules.cue`.
  Полдня отладки «почему шедулер не тикает».
- Unique-index, объявленный в домене, не попадает в сгенерённый
  `schema.sql` — расхождение всплыло только при ручном сравнении с БД.

**Предложение.** (a) Файл/каталог под `cue/`, не вошедший ни в один
загруженный пакет → warning с подсказкой ближайшего «живого» места.
(b) Любой элемент домена, который не может быть отражён в DDL — ошибка,
а не молчаливый пропуск.

---

## C. Архитектурные дефолты рантайма

### 9. Queue-group подписки по умолчанию

**Инцидент (прод).** Сгенерённый NATS-подписчик — plain `Subscribe`. При
двух репликах gateway каждое событие `SendTelegramAction` обрабатывалось
каждой репликой → каждая карточка товара постилась в Telegram-канал дважды.
Диагностика заняла день (симптом маскировался под гонку публикации);
временное решение — приколотить `replicas: 1`.

**Предложение.** Генерённые подписчики по умолчанию — queue-subscription
(одна реплика на сообщение); broadcast — только явным выбором в схеме:

```cue
subscribes: { SendTelegramAction: { op: "...", delivery: "queue" | "broadcast" } }
```

Это одновременно разблокирует горизонтальное масштабирование, под которое
у приложений уже готовы Redis-бэкенды лимитеров/дедупа.

### 10. Полный DI из шаблона: недоподключённая capability = ошибка компиляции

**Инцидент (прод).** `authMode: "opaque_session_cookie"` генерился в
`common.go`, но сторы (`authSessionStore`, `authRefreshStore`) в `main` не
подключались никем → каждый cookie-логин в проде получал 401. Чинили
трижды: два хэндэдита стёрты регеном, финально — правка
`templates/http_common.tmpl` (SetRedisClient теперь ваярит оба стора).

**Предложение.** Инвариант: если схема включает capability, шаблоны обязаны
эмитить **всю** проводку её зависимостей; «capability включена, но
зависимость не подключена» должно быть ошибкой генерации, а не рантайм-401
через неделю. Дешёвая проверка: пройтись по capability-матрице и убедиться,
что каждый требуемый сеттер вызван в сгенерённом `main`/бутстрапе.

---

## D. Агентно-ориентированный тулинг

Основной «пользователь» ANG сегодня — LLM-агент, и это стоит признать
дизайн-фактом. У компилятора уже есть задел: `error_codes.go`, LSP, MCP-тулы
(`ang_validate`, `ang_explain_error`, `cue_add_endpoint`, ...). Довести до
конца:

### 11. Машиночитаемые диагностики по умолчанию

`ang build --json` (и авто-включение при не-TTY): поток объектов

```json
{"code":"DTO_FIELD_UNKNOWN","cueFile":"cue/api/impl_customer.cue","line":73,
 "message":"...","suggestedFix":{"replace":"TelegramUserId","with":"TelegramUserID"}}
```

Цикл агента превращается из «грепай stdout и гадай» в «применил
suggestedFix — перегенерил».

### 12. Генерённые тесты по флоу

IR знает всё, чтобы бесплатно эмитить table-driven тесты на каждый оп:
policy → кейс «403 для чужой компании», обязательные поля → «400 на
пустой», finder → «404 на несуществующий id», tx.Block → «откат при ошибке
шага». Такие тесты поймали бы и session-store (§10), и рассинхрон моков
(§1) до прода.

### 13. `ang doctor` для прикладного репо

Одна команда, агрегирующая проверки из этого документа: артефакты vs
манифест (§3), полнота DI (§10), мёртвые CUE-каталоги (§8), дифф контракта
(§5), идемпотентность регена (§5), битые ссылки полей в лямбдах (§2).
Выход — тот же JSON-формат диагностик (§11).

---

## Итог: если делать только три вещи

1. **§1 Атомарный эмит** — убирает полусгенерённые состояния.
2. **§2 Валидация лямбд с маппингом на CUE** — убирает самый частый и самый
   дорогой в диагностике класс ошибок.
3. **§6 Sidecar-Go** — убирает саму почву для §2 и главную боль авторинга.

Квик-вин вне конкурса: **§4 templateHash** (реально битый, ~полчаса).

Вместе они переводят ANG из «генератора, которому нужен шаман со знанием
ритуалов» в инструмент, которому доверяют реген без страха — а доверие к
регену и есть то единственное, что делает schema-first генератор
по-настоящему крутым.

---

## Приложение: прежний бэклог (Engineering Quality, май 2026)

### Priority A: Generator Safety

- Continue replacing string-based codegen with AST-based emission in flow-heavy paths.
- Add golden tests for every new flow action contract and edge-case branch.
- Add deterministic variable naming tests for nested control-flow and retries.

### Priority A: Type Strictness

- Reduce `any` usage in normalized flow args where concrete types are known.
- Move runtime shape checks into compile-time validation where possible.
- Tighten cross-checks between action args and repository/service signatures.

### Priority B: DX Hardening

- Expand `ang doctor start` with optional dependency checks for Atlas/Node when relevant.
- Add `ang up --frontend` workflow for SDK/frontend projects.
- Add auto-generated runbook snippet to project root after `ang init`.

### Priority B: Observability

- Emit structured diagnostics IDs consistently across normalizer/flowsem/emitter.
- Generate troubleshooting links from error codes to docs.
- Add `ang doctor --code <ERR_CODE>` shortcut for instant guidance.

### Priority C: Performance & Scale

- Benchmark large-flow compile and emission latency.
- Introduce parallel emission where outputs are independent.
- Add cache invalidation tests for incremental build plans.

### Priority C: Documentation

- Keep command docs synchronized with CLI flag changes per release.
- Add minimal "copy/paste recipes" per common use case (auth service, event worker, webhook).
- Publish a short "migration notes" file for each breaking behavior change.
