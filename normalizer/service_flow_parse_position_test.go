package normalizer

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// Every parsed step must carry a position: warnings point at it, and
// //ang:nolint above the step is looked up from it. A step whose func string
// interpolates a definition is assembled by evaluation; the action field keeps
// the source position when the step value itself does not.
func TestRawParseFlowStepsKeepsPositionForInterpolatedStep(t *testing.T) {
	src := `
#Parse: "parsed := strings.TrimSpace(raw)"

flow: [
	{ action: "str.Normalize", input: "req.Name", output: "name" },
	{
		action: "logic.Call"
		func: """
			(func(raw string) (string, error) {
				\(#Parse)
				return parsed, nil
			})
			"""
		args: ["req.Title"]
		output: "title"
	},
]
`
	val := cuecontext.New().CompileString(src, cue.Filename("cue/api/impl_positions.cue"))
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	steps, err := New().rawParseFlowSteps(val.LookupPath(cue.ParsePath("flow")))
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(steps))
	}
	for i, step := range steps {
		if step.File == "" || step.Line == 0 {
			t.Fatalf("step %d (%s) has no position: file=%q line=%d", i, step.Action, step.File, step.Line)
		}
	}
	if steps[1].Line < 6 {
		t.Fatalf("logic.Call step line = %d, want the line of its struct (6) or its action field (7)", steps[1].Line)
	}
}
