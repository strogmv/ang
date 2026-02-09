# ANG v0.1.0 Release Notes

Date: 2026-02-09

## Highlights

- Multi-target generation from one CUE intent (`go` + `python/fastapi`).
- Python SDK generation (sync + async clients, auth hook, packaging metadata).
- Python FastAPI backend scaffold generation.
- CUE-first example: `examples/pdf-chart-python` (PDF chart endpoint generation).

## Build And Verify

```bash
go test ./compiler/... ./cmd/ang
go run ./cmd/ang build
SKIP_PIP_SMOKE=1 ./scripts/release-smoke.sh
```

## Known Limitations (v1)

- Python backend is scaffold-first; repository and flow parity with Go is partial.
- Some generated service implementations require manual completion for production behavior.
- Generated Python dependencies are minimal and may require explicit extra install (`matplotlib`, `reportlab`) for example workloads.

## Artifacts

- Go release tree: `dist/release/go-service`
- Python release tree: `dist/release/python-service`
- OpenAPI compatibility report:
  - `dist/release/reports/openapi-go-vs-python.md`
  - `dist/release/reports/openapi-go-vs-python.diff`
