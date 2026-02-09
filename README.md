# ANG — Architectural Normalized Generator

**ANG is an AI-friendly intent compiler that normalizes backend architecture.**

> **Philosophy:**
> CUE defines the law.
> Go executes the law.
> ANG enforces the law.

ANG is not a framework, not an ORM, and not a low-code platform. It is a compiler that accepts declarative architectural intent (CUE) and generates normalized Go backend scaffolds, leaving business algorithms to the developer.

## Why AI-Friendly?

ANG is designed from the ground up to work seamlessly with AI coding agents:

- **Declarative DSL:** CUE-based Flow DSL is structured and predictable — perfect for LLMs to generate
- **Golden Examples:** Reference patterns in `cue/GOLDEN_EXAMPLES.cue` teach AI the correct idioms
- **Self-Documenting Schema:** Type definitions include descriptions that AI can read and understand
- **Deterministic Output:** Same input always produces same output — AI can verify its work
- **Validation at Compile Time:** Errors caught early, before runtime — AI gets immediate feedback

## Core Mandates

1.  **Separation of Concerns:**
    *   **CUE:** Defines *what* must exist (intent, rules, invariants).
    *   **Go:** Defines *how* it is executed (algorithms, concurrency).
    *   **ANG:** Compiles intent into form.
2.  **Compiler, not Generator:**
    *   Uses compiler terminology: source of truth, normalization, validation, deterministic output.
    *   Avoids: "magic", runtime hacks, auto-ORM.
3.  **No Magic:**
    *   ANG does not accelerate code writing; it eliminates incorrect code.
    *   ANG is a tool of discipline, not comfort.

## Architecture: Core + Providers + Transformers

ANG uses a plugin-based architecture that separates concerns and enables multi-language support:

```
CUE Files → Parser → Normalizer → IR → Transformers → Providers → Code
```

### Intermediate Representation (IR)

The IR is a language-agnostic data structure that represents your architecture:

```go
// compiler/ir/ir.go
type Schema struct {
    Project    Project      // Name, version, target
    Entities   []Entity     // Domain entities
    Services   []Service    // Service interfaces
    Events     []Event      // Domain events
    Endpoints  []Endpoint   // HTTP/WS routes
    // ...
}

type TypeRef struct {
    Kind     TypeKind  // string, int, time, uuid, entity, list...
    Name     string    // For entity references
    ItemType *TypeRef  // For collections
}
```

### Transformers

Transformers enrich the IR based on attributes and conventions:

```go
// compiler/transformers/transformer.go
type Transformer interface {
    Name() string
    Transform(schema *ir.Schema) error
}

// Built-in transformers:
// - ImageTransformer: @image → adds thumbnail fields
// - TimestampTransformer: adds created_at/updated_at
// - SoftDeleteTransformer: adds deleted_at
// - ValidationTransformer: processes @validate
```

### Attribute Hooks

Hooks are triggered when specific CUE attributes are found:

```go
// compiler/transformers/hooks.go
type Hook interface {
    Attribute() string
    OnField(schema, entity, field, attr) error
    OnEntity(schema, entity, attr) error
    OnService(schema, service, attr) error
    OnMethod(schema, service, method, attr) error
}

// Built-in hooks:
// @db, @validate, @image, @file, @env, @cache, @stripe_payment
```

### Providers (Template Bundles)

Providers supply templates for specific technology stacks:

```go
// compiler/providers/provider.go
type Provider interface {
    Name() string                    // "go-chi-postgres"
    Supports(target ir.Target) bool  // Check compatibility
    Templates() fs.FS                // Template filesystem
    FuncMap() template.FuncMap       // Custom functions
    TypeMapping() TypeMap            // Type conversions
}
```

**To add Rust/Axum support:**
1. Create `templates/rust-axum/` with `.tmpl` files
2. Implement `RustAxumProvider`
3. In `project.cue`: `target: { lang: "rust", framework: "axum" }`

### Type Mapping in CUE

Type mappings are defined in CUE, not hardcoded in Go:

```cue
// cue/schema/type_mapping.cue
#TypeMapping: {
    "time.Time": {
        go:   { type: "time.Time", pkg: "time", null_helper: "sql.NullTime" }
        ts:   { type: "string", zod: "z.string().datetime()" }
        sql:  { type: "TIMESTAMPTZ" }
        rust: { type: "chrono::DateTime<chrono::Utc>", pkg: "chrono" }
    }
    // ...
}
```

