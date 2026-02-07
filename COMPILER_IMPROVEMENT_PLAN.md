# План поэтапного улучшения ANG Compiler

Этот документ фиксирует шаги по стабилизации и развитию компилятора.
**КРИТИЧЕСКОЕ ПРАВИЛО:** Переход к следующему этапу разрешен ТОЛЬКО после успешного выполнения `npm test` (e2e тесты).

## Статус: Стабилен (32/32 теста пройдены)

### [x] Этап 1: Унификация Helpers и стабилизация генерации Main
### [x] Этап 2: Рефакторинг Storage Emitter
### [x] Этап 3: Фикс экстракции репозиториев
### [x] Этап 4: Улучшение Postgres шаблона
### [x] Этап 5: Исправление инъекций и RBAC (Микро-фикс)
### [x] Этап 6: Инициализация коллекций (Null Safety)
### [x] Этап 7: Smart Mapping & Strict Naming Engine
### [x] Этап 8: Error Registry & Auto-Validation
### [x] Этап 9: AI Traceability (Meta-Comments, Deep Logging & System Manifest)
### [x] Этап 10: SDK Errors & TypeScript Enums
### [x] Этап 11: Smart Health-Check (Quick Win)
### [x] Этап 12: Auto-Audit & Transactional Reliability
### [x] Этап 13: E2E Test Infrastructure & Idempotency Fixes
### [x] Этап 14: Default Rate Limits & Унификация
### [x] Этап 15: Timeout Middleware
### [x] Этап 16: Circuit Breaker
### [x] Этап 17: Architecture Visualization (Mermaid)
### [x] Этап 18: Smart CRUD Generation (MUI + React Framework)
### [x] Этап 19: Body Size & Upload Limits (DoS Protection)

### [x] Этап 21: MCP Server Implementation
- **Статус:** ВЫПОЛНЕНО.
- **Выполненные работы:**
  - Реализован встроенный MCP сервер (команда `ang mcp`).
  - Добавлены инструменты: `ang_validate`, `ang_build`, `ang_graph`.
  - Добавлены ресурсы: `resource://ang/manifest`, `resource://ang/project/summary`.
  - Добавлены промпты для AI-агентов: `add-entity`, `add-crud`.
  - Рефакторинг `compiler.RunPipeline` для поддержки внешних путей к проектам.
  - Повышена отказоустойчивость нормализатора (защита от паник при отсутствии CUE-блоков).

### [ ] Этап 22: Dynamic Flow (Level 4) - В ОЧЕРЕДИ
