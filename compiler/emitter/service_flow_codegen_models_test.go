package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestRenderFlow_ModelResolveUsesInfraModels(t *testing.T) {
	t.Parallel()

	steps := []normalizer.FlowStep{
		{
			Action: "model.Resolve",
			Args: map[string]any{
				"name":   "\"Cheap\"",
				"output": "modelName",
			},
		},
	}

	code := renderFlowForServiceWithSchemaAndSinkModeWithInfra(
		"Sandbox",
		"Resolve",
		false,
		steps,
		nil,
		nil,
		nil,
		map[string]any{
			normalizer.InfraKeyModels: &normalizer.ModelsDef{
				Aliases: map[string]string{
					"Cheap": "gpt-5-nano",
				},
			},
		},
	)

	if !strings.Contains(code, `var modelName string = "gpt-5-nano"`) {
		t.Fatalf("expected resolved model literal in generated code, got:\n%s", code)
	}
}