### Target Configuration

Configure your target stack in `project.cue`:

```cue
// cue/project.cue
#Target: {
    lang:      "go"       // "go", "rust", "typescript"
    framework: "chi"      // "chi", "echo", "fiber", "axum"
    db:        "postgres" // "postgres", "mysql", "mongodb"
    cache:     "redis"    // "redis", "memcached", "none"
    queue:     "nats"     // "nats", "kafka", "rabbitmq"
    storage:   "s3"       // "s3", "gcs", "minio"
}

#Transformers: {
    timestamps:  { enabled: true }
    soft_delete: { enabled: false }
    image:       { enabled: true, thumb_suffix: "_thumb" }
    validation:  { enabled: true }
}
```

## Project Structure

```
ANG/
├── cue/                    ← Input intent models
│   ├── domain/             ← Entities, fields, types
│   ├── api/                ← Endpoints, contracts
│   ├── policies/           ← Auth, cache, rate limits
│   ├── invariants/         ← System laws
│   ├── architecture/       ← Services, boundaries
│   ├── schema/             ← Type mappings, codegen config
│   └── project.cue         ← Project config + target
├── compiler/
│   ├── parser/             ← CUE loading
│   ├── normalizer/         ← CUE → legacy types
│   ├── ir/                 ← Universal IR + converter
│   ├── transformers/       ← IR enrichment plugins
│   ├── providers/          ← Template bundles
│   ├── validator/          ← Architecture validation
│   ├── emitter/            ← Code generation
│   └── pipeline.go         ← Orchestrator
├── templates/              ← Go templates
│   ├── *.tmpl              ← Backend templates
│   ├── frontend/           ← SDK templates
│   └── k8s/                ← Kubernetes manifests
├── cmd/
│   ├── ang/                ← CLI
│   └── server/             ← Generated server
├── internal/               ← Generated application code
├── sdk/                    ← Generated frontend SDK
├── tests/                  ← Test suites
│   ├── contract/           ← API contract tests
│   ├── integration/        ← Business logic tests
│   └── tests/e2e/          ← End-to-end tests
└── db/
    ├── schema/             ← SQL schema
    └── queries/            ← SQLC queries
```

## Quick Start

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- Node.js 18+ (for SDK tests)

### Feature Checklist (For New CUE Ops)

- Read operations: define `sources` and `impls` (no empty responses).
- Auth wiring: add HTTP inject for `userId` / `companyId` where needed.
- New fields: update schema and run migrations (auto-run on `make gen`).

### Run Infrastructure

```bash
# Start all services (Postgres, Redis, NATS, Minio, etc.)
docker compose up -d

# Verify
docker compose ps
```

### Build & Run Server

```bash
# Generate JWT keys
openssl genpkey -algorithm RSA -out /tmp/private.pem -pkeyopt rsa_keygen_bits:2048
openssl rsa -pubout -in /tmp/private.pem -out /tmp/public.pem

# Build
go build -o server ./cmd/server

# Run
export JWT_PRIVATE_KEY="$(cat /tmp/private.pem)"
export JWT_PUBLIC_KEY="$(cat /tmp/public.pem)"
./server
```

### Run Tests

```bash
# Infrastructure tests
make test-infra

# Contract tests
CONTRACT_BASE_URL=http://localhost:8080 \
CONTRACT_TOKEN=... \
go test -tags=contract ./tests/contract/...

# Integration tests
DATABASE_URL="postgres://user:password@localhost:5432/app?sslmode=disable" \
go test -tags=integration ./tests/integration/...

# E2E tests (Vitest)
docker compose up -d
go run ./cmd/server
cd tests && npm install && npm run test:e2e
```

## CLI Usage

*   `ang init` — Initialize project
*   `ang validate` — Validate CUE architecture
*   `ang build` — Generate code
*   `ang migrate diff <name>` — Create migration
*   `ang migrate apply` — Apply migrations
*   `ang vet` — Lint generated code
*   `ang draw` — Visualize architecture
*   `ang api-diff` — Check API breaking changes
*   `ang contract-test` — Run contract tests
*   `ang_doctor` — AI-powered self-diagnosis and fix suggestions (MCP only)

## Release Demo: Python SDK Generation

Show Python code generation in release demos with the Phase 1 flag:

```bash
ANG_PY_SDK=1 go run ./cmd/ang build
```

