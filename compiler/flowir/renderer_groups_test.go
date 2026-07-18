package flowir

import (
	"sort"
	"strings"
	"testing"
)

func TestEveryRegisteredActionHasRendererGroup(t *testing.T) {
	t.Parallel()

	var unrouted []string
	for _, spec := range All() {
		if spec.RendererGroup == "" || spec.RendererGroup == RendererGroupUnrouted {
			unrouted = append(unrouted, spec.Name)
		}
	}
	if len(unrouted) > 0 {
		sort.Strings(unrouted)
		t.Fatalf("registered actions without renderer group: %s", strings.Join(unrouted, ", "))
	}
}
