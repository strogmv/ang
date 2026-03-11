package main

import (
	"os"
	"path/filepath"
	"testing"

	transform "github.com/strogmv/ang-transform/pkg/transform"
)

func TestExtractViaTransform_AutoOpenAPI(t *testing.T) {
	dir := t.TempDir()
	spec := `openapi: 3.0.0
paths:
  /owners:
    get:
      operationId: listOwners
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(filepath.Join(dir, "openapi.yml"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	env, err := extractViaTransform("auto", dir)
	if err != nil {
		t.Fatalf("extractViaTransform: %v", err)
	}
	if len(env.Operations) == 0 {
		t.Fatalf("expected operations extracted from openapi, got none")
	}
}

func TestTransformFactsToLocal_RoundTrip(t *testing.T) {
	tf := transformFactsFixture()
	local, err := transformFactsToLocal(tf)
	if err != nil {
		t.Fatalf("transformFactsToLocal: %v", err)
	}
	if len(local.Entities) != 1 || local.Entities[0].Name != "Owner" {
		t.Fatalf("unexpected entities after conversion: %+v", local.Entities)
	}
}

func TestTransformFactsToLocal_CanonicalizesMediaProfileFields(t *testing.T) {
	tf := transform.FactsEnvelope{
		Schema: "ang/facts/v1",
		Entities: []transform.FactEntity{{
			Name: "UserProfile",
			Fields: []transform.FactField{
				{Name: "avatarURL", CueTypeHint: "string"},
				{Name: "profilePicture", CueTypeHint: "string"},
			},
		}},
		Operations: []transform.FactOp{{
			Name: "UpdateProfileAvatar",
			InputFields: []transform.FactField{
				{Name: "image", CueTypeHint: "string"},
			},
			OutputFields: []transform.FactField{
				{Name: "avatarURL", CueTypeHint: "string"},
			},
		}},
	}

	local, err := transformFactsToLocal(tf)
	if err != nil {
		t.Fatalf("transformFactsToLocal: %v", err)
	}
	if got := len(local.Entities[0].Fields); got != 1 {
		t.Fatalf("entity fields len=%d, want 1 after canonical dedupe: %+v", got, local.Entities[0].Fields)
	}
	if local.Entities[0].Fields[0].Name != "PhotoURL" {
		t.Fatalf("entity field name=%q, want PhotoURL", local.Entities[0].Fields[0].Name)
	}
	if local.Operations[0].InputFields[0].Name != "PhotoURL" {
		t.Fatalf("input field name=%q, want PhotoURL", local.Operations[0].InputFields[0].Name)
	}
	if local.Operations[0].OutputFields[0].Name != "PhotoURL" {
		t.Fatalf("output field name=%q, want PhotoURL", local.Operations[0].OutputFields[0].Name)
	}
}

func TestExtractViaTransformWithOptions_InvalidJavaParserBackend(t *testing.T) {
	dir := t.TempDir()
	java := `package demo;
public class Demo { }
`
	if err := os.WriteFile(filepath.Join(dir, "Demo.java"), []byte(java), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := extractViaTransformWithOptions("java", dir, extractTransformOptions{
		JavaParserBackend: "nope",
	})
	if err == nil {
		t.Fatal("expected error for invalid java parser backend")
	}
}

func transformFactsFixture() transform.FactsEnvelope {
	return transform.FactsEnvelope{
		Schema: "ang/facts/v1",
		Entities: []transform.FactEntity{{
			Name:   "Owner",
			Fields: []transform.FactField{{Name: "id", CueTypeHint: "string", Required: true}},
		}},
	}
}
