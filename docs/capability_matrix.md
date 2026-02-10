# Target Capability Matrix

ANG resolves each `state.target(s)` entry into a capability set.

Core capabilities:

- `http`
- `sql_repo`
- `ws`
- `events`
- `auth`

Profile capabilities:

- `profile_go_legacy`
- `profile_python_fastapi`

## Why

Generation should be selected by capabilities, not by hardcoded language branches.
This keeps pipeline logic extensible as new targets are added.

## Current Resolution Rules

Source: `compiler/capabilities.go`

- `go/*/*` => `profile_go_legacy`
- `python/fastapi/postgres` => `profile_python_fastapi`
- DB `postgres|mysql|sqlite` => `sql_repo`
- Queue not `none` => `events`
- Go target => `ws`
- Go and Python/FastAPI targets => `auth`

## Build Integration

`ang build` now executes emitter steps via capability requirements.
Each step declares `requires: []Capability`.
Missing capabilities cause a step skip with explicit log output.

