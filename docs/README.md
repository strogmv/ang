# ANG Documentation Map

This folder is organized so both humans and AI agents can find the right entry point quickly.

## Start Here

- [Getting Started](./getting-started.md): first run, bootstrap flow, and generated project layout.
- [Project Understanding Guide](./project-understanding.md): architecture map and recommended reading order before writing docs.
- [Commands](./commands.md): CLI reference with practical examples.
- [Expert Report Versioning](./expert-report-versioning.md): compatibility and determinism policy for `ang/expert-report/v1`.
- [Import Report](./import-report.md): JSON report schema and triage workflow for `ang import java`.
- [Java -> CUE E2E](./java-to-cue-e2e.md): practical migration flow from Java sources to CUE contracts.
- [Java Parsing Improvements](./java-parsing-improvements.md): roadmap for higher-fidelity Java -> IR -> CUE extraction.
- [Config](./config.md): `.env` contract, precedence, and validation workflow.
- [Troubleshooting](./troubleshooting.md): common failures and fast recovery paths.
- [AI Playbook](./ai-playbook.md): MCP-first workflow and safe edit rules for coding agents.
- [Improvement Backlog](./improvements.md): prioritized engineering hardening tasks.

## Compiler Internals

- [Architecture](./architecture.md): parser/normalizer/flowsem/emitter pipeline and extension points.
- [Flow Semantics](./flow-semantics.md): flow action contracts, scope model, and control flow semantics.
- [Compiler Contract](./compiler_contract.md)
- [IR Versioning](./ir_versioning.md)
- [Planner Contract](./planner_contract.md)
- [Capability Matrix](./capability_matrix.md)
- [Generator Modules](./generator_modules.md)
- [Universal Flow Actions](./universal-flow-actions.md)
- [Error Codes](./error_codes.md)

## Release Notes

- [2026-03-01: Init DX + Startup Reliability](./changelog/2026-03-01-init-dx-and-startup.md)
