# Getting Started

This guide targets a clean machine and a new project.

## Prerequisites

- Go `1.25+`
- Docker with Compose (`docker compose`)
- ANG binary available in `PATH`

Check:

```bash
ang version
go version
docker compose version
```

## Create a Project

Template bootstrap:

```bash
ang init my-app --template saas --module github.com/acme/my-app
cd my-app
```

Minimal bootstrap:

```bash
ang init my-app --module github.com/acme/my-app
cd my-app
```

## What `ang init` Creates

Core scaffold:

- `cue/` and `cue.mod/`
- `go.mod`, `go.work`
- `.env.example`
- `docker-compose.yml`
- `Makefile`, `Taskfile.yml`, `scripts/dev-*.sh`
- `AGENTS.md`
- `RUNBOOK.md`
- `tests/contract/contract_test.go`

## First Run (Recommended)

```bash
ang up
```

`ang up` executes:

1. `ang doctor start` (preflight checks)
2. `docker compose up` (if compose file exists)
3. `ang build`
4. `ang smoke`

## Manual Run (Step-by-Step)

```bash
ang doctor start
docker compose up -d
ang build
ang smoke
```

## Daily Workflow

```bash
ang validate
ang lint
ang build
go test ./...
```

## Fast Task Runner (Optional)

If [`task`](https://taskfile.dev/) is installed:

```bash
task up
task validate
task lint
task build
```

## Existing Projects (Upgrade Path)

If your project was initialized before startup DX updates:

1. Ensure `go.mod` exists in project root.
2. Add or regenerate `.env.example`.
3. Run `ang doctor start` and fix all `FAIL` items.
4. Prefer `build.mode = "in_place"` for root-runtime projects.

## Next Reads

- [Commands](./commands.md)
- [Config](./config.md)
- [Troubleshooting](./troubleshooting.md)
- [Copy/paste recipes](./recipes.md)
