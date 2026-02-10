package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	pyemitter "github.com/strogmv/ang/compiler/emitter/python"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestBuildPythonModels_AliasAndEntityRefs(t *testing.T) {
	entities := []normalizer.Entity{
		{
			Name: "Post",
			Fields: []normalizer.Field{
				{Name: "id", Type: "domain.ID"},
				{Name: "author", Type: "domain.User"},
				{Name: "tags", Type: "[]domain.Tag"},
			},
		},
		{
			Name: "User",
			Fields: []normalizer.Field{
				{Name: "id", Type: "domain.ID"},
			},
		},
		{
			Name: "Tag",
			Fields: []normalizer.Field{
				{Name: "id", Type: "domain.ID"},
			},
		},
	}

	models := pyemitter.BuildSDKModels(entities)
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}

	var post pyemitter.SDKModel
	for _, m := range models {
		if m.Name == "Post" {
			post = m
			break
		}
	}
	if post.Name == "" {
		t.Fatalf("post model not found")
	}
	if post.Fields[0].Type != "str" {
		t.Fatalf("expected id as str, got %s", post.Fields[0].Type)
	}
	if post.Fields[1].Type != "User" {
		t.Fatalf("expected author as User, got %s", post.Fields[1].Type)
	}
	if post.Fields[2].Type != "list[Tag]" {
		t.Fatalf("expected tags as list[Tag], got %s", post.Fields[2].Type)
	}
}

func TestEmitPythonFastAPIBackend_TypedRouterSignatures(t *testing.T) {
	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	em.Version = "0.1.0"

	entities := []normalizer.Entity{
		{Name: "LoginRequest", Fields: []normalizer.Field{{Name: "email", Type: "string"}}},
		{Name: "AuthTokens", Fields: []normalizer.Field{{Name: "accessToken", Type: "string"}}},
	}
	services := []normalizer.Service{
		{
			Name: "Auth",
			Methods: []normalizer.Method{
				{
					Name:   "Login",
					Input:  normalizer.Entity{Name: "LoginRequest"},
					Output: normalizer.Entity{Name: "AuthTokens"},
				},
			},
		},
	}
	endpoints := []normalizer.Endpoint{
		{Method: "POST", Path: "/auth/login", ServiceName: "Auth", RPC: "Login"},
	}

	if err := em.EmitPythonFastAPIBackend(entities, services, endpoints, nil, nil); err != nil {
		t.Fatalf("emit python fastapi backend: %v", err)
	}

	routerPath := filepath.Join(tmp, "app", "routers", "auth.py")
	data, err := os.ReadFile(routerPath)
	if err != nil {
		t.Fatalf("read router: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "from app import models") {
		t.Fatalf("expected models import in router")
	}
	if !strings.Contains(text, "payload: models.LoginRequest") {
		t.Fatalf("expected typed payload in router signature")
	}
	if !strings.Contains(text, "-> models.AuthTokens:") {
		t.Fatalf("expected typed return in router signature")
	}
}
