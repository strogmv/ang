package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/flowir"
)

func TestTypedDispatchUsesPredecodedAction(t *testing.T) {
	n := 0
	state := &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
	step := flowir.TypedStep{
		Name:   "uuid.New",
		Action: flowir.UUIDNew{Output: "generatedID"},
	}

	got := renderTypedFlowSteps(state, []flowir.TypedStep{step}, 0)
	if !strings.Contains(got, "generatedID") {
		t.Fatalf("typed action was not used:\n%s", got)
	}
}
