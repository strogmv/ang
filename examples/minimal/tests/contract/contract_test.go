//go:build contract

package tests

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func contractBaseURL() string {
	if v := os.Getenv("CONTRACT_BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

func contractWSURL() string {
	if v := os.Getenv("CONTRACT_WS_URL"); v != "" {
		return v
	}
	return "ws://localhost:8080"
}

func contractToken() string {
	return os.Getenv("CONTRACT_TOKEN")
}

func contractRefreshToken() string {
	return os.Getenv("CONTRACT_REFRESH_TOKEN")
}

func contractEmail() string {
	return os.Getenv("CONTRACT_EMAIL")
}

func contractPassword() string {
	return os.Getenv("CONTRACT_PASSWORD")
}

func fillPathParams(path string) string {
	re := regexp.MustCompile(`\{[a-zA-Z0-9]+\}`)
	return re.ReplaceAllString(path, "test")
}

func fillPathParamsRequired(t *testing.T, path string) string {
	re := regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)
	missing := false
	out := re.ReplaceAllStringFunc(path, func(match string) string {
		name := strings.Trim(match, "{}")
		envKey := "CONTRACT_PARAM_" + strings.ToUpper(name)
		if v := os.Getenv(envKey); v != "" {
			return v
		}
		envKey = "CONTRACT_" + strings.ToUpper(name)
		if v := os.Getenv(envKey); v != "" {
			return v
		}
		missing = true
		return "test"
	})
	if missing {
		t.Skip("path params not provided via CONTRACT_PARAM_*")
	}
	return out
}

type authState struct {
	accessToken  string
	refreshToken string
	email        string
	password     string
	ready        bool
}

var authOnce sync.Once
var authCtx authState

func ensureAuth(t *testing.T) authState {
	authOnce.Do(func() {
		if token := contractToken(); token != "" {
			authCtx.accessToken = token
			authCtx.refreshToken = contractRefreshToken()
			authCtx.email = contractEmail()
			authCtx.password = contractPassword()
			authCtx.ready = true
			return
		}
	})
	if !authCtx.ready {
		t.Skip("auth bootstrap not available")
	}
	return authCtx
}

func TestContractHTTPUnauthorized(t *testing.T) {
	baseURL := contractBaseURL()
	client := &http.Client{Timeout: 10 * time.Second}
}

func TestContractHTTPValidation(t *testing.T) {
	baseURL := contractBaseURL()
	client := &http.Client{Timeout: 10 * time.Second}
	token := contractToken()
}

func TestContractHTTPPositive(t *testing.T) {
	baseURL := contractBaseURL()
	client := &http.Client{Timeout: 10 * time.Second}
	token := contractToken()
}

func TestContractWebSocket(t *testing.T) {
	baseURL := contractWSURL()
	token := contractToken()
}
