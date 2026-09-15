package flowir

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

// Every action in the catalog shows a real step, and the step decodes with the
// action's own Decode: an example cannot drift from what the compiler accepts.
func TestActionExamplesDecode(t *testing.T) {
	for _, spec := range All() {
		example, ok := actionExamples[spec.Name]
		if !ok {
			t.Errorf("%s has no example", spec.Name)
			continue
		}
		if example.Action != spec.Name {
			t.Errorf("example for %s is a %s step", spec.Name, example.Action)
			continue
		}
		if _, err := DecodeSteps([]normalizer.FlowStep{example}); err != nil {
			t.Errorf("example for %s does not decode: %v", spec.Name, err)
		}
	}
	for name := range actionExamples {
		if _, ok := Lookup(name); !ok {
			t.Errorf("example for unknown action %s", name)
		}
	}
}

func TestExampleCUERendersNestedStepsAsCUE(t *testing.T) {
	got, ok := ExampleCUE("tx.Block")
	if !ok {
		t.Fatal("tx.Block has no example")
	}
	if !strings.HasPrefix(got, `{action: "tx.Block", do: [{action: `) || strings.Contains(got, "_do") {
		t.Fatalf("ExampleCUE(tx.Block) = %s", got)
	}
	got, _ = ExampleCUE("repo.Query")
	if strings.Contains(got, "<expr>") || !strings.Contains(got, "source: ") {
		t.Fatalf("ExampleCUE(repo.Query) = %s", got)
	}
	if _, ok := ExampleCUE("no.SuchAction"); ok {
		t.Fatal("unknown action must have no example")
	}
}
