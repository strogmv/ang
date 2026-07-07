package emitter

import (
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
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
		"AuthCookieRefreshField": func(rpc string) string {
			_, refresh, ok := authCookieFields(auth, rpc)
			if !ok {
				return ""
			}
			return refresh
		},
	}
}
