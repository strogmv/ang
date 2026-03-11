package main

import (
	"testing"
)

func TestMergeImportOperations_OpenAPIWinsAndReportsConflict(t *testing.T) {
	javaOps := []FactOp{{
		Name:        "AddOwner",
		ServiceHint: "owner",
		HTTPMethod:  "PUT",
		HTTPPath:    "/api/owners/{id}",
		AuthExpr:    "hasRole('OWNER_ADMIN')",
	}}
	openapiOps := []FactOp{{
		Name:        "addOwner",
		ServiceHint: "owners",
		HTTPMethod:  "POST",
		HTTPPath:    "/owners",
	}}
	var conflicts []importConflict
	var todos []importTodo
	ops := mergeImportOperations(javaOps, openapiOps, &conflicts, &todos)
	if len(ops) != 1 {
		t.Fatalf("ops len = %d, want 1", len(ops))
	}
	op := ops[0]
	if op.HTTPMethod != "POST" {
		t.Fatalf("method = %q, want POST", op.HTTPMethod)
	}
	if op.HTTPPath != "/owners" {
		t.Fatalf("path = %q, want /owners", op.HTTPPath)
	}
	if len(conflicts) == 0 {
		t.Fatal("expected conflicts for method/path mismatch")
	}
}

func TestMergeImportEntities_JavaPreferredOnTypeConflict(t *testing.T) {
	javaEntities := []FactEntity{{
		Name:   "Owner",
		Fields: []FactField{{Name: "Id", CueTypeHint: "int"}},
	}}
	sqlEntities := []FactEntity{{
		Name:   "owner",
		Fields: []FactField{{Name: "Id", CueTypeHint: "string"}},
	}}
	var conflicts []importConflict
	entities := mergeImportEntities(javaEntities, nil, sqlEntities, &conflicts)
	if len(entities) != 1 {
		t.Fatalf("entities len = %d, want 1", len(entities))
	}
	if len(conflicts) == 0 {
		t.Fatal("expected at least one type conflict")
	}
	if entities[0].Confidence != confidenceHigh {
		t.Fatalf("confidence = %q, want %q", entities[0].Confidence, confidenceHigh)
	}
}

func TestLocalJavaImportOptionsToTransform_ParserBackend(t *testing.T) {
	in := importJavaOptions{
		Root:              ".",
		OutDir:            "cue/import",
		JavaParserBackend: "regex",
	}
	out, err := localJavaImportOptionsToTransform(in)
	if err != nil {
		t.Fatalf("localJavaImportOptionsToTransform: %v", err)
	}
	if out.JavaParserBackend != "regex" {
		t.Fatalf("java parser backend = %q, want regex", out.JavaParserBackend)
	}
}

func TestMergeImportEntities_CanonicalizesMediaProfileFields(t *testing.T) {
	javaEntities := []FactEntity{{
		Name: "UserProfile",
		Fields: []FactField{
			{Name: "avatarURL", CueTypeHint: "string"},
			{Name: "profilePicture", CueTypeHint: "string"},
		},
	}}

	var conflicts []importConflict
	entities := mergeImportEntities(javaEntities, nil, nil, &conflicts)
	if len(entities) != 1 {
		t.Fatalf("entities len = %d, want 1", len(entities))
	}
	if got := len(entities[0].Fields); got != 1 {
		t.Fatalf("fields len = %d, want 1 after canonical dedupe: %+v", got, entities[0].Fields)
	}
	if entities[0].Fields[0].Name != "PhotoURL" {
		t.Fatalf("field name = %q, want PhotoURL", entities[0].Fields[0].Name)
	}
}
