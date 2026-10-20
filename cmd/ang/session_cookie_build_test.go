package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// sessionCookieAuthCUE turns the saas scaffold's auth service into an
// opaque-session login: LoginUser's tokens become a session cookie.
const sessionCookieAuthCUE = `package infra

#Auth: {
	service: "auth"
	jwt: {
		mode: "opaque_session_cookie"
		alg:  "HS256"
		tokens: {access_ttl: "15m", refresh_ttl: "168h", store: "redis"}
		ops: login: {op: "LoginUser", access_field: "accessToken", refresh_field: "refreshToken"}
	}
}
`

const sessionCookieLoginCUE = `package api

LoginUser: {
	service: "auth"
	input: {email: string}
	output: {accessToken: string, refreshToken: string}
	flow: [
		{action: "mapping.Assign", to: "resp.AccessToken", value: "req.Email"},
	]
}

HTTP: LoginUser: {method: "POST", path: "/auth/login", auth: {public: true, reason: "sign-in"}}
`

// sessionCookieHandlerTest runs inside the generated transport package: the
// login handler against a session store that fails, recovers, or is never
// asked (no tokens, a second-factor challenge).
const sessionCookieHandlerTest = `package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/example/sessapp/internal/config"
	"github.com/example/sessapp/internal/pkg/auth"
	"github.com/example/sessapp/internal/port"
)

type flakyStore struct {
	mu        sync.Mutex
	failFirst int // Set of a session key fails this many times
	sessionSets int
	data      map[string][]byte
}

func (s *flakyStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[key], nil
}

func (s *flakyStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.HasPrefix(key, "auth_session:") {
		s.sessionSets++
		if s.sessionSets <= s.failFirst {
			return errors.New("redis: connection pool timeout")
		}
	}
	if s.data == nil {
		s.data = map[string][]byte{}
	}
	s.data[key] = value
	return nil
}

func (s *flakyStore) Delete(_ context.Context, key string) error { return nil }

type loginStub struct {
	port.Auth
	access, refresh string
}

func (l loginStub) LoginUser(context.Context, port.LoginUserRequest) (port.LoginUserResponse, error) {
	return port.LoginUserResponse{AccessToken: l.access, RefreshToken: l.refresh}, nil
}

func TestLoginNeverAnswersSuccessWithoutTheSessionCookie(t *testing.T) {
	cfg := &config.Config{JWTAlg: "HS256", JWTPrivateKey: "test-secret-test-secret-test-secret", JWTAccessTTL: "15m", JWTRefreshTTL: "168h"}
	if err := SetAuthConfigFromConfig(cfg); err != nil {
		t.Fatal(err)
	}
	access, err := auth.IssueAccessToken(cfg, "u1", "c1", []string{"owner"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sessionSaveBackoff = []time.Duration{time.Millisecond}
	login := func(store *flakyStore, stub loginStub) *httptest.ResponseRecorder {
		SetAuthSessionStore(store)
		r := chi.NewRouter()
		RegisterAuthRoutes(r, stub)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(` + "`" + `{"email":"a@b.example"}` + "`" + `)))
		return rec
	}
	sessionCookie := func(rec *httptest.ResponseRecorder) string {
		for _, c := range rec.Result().Cookies() {
			if c.Name == authSessionCookieName && c.MaxAge > 0 {
				return c.Value
			}
		}
		return ""
	}

	down := &flakyStore{failFirst: 100}
	rec := login(down, loginStub{access: access, refresh: "r1"})
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "SESSION_STORE_UNAVAILABLE") {
		t.Fatalf("store down: %d %s, want 503 SESSION_STORE_UNAVAILABLE", rec.Code, rec.Body.String())
	}
	if c := sessionCookie(rec); c != "" || strings.Contains(rec.Body.String(), access) {
		t.Fatalf("store down: no cookie and no tokens in the answer, got cookie %q body %s", c, rec.Body.String())
	}
	if down.sessionSets != sessionSaveAttempts {
		t.Fatalf("store down: %d save attempts, want %d", down.sessionSets, sessionSaveAttempts)
	}

	busy := &flakyStore{failFirst: 1}
	rec = login(busy, loginStub{access: access, refresh: "r1"})
	if rec.Code != http.StatusOK || sessionCookie(rec) == "" {
		t.Fatalf("store busy once: %d, cookie %q; want 200 with the cookie after a retry", rec.Code, sessionCookie(rec))
	}
	if _, ok := busy.data["auth_session:"+sessionCookie(rec)]; !ok {
		t.Fatal("the cookie names a session the store does not hold")
	}

	challenge := &flakyStore{failFirst: 100}
	rec = login(challenge, loginStub{})
	if rec.Code != http.StatusOK || sessionCookie(rec) != "" || challenge.sessionSets != 0 {
		t.Fatalf("no tokens (second factor due): %d, cookie %q, %d saves; want 200, no session", rec.Code, sessionCookie(rec), challenge.sessionSets)
	}
}
`

// The session-cookie bug (dealingi-back, phone live room under load): when
// the session store failed to save a session, writeAuthCookies returned
// silently and the login answered 200 without a Set-Cookie, so the very next
// request was a 401. Now a failing store is retried, and a session that is
// still not stored answers 503 SESSION_STORE_UNAVAILABLE without the tokens.
func TestLoginFailsLoudlyWhenTheSessionStoreDoesNotSave(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and tests a generated project")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	projectDir := filepath.Join(t.TempDir(), "sessapp")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "sessapp",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/sessapp",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(projectDir, "cue", "infra", "auth.cue"), sessionCookieAuthCUE)
	writeTestFile(t, filepath.Join(projectDir, "cue", "api", "login.cue"), sessionCookieLoginCUE)
	if err := runBuild([]string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}); err != nil {
		t.Fatalf("build: %v", err)
	}
	writeTestFile(t, filepath.Join(projectDir, "internal", "transport", "http", "session_cookie_test.go"), sessionCookieHandlerTest)
	runGoOffline(t, projectDir, "github.com/example/sessapp", "test", "-count=1", "-run", "TestLoginNeverAnswersSuccessWithoutTheSessionCookie", "./internal/transport/http/")
}
