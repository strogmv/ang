package http

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
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
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"github.com/strogmv/ang/internal/config"
	"github.com/strogmv/ang/internal/pkg/circuitbreaker"
	"github.com/strogmv/ang/internal/pkg/errors"
	"github.com/strogmv/ang/internal/pkg/rbac"
)

var validate = validator.New()

type authContextKey struct{}

type authContext struct {
	UserID    string
	CompanyID string
	Roles     []string
	Perms     []string
}

var (
	authAlg          = "RS256"
	authIssuer       = ""
	authAudience     = ""
	authUserClaim    = "sub"
	authCompanyClaim = "cid"
	authRolesClaim   = "roles"
	authPermsClaim   = "perms"

	authRSAPublicKey   *rsa.PublicKey
	authECDSAPublicKey *ecdsa.PublicKey
	authHMACSecret     []byte
)

func SetAuthConfigFromConfig(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
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

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		claims, err := parseAndVerifyJWT(token)
		if err != nil {
			errors.WriteError(w, r, errors.New(http.StatusUnauthorized, "Unauthorized", "Invalid JWT"))
			return
		}

		ac := authContext{
			UserID:    getStringClaim(claims, authUserClaim),
			CompanyID: getStringClaim(claims, authCompanyClaim),
			Roles:     getStringSliceClaim(claims, authRolesClaim),
			Perms:     getStringSliceClaim(claims, authPermsClaim),
		}
		ctx := context.WithValue(r.Context(), authContextKey{}, ac)
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

func CurrentRole(r *http.Request) string {
	roles := CurrentRoles(r)
	if len(roles) > 0 {
		return roles[0]
	}
	return ""
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

func parseAndVerifyJWT(token string) (map[string]any, error) {
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
	if !validateTimes(claims) {
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

type rateState struct {
	windowStart time.Time
	count       int
}

var (
	rateMu      sync.Mutex
	rateByIP    = map[string]*rateState{}
	redisClient *redis.Client
)

func SetRedisClient(c *redis.Client) {
	redisClient = c
}

func RateLimitMiddleware(rps, burst int) func(http.Handler) http.Handler {
	max := rps
	if burst > max {
		max = burst
	}
	if max <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			now := time.Now()
			if redisClient != nil {
				key := "rate:" + ip
				ctx := context.Background()
				count, err := redisClient.Incr(ctx, key).Result()
				if err == nil {
					_ = redisClient.Expire(ctx, key, time.Second).Err()
					if int(count) > max {
						errors.WriteError(w, r, errors.New(http.StatusTooManyRequests, "Too Many Requests", "Rate limit exceeded"))
						return
					}
					next.ServeHTTP(w, r)
					return
				}
			}

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
