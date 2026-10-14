# AGENTS.md

This is the ANG generator repository; read CLAUDE.md for the layout and checks.

1. Go code under `cmd/`, `compiler/`, `angir/`, `templates/` and `tool/` is hand-written source: edit
   it directly, with a test.
2. Generated code exists only in `examples/*` (everything there except `cue/` and `ang.yaml`). Never
   hand-edit it: change the CUE, templates or emitters and regenerate with
   `go run ./cmd/ang build --project examples/blog`.
3. Before saying a change is done: `go build ./...`, `go vet ./...`, `go test ./... -count=1`.
4. Prefer minimal, deterministic changes and preserve unrelated edits.
