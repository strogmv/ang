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
	case strings.TrimSpace(auth.MfaVerifyOp):
		return auth.MfaVerifyAccessField, auth.MfaVerifyRefreshField, auth.MfaVerifyAccessField != "" && auth.MfaVerifyRefreshField != ""
	case strings.TrimSpace(auth.PasswordChangeOp):
		return auth.PasswordChangeAccessField, auth.PasswordChangeRefreshField, auth.PasswordChangeAccessField != "" && auth.PasswordChangeRefreshField != ""
	case strings.TrimSpace(auth.RefreshOp):
		return auth.RefreshAccessField, auth.RefreshRefreshField, auth.RefreshAccessField != "" && auth.RefreshRefreshField != ""
	default:
		for _, so := range auth.SessionOps {
			if strings.TrimSpace(so.Op) == strings.TrimSpace(rpc) {
				return so.AccessField, so.RefreshField, so.AccessField != "" && so.RefreshField != ""
			}
		}
		return "", "", false
	}
}

// authHeaderSessionOp returns the session operation rpc names when it issues
// a header session (credential: "header").
func authHeaderSessionOp(auth *normalizer.AuthDef, rpc string) (normalizer.AuthSessionOp, bool) {
	if auth == nil {
		return normalizer.AuthSessionOp{}, false
	}
	for _, so := range auth.SessionOps {
		if strings.TrimSpace(so.Op) == strings.TrimSpace(rpc) && so.HeaderCredential() {
			return so, true
		}
	}
	return normalizer.AuthSessionOp{}, false
}

func authCookieFuncMap(auth *normalizer.AuthDef) map[string]any {
	return map[string]any{
		// AuthHeaderSessionField names the response field that carries the
		// opaque id of a header session; empty for every other operation.
		"AuthHeaderSessionField": func(rpc string) string {
			so, ok := authHeaderSessionOp(auth, rpc)
			if !ok {
				return ""
			}
			return so.SessionField
		},
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
		// AuthIsPasswordChange: the operation that changes the password of
		// the signed-in user. The handler ends the user's other sessions
		// before writing the fresh one it returns.
		"AuthIsPasswordChange": func(rpc string) bool {
			return auth != nil && strings.TrimSpace(auth.PasswordChangeOp) != "" && strings.TrimSpace(rpc) == strings.TrimSpace(auth.PasswordChangeOp)
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
