package flowfn

import "testing"

func TestParseAndTranspile(t *testing.T) {
	t.Parallel()

	src := `
repo.Find(source: "Project", input: req.ProjectID, output: project)
if req.Admin {
  repo.Save(source: "AuditLog", input: project)
} else {
  repo.Save(source: "AuditLog", input: project)
}
for item in req.Items {
  repo.Save(source: "LineItem", input: item)
}
try {
  repo.Save(source: "Project", input: project)
} catch {
  event.Publish(event: "Failed")
}
`
	program, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if err := Validate(program); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	steps, err := Transpile(program)
	if err != nil {
		t.Fatalf("Transpile failed: %v", err)
	}
	if len(steps) != 4 {
		t.Fatalf("expected 4 top-level steps, got %d", len(steps))
	}
	if steps[0].Action != "repo.Find" {
		t.Fatalf("unexpected first action %q", steps[0].Action)
	}
	if steps[1].Action != "flow.If" {
		t.Fatalf("expected flow.If, got %q", steps[1].Action)
	}
	if steps[2].Action != "flow.For" {
		t.Fatalf("expected flow.For, got %q", steps[2].Action)
	}
	if steps[3].Action != "flow.Try" {
		t.Fatalf("expected flow.Try, got %q", steps[3].Action)
	}
}

func TestValidateUnknownAction(t *testing.T) {
	t.Parallel()

	_, err := Parse(`unknown.Action(foo: "bar")`)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	program, _ := Parse(`unknown.Action(foo: "bar")`)
	if err := Validate(program); err == nil {
		t.Fatal("expected validation error for unknown action")
	}
}
