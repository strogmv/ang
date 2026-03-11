package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeCueTopLevelDeclsPreservesManualDecl(t *testing.T) {
	desired := "package api\nA: {x: 1}\n"
	current := "package api\nA: {x: 1}\nB: {y: 2}\n"
	merged, ok := mergeCueTopLevelDecls(desired, current)
	if !ok {
		t.Fatal("expected merge to succeed")
	}
	if !containsAll(merged, "A:", "B:") {
		t.Fatalf("expected merged CUE to preserve B, got:\n%s", merged)
	}
}

func TestMergeCueTopLevelDeclsRejectsPackageMismatch(t *testing.T) {
	if _, ok := mergeCueTopLevelDecls("package a\nA: 1\n", "package b\nB: 2\n"); ok {
		t.Fatal("expected merge to fail on package mismatch")
	}
}

func TestTryTemplatePreserveMergeGoCustomBlocks(t *testing.T) {
	desired := "package bootstrap\n// ANG:BEGIN_CUSTOM runtime_container.after_init\n// default\n// ANG:END_CUSTOM runtime_container.after_init\n"
	current := "package bootstrap\n// ANG:BEGIN_CUSTOM runtime_container.after_init\nfmt.Println(\"keep\")\n// ANG:END_CUSTOM runtime_container.after_init\n"
	merged, strategy, ok := tryTemplatePreserveMerge("internal/bootstrap/runtime_container.go", desired, current)
	if !ok || strategy != "go_custom_blocks" {
		t.Fatalf("expected go custom block merge, got ok=%v strategy=%q", ok, strategy)
	}
	if !containsAll(merged, `fmt.Println("keep")`, "runtime_container.after_init") {
		t.Fatalf("expected preserved go custom body, got:\n%s", merged)
	}
}

func TestResolveTemplatePlanFromDiffReport(t *testing.T) {
	report := templateDiffReport{Schema: "ang/template-diff/v1", Files: []templateDriftFile{{Path: "a.go", Action: "update", Classification: "safe_drift", AutoApplicable: true}}}
	path := filepath.Join(t.TempDir(), "diff.json")
	if err := writeJSONFile(path, report); err != nil {
		t.Fatal(err)
	}
	plan, err := resolveTemplatePlan(".", templateDiffReport{}, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Path != "a.go" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func containsAll(s string, needles ...string) bool {
	for _, n := range needles {
		if !strings.Contains(s, n) {
			return false
		}
	}
	return true
}
