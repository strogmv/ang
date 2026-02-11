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
	t.Run("GetProfile_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/auth/profile")
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("UpdateProfile_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/auth/profile")
		req, err := http.NewRequest("PUT", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("CreateTag_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/tags")
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("UpdateTag_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/tags/{id}")
		req, err := http.NewRequest("PUT", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("DeleteTag_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/tags/{id}")
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("CreatePost_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/posts")
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("ListMyPosts_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/my/posts")
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("UpdatePost_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/posts/{id}")
		req, err := http.NewRequest("PUT", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("SubmitPost_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/posts/{id}/submit")
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("PublishPost_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/posts/{id}/publish")
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("ArchivePost_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/posts/{id}/archive")
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("DeletePost_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/posts/{id}")
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("CreateComment_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/posts/{postID}/comments")
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("UpdateComment_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/comments/{id}")
		req, err := http.NewRequest("PUT", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
	t.Run("DeleteComment_unauthorized", func(t *testing.T) {
		url := baseURL + fillPathParams("/comments/{id}")
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
}

func TestContractHTTPValidation(t *testing.T) {
	baseURL := contractBaseURL()
	client := &http.Client{Timeout: 10 * time.Second}
	token := contractToken()
	t.Run("Register_validation", func(t *testing.T) {
		url := baseURL + fillPathParams("/auth/register") + "?email=test&password=test&name=test"
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
	t.Run("Login_validation", func(t *testing.T) {
		url := baseURL + fillPathParams("/auth/login") + "?email=test&password=test"
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
	t.Run("CreateTag_validation", func(t *testing.T) {
		url := baseURL + fillPathParams("/tags") + "?name=test"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("CreatePost_validation", func(t *testing.T) {
		url := baseURL + fillPathParams("/posts") + "?title=test&content=test"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("CreateComment_validation", func(t *testing.T) {
		url := baseURL + fillPathParams("/posts/{postID}/comments") + "?content=test"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("UpdateComment_validation", func(t *testing.T) {
		url := baseURL + fillPathParams("/comments/{id}") + "?content=test"
		req, err := http.NewRequest("PUT", url, bytes.NewBufferString("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("Register_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/auth/register") + "?email=test&password=test&name=test"
		payload := "{\"email\":\"test\",\"name\":\"test\",\"password\":\"test1234\"}"
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
	t.Run("Login_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/auth/login") + "?email=test&password=test"
		payload := "{\"email\":\"test\",\"password\":\"test1234\"}"
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
	t.Run("UpdateProfile_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/auth/profile")
		payload := "{}"
		req, err := http.NewRequest("PUT", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("CreateTag_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/tags") + "?name=test"
		payload := "\"test\""
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("UpdateTag_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/tags/{id}")
		payload := "{}"
		req, err := http.NewRequest("PUT", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("DeleteTag_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/tags/{id}")
		payload := "{}"
		req, err := http.NewRequest("DELETE", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("CreatePost_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/posts") + "?title=test&content=test"
		payload := "{\"content\":\"test\",\"title\":\"test\"}"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("UpdatePost_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/posts/{id}")
		payload := "{}"
		req, err := http.NewRequest("PUT", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("SubmitPost_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/posts/{id}/submit")
		payload := "{}"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("PublishPost_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/posts/{id}/publish")
		payload := "{}"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("ArchivePost_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/posts/{id}/archive")
		payload := "{}"
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("DeletePost_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/posts/{id}")
		payload := "{}"
		req, err := http.NewRequest("DELETE", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("CreateComment_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/posts/{postID}/comments") + "?content=test"
		payload := "\"test\""
		req, err := http.NewRequest("POST", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("UpdateComment_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/comments/{id}") + "?content=test"
		payload := "\"test\""
		req, err := http.NewRequest("PUT", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
	t.Run("DeleteComment_positive", func(t *testing.T) {
		url := baseURL + fillPathParamsRequired(t, "/comments/{id}")
		payload := "{}"
		req, err := http.NewRequest("DELETE", url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token == "" {
			auth := ensureAuth(t)
			token = auth.accessToken
		}
		if token == "" {
			t.Skip("CONTRACT_TOKEN not set")
		}
		req.Header.Set("Authorization", "Bearer "+token)
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
