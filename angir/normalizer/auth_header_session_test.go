package normalizer

import (
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func extractAuthFrom(t *testing.T, jwt string) (*AuthDef, error) {
	t.Helper()
	val := cuecontext.New().CompileString(`#Auth: {service: "auth", jwt: {` + jwt + `}}`)
	if val.Err() != nil {
		t.Fatal(val.Err())
	}
	return New().ExtractAuth(val)
}

func TestExtractAuth_HeaderSessionAndSessionClaims(t *testing.T) {
	t.Parallel()

	auth, err := extractAuthFrom(t, `
		mode: "opaque_session_cookie"
		session_claims: ["imp", "act"]
		session_expiry_claim: "sexp"
		ops: sessions: {
			exchange: {op: "ExchangeCode", access_field: "accessToken", refresh_field: "refreshToken", credential: "header", session_field: "sessionToken"}
			google: {op: "CompleteGoogleAuth", access_field: "accessToken", refresh_field: "refreshToken"}
		}`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(auth.SessionClaims, []string{"imp", "act"}) || auth.SessionExpiryClaim != "sexp" {
		t.Fatalf("session claims = %v, expiry = %q", auth.SessionClaims, auth.SessionExpiryClaim)
	}
	byOp := map[string]AuthSessionOp{}
	for _, so := range auth.SessionOps {
		byOp[so.Op] = so
	}
	if so := byOp["ExchangeCode"]; !so.HeaderCredential() || so.SessionField != "sessionToken" {
		t.Fatalf("exchange = %+v", so)
	}
	if so := byOp["CompleteGoogleAuth"]; so.HeaderCredential() || so.SessionField != "" {
		t.Fatalf("a session op without credential stays a cookie one: %+v", so)
	}
}

func TestExtractAuth_HeaderSessionMisconfigured(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ jwt, want string }{
		"no session field": {`mode: "opaque_session_cookie", ops: sessions: x: {op: "X", access_field: "a", refresh_field: "r", credential: "header"}`, "no session_field"},
		"not opaque":       {`mode: "web_session_cookie", ops: sessions: x: {op: "X", access_field: "a", refresh_field: "r", credential: "header", session_field: "s"}`, "opaque_session_cookie"},
		"unknown":          {`mode: "opaque_session_cookie", ops: sessions: x: {op: "X", access_field: "a", refresh_field: "r", credential: "query"}`, `"query"`},
	} {
		_, err := extractAuthFrom(t, tc.jwt)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err = %v, want it to mention %s", name, err, tc.want)
		}
	}
}
