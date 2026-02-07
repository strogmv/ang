# ANG: AI-Native Compiler Toolchain for Backend Development

ANG is a deterministic compiler that transforms **Intent (CUE)** into **Clean Implementation (Go/TS/SQL)**. 

## The Formula
`Natural Language -> CUE (Contract) -> ANG (Compiler) -> Code -> Tests/Feedback`

## Core Philosophy: Intent-First
In the age of AI, the bottleneck is not "writing code", but **"maintaining truth"**.
- **CUE is the Truth**: Formal, typed, and validatable source of intent.
- **ANG is the Law**: Deterministic generation. If it compiles, it follows the architecture.
- **LLM is the Student**: AI does not "know" how to build the system; it learns through iterative feedback from the ANG compiler.

## Roles & Responsibilities

### AI Agent (The Architect)
- **Writes only CUE** in the `/cue` directory.
- Reads `resource://ang/ai_hints` to understand patterns.
- Activates **Deterministic Transformers** via declarative hooks (e.g., `@cache`, `@encrypt`).
- Reacts to `ang_validate` structured errors.
- **NEVER** touches generated code.

## Deterministic Transformers
ANG uses IR-level transformers to inject cross-cutting concerns safely:
- **Tracing**: Automated OpenTelemetry spans for all services.
- **Observability**: Integrated profiling (pprof) and metrics.
- **Security**: Field-level encryption (`@encrypt`) and redaction (`@redact`).
- **Audit**: Comprehensive audit logs for regulated actions (`@audit`).

## Compliance Profiles
Architecture can be governed by profiles defined in CUE:
- **dev**: Optimized for speed, loose security, full observability.
- **prod**: Strict security, rate limiting, and monitoring.
- **regulated**: Mandatory audit trails, PII redaction, and strict access controls (HIPAA/GDPR ready).

### ANG Compiler (The Guardrail)
- Validates architectural invariants (e.g., cross-service ownership).
- Generates 100% of the implementation code.
- Measures **Contract Coverage** (how many CUE invariants are tested).
- Provides machine-fixable diagnostics via MCP.
- Enforces the "Agent writes only CUE" policy.

## The Goal: "3-Iteration Build"
Any complex backend feature should be successfully implemented from a single Natural Language prompt in **3 iterations or less** of the `CUE -> Validate -> Generate -> Test` loop. Contract-level test coverage ensures that 100% of business invariants are verified.
