package emitter

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/flowsem"
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

// TestActionCatalogEmitterParityReverse ensures emitter does not silently support
// actions that are absent from flowsem catalog.
func TestActionCatalogEmitterParityReverse(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	supported := parseFlowActionSupported(t, filepath.Join(root, "compiler", "emitter", "service_flow_codegen.go"))

	catalogSet := make(map[string]struct{}, 256)
	for _, entry := range flowsem.ActionCatalog() {
		catalogSet[entry.Name] = struct{}{}
	}

	var extras []string
	for action := range supported {
		if _, ok := catalogSet[action]; ok {
			continue
		}
		extras = append(extras, action)
	}
	sort.Strings(extras)
	if len(extras) > 0 {
		t.Fatalf("emitter/catalog mismatch; actions supported by emitter but missing in flowsem catalog: %s", strings.Join(extras, ", "))
	}
}

// TestActionCatalogFlowStepParity ensures flowsem catalog is aligned with the
// public DSL surface (#FlowStep union).
func TestActionCatalogFlowStepParity(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	unionActions := parseFlowStepUnionActions(t, filepath.Join(root, "cue", "schema", "types.cue"))

	catalogSet := make(map[string]struct{}, 256)
	for _, entry := range flowsem.ActionCatalog() {
		catalogSet[entry.Name] = struct{}{}
	}

	var missingInUnion []string
	for action := range catalogSet {
		if _, ok := unionActions[action]; !ok {
			missingInUnion = append(missingInUnion, action)
		}
	}
	sort.Strings(missingInUnion)
	if len(missingInUnion) > 0 {
		t.Fatalf("catalog/#FlowStep mismatch; actions in flowsem catalog but not reachable via #FlowStep: %s", strings.Join(missingInUnion, ", "))
	}

	var missingInCatalog []string
	for action := range unionActions {
		if _, ok := catalogSet[action]; !ok {
			missingInCatalog = append(missingInCatalog, action)
		}
	}
	sort.Strings(missingInCatalog)
	if len(missingInCatalog) > 0 {
		t.Fatalf("#FlowStep/catalog mismatch; actions in #FlowStep but missing in flowsem catalog: %s", strings.Join(missingInCatalog, ", "))
	}
}
