package emitter

import (
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestAuthCookieFields(t *testing.T) {
	auth := &normalizer.AuthDef{
		Mode:                    "opaque_session_cookie",
		LoginOp:                 "LoginUser",
		LoginAccessField:        "accessToken",
		LoginRefreshField:       "refreshToken",
		RegisterOp:              "RegisterUser",
		RegisterAccessField:     "accessToken",
		RegisterRefreshField:    "refreshToken",
		DemoSessionOp:           "CreateDemoSession",
		DemoSessionAccessField:  "accessToken",
		DemoSessionRefreshField: "refreshToken",
		RefreshOp:               "RefreshToken",
		RefreshAccessField:      "accessToken",
		RefreshRefreshField:     "refreshToken",
	}

	cases := []struct {
		rpc            string
		wantAccess     string
		wantRefresh    string
		wantOK         bool
	}{
		{"LoginUser", "accessToken", "refreshToken", true},
		{"RegisterUser", "accessToken", "refreshToken", true},
		{"CreateDemoSession", "accessToken", "refreshToken", true},
		{"RefreshToken", "accessToken", "refreshToken", true},
		{"LogoutUser", "", "", false},
	}

	for _, tc := range cases {
		access, refresh, ok := authCookieFields(auth, tc.rpc)
		if ok != tc.wantOK || access != tc.wantAccess || refresh != tc.wantRefresh {
			t.Fatalf("authCookieFields(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.rpc, access, refresh, ok, tc.wantAccess, tc.wantRefresh, tc.wantOK)
		}
	}

	if !cookieAuthEnabled(auth) {
		t.Fatal("expected cookie auth enabled for opaque_session_cookie mode")
	}
}
