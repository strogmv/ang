package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestLoadSidecarFunctionExtractsBodyAndImports(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "impl.go")
	source := `package sidecar
import "strings"
func getCustomerBody() (any, error) {
	resp.Name = strings.TrimSpace(req.Name)
	return resp, nil
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	body, imports, err := loadSidecarFunction(root, filepath.Join(root, "operation.cue"), "impl.go#getCustomerBody")
	if err != nil {
		t.Fatal(err)
	}
	if body != "resp.Name = strings.TrimSpace(req.Name)\n\treturn resp, nil" {
		t.Fatalf("unexpected body: %q", body)
	}
	if len(imports) != 1 || imports[0] != "strings" {
		t.Fatalf("unexpected imports: %#v", imports)
	}
}

func TestLoadSidecarFunctionLiteral(t *testing.T) {
	root := t.TempDir()
	implDir := filepath.Join(root, "cue", "api", "impl")
	if err := os.MkdirAll(implDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(implDir, "helpers.go"), []byte("package impl\n\nfunc normalize(value string) (string, error) { return value, nil }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	literal, _, err := loadSidecarFunctionLiteral(root, "cue/api/ops.cue", "impl/helpers.go#normalize")
	if err != nil {
		t.Fatal(err)
	}
	if literal != "func(value string) (string, error) { return value, nil }" {
		t.Fatalf("literal=%q", literal)
	}
}

func TestResolveLogicCallSidecarsRewritesFunctionAndImports(t *testing.T) {
	root := t.TempDir()
	implDir := filepath.Join(root, "cue", "api", "impl")
	if err := os.MkdirAll(implDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package impl\nimport \"strings\"\nfunc normalize(value string) (string, error) { return strings.TrimSpace(value), nil }\n"
	if err := os.WriteFile(filepath.Join(implDir, "helpers.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	services := []normalizer.Service{{Name: "Customer", Methods: []normalizer.Method{{Name: "Normalize", Flow: []normalizer.FlowStep{{
		Action: "logic.Call", File: "cue/api/ops.cue", Line: 12,
		Args: map[string]any{"funcRef": "impl/helpers.go#normalize", "args": []string{"req.Name"}, "output": "name"},
	}}}}}}
	resolved, err := resolveLogicCallSidecars(root, services)
	if err != nil {
		t.Fatal(err)
	}
	step := resolved[0].Methods[0].Flow[0]
	if !strings.Contains(step.Args["func"].(string), "func(value string)") {
		t.Fatalf("func=%q", step.Args["func"])
	}
	imports, ok := step.Args["_funcRefImports"].([]string)
	if !ok || len(imports) != 1 || imports[0] != "strings" {
		t.Fatalf("imports=%#v", step.Args["_funcRefImports"])
	}
}

func TestResolveLogicCallSidecarsInNestedGroups(t *testing.T) {
	root := t.TempDir()
	implDir := filepath.Join(root, "impl")
	if err := os.MkdirAll(implDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(implDir, "helpers.go"), []byte("package impl\nfunc normalize(value string) string { return value }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := normalizer.FlowStep{Action: "logic.Call", File: filepath.Join(root, "cue", "api.cue"), Args: map[string]any{"funcRef": "../impl/helpers.go#normalize"}}
	services := []normalizer.Service{{Methods: []normalizer.Method{{Flow: []normalizer.FlowStep{{
		Action: "flow.Switch",
		Args:   map[string]any{"_cases": map[string][]normalizer.FlowStep{"one": {nested}}},
	}}}}}}

	resolved, err := resolveLogicCallSidecars(root, services)
	if err != nil {
		t.Fatal(err)
	}
	got := resolved[0].Methods[0].Flow[0].Args["_cases"].(map[string][]normalizer.FlowStep)["one"][0].Args["func"]
	if !strings.Contains(got.(string), "func(value string) string") {
		t.Fatalf("func=%#v", got)
	}
}
