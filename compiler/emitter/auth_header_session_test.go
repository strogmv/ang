package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
)

func headerSessionAuth() *normalizer.AuthDef {
	return &normalizer.AuthDef{
		Mode:               "opaque_session_cookie",
		Alg:                "RS256",
		UserIDClaim:        "uid",
		CompanyIDClaim:     "cid",
		RolesClaim:         "roles",
		PermissionsClaim:   "perms",
		LoginOp:            "LoginUser",
		LoginAccessField:   "accessToken",
		LoginRefreshField:  "refreshToken",
		SessionClaims:      []string{"imp", "act"},
		SessionExpiryClaim: "sexp",
		SessionOps: []normalizer.AuthSessionOp{
			{Op: "ExchangeCode", AccessField: "accessToken", RefreshField: "refreshToken", Credential: "header", SessionField: "sessionToken"},
			{Op: "CompleteGoogleAuth", AccessField: "accessToken", RefreshField: "refreshToken"},
		},
	}
}

func TestAuthHeaderSessionField(t *testing.T) {
	t.Parallel()

	field := authCookieFuncMap(headerSessionAuth())["AuthHeaderSessionField"].(func(string) string)
	if got := field("ExchangeCode"); got != "sessionToken" {
		t.Fatalf("AuthHeaderSessionField(ExchangeCode) = %q, want sessionToken", got)
	}
	for _, rpc := range []string{"CompleteGoogleAuth", "LoginUser", "Other"} {
		if got := field(rpc); got != "" {
			t.Fatalf("AuthHeaderSessionField(%s) = %q, want empty: only a header session op has one", rpc, got)
		}
	}
	if authCookieFuncMap(nil)["AuthHeaderSessionField"].(func(string) string)("ExchangeCode") != "" {
		t.Fatal("no auth config, no header session")
	}
}

func TestEmitHTTPCommon_HeaderSessionsAndSessionAttrs(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	if err := em.EmitHTTPCommon(headerSessionAuth()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "internal", "transport", "http", "common.go"))
	if err != nil {
		t.Fatal(err)
	}
	generated := string(data)
	for _, want := range []string{
		`authSessionHeaderName       = "X-Auth-Session"`,
		`authSessionQueryParam       = "auth_session"`,
		`authSessionClaims      = []string{"imp", "act"}`,
		`authSessionExpiryClaim = "sexp"`,
		// The header is read before the cookie, the query only on a socket.
		"func sessionCredential(r *http.Request) (string, bool)",
		`strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket")`,
		// A header session never doubles as a cookie one, nor the other way round.
		"if (payload.Credential == authSessionCredentialHeader) != viaHeader {",
		// The expiry claim caps the session.
		"if exp := sessionExpiryFromClaims(claims); exp > 0 && exp < payload.ExpiresAt {",
		"func writeHeaderSession(ctx context.Context, accessToken, refreshToken string) (string, error)",
		"auth.RegisterSessionEnder(endSessionByHandle)",
		"rCtx = reqctx.WithAuthSession(rCtx, sessionHandle, sessionAttrs)",
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("generated common.go is missing %q", want)
		}
	}
	// writeHeaderSession must not touch cookies.
	start := strings.Index(generated, "func writeHeaderSession(")
	end := strings.Index(generated[start:], "\n}\n")
	if body := generated[start : start+end]; strings.Contains(body, "SetCookie") || strings.Contains(body, "r.Cookie(") {
		t.Fatalf("writeHeaderSession touches cookies:\n%s", body)
	}
}

func TestEmitHTTPCommon_NoSessionClaimsByDefault(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	if err := em.EmitHTTPCommon(&normalizer.AuthDef{Mode: "opaque_session_cookie"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "internal", "transport", "http", "common.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `authSessionExpiryClaim = ""`) || !strings.Contains(string(data), "authSessionClaims      = []string{}") {
		t.Fatal("without session_claims the session keeps no claims and has no expiry claim")
	}
}

func TestEmitHTTP_HeaderSessionHandler(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	tokenFields := []ir.Field{
		{Name: "accessToken", Type: ir.TypeRef{Kind: ir.KindString}},
		{Name: "refreshToken", Type: ir.TypeRef{Kind: ir.KindString}},
	}
	schema := &ir.Schema{
		Services: []ir.Service{{
			Name: "Auth",
			Methods: []ir.Method{
				{
					Name:   "ExchangeCode",
					Input:  &ir.Entity{Name: "ExchangeCodeRequest", Fields: []ir.Field{{Name: "code", Type: ir.TypeRef{Kind: ir.KindString}}}},
					Output: &ir.Entity{Name: "ExchangeCodeResponse", Fields: append([]ir.Field{{Name: "sessionToken", Type: ir.TypeRef{Kind: ir.KindString}}}, tokenFields...)},
				},
				{
					Name:   "LoginUser",
					Input:  &ir.Entity{Name: "LoginUserRequest", Fields: []ir.Field{{Name: "email", Type: ir.TypeRef{Kind: ir.KindString}}}},
					Output: &ir.Entity{Name: "LoginUserResponse", Fields: tokenFields},
				},
			},
		}},
		Endpoints: []ir.Endpoint{
			{Method: "POST", Path: "/api/auth/exchange", Service: "Auth", RPC: "ExchangeCode"},
			{Method: "POST", Path: "/api/auth/login", Service: "Auth", RPC: "LoginUser"},
		},
	}
	em := New(root, filepath.Join(root, "sdk"), "templates")
	if err := em.EmitServiceFromIR(schema); err != nil {
		t.Fatal(err)
	}
	if err := em.EmitHTTPFromIR(schema, headerSessionAuth()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "internal", "transport", "http", "auth.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	exchange := handlerBody(t, src, "handlerExchangeCode")
	for _, want := range []string{
		"headerSessionID, err := writeHeaderSession(r.Context(), resp.AccessToken, resp.RefreshToken)",
		"resp.SessionToken = headerSessionID",
		`resp.AccessToken = ""`,
		`resp.RefreshToken = ""`,
	} {
		if !strings.Contains(exchange, want) {
			t.Fatalf("header session handler is missing %q:\n%s", want, exchange)
		}
	}
	if strings.Contains(exchange, "writeAuthCookies") || strings.Contains(exchange, "AUTH_SESSION_HEADER_PRESENT") {
		t.Fatalf("a header session handler writes no cookie and accepts a tab that has one:\n%s", exchange)
	}
	login := handlerBody(t, src, "handlerLoginUser")
	if !strings.Contains(login, "if requestHasHeaderSession(r) {") || !strings.Contains(login, "writeAuthCookies(w, r, resp.AccessToken, resp.RefreshToken)") {
		t.Fatalf("a cookie sign-in must refuse a tab with a header session and write the cookie otherwise:\n%s", login)
	}
}

func handlerBody(t *testing.T, src, name string) string {
	t.Helper()
	start := strings.Index(src, "func "+name+"(")
	if start < 0 {
		t.Fatalf("no %s in:\n%s", name, src)
	}
	end := strings.Index(src[start:], "\n}\n")
	if end < 0 {
		return src[start:]
	}
	return src[start : start+end]
}
