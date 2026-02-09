package emitter

import (
	"testing"

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

	models := buildPythonModels(entities)
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}

	var post pythonModel
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
