package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractServicesAndEndpoints_StreamFlag(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
		StreamAIEdit: {
			service: "sandbox"
			stream:  true
			input: {
				projectID: string
				prompt:    string
			}
			output: {}
			flow: [
				{action: "openai.Stream", user_message: "req.Prompt"},
			]
		}
		HTTP: {
			StreamAIEdit: {
				method: "POST"
				path:   "/projects/{projectID}/ai-edit/stream"
			}
		}
	`)

	n := New()
	services, err := n.ExtractServices(val, nil)
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}
	if len(services) != 1 || len(services[0].Methods) != 1 {
		t.Fatalf("expected one service with one method, got %#v", services)
	}
	if !services[0].Methods[0].IsStreaming {
		t.Fatalf("expected method IsStreaming=true")
	}

	endpoints, err := n.ExtractEndpoints(val)
	if err != nil {
		t.Fatalf("ExtractEndpoints failed: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected one endpoint, got %d", len(endpoints))
	}
	if !endpoints[0].IsStreaming {
		t.Fatalf("expected endpoint IsStreaming=true")
	}
}
