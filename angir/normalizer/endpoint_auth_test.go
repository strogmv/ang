package normalizer

import (
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractEndpointsReadsAuthIntent(t *testing.T) {
	t.Parallel()

	eps, err := extractRateLimitEndpoints(t, `
  PlaceBid: {
    method: "POST"
    path: "/api/bids"
    auth: {required: true, scope: ["bids:write"]}
  }
  Login: {
    method: "POST"
    path: "/api/login"
    auth: {public: true, reason: "  anyone signs in; rate-limited  "}
  }`)
	if err != nil {
		t.Fatal(err)
	}
	bid := endpointByRPC(t, eps, "PlaceBid")
	if bid.AuthType != "jwt" || !reflect.DeepEqual(bid.RequiredScopes, []string{"bids:write"}) {
		t.Fatalf("required: true must mean a jwt caller with the scope, got type %q scopes %v", bid.AuthType, bid.RequiredScopes)
	}
	if EndpointIsExplicitlyPublic(bid.Metadata) {
		t.Fatal("a required endpoint is not public")
	}
	login := endpointByRPC(t, eps, "Login")
	if login.AuthType != "" || !EndpointIsExplicitlyPublic(login.Metadata) {
		t.Fatalf("public: true must stay without auth and be marked public, got type %q metadata %v", login.AuthType, login.Metadata)
	}
	if got := EndpointPublicReason(login.Metadata); got != "anyone signs in; rate-limited" {
		t.Fatalf("reason = %q", got)
	}
}

func TestExtractEndpointsWithoutAuthIntentIsUnchanged(t *testing.T) {
	t.Parallel()

	eps, err := extractRateLimitEndpoints(t, `
  PlaceBid: {method: "POST", path: "/api/bids", auth: {type: "jwt", inject: ["userId"]}}
  Login: {method: "POST", path: "/api/login"}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, ep := range eps {
		if len(ep.Metadata) != 0 {
			t.Fatalf("%s: no new syntax must add no metadata, got %v", ep.RPC, ep.Metadata)
		}
	}
	if login := endpointByRPC(t, eps, "Login"); login.AuthType != "" {
		t.Fatalf("an endpoint without auth stays public, got %q", login.AuthType)
	}
}

func TestExtractEndpointsRejectsContradictoryAuth(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"public with a type":     `auth: {public: true, type: "jwt"}`,
		"public with roles":      `auth: {public: true, roles: ["admin"]}`,
		"public with inject":     `auth: {public: true, inject: ["userId"]}`,
		"public and required":    `auth: {public: true, required: true}`,
		"reason without public":  `auth: {required: true, reason: "because"}`,
		"required false + scope": `auth: {required: false, scope: "x"}`,
	}
	for name, auth := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := extractRateLimitEndpoints(t, `
  Login: {method: "POST", path: "/api/login", `+auth+`}`)
			if err == nil || !strings.Contains(err.Error(), "HTTP endpoint Login") {
				t.Fatalf("want an error naming the endpoint, got %v", err)
			}
		})
	}
}

func TestExtractTargetReadsHTTPSecurityOptions(t *testing.T) {
	val := cuecontext.New().CompileString(`
state: target: {
	lang: "go"
	auth_default: "deny"
	service_config: true
	operation_metrics: true
}
`)
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	td, err := New().ExtractTarget(val)
	if err != nil {
		t.Fatal(err)
	}
	if td.AuthDefault != "deny" || !td.ServiceConfig || !td.OperationMetrics {
		t.Fatalf("got auth_default %q service_config %v operation_metrics %v", td.AuthDefault, td.ServiceConfig, td.OperationMetrics)
	}
	empty := cuecontext.New().CompileString(`state: target: lang: "go"`)
	if td, _ := New().ExtractTarget(empty); td.AuthDefault != "" || td.ServiceConfig || td.OperationMetrics {
		t.Fatalf("absent options must stay off, got %+v", td)
	}
}
