package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestRenderFlow_OpenAIStream_StreamingMethod(t *testing.T) {
	t.Parallel()

	steps := []normalizer.FlowStep{
		{
			Action: "openai.Stream",
			Args: map[string]any{
				"user_message": "req.Prompt",
				"model":        `"gpt-4o"`,
				"output":       "reply",
			},
		},
	}

	got := renderFlowForServiceWithSchemaAndSinkMode("Sandbox", "StreamAIEdit", true, steps, nil, nil, nil)
	mustContain := []string{
		`"stream": true`,
		`bufio.NewScanner`,
		`strings.HasPrefix`,
		`chunks <- _chunk`,
		`reply += _chunk`,
	}
	for _, needle := range mustContain {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected generated flow to contain %q, got:\n%s", needle, got)
		}
	}
}

func TestRenderFlow_OpenAIStream_NonStreamingMethod(t *testing.T) {
	t.Parallel()

	steps := []normalizer.FlowStep{
		{
			Action: "openai.Stream",
			Args: map[string]any{
				"user_message": "req.Prompt",
			},
		},
	}

	got := renderFlowForServiceWithSchemaAndSinkMode("Sandbox", "EditOnce", false, steps, nil, nil, nil)
	if !strings.Contains(got, "openai.Stream requires operation stream: true") {
		t.Fatalf("expected streaming guard error in generated flow, got:\n%s", got)
	}
}