Expected generated files:

- `sdk/python/pyproject.toml`
- `sdk/python/README.md`
- `sdk/python/ang_sdk/__init__.py`
- `sdk/python/ang_sdk/client.py`

Quick smoke:

```bash
python -m pip install -e sdk/python
python -c "from ang_sdk import AngClient; print('ok')"
```

## Features

### AI-Native Intelligence

*   **Behavioral Scenarios (Stage 31):** Define complex business flows in CUE to automatically generate Go-based E2E integration tests.
*   **AI Healer (Stage 32):** The `ang_doctor` tool analyzes build logs and CUE intent to provide structured "diagnoses" and automated fix suggestions for common errors.
*   **Client-Side Security SDK (Stage 33):** Support for Zero-Knowledge architecture via `@encrypt(client_side="true")`. ANG generates TypeScript code to encrypt sensitive data on the client before it ever touches the server.

### Backend Generation

*   **Declarative Logic Flow:** Business logic is described as a chain of abstract actions. Supports nested control structures like `flow.If`, `flow.For`, and `tx.Block`, enabling complex orchestration without manual Go boilerplate.
*   **State Machine (FSM):** Entities define strict state transition rules. ANG automatically generates `CanTransitionTo` and `TransitionTo` methods, ensuring process integrity.
*   **Domain:** Entities, ports, repositories, stubs, adapters. Standardized `ID` naming convention (e.g., `UserID`, `CompanyID`) automatically enforced by the compiler.
*   **Documentation:** Automatic propagation of `description` from CUE to Go comments for `godoc` support.
*   **HTTP:** Handlers, routing, validation, RFC 9457 errors. Conditional imports to prevent unused package errors.
*   **Auth:** JWT with automated key loading from `.pem` files or ENV.
*   **RBAC:** Role/permission matrix with generated checks.
*   **Events:** Typed domain events, publisher interfaces, NATS adapter.
*   **Scheduler:** Autonomous cron-like tasks with configurable intervals.
*   **Observability:** Tracing, metrics, structured logging.
*   **Specs:** OpenAPI & AsyncAPI from CUE.

### Frontend SDK (TypeScript)

*   **API Client:** Axios with standardized naming, trace context, and RFC 9457 errors.
*   **Strict Typing:** Zero-error TypeScript generation with support for TanStack Query v5.
*   **WebSocket:** Typed events with dynamic MSW v2 mocking support based on CUE definitions.
*   **React Query:** Modern hooks with automatic invalidation and optimistic updates.
*   **MSW Mocks:** Dynamically generated handlers for both HTTP and WebSocket streams.

## Validation Pipeline

CUE `@validate` tags power all layers:

```cue
email: string @validate("required,email")
password: string @validate("min=8,max=64")
amount: int @validate("gte=0")
```

Enforced in:
- **Backend (Go):** Request validation
- **Frontend (Zod):** Form/schema validation
- **OpenAPI:** Schema constraints

## Extending ANG

### Add Custom Transformer

```go
type SearchTransformer struct{}

func (t *SearchTransformer) Name() string { return "search" }

func (t *SearchTransformer) Transform(schema *ir.Schema) error {
    for i := range schema.Entities {
        entity := &schema.Entities[i]
        if entity.Metadata["searchable"] == true {
            // Add search index configuration
            // Generate search methods
        }
    }
    return nil
}

// Register
pipeline.RegisterTransformer(&SearchTransformer{})
```

### Add Custom Attribute Hook

```go
type StripeHook struct{ transformers.BaseHook }

func (h *StripeHook) Attribute() string { return "stripe_payment" }

func (h *StripeHook) OnService(schema *ir.Schema, svc *ir.Service, attr ir.Attribute) error {
    // Add CreatePaymentIntent method
    svc.Methods = append(svc.Methods, ir.Method{
        Name: "CreatePaymentIntent",
        // ...
    })
    return nil
}

// Register
pipeline.RegisterHook(&StripeHook{})
```

### Add New Provider

