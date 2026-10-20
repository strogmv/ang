package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractTargetReadsSecurityHeadersMetricsPortAndRouteRights(t *testing.T) {
	val := cuecontext.New().CompileString(`
#Target: {
	lang: string
	security_headers?: {csp?: string, coop?: string, permissions_policy?: string}
	metrics_port?: string | int
	route_rights?: {roles: [...string], rights?: [...string]}
}
state: target: #Target & {
	lang: "go"
	security_headers: {csp: "default-src 'self'", coop: ""}
	metrics_port: 9090
	route_rights: {roles: ["admin"], rights: ["view", "billing"]}
}
`)
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	td, err := New().ExtractTarget(val)
	if err != nil {
		t.Fatal(err)
	}
	h := td.SecurityHeaders
	if h == nil || h.CSP == nil || *h.CSP != "default-src 'self'" || h.COOP == nil || *h.COOP != "" || h.PermissionsPolicy != nil {
		t.Fatalf("security_headers: %+v", h)
	}
	if td.MetricsPort != "9090" {
		t.Fatalf("metrics_port: %q", td.MetricsPort)
	}
	if r := td.RouteRights; r == nil || len(r.Roles) != 1 || r.Roles[0] != "admin" || len(r.Rights) != 2 {
		t.Fatalf("route_rights: %+v", r)
	}
	empty := cuecontext.New().CompileString(`state: target: lang: "go"`)
	if td, _ := New().ExtractTarget(empty); td.SecurityHeaders != nil || td.MetricsPort != "" || td.RouteRights != nil {
		t.Fatalf("absent options stay unset, got %+v", td)
	}
}
