#!/usr/bin/env bash
# Static checks for the generator (not for examples/*, which is generated and
# has its own module): gofmt, then staticcheck with
#   U1000  unused code — dead functions, types, vars and fields
#   SA*    misuse and bugs, deprecated APIs included (SA1019)
#   S*     simplifications
# ST1005 (error string style) is left out: ANG's messages end with CUE field
# names such as `where:` and `service:`, which it would flag.
#
# A func that only a template calls must be registered in a template.FuncMap;
# that registration is a use, so U1000 does not flag it.
#
# staticcheck runs pinned through `go run` unless STATICCHECK names a binary.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

STATICCHECK_VERSION="${STATICCHECK_VERSION:-v0.8.2}"
CHECKS="${STATICCHECK_CHECKS:-U1000,SA*,S*}"

echo "== gofmt =="
# Only files under version control: ignored build output (dist/, bin/) is not ours to format.
unformatted="$(git ls-files -z -- '*.go' ':!:examples/**' | xargs -0 gofmt -l)"
if [ -n "$unformatted" ]; then
	echo "gofmt -w these files:"
	echo "$unformatted"
	exit 1
fi

echo "== staticcheck -checks $CHECKS =="
if [ -n "${STATICCHECK:-}" ]; then
	"$STATICCHECK" -checks "$CHECKS" ./...
else
	go run "honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION}" -checks "$CHECKS" ./...
fi
echo "lint: ok"
