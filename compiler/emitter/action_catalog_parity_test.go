package emitter

import (
	"sort"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/flowsem"
)

// TestActionCatalogEmitterParity ensures that every action present in flowsem
// catalog is recognized as renderable by emitter dispatcher.
func TestActionCatalogEmitterParity(t *testing.T) {
	t.Parallel()

	catalog := flowsem.ActionCatalog()
	missing := make([]string, 0)
	for _, entry := range catalog {
		if !flowActionSupported(entry.Name) {
			missing = append(missing, entry.Name)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("catalog/emitter mismatch; unsupported actions: %s", strings.Join(missing, ", "))
	}
}
