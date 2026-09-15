package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func readGoldenSteps(t *testing.T, name string) []goldenFlowStep {
	t.Helper()
	path := filepath.Join("..", "..", "cue", name)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	steps, err := goldenFlowSteps(name, src)
	if err != nil {
		t.Fatal(err)
	}
	return steps
}

// Golden files teach agents the flow DSL; a step that does not decode teaches
// them something the compiler rejects.
func TestGoldenFilesDecode(t *testing.T) {
	for _, name := range []string{"GOLDEN_EXAMPLES.cue", "GOLDEN_ACTIONS_REFERENCE.cue"} {
		steps := readGoldenSteps(t, name)
		if len(steps) == 0 {
			t.Fatalf("%s has no flow steps", name)
		}
		for _, s := range steps {
			if _, err := flowir.DecodeSteps([]normalizer.FlowStep{s.Step}); err != nil {
				t.Errorf("%s:%d %s: %v", name, s.Line, s.Step.Action, err)
			}
		}
	}
}

func TestGoldenActionsReferenceInSync(t *testing.T) {
	covered := map[string]bool{}
	for _, s := range readGoldenSteps(t, "GOLDEN_EXAMPLES.cue") {
		covered[s.Step.Action] = true
	}
	want := renderGoldenActionsReference(covered)
	path := filepath.Join("..", "..", "cue", "GOLDEN_ACTIONS_REFERENCE.cue")
	if os.Getenv("ANG_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatal("cue/GOLDEN_ACTIONS_REFERENCE.cue is out of date; regenerate with ANG_UPDATE_GOLDEN=1 go test ./cmd/ang -run TestGoldenActionsReferenceInSync")
	}
}
