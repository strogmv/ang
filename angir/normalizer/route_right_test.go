package normalizer

import (
	"strings"
	"testing"
)

func TestExtractEndpointsReadsRouteRight(t *testing.T) {
	t.Parallel()
	eps, err := extractRateLimitEndpoints(t, `
  PlaceBid: {
    method: "GET"
    path: "/api/admin/bids"
    auth: {type: "jwt", roles: ["admin"], right: "billing"}
  }`)
	if err != nil {
		t.Fatal(err)
	}
	if ep := endpointByRPC(t, eps, "PlaceBid"); ep.AuthRight != "billing" {
		t.Fatalf("auth.right = %q", ep.AuthRight)
	}
	for name, block := range map[string]string{
		"no roles":  `auth: {type: "jwt", right: "billing"}`,
		"public":    `auth: {public: true, right: "billing"}`,
		"empty":     `auth: {type: "jwt", roles: ["admin"], right: ""}`,
		"websocket": `auth: {type: "jwt", roles: ["admin"], right: "billing"}, method: "WS"`,
	} {
		method := `method: "POST"`
		if strings.Contains(block, `method: "WS"`) {
			method = ""
		}
		_, err := extractRateLimitEndpoints(t, `
  PlaceBid: {
    `+method+`
    path: "/api/admin/bids"
    `+block+`
  }`)
		if err == nil || !strings.Contains(err.Error(), "right") {
			t.Errorf("%s: expected an auth.right error, got %v", name, err)
		}
	}
}
