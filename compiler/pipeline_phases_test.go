package compiler

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"github.com/strogmv/ang-ir/normalizer"
)

func TestMergeArchitectureServiceMetadata_MergesDependsIntoServices(t *testing.T) {
	t.Parallel()

	ctx := cuecontext.New()
	arch := ctx.CompileString(`
package architecture

#Services: {
	assistant: {
		name: "Assistant"
		depends: ["Blog", "Auth"]
		publishes: ["AssistantAsked"]
		subscribes: {
			PostPublished: "OnPostPublished"
			PostDeleted: {op: "OnPostDeleted", delivery: "broadcast"}
		}
	}
}
`)
	if err := arch.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	services := []normalizer.Service{
		{Name: "Assistant"},
	}
	got := mergeArchitectureServiceMetadata(services, arch)
	if len(got) != 1 {
		t.Fatalf("expected 1 service, got %d", len(got))
	}
	if len(got[0].Uses) != 2 || got[0].Uses[0] != "Blog" || got[0].Uses[1] != "Auth" {
		t.Fatalf("unexpected merged uses: %#v", got[0].Uses)
	}
	if len(got[0].Publishes) != 1 || got[0].Publishes[0] != "AssistantAsked" {
		t.Fatalf("unexpected merged publishes: %#v", got[0].Publishes)
	}
	if got[0].Subscribes["PostPublished"] != "OnPostPublished" {
		t.Fatalf("unexpected merged subscribes: %#v", got[0].Subscribes)
	}
	if got[0].Subscribes["PostDeleted"] != "OnPostDeleted" {
		t.Fatalf("unexpected object subscription: %#v", got[0].Subscribes)
	}
	delivery, ok := got[0].Metadata["subscriptionDelivery"].(map[string]any)
	if !ok || delivery["PostPublished"] != "queue" || delivery["PostDeleted"] != "broadcast" {
		t.Fatalf("unexpected subscription delivery metadata: %#v", got[0].Metadata)
	}
}

func TestRunFlowSemPhasePublishesTypedVariableDiagnostic(t *testing.T) {
	var diagnostics []normalizer.Warning
	service := normalizer.Service{Methods: []normalizer.Method{{
		Name:   "Run",
		Input:  normalizer.Entity{Name: "RunRequest", Fields: []normalizer.Field{{Name: "name", Type: "string"}}},
		Output: normalizer.Entity{Name: "RunResponse", Fields: []normalizer.Field{{Name: "name", Type: "string"}}},
		Flow: []normalizer.FlowStep{
			{Action: "flow.If", Args: map[string]any{"condition": "true", "_then": []normalizer.FlowStep{{Action: "mapping.Assign", Args: map[string]any{"to": "branchValue", "value": "req.Name", "declare": true}}}}},
			{Action: "mapping.Assign", Args: map[string]any{"to": "resp.Name", "value": "branchValue"}},
		},
	}}}

	runFlowSemPhase(FlowSemPhaseInput{Services: []normalizer.Service{service}}, PipelineOptions{
		WarningSink: func(w normalizer.Warning) { diagnostics = append(diagnostics, w) },
	})
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "FLOW_VARIABLE_UNKNOWN" {
			if diagnostic.Severity != "error" {
				t.Fatalf("FLOW_VARIABLE_UNKNOWN severity = %q, want error", diagnostic.Severity)
			}
			return
		}
	}
	t.Fatalf("FLOW_VARIABLE_UNKNOWN not forwarded to pipeline diagnostics: %#v", diagnostics)
}
