package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractServices_FlowFn(t *testing.T) {
	t.Parallel()

	val := cuecontext.New().CompileString(`
Login: {
	service: "Auth"
	input: { email: string }
	output: { ok: bool }
	flowfn: """
repo.Find(source: "User", input: req.Email, output: user)
if user.Active {
  repo.Save(source: "User", input: user)
}
"""
}
`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	services, err := New().ExtractServices(val, []Entity{{Name: "User"}})
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}
	if len(services) != 1 || len(services[0].Methods) != 1 {
		t.Fatalf("unexpected services shape: %+v", services)
	}
	steps := services[0].Methods[0].Flow
	if len(steps) != 2 {
		t.Fatalf("expected 2 flow steps, got %d", len(steps))
	}
	if steps[0].Action != "repo.Find" {
		t.Fatalf("unexpected first step %q", steps[0].Action)
	}
	if steps[1].Action != "flow.If" {
		t.Fatalf("unexpected second step %q", steps[1].Action)
	}
}
