package emitter

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var (
	flowActionFuncRe = regexp.MustCompile(`func flowActionSupported\(action string\) bool \{(?s)(.*?)\n\}`)
	actionStringRe   = regexp.MustCompile(`"([a-zA-Z0-9_.]+)"`)
	schemaActionRe   = regexp.MustCompile(`action:\s*("[^"]+"(?:\s*\|\s*"[^"]+")*)`)
)

// These actions are supported in emitter/flowsem but still intentionally schema-loose.
// Keep this list explicit so any NEW mismatch fails immediately.
var schemaParityAllowlist = map[string]struct{}{}

func TestFlowActionSupportedSchemaParity(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	supported := parseFlowActionSupported(t, filepath.Join(root, "compiler", "emitter", "service_flow_codegen.go"))
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
			stale = append(stale, action+" (not in flowActionSupported)")
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

func parseFlowActionSupported(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	m := flowActionFuncRe.FindStringSubmatch(string(b))
	if len(m) < 2 {
		t.Fatalf("could not locate flowActionSupported in %s", path)
	}
	body := m[1]
	out := make(map[string]struct{}, 128)
	for _, match := range actionStringRe.FindAllStringSubmatch(body, -1) {
		if len(match) < 2 {
			continue
		}
		out[match[1]] = struct{}{}
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
