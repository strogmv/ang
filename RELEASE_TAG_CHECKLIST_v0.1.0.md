# Release Tag Checklist (`v0.1.0`)

## Pre-Tag

```bash
go test ./compiler/... ./cmd/ang
SKIP_PIP_SMOKE=1 ./scripts/release-smoke.sh
```

## Verify Notes

- Confirm `RELEASE_NOTES_v0.1.0.md` is final.
- Confirm "Known Limitations (v1)" section is unchanged.

## Tag

```bash
git tag -a v0.1.0 -m "ANG v0.1.0"
git push origin v0.1.0
```

## Post-Tag

- Ensure CI for tag is green.
- Attach release artifacts and OpenAPI compatibility report from `dist/release/reports`.
