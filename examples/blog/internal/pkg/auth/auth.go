package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/strogmv/ang/internal/config"
)

// IssueAccessToken builds and signs an access JWT.
func IssueAccessToken(cfg *config.Config, userID, companyID string, roles, perms []string) (string, error) {
	claims := buildClaims(cfg, userID, companyID, roles, perms, "access")
	return signToken(cfg, claims, cfg.JWTAccessTTL)
}

// IssueRefreshToken builds and signs a refresh JWT.
func IssueRefreshToken(cfg *config.Config, userID string) (string, error) {
	claims := buildClaims(cfg, userID, "", nil, nil, "refresh")
	claims["jti"] = randomTokenID()
	return signToken(cfg, claims, cfg.JWTRefreshTTL)
}

func buildClaims(cfg *config.Config, userID, companyID string, roles, perms []string, tokenType string) map[string]any {
	now := time.Now().Unix()
	claims := map[string]any{
		"iat": now,
		"nbf": now,
		"typ": tokenType,
	}
	if cfg.JWTIssuer != "" {
		claims["iss"] = cfg.JWTIssuer
	}
	if cfg.JWTAudience != "" {
		claims["aud"] = cfg.JWTAudience
	}
	if userID != "" {
		claims[""] = userID
	}
	if companyID != "" {
		claims[""] = companyID
	}
	if len(roles) > 0 {
		claims[""] = roles
	}
	if len(perms) > 0 {
		claims[""] = perms
	}
	return claims
}

func signToken(cfg *config.Config, claims map[string]any, ttl string) (string, error) {
	dur, err := time.ParseDuration(ttl)
	if err != nil || dur <= 0 {
		dur = 15 * time.Minute
	}
	claims["exp"] = time.Now().Add(dur).Unix()

	header := map[string]any{
		"alg": cfg.JWTAlg,
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(claims)
	enc := base64.RawURLEncoding
	unsigned := enc.EncodeToString(headerJSON) + "." + enc.EncodeToString(payloadJSON)
	hash := sha256.Sum256([]byte(unsigned))

	switch cfg.JWTAlg {
	case "RS256":
		priv, err := parseRSAPrivateKey(cfg.JWTPrivateKey)
		if err != nil {
			return "", err
		}
		sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hash[:])
		if err != nil {
			return "", err
		}
		return unsigned + "." + enc.EncodeToString(sig), nil
	case "ES256":
		priv, err := parseECPrivateKey(cfg.JWTPrivateKey)
		if err != nil {
			return "", err
		}
		r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
		if err != nil {
			return "", err
		}
		sig := make([]byte, 64)
		r.FillBytes(sig[:32])
		s.FillBytes(sig[32:])
		return unsigned + "." + enc.EncodeToString(sig), nil
	case "HS256":
		key := []byte(cfg.JWTPrivateKey)
		if len(key) == 0 {
			key = []byte(cfg.JWTPublicKey)
		}
		if len(key) == 0 {
			return "", fmt.Errorf("JWT_PRIVATE_KEY is required for HS256")
		}
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(unsigned))
		sig := mac.Sum(nil)
		return unsigned + "." + enc.EncodeToString(sig), nil
	default:
		return "", fmt.Errorf("unsupported JWT algorithm")
	}
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not RSA private key")
	}
	return rsaKey, nil
}

func parseECPrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not EC private key")
	}
	return ecKey, nil
}

func randomTokenID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
