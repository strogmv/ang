package emitter

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/flowir"
)

var (
	schemaActionRe = regexp.MustCompile(`action:\s*("[^"]+"(?:\s*\|\s*"[^"]+")*)`)
)

// These actions are supported in emitter/flowsem but still intentionally schema-loose.
// Keep this list explicit so any NEW mismatch fails immediately.
var schemaParityAllowlist = map[string]struct{}{}

func TestFlowActionSupportedSchemaParity(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	supported := typedFlowActionSet()
	schema := parseSchemaActions(t, filepath.Join(root, "cue", "schema", "types.cue"))

	var missing []string
	for action := range supported {
		if _, ok := schema[action]; ok {
			continue
		}
		if _, ok := schemaParityAllowlist[action]; ok {
			continue
		}
		missing = append(missing, action)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("flowActionSupported/schema mismatch; missing schema actions: %s", strings.Join(missing, ", "))
	}

	var stale []string
	for action := range schemaParityAllowlist {
		if _, ok := supported[action]; !ok {
			stale = append(stale, action+" (not in typed Flow IR)")
			continue
		}
		if _, ok := schema[action]; ok {
			stale = append(stale, action+" (already in schema; remove from allowlist)")
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Fatalf("schema parity allowlist is stale: %s", strings.Join(stale, ", "))
	}
}

func typedFlowActionSet() map[string]struct{} {
	actions := flowir.All()
	out := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		out[action.Name] = struct{}{}
	}
	return out
}

func parseSchemaActions(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	out := make(map[string]struct{}, 128)
	for _, match := range schemaActionRe.FindAllStringSubmatch(string(b), -1) {
		if len(match) < 2 {
			continue
		}
		for _, raw := range strings.Split(match[1], "|") {
			action := strings.Trim(strings.TrimSpace(raw), `"`)
			if action == "" {
				continue
			}
			out[action] = struct{}{}
		}
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// compiler/emitter -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
