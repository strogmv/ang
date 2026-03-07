package flowsem

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var reActionLiteral = regexp.MustCompile(`action:\s*"([^"]+)"`)

func collectActionsFromCUE(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	matches := reActionLiteral.FindAllStringSubmatch(string(raw), -1)
	out := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		a := strings.TrimSpace(m[1])
		if a == "" {
			continue
		}
		out[a] = struct{}{}
	}
	return out
}

// TestGoldenCatalogCoverage ensures that all actions from flowsem catalog
// have at least one reference in GOLDEN_EXAMPLES or GOLDEN_ACTIONS_REFERENCE.
func TestGoldenCatalogCoverage(t *testing.T) {
	t.Parallel()

	mainPath := filepath.Join("..", "..", "cue", "GOLDEN_EXAMPLES.cue")
	refPath := filepath.Join("..", "..", "cue", "GOLDEN_ACTIONS_REFERENCE.cue")

	covered := collectActionsFromCUE(t, mainPath)
	for a := range collectActionsFromCUE(t, refPath) {
		covered[a] = struct{}{}
	}

	catalog := ActionCatalog()
	missing := make([]string, 0)
	for _, entry := range catalog {
		if _, ok := covered[entry.Name]; !ok {
			missing = append(missing, entry.Name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("golden coverage is incomplete; missing actions: %s", strings.Join(missing, ", "))
	}
}
