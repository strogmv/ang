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

## Why

Generation should be selected by capabilities, not by hardcoded language branches.
This keeps pipeline logic extensible as new targets are added.

## Current Resolution Rules

Source: `compiler/capabilities.go`

- Only the stacks in `SupportedTargetValues` are accepted: lang `go`; framework `chi` or `grpc`;
  db `postgres`; cache `redis|none`; queue `nats|none`; storage `s3|none` (empty = default).
  Anything else fails with "unsupported target".
- `go` => `profile_go_legacy`, `ws`, `auth`
- framework `chi|grpc` => `http`; `grpc` => `grpc`
- db `postgres` => `sql_repo` (MongoDB repositories are a usage capability, chosen per entity)
- queue not `none` => `events`

## Build Integration

`ang build` now executes emitter steps via capability requirements.
Each step declares `requires: []Capability`.
Missing capabilities cause a step skip with explicit log output.

