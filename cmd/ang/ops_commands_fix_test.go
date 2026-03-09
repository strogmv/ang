package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestApplyStructuredFix_ReplaceAction(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cue", "api", "ops.cue")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `package api

Create: {
	flow: [
		{action: "http.Reqeust", method: "GET", url: "https://example.com"},
	]
}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	changed, err := applyStructuredFix(".", normalizer.Warning{
		File:   path,
		Line:   5,
		Action: "http.Reqeust",
	}, normalizer.Fix{
		Op:    "merge",
		Value: map[string]any{"action": "http.Request"},
	})
	if err != nil {
		t.Fatalf("applyStructuredFix: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), `action: "http.Request"`) {
		t.Fatalf("expected action replacement, got:\n%s", string(after))
	}
}

func TestApplyStructuredFix_AddTimeout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cue", "api", "ops.cue")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `package api

Create: {
	flow: [
		{action: "http.Request", method: "GET", url: "https://example.com"},
	]
}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	changed, err := applyStructuredFix(".", normalizer.Warning{
		File:   path,
		Line:   5,
		Action: "http.Request",
	}, normalizer.Fix{
		Op:    "merge",
		Value: map[string]any{"timeout": "5s"},
	})
	if err != nil {
		t.Fatalf("applyStructuredFix: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), `timeout: "5s"`) {
		t.Fatalf("expected timeout merge, got:\n%s", string(after))
	}
}

func TestApplyStructuredFix_ReplaceAssignValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cue", "api", "ops.cue")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `package api

Create: {
	flow: [
		{action: "mapping.Assign", to: "post.Status", value: "draft"},
	]
}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	changed, err := applyStructuredFix(".", normalizer.Warning{
		File:   path,
		Line:   5,
		Action: "mapping.Assign",
	}, normalizer.Fix{
		Op:    "merge",
		Value: map[string]any{"value": "\"draft\""},
	})
	if err != nil {
		t.Fatalf("applyStructuredFix: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), `value: "\"draft\""`) {
		t.Fatalf("expected quoted status literal, got:\n%s", string(after))
	}
}

func TestEnrichSuggestedFixDiags_FillsDefaults(t *testing.T) {
	t.Parallel()
	in := []normalizer.Warning{
		{
			Code:         "NEEDS_QUOTES",
			File:         "cue/api/blog.cue",
			Line:         10,
			CUEPath:      "api.Create.flow[1]",
			CanAutoApply: true,
			SuggestedFix: []normalizer.Fix{{
				Kind: "merge",
				Text: `value: "\"draft\""`,
			}},
		},
	}
	out := enrichSuggestedFixDiags(in)
	if len(out[0].SuggestedFix) != 1 {
		t.Fatalf("expected one suggested fix")
	}
	fx := out[0].SuggestedFix[0]
	if fx.Op != "merge" {
		t.Fatalf("expected op=merge, got %q", fx.Op)
	}
	if fx.File != "cue/api/blog.cue" {
		t.Fatalf("expected file propagated, got %q", fx.File)
	}
	if fx.CUEPath != "api.Create.flow[1]" {
		t.Fatalf("expected cue_path propagated, got %q", fx.CUEPath)
	}
	if !out[0].CanAutoApply {
		t.Fatalf("expected can_auto_apply=true")
	}
}

func TestApplyStructuredFix_SkipsSchemaFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "cue", "schema", "types.cue")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `package schema

#EventStep: {
	action: "event.Publish" | "event.Broadcast"
}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	changed, err := applyStructuredFix(".", normalizer.Warning{
		File:   path,
		Line:   4,
		Action: "event.Publish",
	}, normalizer.Fix{
		Op:    "merge",
		Value: map[string]any{"action": "event.Outbox"},
	})
	if err != nil {
		t.Fatalf("applyStructuredFix: %v", err)
	}
	if changed {
		t.Fatalf("expected schema file to be skipped")
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), `action: "event.Publish" | "event.Broadcast"`) {
		t.Fatalf("schema file changed unexpectedly:\n%s", string(after))
	}
}

func TestRewriteDiagnosticSources_RewritesSchemaLocationToOperationSource(t *testing.T) {
	t.Parallel()
	services := []normalizer.Service{{
		Name: "Auth",
		Methods: []normalizer.Method{{
			Name:   "Register",
			Source: "cue/api/auth.cue:51",
		}},
	}}
	in := []normalizer.Warning{{
		Op:   "Auth.Register",
		File: "cue/schema/types.cue",
		Line: 44,
		SuggestedFix: []normalizer.Fix{{
			File: "cue/schema/types.cue",
			Op:   "merge",
		}},
	}}

	out := rewriteDiagnosticSources(in, services)
	if out[0].File != "cue/api/auth.cue" {
		t.Fatalf("expected diag file rewrite, got %q", out[0].File)
	}
	if out[0].Line != 51 {
		t.Fatalf("expected diag line rewrite, got %d", out[0].Line)
	}
	if out[0].SuggestedFix[0].File != "cue/api/auth.cue" {
		t.Fatalf("expected suggested fix file rewrite, got %q", out[0].SuggestedFix[0].File)
	}
}

func TestEnrichSuggestedFixDiags_DoesNotPromoteUnsafeCode(t *testing.T) {
	t.Parallel()
	in := []normalizer.Warning{
		{
			Code:         "W_FLOW_OUTBOX_PREFERRED",
			File:         "cue/api/auth.cue",
			Line:         51,
			CanAutoApply: false,
			SuggestedFix: []normalizer.Fix{{
				Op:    "merge",
				Value: map[string]any{"action": "event.Outbox"},
			}},
		},
	}
	out := enrichSuggestedFixDiags(in)
	if out[0].CanAutoApply {
		t.Fatalf("unsafe code should remain manual")
	}
}
