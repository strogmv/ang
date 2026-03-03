package main

import (
	"strings"
	"testing"
)

func TestLintPatchDocument_UnknownOp(t *testing.T) {
	t.Parallel()
	doc := patchDocument{
		Schema: "ang/patch/v1",
		Ops: []map[string]interface{}{
			{"op": "unknown", "file": "cue/api/operations.cue"},
		},
	}
	issues := lintPatchDocument(doc)
	if len(issues) == 0 {
		t.Fatalf("expected lint issues")
	}
	if issues[0].Code != "UNKNOWN_OP" {
		t.Fatalf("unexpected issue: %+v", issues[0])
	}
}

func TestPatchAddEntityAndField(t *testing.T) {
	t.Parallel()
	src := "package domain\n\n"
	withEntity, err := patchAddEntity(src, map[string]interface{}{
		"entity": "Project",
		"fields": map[string]interface{}{
			"id":   "uuid",
			"name": "string",
		},
	})
	if err != nil {
		t.Fatalf("patchAddEntity: %v", err)
	}
	if !strings.Contains(withEntity, "#Project: {") {
		t.Fatalf("expected entity block:\n%s", withEntity)
	}
	withField, err := patchAddField(withEntity, map[string]interface{}{
		"entity": "Project",
		"field":  "slug",
		"type":   "string",
	})
	if err != nil {
		t.Fatalf("patchAddField: %v", err)
	}
	if !strings.Contains(withField, "slug: {type: \"string\"}") {
		t.Fatalf("expected slug field:\n%s", withField)
	}
}

func TestPatchAddEndpoint(t *testing.T) {
	t.Parallel()
	src := `package api

HTTP: {
	ListProjects: {
		method: "GET"
		path:   "/projects"
	}
}
`
	out, err := patchAddEndpoint(src, map[string]interface{}{
		"name":   "CreateProject",
		"method": "POST",
		"path":   "/projects",
	})
	if err != nil {
		t.Fatalf("patchAddEndpoint: %v", err)
	}
	if !strings.Contains(out, "CreateProject: {") {
		t.Fatalf("expected new endpoint:\n%s", out)
	}
}

func TestPatchAppendAndReplaceFlowStep(t *testing.T) {
	t.Parallel()
	src := `package api

CreateProject: {
	service: "project"
	flow: [
		{action: "logic.Check", condition: "req.Name != \"\"", throw: "name required"},
	]
}
`
	out, err := patchAppendFlowStep(src, map[string]interface{}{
		"operation": "CreateProject",
		"step": map[string]interface{}{
			"action":  "flow.RecordEvent",
			"name":    "project.created",
			"payload": "req",
		},
	})
	if err != nil {
		t.Fatalf("patchAppendFlowStep: %v", err)
	}
	if !strings.Contains(out, "flow.RecordEvent") {
		t.Fatalf("expected appended flow step:\n%s", out)
	}

	replaced, err := patchReplaceFlowStep(out, map[string]interface{}{
		"operation": "CreateProject",
		"index":     0,
		"step": map[string]interface{}{
			"action":    "flow.Validate",
			"condition": "req.Name != \"\"",
			"throw":     "name required",
		},
	})
	if err != nil {
		t.Fatalf("patchReplaceFlowStep: %v", err)
	}
	if !strings.Contains(replaced, "flow.Validate") {
		t.Fatalf("expected replaced flow step:\n%s", replaced)
	}
}