```go
type RustAxumProvider struct{}

func (p *RustAxumProvider) Name() string { return "rust-axum-postgres" }

func (p *RustAxumProvider) Supports(target ir.Target) bool {
    return target.Lang == "rust" && target.Framework == "axum"
}

func (p *RustAxumProvider) Templates() fs.FS {
    return os.DirFS("templates/rust-axum")
}

func (p *RustAxumProvider) TypeMapping() providers.TypeMap {
    return providers.TypeMap{
        Mappings: map[ir.TypeKind]providers.TypeInfo{
            ir.KindString: {Type: "String"},
            ir.KindInt:    {Type: "i32"},
            ir.KindTime:   {Type: "chrono::DateTime<chrono::Utc>", Package: "chrono"},
        },
    }
}

// Register
pipeline.RegisterProvider(&RustAxumProvider{})
```

## AI-Friendly Resources

ANG includes resources to help AI models generate correct Flow DSL:

| File | Purpose |
|------|---------|
| `cue/GOLDEN_EXAMPLES.cue` | Canonical patterns with CUE → Go output |
| `cue/FLOW_DSL_REFERENCE.md` | Quick reference for all flow actions |
| `cue/schema/flow_helpers.cue` | Shorthand definitions (`#FindByID`, `#Save`, etc.) |
| `cue/schema/types.cue` | Full Flow DSL schema with documentation |

### Auto-completion

The compiler automatically injects missing fields before `repo.Save`:
- `ID` → `uuid.NewString()` (for new entities with "new" prefix)
- `CreatedAt` → `time.Now().UTC().Format(time.RFC3339)`

### Flow DSL Quick Example

```cue
CreateOrder: {
    flow: [
        {action: "mapping.Map", output: "newOrder", entity: "Order"},
        {action: "mapping.Assign", to: "newOrder.UserID", value: "req.UserID"},
        {action: "repo.Save", source: "Order", input: "newOrder"},
        {action: "mapping.Assign", to: "resp.ID", value: "newOrder.ID"},
    ]
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | 8080 | Server port |
| `DATABASE_URL` | postgres://user:password@localhost:5432/app | PostgreSQL |
| `REDIS_ADDR` | localhost:6379 | Redis |
| `NATS_URL` | nats://localhost:4222 | NATS |
| `JWT_PRIVATE_KEY` | — | RSA private key (PEM) |
| `JWT_PUBLIC_KEY` | — | RSA public key (PEM) |
| `S3_ENDPOINT` | http://localhost:9000 | S3/Minio endpoint |
| `S3_BUCKET` | attachments | S3 bucket name |

## AI & Agent Integration

ANG is built from the ground up to be **AI-Native**. It includes a sophisticated **MCP (Model Context Protocol)** server that allows agents (Cursor, Claude Code, Windsurf) to architect systems instead of just writing code.

### The Intent-First Policy (Mandatory for AI)

To ensure system integrity and prevent hallucinations, all AI agents must follow this protocol:
1. **Agent writes ONLY CUE**: All business logic and architecture must be defined in the `cue/` directory.
2. **ANG generates ALL code**: Implementation in `internal/`, `api/`, and `sdk/` is managed by the compiler. **NEVER** modify these files manually.
3. **Agent verifies via MCP**: Use `repo_diff` and `run_preset('unit')` to verify the generated implementation.

### Agent Quickstart (For AI)

If you are an AI agent, follow these steps to start:
1. **Bootstrap**: Always call `ang_bootstrap` first to load workflows and policies.
2. **Plan**: Call `ang_plan(goal="...")` to get a step-by-step architectural plan.
3. **Navigate**: Use `ang_search` and `repo_read_symbol` to find relevant logic without reading large files.
4. **Iterate**: Patch CUE -> `run_preset('build')` -> `repo_diff` -> `run_preset('unit')`.

### Setup for Claude Desktop

Add this to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "ang": {
      "command": "ang",
      "args": ["mcp"],
      "env": {
        "CWD": "/absolute/path/to/your/project"
      }
    }
  }
}
```

## MCP Response Compatibility

For MCP tool output, ANG supports two modes controlled by `ANG_MCP_ENVELOPE`:

- Unified envelope mode (default): enabled when var is unset (or `1/true/on/yes`).
- Legacy compatibility mode: set `ANG_MCP_ENVELOPE=off` (or `0/false/no`).

```bash
# default (unified envelope)
ang mcp

# compatibility (legacy/raw tool output)
ANG_MCP_ENVELOPE=off ang mcp
```

## MCP Health Tool

Run `ang_mcp_health` from MCP clients to inspect effective profile, limits, workflows,
runtime config status, and envelope mode.

Unified MCP envelope responses include `schema_version` (current default: `mcp-envelope/v1`).
