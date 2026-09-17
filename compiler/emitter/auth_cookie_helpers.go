package emitter

import (
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
)

func cookieAuthEnabled(auth *normalizer.AuthDef) bool {
	if auth == nil {
		return false
	}
	mode := strings.TrimSpace(auth.Mode)
	return mode == "web_session_cookie" || mode == "opaque_session_cookie"
}

func authCookieFields(auth *normalizer.AuthDef, rpc string) (accessField, refreshField string, ok bool) {
	if auth == nil {
		return "", "", false
	}
	switch strings.TrimSpace(rpc) {
	case strings.TrimSpace(auth.LoginOp):
		return auth.LoginAccessField, auth.LoginRefreshField, auth.LoginAccessField != "" && auth.LoginRefreshField != ""
	case strings.TrimSpace(auth.RegisterOp):
		return auth.RegisterAccessField, auth.RegisterRefreshField, auth.RegisterAccessField != "" && auth.RegisterRefreshField != ""
	case strings.TrimSpace(auth.DemoSessionOp):
		return auth.DemoSessionAccessField, auth.DemoSessionRefreshField, auth.DemoSessionAccessField != "" && auth.DemoSessionRefreshField != ""
	case strings.TrimSpace(auth.RefreshOp):
		return auth.RefreshAccessField, auth.RefreshRefreshField, auth.RefreshAccessField != "" && auth.RefreshRefreshField != ""
	default:
		return "", "", false
	}
}

func authCookieFuncMap(auth *normalizer.AuthDef) map[string]any {
	return map[string]any{
		"CookieAuthEnabled": func() bool {
			return cookieAuthEnabled(auth)
		},
		"AuthCookieAccessField": func(rpc string) string {
			access, _, ok := authCookieFields(auth, rpc)
			if !ok {
				return ""
			}
			return access
		},
		// AuthLogoutTokenField names the request field of the logout operation
		// that carries the refresh token; empty for every other operation.
		"AuthLogoutTokenField": func(rpc string) string {
			if auth == nil || strings.TrimSpace(auth.LogoutOp) == "" || strings.TrimSpace(rpc) != strings.TrimSpace(auth.LogoutOp) {
				return ""
			}
			if field := strings.TrimSpace(auth.LogoutTokenField); field != "" {
				return field
			}
			return "refreshToken"
		},
		"AuthIsLogoutAll": func(rpc string) bool {
			return auth != nil && strings.TrimSpace(auth.LogoutAllOp) != "" && strings.TrimSpace(rpc) == strings.TrimSpace(auth.LogoutAllOp)
		},
		"AuthCookieRefreshField": func(rpc string) string {
			_, refresh, ok := authCookieFields(auth, rpc)
			if !ok {
				return ""
			}
			return refresh
		},
	}
}
