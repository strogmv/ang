package emitter

import (
	"reflect"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

// imports: of a logic.Call goes into its own method file and replaces a
// service import of the same name there, not in the other files.
func TestMethodImportsAddsStepImportsToItsFileOnly(t *testing.T) {
	service := map[string]string{"math/rand": "", "strings": ""}
	list := []string{`"math/rand"`, `"strings"`}
	flow := []normalizer.FlowStep{{
		Action: "flow.If",
		Args: map[string]any{"_then": []normalizer.FlowStep{{
			Action: "logic.Call",
			Args:   map[string]any{"func": "(func() {})", "imports": []string{"crypto/rand"}},
		}}},
	}}
	got := methodImports(service, list, flow)
	if want := []string{`"crypto/rand"`, `"strings"`}; !reflect.DeepEqual(got, want) {
		t.Errorf("method imports = %v, want %v", got, want)
	}
	if got := methodImports(service, list, nil); !reflect.DeepEqual(got, list) {
		t.Errorf("a method without step imports = %v, want the service list", got)
	}
	if service["math/rand"] != "" || len(service) != 2 {
		t.Errorf("the service map changed: %v", service)
	}
}
