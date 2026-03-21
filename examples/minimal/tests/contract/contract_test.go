//go:build contract

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sync"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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
	t.Run("SendNoticeEmail_validation", func(t *testing.T) {
		url := baseURL + fillPathParams("/notifications/email") + "?email=user@example.com&title=test&body=test"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(body))
		}
	})
	t.Run("SendInvitationEmail_validation", func(t *testing.T) {
		url := baseURL + fillPathParams("/notifications/invitation") + "?email=user@example.com&invitername=test&inviteurl=test"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(body))
		}
	})
	t.Run("SendPasswordResetEmail_validation", func(t *testing.T) {
		url := baseURL + fillPathParams("/notifications/password-reset") + "?email=user@example.com&reseturl=test"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(body))
		}
	})
}

func TestContractHTTPPositive(t *testing.T) {
	baseURL := contractBaseURL()
	client := &http.Client{Timeout: 10 * time.Second}
	token := contractToken()
	t.Run("SendNoticeEmail_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/notifications/email") + "?email=user@example.com&title=test&body=test"
		payload := "{\"body\":\"test\",\"email\":\"user@example.com\",\"title\":\"test\"}"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 2xx, got %d: %s", resp.StatusCode, string(body))
		}
	})
	t.Run("SendInvitationEmail_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/notifications/invitation") + "?email=user@example.com&invitername=test&inviteurl=test"
		payload := "{\"email\":\"user@example.com\",\"invitername\":\"test\",\"inviteurl\":\"test\"}"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 2xx, got %d: %s", resp.StatusCode, string(body))
		}
	})
	t.Run("SendPasswordResetEmail_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/notifications/password-reset") + "?email=user@example.com&reseturl=test"
		payload := "{\"email\":\"user@example.com\",\"reseturl\":\"test\"}"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 2xx, got %d: %s", resp.StatusCode, string(body))
		}
	})
}

func TestContractWebSocket(t *testing.T) {
	baseURL := contractWSURL()
	token := contractToken()
}
