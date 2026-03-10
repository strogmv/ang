package emitter

import (
	"strings"
	"testing"
)

func TestRenderCueEmitProjectCode_NormalizesServiceContext(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"single_file"`, "projectFiles", ":=", "_files", "_err")
	if !strings.Contains(code, `_serviceName := _contextName(_usecases.ServiceName)`) {
		t.Fatalf("expected service context normalization in generated code, got:\n%s", code)
	}
	if !strings.Contains(code, `owner: \"%s\"\n", _serviceName`) {
		t.Fatalf("expected entities to use normalized service context, got:\n%s", code)
	}
	if !strings.Contains(code, `service:     \"%s\"\n", _serviceName`) {
		t.Fatalf("expected operations to use normalized service context, got:\n%s", code)
	}
}

func TestRenderPlanBuildMicroPlanCode_AddsCreateAndReplyHeuristics(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`_startsWithAny(name, "create", "submit", "add")`,
		`_startsWithAny(name, "approve", "reject", "resolve", "close", "open", "cancel", "complete", "activate", "deactivate")`,
		`_appendEntityReplies(&steps, uc.OutputFields, uc.PrimaryEntity, entityVar)`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected generated micro-plan heuristics %q, got:\n%s", snippet, code)
		}
	}
}
