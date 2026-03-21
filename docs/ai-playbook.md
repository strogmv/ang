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

## AI Migration Pipeline (Legacy Code → CUE)

When migrating existing Go/Java/OpenAPI/SQL code into ANG intent:

```bash
# Step 1 — deterministic fact extraction (no AI)
ang extract ./src --from auto --out facts.json
# MCP: ang_extract(source_path="./src", out="facts.json")

# Step 2 — AI-powered CUE generation (requires ANTHROPIC_API_KEY)
ang gen --facts facts.json --dry-run            # preview
ang gen --facts facts.json --out cue/api        # write
# MCP: ang_gen(facts_path="facts.json", dry_run=true)

# Step 3 — validate + auto-fix
ang ops vet --fix
# MCP: ang_ops_vet(fix=true)

# Step 4 — compile
ang build
# MCP: ang_generate
```

**Agent rules for migration:**
- Always run `ang_extract` first — never invent field names, use extracted facts as ground truth.
- Use `dry_run=true` on `ang_gen` first to review before writing.
- After `ang_gen`, always run `ang ops vet --fix` before `ang build`.
- Use `ang ops vet --proof --json` to verify correctness properties (auth, validation).

## Available MCP Tools

| Tool | Purpose |
|------|---------|
| `ang_mcp_health` | Session bootstrap health check |
| `ang_schema` | Get operation CUE schema for AI |
| `ang_validate` | Fast CUE validation |
| `ang_dry_run` | Validate + diff without writing files |
| `ang_generate` | Full build (CUE → Go/Python) |
| `ang_ops_vet` | Semantic linting (`fix`, `proof`, `explain` flags) |
| `ang_openapi` | Generate OpenAPI spec without full build |
| `ang_extract` | Extract facts from Go/Java/OpenAPI/SQL |
| `ang_gen` | Generate CUE from facts via Claude API |
| `ang_sdk_version` | Show current project semver |
| `ang_sdk_bump` | Bump `patch`/`minor`/`major` in `project.cue` |
| `ang_event_map` | Publisher/subscriber event map |
| `ang_rbac_inspector` | RBAC policy audit |
| `ang_validate_logic` | Syntax-check Go snippet before inserting |
| `ang_search` | Search across CUE/generated/templates |
| `ang_cue_apply_patch` | Apply structured CUE edit |
| `repo_diff` | Token-efficient diff of generated Go |

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
