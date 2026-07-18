package http

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	authredis "github.com/strogmv/ang/internal/adapter/auth/redis"
	statestoreredis "github.com/strogmv/ang/internal/adapter/statestore/redis"
	"github.com/strogmv/ang/internal/config"
	"github.com/strogmv/ang/internal/pkg/auth"
	"github.com/strogmv/ang/internal/pkg/circuitbreaker"
	"github.com/strogmv/ang/internal/pkg/errors"
	"github.com/strogmv/ang/internal/pkg/rbac"
	"github.com/strogmv/ang/internal/pkg/reqctx"
	"github.com/strogmv/ang/internal/port"
)

var validate = validator.New()

type authContextKey struct{}

type authContext struct {
	UserID    string
	CompanyID string
	Roles     []string
	Perms     []string
	Scopes    []string
	Locale    string
	Timezone  string
}

type opaqueSessionPayload struct {
	ID           string   `json:"id,omitempty"`
	RefreshToken string   `json:"rt"`
	UserID       string   `json:"uid"`
	CompanyID    string   `json:"cid,omitempty"`
	Roles        []string `json:"roles,omitempty"`
	Perms        []string `json:"perms,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	Locale       string   `json:"locale,omitempty"`
	Timezone     string   `json:"timezone,omitempty"`
	ExpiresAt    int64    `json:"exp,omitempty"`
}

var (
	authMode              = ""
	authAccessCookieName  = "access_token"
	authRefreshCookieName = "refresh_token"
	authSessionCookieName = "sid"
	authAlg               = "RS256"
	authIssuer            = ""
	authAudience          = ""
	authUserClaim         = "sub"
	authCompanyClaim      = "cid"
	authRolesClaim        = "roles"
	authPermsClaim        = "perms"
	authScopesClaim       = "scopes"
	authLocaleClaim       = "locale"
	authTimezoneClaim     = "timezone"

	authRSAPublicKey    *rsa.PublicKey
	authECDSAPublicKey  *ecdsa.PublicKey
	authHMACSecret      []byte
	authCfg             *config.Config
	authRefreshStore    port.RefreshTokenStore
	authSessionStore    port.StateStore
	verifiedUserChecker func(ctx context.Context, userID string) (bool, error)
	authSessionMu       sync.Mutex
	authSessionMemory   = map[string]opaqueSessionPayload{}
	authSessionByUser   = map[string]map[string]struct{}{} // userID → set of SIDs (memory fallback)
)

func SetAuthConfigFromConfig(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	authCfg = cfg
	if cfg.JWTAlg != "" {
		authAlg = cfg.JWTAlg
	}
	authIssuer = cfg.JWTIssuer
	authAudience = cfg.JWTAudience

	// claims mapping from CUE
	authUserClaim = "sub"
	authCompanyClaim = "cid"
	authRolesClaim = "roles"
	authPermsClaim = "perms"
	authScopesClaim = "scopes"

	switch authAlg {
	case "RS256":
		if cfg.JWTPublicKey == "" {
			return errors.New(http.StatusInternalServerError, "Auth config error", "JWT_PUBLIC_KEY is required for RS256")
		}
		pub, err := parseRSAPublicKey(cfg.JWTPublicKey)
		if err != nil {
			return errors.New(http.StatusInternalServerError, "Auth config error", "invalid RSA public key")
		}
		authRSAPublicKey = pub
	case "ES256":
		if cfg.JWTPublicKey == "" {
			return errors.New(http.StatusInternalServerError, "Auth config error", "JWT_PUBLIC_KEY is required for ES256")
		}
		pub, err := parseECPublicKey(cfg.JWTPublicKey)
		if err != nil {
			return errors.New(http.StatusInternalServerError, "Auth config error", "invalid EC public key")
		}
		authECDSAPublicKey = pub
	case "HS256":
		key := cfg.JWTPrivateKey
		if key == "" {
			key = cfg.JWTPublicKey
		}
		if key == "" {
			return errors.New(http.StatusInternalServerError, "Auth config error", "JWT_PRIVATE_KEY is required for HS256")
		}
		authHMACSecret = []byte(key)
	default:
		return errors.New(http.StatusInternalServerError, "Auth config error", "unsupported JWT algorithm")
	}
	return nil
}

func SetVerifiedUserChecker(checker func(ctx context.Context, userID string) (bool, error)) {
	verifiedUserChecker = checker
}

func SetAuthRefreshStore(store port.RefreshTokenStore) {
	authRefreshStore = store
}

func SetAuthSessionStore(store port.StateStore) {
	authSessionStore = store
}

func decodeJSONRequest(r *http.Request, out interface{}) error {
	if r.Body == nil {
		return io.EOF
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return io.EOF
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err == nil {
		normalized := make(map[string]interface{}, len(obj))
		for k, v := range obj {
			normalized[k] = v
		}
		for k, v := range obj {
			lower := strings.ToLower(k)
			if lower != k {
				if _, exists := normalized[lower]; !exists {
					normalized[lower] = v
				}
			}
		}
		body, err = json.Marshal(normalized)
		if err != nil {
			return err
		}
	}
	return json.Unmarshal(body, out)
}

// decodeMultipartOrJSONUpload accepts application/json (same as decodeJSONRequest) or multipart/form-data
// with a "file" part and optional text fields (ownerType, ownerId, folderId, name, mime, url, size).
// Populates any struct with matching JSON tags (e.g. port.UploadAttachmentRequest).
func decodeMultipartOrJSONUpload(r *http.Request, out interface{}) error {
	ct := r.Header.Get("Content-Type")
	mediatype, _, err := mime.ParseMediaType(ct)
	if err != nil || mediatype != "multipart/form-data" {
		return decodeJSONRequest(r, out)
	}

	const maxMemory = 64 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return fmt.Errorf("multipart parse: %w", err)
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}

	file, hdr, err := r.FormFile("file")
	if err != nil {
		return fmt.Errorf("multipart form field \"file\" is required")
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read upload: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("empty file upload")
	}

	payload := map[string]interface{}{
		"fileBase64": base64.StdEncoding.EncodeToString(data),
		"size":       len(data),
	}
	if fn := strings.TrimSpace(hdr.Filename); fn != "" {
		payload["name"] = filepath.Base(fn)
	}
	if v := strings.TrimSpace(hdr.Header.Get("Content-Type")); v != "" {
		payload["mime"] = v
	}

	for key, formKey := range map[string]string{
		"url":       "url",
		"name":      "name",
		"mime":      "mime",
		"ownerType": "ownerType",
		"ownerId":   "ownerId",
		"folderId":  "folderId",
		"companyId": "companyId",
		"userId":    "userId",
	} {
		if v := strings.TrimSpace(r.FormValue(formKey)); v != "" {
			payload[key] = v
		}
	}
	// Lowercase aliases from HTML forms
	if v := strings.TrimSpace(r.FormValue("ownerid")); v != "" {
		if _, ok := payload["ownerId"]; !ok {
			payload["ownerId"] = v
		}
	}
	if v := strings.TrimSpace(r.FormValue("folderid")); v != "" {
		if _, ok := payload["folderId"]; !ok {
			payload["folderId"] = v
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// TestHeadersMiddleware reads test-only request headers and injects them into the context.
// In production environments these headers are never sent, so this middleware is a no-op.
func TestHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-test-skip-auto-verify") == "true" {
			r = r.WithContext(reqctx.WithSkipAutoVerify(r.Context()))
		}
		next.ServeHTTP(w, r)
	})
}

func authContextFromClaims(claims map[string]any) authContext {
	return authContext{
		UserID:    getStringClaim(claims, authUserClaim),
		CompanyID: getStringClaim(claims, authCompanyClaim),
		Roles:     getStringSliceClaim(claims, authRolesClaim),
		Perms:     getStringSliceClaim(claims, authPermsClaim),
		Scopes:    getStringSliceClaim(claims, authScopesClaim),
		Locale:    getStringClaim(claims, authLocaleClaim),
		Timezone:  getStringClaim(claims, authTimezoneClaim),
	}
}

func authClaimsFromContext(ac authContext) map[string]any {
	claims := map[string]any{
		authUserClaim: ac.UserID,
	}
	if ac.CompanyID != "" {
		claims[authCompanyClaim] = ac.CompanyID
	}
	if len(ac.Roles) > 0 {
		claims[authRolesClaim] = ac.Roles
	}
	if len(ac.Perms) > 0 {
		claims[authPermsClaim] = ac.Perms
	}
	if len(ac.Scopes) > 0 {
		claims[authScopesClaim] = ac.Scopes
	}
	if ac.Locale != "" {
		claims[authLocaleClaim] = ac.Locale
	}
	if ac.Timezone != "" {
		claims[authTimezoneClaim] = ac.Timezone
	}
	return claims
}

func cookieAuthEnabled() bool {
	return authMode == "web_session_cookie" || authMode == "opaque_session_cookie"
}

func opaqueSessionEnabled() bool {
	return authMode == "opaque_session_cookie"
}

func authSessionKey(sessionID string) string {
	return "auth_session:" + strings.TrimSpace(sessionID)
}

func authSessionTTL(payload opaqueSessionPayload) time.Duration {
	if payload.ExpiresAt > 0 {
		if ttl := time.Until(time.Unix(payload.ExpiresAt, 0)); ttl > 0 {
			return ttl
		}
		return time.Second
	}
	if authCfg != nil {
		if d, err := time.ParseDuration(strings.TrimSpace(authCfg.JWTRefreshTTL)); err == nil && d > 0 {
			return d
		}
	}
	return 7 * 24 * time.Hour
}

func generateOpaqueSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(crand.Reader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func saveOpaqueSession(ctx context.Context, payload opaqueSessionPayload) error {
	if strings.TrimSpace(payload.ID) == "" {
		return fmt.Errorf("opaque session id is required")
	}
	if authSessionStore != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return authSessionStore.Set(ctx, authSessionKey(payload.ID), raw, authSessionTTL(payload))
	}
	authSessionMu.Lock()
	defer authSessionMu.Unlock()
	authSessionMemory[payload.ID] = payload
	return nil
}

func deleteOpaqueSession(ctx context.Context, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	if authSessionStore != nil {
		return authSessionStore.Delete(ctx, authSessionKey(sessionID))
	}
	authSessionMu.Lock()
	defer authSessionMu.Unlock()
	delete(authSessionMemory, sessionID)
	return nil
}

func findOpaqueSession(ctx context.Context, sessionID string) (*opaqueSessionPayload, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("missing session id")
	}
	if authSessionStore != nil {
		raw, err := authSessionStore.Get(ctx, authSessionKey(sessionID))
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			return nil, nil
		}
		var payload opaqueSessionPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return &payload, nil
	}
	authSessionMu.Lock()
	defer authSessionMu.Unlock()
	payload, ok := authSessionMemory[sessionID]
	if !ok {
		return nil, nil
	}
	return &payload, nil
}

// ── user→sessions index ─────────────────────────────────────────────────────
// Tracks which SIDs belong to each user so logout-all-sessions can revoke them.

func userSessionsIndexKey(userID string) string {
	return "user_sessions:" + strings.TrimSpace(userID)
}

func registerSessionForUser(ctx context.Context, userID, sessionID string) {
	if userID == "" || sessionID == "" {
		return
	}
	if authSessionStore != nil {
		raw, _ := authSessionStore.Get(ctx, userSessionsIndexKey(userID))
		var sids []string
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &sids)
		}
		for _, s := range sids {
			if s == sessionID {
				return
			}
		}
		sids = append(sids, sessionID)
		if data, err := json.Marshal(sids); err == nil {
			_ = authSessionStore.Set(ctx, userSessionsIndexKey(userID), data, 30*24*time.Hour)
		}
		return
	}
	authSessionMu.Lock()
	defer authSessionMu.Unlock()
	if authSessionByUser[userID] == nil {
		authSessionByUser[userID] = make(map[string]struct{})
	}
	authSessionByUser[userID][sessionID] = struct{}{}
}

func unregisterSessionForUser(ctx context.Context, userID, sessionID string) {
	if userID == "" || sessionID == "" {
		return
	}
	if authSessionStore != nil {
		raw, _ := authSessionStore.Get(ctx, userSessionsIndexKey(userID))
		if len(raw) == 0 {
			return
		}
		var sids []string
		if err := json.Unmarshal(raw, &sids); err != nil {
			return
		}
		filtered := sids[:0]
		for _, s := range sids {
			if s != sessionID {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			_ = authSessionStore.Delete(ctx, userSessionsIndexKey(userID))
			return
		}
		if data, err := json.Marshal(filtered); err == nil {
			_ = authSessionStore.Set(ctx, userSessionsIndexKey(userID), data, 30*24*time.Hour)
		}
		return
	}
	authSessionMu.Lock()
	defer authSessionMu.Unlock()
	delete(authSessionByUser[userID], sessionID)
}

// RevokeAllUserSessions invalidates every active session for the given user.
// Call from the logout-all-sessions endpoint after verifying the user's identity.
func RevokeAllUserSessions(ctx context.Context, w http.ResponseWriter, r *http.Request, userID string) {
	if userID == "" {
		return
	}
	if authSessionStore != nil {
		raw, _ := authSessionStore.Get(ctx, userSessionsIndexKey(userID))
		if len(raw) > 0 {
			var sids []string
			if err := json.Unmarshal(raw, &sids); err == nil {
				for _, sid := range sids {
					_ = authSessionStore.Delete(ctx, authSessionKey(sid))
				}
			}
		}
		_ = authSessionStore.Delete(ctx, userSessionsIndexKey(userID))
	} else {
		authSessionMu.Lock()
		for sid := range authSessionByUser[userID] {
			delete(authSessionMemory, sid)
		}
		delete(authSessionByUser, userID)
		authSessionMu.Unlock()
	}
	clearAuthCookies(w, r)
}

func buildOpaqueSessionPayload(accessToken, refreshToken string) (*opaqueSessionPayload, error) {
	if strings.TrimSpace(accessToken) == "" || strings.TrimSpace(refreshToken) == "" {
		return nil, fmt.Errorf("missing access or refresh token for opaque session")
	}
	claims, err := parseAndVerifyJWT(accessToken)
	if err != nil {
		return nil, err
	}
	ac := authContextFromClaims(claims)
	payload := &opaqueSessionPayload{
		ID:           "",
		RefreshToken: refreshToken,
		UserID:       ac.UserID,
		CompanyID:    ac.CompanyID,
		Roles:        ac.Roles,
		Perms:        ac.Perms,
		Scopes:       ac.Scopes,
		Locale:       ac.Locale,
		Timezone:     ac.Timezone,
	}
	if authCfg != nil {
		if d, err := time.ParseDuration(strings.TrimSpace(authCfg.JWTRefreshTTL)); err == nil && d > 0 {
			payload.ExpiresAt = time.Now().Add(d).Unix()
		}
	}
	return payload, nil
}

func readOpaqueSessionPayload(r *http.Request) (*opaqueSessionPayload, error) {
	if !opaqueSessionEnabled() {
		return nil, fmt.Errorf("opaque session mode is disabled")
	}
	cookie, err := r.Cookie(authSessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil, fmt.Errorf("missing session cookie")
	}
	payload, err := findOpaqueSession(r.Context(), cookie.Value)
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, fmt.Errorf("opaque session not found")
	}
	if strings.TrimSpace(payload.ID) == "" {
		payload.ID = strings.TrimSpace(cookie.Value)
	}
	if payload == nil || strings.TrimSpace(payload.RefreshToken) == "" || strings.TrimSpace(payload.UserID) == "" {
		return nil, fmt.Errorf("invalid opaque session payload")
	}
	if payload.ExpiresAt > 0 && time.Now().Unix() > payload.ExpiresAt {
		_ = deleteOpaqueSession(r.Context(), payload.ID)
		return nil, fmt.Errorf("opaque session expired")
	}
	return payload, nil
}

func authRefreshTokenFromRequest(r *http.Request) (string, error) {
	if opaqueSessionEnabled() {
		payload, err := readOpaqueSessionPayload(r)
		if err != nil {
			return "", err
		}
		return payload.RefreshToken, nil
	}
	c, err := r.Cookie(authRefreshCookieName)
	if err != nil || strings.TrimSpace(c.Value) == "" {
		return "", fmt.Errorf("missing refresh token cookie")
	}
	return c.Value, nil
}

func authClaimsFromCookieSession(r *http.Request) (map[string]any, error) {
	if opaqueSessionEnabled() {
		payload, err := readOpaqueSessionPayload(r)
		if err != nil {
			return nil, err
		}
		if authRefreshStore == nil {
			return nil, fmt.Errorf("opaque session store is not configured")
		}
		rec, err := authRefreshStore.Find(r.Context(), payload.RefreshToken)
		if err != nil {
			return nil, err
		}
		if rec == nil || rec.Revoked || (!rec.ExpiresAt.IsZero() && time.Now().After(rec.ExpiresAt)) {
			return nil, fmt.Errorf("opaque session is invalid")
		}
		if rec.UserID != payload.UserID {
			return nil, fmt.Errorf("opaque session user mismatch")
		}
		return authClaimsFromContext(authContext{
			UserID:    payload.UserID,
			CompanyID: payload.CompanyID,
			Roles:     payload.Roles,
			Perms:     payload.Perms,
			Scopes:    payload.Scopes,
			Locale:    payload.Locale,
			Timezone:  payload.Timezone,
		}), nil
	}
	if c, cookieErr := r.Cookie(authAccessCookieName); cookieErr == nil && strings.TrimSpace(c.Value) != "" {
		return parseAndVerifyJWT(c.Value)
	}
	return nil, fmt.Errorf("missing access token cookie")
}

func authCookieSecure(r *http.Request) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	if r != nil {
		if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); strings.EqualFold(proto, "https") {
			return true
		}
	}
	return false
}

func authCookieMaxAge(ttl string, fallback time.Duration) int {
	if d, err := time.ParseDuration(strings.TrimSpace(ttl)); err == nil && d > 0 {
		return int(d.Seconds())
	}
	return int(fallback.Seconds())
}

func writeAuthCookies(w http.ResponseWriter, r *http.Request, accessToken, refreshToken string) {
	accessTTL := "15m"
	refreshTTL := "168h"
	if authCfg != nil {
		if strings.TrimSpace(authCfg.JWTAccessTTL) != "" {
			accessTTL = authCfg.JWTAccessTTL
		}
		if strings.TrimSpace(authCfg.JWTRefreshTTL) != "" {
			refreshTTL = authCfg.JWTRefreshTTL
		}
	}
	if opaqueSessionEnabled() {
		payload, err := buildOpaqueSessionPayload(accessToken, refreshToken)
		if err != nil {
			return
		}
		// SID rotation: always invalidate the old session and issue a fresh SID.
		// Prevents session fixation and ensures refresh-token rotation is reflected
		// in the session store immediately.
		if existing, cookieErr := r.Cookie(authSessionCookieName); cookieErr == nil && strings.TrimSpace(existing.Value) != "" {
			oldSID := strings.TrimSpace(existing.Value)
			if oldPayload, _ := findOpaqueSession(r.Context(), oldSID); oldPayload != nil {
				unregisterSessionForUser(r.Context(), oldPayload.UserID, oldSID)
			}
			_ = deleteOpaqueSession(r.Context(), oldSID)
		}
		payload.ID, err = generateOpaqueSessionID()
		if err != nil {
			return
		}
		if err := saveOpaqueSession(r.Context(), *payload); err != nil {
			return
		}
		registerSessionForUser(r.Context(), payload.UserID, payload.ID)
		http.SetCookie(w, &http.Cookie{
			Name:     authSessionCookieName,
			Value:    payload.ID,
			Path:     "/",
			MaxAge:   authCookieMaxAge(refreshTTL, 7*time.Hour*24),
			HttpOnly: true,
			Secure:   authCookieSecure(r),
			SameSite: http.SameSiteLaxMode,
		})
		for _, legacyName := range []string{authAccessCookieName, authRefreshCookieName} {
			http.SetCookie(w, &http.Cookie{
				Name:     legacyName,
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   authCookieSecure(r),
				SameSite: http.SameSiteLaxMode,
			})
		}
		return
	}
	if strings.TrimSpace(accessToken) != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     authAccessCookieName,
			Value:    accessToken,
			Path:     "/",
			MaxAge:   authCookieMaxAge(accessTTL, 15*time.Minute),
			HttpOnly: true,
			Secure:   authCookieSecure(r),
			SameSite: http.SameSiteLaxMode,
		})
	}
	if strings.TrimSpace(refreshToken) != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     authRefreshCookieName,
			Value:    refreshToken,
			Path:     "/",
			MaxAge:   authCookieMaxAge(refreshTTL, 7*24*time.Hour),
			HttpOnly: true,
			Secure:   authCookieSecure(r),
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func clearAuthCookies(w http.ResponseWriter, r *http.Request) {
	if opaqueSessionEnabled() && r != nil {
		if cookie, err := r.Cookie(authSessionCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
			sid := strings.TrimSpace(cookie.Value)
			if payload, _ := findOpaqueSession(r.Context(), sid); payload != nil {
				unregisterSessionForUser(r.Context(), payload.UserID, sid)
			}
			_ = deleteOpaqueSession(r.Context(), sid)
		}
	}
	for _, name := range []string{authAccessCookieName, authRefreshCookieName, authSessionCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   authCookieSecure(r),
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func tryRefreshCookieSession(w http.ResponseWriter, r *http.Request) (map[string]any, error) {
	if authCfg == nil || authRefreshStore == nil {
		return nil, fmt.Errorf("cookie auth refresh not configured")
	}
	accessCookie, err := r.Cookie(authAccessCookieName)
	if err != nil || strings.TrimSpace(accessCookie.Value) == "" {
		return nil, fmt.Errorf("missing access token cookie")
	}
	refreshCookie, err := r.Cookie(authRefreshCookieName)
	if err != nil || strings.TrimSpace(refreshCookie.Value) == "" {
		return nil, fmt.Errorf("missing refresh token cookie")
	}
	expiredClaims, err := parseAndVerifyJWTAllowExpired(accessCookie.Value)
	if err != nil {
		return nil, err
	}
	rec, err := authRefreshStore.Find(r.Context(), refreshCookie.Value)
	if err != nil {
		return nil, err
	}
	if rec == nil || rec.Revoked || (!rec.ExpiresAt.IsZero() && time.Now().After(rec.ExpiresAt)) {
		return nil, fmt.Errorf("refresh token invalid")
	}
	ac := authContextFromClaims(expiredClaims)
	if ac.UserID == "" || rec.UserID != ac.UserID {
		return nil, fmt.Errorf("refresh token user mismatch")
	}
	extraClaims := map[string]string{}
	if ac.Locale != "" {
		extraClaims["locale"] = ac.Locale
	}
	if ac.Timezone != "" {
		extraClaims["timezone"] = ac.Timezone
	}
	accessToken, err := auth.IssueAccessToken(authCfg, ac.UserID, ac.CompanyID, ac.Roles, ac.Perms, extraClaims)
	if err != nil {
		return nil, err
	}
	newRefreshToken := refreshCookie.Value
	if authCfg.JWTRotation {
		newRefreshToken, err = auth.IssueRefreshToken(authCfg, ac.UserID)
		if err != nil {
			return nil, err
		}
		exp := time.Now().Add(24 * time.Hour)
		if d, derr := time.ParseDuration(authCfg.JWTRefreshTTL); derr == nil && d > 0 {
			exp = time.Now().Add(d)
		}
		if err := authRefreshStore.Rotate(r.Context(), refreshCookie.Value, newRefreshToken, ac.UserID, exp); err != nil {
			return nil, err
		}
	}
	writeAuthCookies(w, r, accessToken, newRefreshToken)
	return parseAndVerifyJWT(accessToken)
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var claims map[string]any
		var err error
		if cookieAuthEnabled() {
			if opaqueSessionEnabled() {
				claims, err = authClaimsFromCookieSession(r)
			} else {
				if c, cookieErr := r.Cookie(authAccessCookieName); cookieErr == nil && strings.TrimSpace(c.Value) != "" {
					claims, err = parseAndVerifyJWT(c.Value)
				}
				if err != nil || claims == nil {
					claims, err = tryRefreshCookieSession(w, r)
				}
			}
			if err != nil || claims == nil {
			}
			if err != nil || claims == nil {
				clearAuthCookies(w, r)
				errors.WriteError(w, r, errors.New(http.StatusUnauthorized, "Unauthorized", "Valid session cookie required"))
				return
			}
		} else {
			token := r.Header.Get("Authorization")
			if token == "" {
				token = r.URL.Query().Get("token")
			}
			if token == "" {
				errors.WriteError(w, r, errors.New(http.StatusUnauthorized, "Unauthorized", "JWT token required"))
				return
			}
			if strings.HasPrefix(strings.ToLower(token), "bearer ") {
				token = strings.TrimSpace(token[7:])
			}
			if token == "" {
				errors.WriteError(w, r, errors.New(http.StatusUnauthorized, "Unauthorized", "JWT token required"))
				return
			}
			claims, err = parseAndVerifyJWT(token)
			if err != nil {
				errors.WriteError(w, r, errors.New(http.StatusUnauthorized, "Unauthorized", "Invalid JWT"))
				return
			}
		}

		ac := authContextFromClaims(claims)
		if verifiedUserChecker != nil {
			ok, err := verifiedUserChecker(r.Context(), ac.UserID)
			if err != nil {
				errors.WriteError(w, r, errors.New(http.StatusInternalServerError, "Internal Server Error", "verification check failed"))
				return
			}
			if !ok {
				errors.WriteError(w, r, errors.New(http.StatusForbidden, "EMAIL_NOT_VERIFIED", "email is not verified"))
				return
			}
		}
		localeToSet := ac.Locale
		if localeToSet == "" {
			if al := r.Header.Get("Accept-Language"); al != "" {
				// Take the first language tag before comma or semicolon
				localeToSet = strings.SplitN(strings.SplitN(al, ",", 2)[0], ";", 2)[0]
				localeToSet = strings.TrimSpace(localeToSet)
			}
		}
		rCtx := r.Context()
		if localeToSet != "" {
			rCtx = reqctx.WithLocale(rCtx, localeToSet)
		}
		if ac.Timezone != "" {
			rCtx = reqctx.WithTimezone(rCtx, ac.Timezone)
		}
		ctx := context.WithValue(rCtx, authContextKey{}, ac)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func CurrentUserID(r *http.Request) string {
	if ac, ok := r.Context().Value(authContextKey{}).(authContext); ok {
		return ac.UserID
	}
	return ""
}

func CurrentCompanyID(r *http.Request) string {
	if ac, ok := r.Context().Value(authContextKey{}).(authContext); ok {
		return ac.CompanyID
	}
	return ""
}

func CurrentRoles(r *http.Request) []string {
	if ac, ok := r.Context().Value(authContextKey{}).(authContext); ok {
		return ac.Roles
	}
	return nil
}

func CurrentPermissions(r *http.Request) []string {
	if ac, ok := r.Context().Value(authContextKey{}).(authContext); ok {
		return ac.Perms
	}
	return nil
}

func CurrentScopes(r *http.Request) []string {
	if ac, ok := r.Context().Value(authContextKey{}).(authContext); ok {
		return ac.Scopes
	}
	return nil
}

func CurrentRole(r *http.Request) string {
	roles := CurrentRoles(r)
	if len(roles) > 0 {
		return roles[0]
	}
	return ""
}

func CurrentLocale(r *http.Request) string {
	if ac, ok := r.Context().Value(authContextKey{}).(authContext); ok && ac.Locale != "" {
		return ac.Locale
	}
	return reqctx.Locale(r.Context())
}

func CurrentTimezone(r *http.Request) string {
	if ac, ok := r.Context().Value(authContextKey{}).(authContext); ok && ac.Timezone != "" {
		return ac.Timezone
	}
	return reqctx.Timezone(r.Context())
}

func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := CurrentRole(r)
			perms := CurrentPermissions(r)
			if len(perms) > 0 {
				for _, p := range perms {
					if p == perm {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			if role == "" || !rbac.CheckPermission(role, perm) {
				errors.WriteError(w, r, errors.New(http.StatusForbidden, "Forbidden", "Insufficient permissions"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireRoles(roles []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := CurrentRoles(r)
			if len(current) == 0 {
				role := CurrentRole(r)
				if role != "" {
					current = []string{role}
				}
			}
			for _, allowed := range roles {
				for _, role := range current {
					if role == allowed {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			errors.WriteError(w, r, errors.New(http.StatusForbidden, "Forbidden", "Insufficient role"))
		})
	}
}

func RequireScopeMiddleware(scopes []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := CurrentScopes(r)
			for _, required := range scopes {
				for _, s := range current {
					if s == required {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			errors.WriteError(w, r, errors.New(http.StatusForbidden, "Forbidden", "Insufficient scope: "+strings.Join(scopes, " or ")+" required"))
		})
	}
}

func parseAndVerifyJWT(token string) (map[string]any, error) {
	return parseAndVerifyJWTWithOptions(token, false)
}

func parseAndVerifyJWTAllowExpired(token string) (map[string]any, error) {
	return parseAndVerifyJWTWithOptions(token, true)
}

func parseAndVerifyJWTWithOptions(token string, allowExpired bool) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}

	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, err
	}
	if alg, _ := header["alg"].(string); alg != authAlg {
		return nil, fmt.Errorf("invalid alg")
	}

	signed := []byte(parts[0] + "." + parts[1])
	hash := sha256.Sum256(signed)

	switch authAlg {
	case "RS256":
		if authRSAPublicKey == nil {
			return nil, fmt.Errorf("rsa key not configured")
		}
		if err := rsa.VerifyPKCS1v15(authRSAPublicKey, crypto.SHA256, hash[:], signature); err != nil {
			return nil, err
		}
	case "ES256":
		if authECDSAPublicKey == nil {
			return nil, fmt.Errorf("ecdsa key not configured")
		}
		if len(signature) != 64 {
			return nil, fmt.Errorf("invalid ecdsa signature")
		}
		r := new(big.Int).SetBytes(signature[:32])
		s := new(big.Int).SetBytes(signature[32:])
		if !ecdsa.Verify(authECDSAPublicKey, hash[:], r, s) {
			return nil, fmt.Errorf("invalid ecdsa signature")
		}
	case "HS256":
		if len(authHMACSecret) == 0 {
			return nil, fmt.Errorf("hmac key not configured")
		}
		mac := hmac.New(sha256.New, authHMACSecret)
		mac.Write(signed)
		expected := mac.Sum(nil)
		if subtle.ConstantTimeCompare(expected, signature) != 1 {
			return nil, fmt.Errorf("invalid signature")
		}
	default:
		return nil, fmt.Errorf("unsupported alg")
	}

	var claims map[string]any
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, err
	}

	if authIssuer != "" {
		if iss, _ := claims["iss"].(string); iss != authIssuer {
			return nil, fmt.Errorf("invalid issuer")
		}
	}
	if authAudience != "" {
		if !hasAudience(claims, authAudience) {
			return nil, fmt.Errorf("invalid audience")
		}
	}
	if !allowExpired && !validateTimes(claims) {
		return nil, fmt.Errorf("token expired or not valid yet")
	}
	return claims, nil
}

func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not RSA public key")
	}
	return rsaPub, nil
}

func parseECPublicKey(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not EC public key")
	}
	return ecPub, nil
}

func hasAudience(claims map[string]any, aud string) bool {
	if aud == "" {
		return true
	}
	switch v := claims["aud"].(type) {
	case string:
		return v == aud
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == aud {
				return true
			}
		}
	}
	return false
}

func validateTimes(claims map[string]any) bool {
	now := time.Now().Unix()
	if exp, ok := getNumericClaim(claims, "exp"); ok && now > exp {
		return false
	}
	if nbf, ok := getNumericClaim(claims, "nbf"); ok && now < nbf {
		return false
	}
	return true
}

func getNumericClaim(claims map[string]any, key string) (int64, bool) {
	v, ok := claims[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case json.Number:
		i, err := t.Int64()
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

func getStringClaim(claims map[string]any, key string) string {
	if key == "" {
		return ""
	}
	v, ok := claims[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return ""
	}
}

func getStringSliceClaim(claims map[string]any, key string) []string {
	if key == "" {
		return nil
	}
	v, ok := claims[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	default:
		return nil
	}
}

func CacheMiddleware(ttl string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ttl != "" {
				w.Header().Set("Cache-Control", "public, max-age="+ttl)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// realClientIP extracts the real client IP honoring reverse-proxy headers.
func realClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

type rateState struct {
	windowStart time.Time
	count       int
}

type windowState struct {
	mu      sync.Mutex
	count   int
	resetAt time.Time
}

var (
	rateMu      sync.Mutex
	rateByIP    = map[string]*rateState{}
	windowByKey sync.Map
	redisClient *redis.Client
)

func SetRedisClient(c *redis.Client) {
	redisClient = c
	// authMode is "opaque_session_cookie": wire the session + refresh stores from the
	// same Redis client, otherwise every cookie-authenticated request 401s
	// ("Valid session cookie required") because the opaque session can never resolve.
	if c != nil {
		if authSessionStore == nil {
			authSessionStore = statestoreredis.New(c)
		}
		if authRefreshStore == nil {
			authRefreshStore = authredis.NewStore(c)
		}
	}
}

// RateLimitMiddleware enforces per-IP rate limiting at two levels:
//   - rps/burst : per-second token-bucket (Redis when available, in-memory fallback)
//   - windowSecs/windowLimit : fixed-window quota (e.g. 10 builds/hour)
//     windowLimit=0 disables the window check.
func RateLimitMiddleware(rps, burst, windowSecs, windowLimit int) func(http.Handler) http.Handler {
	max := rps
	if burst > max {
		max = burst
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realClientIP(r)
			now := time.Now()

			// ---- per-second RPS check ----
			if max > 0 {
				if redisClient != nil {
					key := fmt.Sprintf("rate:rps:%d:%s", max, ip)
					ctx := context.Background()
					count, err := redisClient.Incr(ctx, key).Result()
					if err == nil {
						_ = redisClient.Expire(ctx, key, time.Second).Err()
						if int(count) > max {
							errors.WriteError(w, r, errors.New(http.StatusTooManyRequests, "Too Many Requests", "Rate limit exceeded"))
							return
						}
					}
				} else {
					rateMu.Lock()
					state, ok := rateByIP[ip]
					if !ok {
						state = &rateState{windowStart: now}
						rateByIP[ip] = state
					}
					if now.Sub(state.windowStart) >= time.Second {
						state.windowStart = now
						state.count = 0
					}
					state.count++
					over := state.count > max
					rateMu.Unlock()
					if over {
						errors.WriteError(w, r, errors.New(http.StatusTooManyRequests, "Too Many Requests", "Rate limit exceeded"))
						return
					}
				}
			}

			// ---- fixed-window quota check (e.g. hourly build limit) ----
			if windowSecs > 0 && windowLimit > 0 {
				windowDur := time.Duration(windowSecs) * time.Second
				wkey := fmt.Sprintf("rate:win:%d:%s", windowSecs, ip)
				if redisClient != nil {
					ctx := context.Background()
					count, err := redisClient.Incr(ctx, wkey).Result()
					if err == nil {
						if count == 1 {
							_ = redisClient.Expire(ctx, wkey, windowDur).Err()
						}
						if int(count) > windowLimit {
							errors.WriteError(w, r, errors.New(http.StatusTooManyRequests, "Too Many Requests", "Rate limit exceeded"))
							return
						}
					}
				} else {
					v, _ := windowByKey.LoadOrStore(wkey, &windowState{resetAt: now.Add(windowDur)})
					ws := v.(*windowState)
					ws.mu.Lock()
					if now.After(ws.resetAt) {
						ws.count = 0
						ws.resetAt = now.Add(windowDur)
					}
					ws.count++
					over := ws.count > windowLimit
					ws.mu.Unlock()
					if over {
						errors.WriteError(w, r, errors.New(http.StatusTooManyRequests, "Too Many Requests", "Rate limit exceeded"))
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TimeoutMiddleware wraps handler with a timeout context.
// Uses Go's http.TimeoutHandler for proper timeout handling.
func TimeoutMiddleware(timeout string) func(http.Handler) http.Handler {
	d, err := time.ParseDuration(timeout)
	if err != nil || d <= 0 {
		d = 30 * time.Second // default timeout
	}
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, `{"type":"about:blank","title":"Gateway Timeout","status":504,"detail":"Request timed out"}`)
	}
}

func IdempotencyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				errors.WriteError(w, r, errors.New(http.StatusBadRequest, "Bad Request", "Idempotency-Key required"))
				return
			}
			if redisClient != nil {
				ctx := context.Background()
				ok, err := redisClient.SetNX(ctx, "idem:"+key, "1", time.Hour).Result()
				if err == nil && !ok {
					errors.WriteError(w, r, errors.New(http.StatusConflict, "Conflict", "Duplicate request"))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func MaxBodySizeMiddleware(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				errors.WriteError(w, r, errors.New(http.StatusRequestEntityTooLarge, "Payload Too Large", fmt.Sprintf("Request body too large (max %d bytes)", limit)))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

func CircuitBreakerMiddleware(threshold int, timeout string, halfOpenMax int) func(http.Handler) http.Handler {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		d = 30 * time.Second
	}
	breaker := circuitbreaker.NewBreaker(threshold, d, halfOpenMax)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !breaker.Allow() {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(d.Seconds())))
				errors.WriteError(w, r, errors.New(http.StatusServiceUnavailable, "Service Unavailable", "Circuit breaker is open"))
				return
			}

			sw := &statusResponseWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)

			if sw.status >= 500 {
				breaker.RecordFailure()
			} else {
				breaker.RecordSuccess()
			}
		})
	}
}

// ConcurrencyMiddleware limits simultaneous in-flight requests via a buffered-channel semaphore.
// When the limit is reached new requests get 503 Service Unavailable immediately (no queuing).
// This is true backpressure: fast fail instead of queuing → prevents latency cascade.
func ConcurrencyMiddleware(n int) func(http.Handler) http.Handler {
	sem := make(chan struct{}, n)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				next.ServeHTTP(w, r)
			default:
				concurrencyShed.WithLabelValues(r.URL.Path).Inc()
				errors.WriteError(w, r, errors.New(http.StatusServiceUnavailable,
					"Service Unavailable", "Server is at capacity, please retry later"))
			}
		})
	}
}

// SingleflightMiddleware collapses identical in-flight GET requests into one handler call.
// All waiting callers get the same response — reduces load on DB/cache by orders of magnitude.
// Each call to SingleflightMiddleware() creates an independent group (one per route).
func SingleflightMiddleware() func(http.Handler) http.Handler {
	var group singleflight.Group
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			type captured struct {
				status  int
				body    []byte
				headers http.Header
			}
			v, _, _ := group.Do(r.URL.RequestURI(), func() (any, error) {
				rec := httptest.NewRecorder()
				next.ServeHTTP(rec, r)
				res := rec.Result()
				body, _ := io.ReadAll(res.Body)
				return &captured{status: res.StatusCode, body: body, headers: res.Header}, nil
			})
			res := v.(*captured)
			for k, vs := range res.headers {
				for _, hv := range vs {
					w.Header().Add(k, hv)
				}
			}
			w.WriteHeader(res.status)
			_, _ = w.Write(res.body)
		})
	}
}

// RetryMiddleware retries safe methods (GET/HEAD) on transient errors with exponential backoff + jitter.
// Uses httptest.ResponseRecorder to buffer the response so it can be replayed on retry.
func RetryMiddleware(maxAttempts, baseDelayMS int, retryStatuses []int) func(http.Handler) http.Handler {
	isRetryable := func(code int) bool {
		for _, s := range retryStatuses {
			if s == code {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			delay := time.Duration(baseDelayMS) * time.Millisecond
			var lastStatus int
			var lastBody []byte
			var lastHeaders http.Header
			for attempt := 0; attempt < maxAttempts; attempt++ {
				rec := httptest.NewRecorder()
				next.ServeHTTP(rec, r)
				res := rec.Result()
				lastStatus = res.StatusCode
				lastBody, _ = io.ReadAll(res.Body)
				lastHeaders = res.Header
				if !isRetryable(lastStatus) || attempt == maxAttempts-1 {
					break
				}
				jitter := time.Duration(rand.Intn(50)) * time.Millisecond
				time.Sleep(delay + jitter)
				delay *= 2
			}
			for k, vs := range lastHeaders {
				for _, hv := range vs {
					w.Header().Add(k, hv)
				}
			}
			w.WriteHeader(lastStatus)
			_, _ = w.Write(lastBody)
		})
	}
}
