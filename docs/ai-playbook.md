# AI Playbook (MCP-First)

This guide defines the safest workflow for AI agents working with ANG projects.

## Session Bootstrap (Mandatory)

If MCP tools are available, always start with:

1. `ang_mcp_health`
2. `ang_schema`
3. `ang_validate`

If MCP is unavailable, state that explicitly and continue with CLI fallback.

## Source-of-Truth Rules

Edit intent first:

- `cue/`
- `cue.mod/`

Do not manually edit generated artifacts unless explicitly required:

- `internal/`
- `api/`
- `sdk/`
- `db/schema/`
- `db/queries/`

## Safe Change Loop

1. Read current intent and affected endpoints/services.
2. Apply minimal CUE change.
3. Run `ang validate`.
4. Run `ang lint`.
5. Run `ang build`.
6. Run relevant tests.
7. Review generated diff for unintended churn.

## Fast Local Startup Loop

```bash
ang doctor start
ang up
```

When debugging runtime only:

```bash
ang smoke
ang config doctor
```

## Prompting Pattern for Agents

Use this structure:

1. Goal (business outcome)
2. Affected CUE area (`cue/domain`, `cue/api`, `cue/architecture`, etc.)
3. Constraints (security/performance/backward compatibility)
4. Required verification commands

Example:

```text
Add tenant-level rate limit to billing endpoints.
Change only cue/policies and cue/api.
Keep OpenAPI backward compatible.
Run: ang validate, ang lint, ang build, go test ./compiler/...
```

## Common Agent Mistakes to Avoid

- Editing generated Go instead of upstream CUE intent.
- Skipping `ang doctor start` and failing later on env/tool issues.
- Mixing release/in_place mode assumptions.
- Ignoring build “blind spots” warnings.

## Commit Hygiene

- Keep commits scoped by concern (flowsem, emitter, docs, templates).
- Include tests with every behavior change.
- Add changelog entry for user-visible CLI or scaffold changes.
