package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/placement"
)

func TestGodotMarketMatchesIndependentOracle(t *testing.T) {
	input := filepath.Join("..", "..", "examples", "godot", "cue", "market")
	out := t.TempDir()
	args := []string{"-layout", "example.market_hub", "-models", filepath.Join(input, "models.json"), "-out", out, input}
	var stdout, stderr bytes.Buffer
	if err := runGodot(args, &stdout, &stderr); err != nil {
		t.Fatalf("%v\n%s", err, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(out, "placements.json"))
	if err != nil {
		t.Fatal(err)
	}
	var actual placement.Manifest
	if err = json.Unmarshal(data, &actual); err != nil {
		t.Fatal(err)
	}
	oracleData, err := os.ReadFile(filepath.Join("..", "..", "compiler", "placement", "testdata", "market-oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		StructuralHash string `json:"structural_hash"`
		Instances      map[string]struct {
			Position placement.Vec3    `json:"position"`
			Basis    [3]placement.Vec3 `json:"basis_columns"`
			ModelRef string            `json:"model_ref"`
		} `json:"instances"`
	}
	if err = json.Unmarshal(oracleData, &oracle); err != nil {
		t.Fatal(err)
	}
	if len(actual.Instances) != 215 || len(actual.Instances) != len(oracle.Instances) || actual.StructuralHash != oracle.StructuralHash {
		t.Fatal("oracle composition or structural hash changed")
	}
	compiledData, err := os.ReadFile(filepath.Join(out, "compiled.json"))
	if err != nil {
		t.Fatal(err)
	}
	var compiled any
	if err = json.Unmarshal(compiledData, &compiled); err != nil {
		t.Fatal(err)
	}
	root := compiled.(map[string]any)["levels"].([]any)[0]
	for id, want := range oracle.Instances {
		got, ok := actual.Instances[id]
		if !ok {
			t.Fatalf("missing oracle ID %s", id)
		}
		if got.ModelRef != want.ModelRef {
			t.Errorf("%s model_ref: got %s, want %s", id, got.ModelRef, want.ModelRef)
		}
		basis := got.WorldTransform.BasisColumns()
		for j := range want.Position {
			if math.Abs(got.WorldTransform.Position[j]-want.Position[j]) > 1e-9 {
				t.Errorf("%s position %v != oracle %v", id, got.WorldTransform.Position, want.Position)
			}
			for k := range want.Basis[j] {
				if math.Abs(basis[j][k]-want.Basis[j][k]) > 1e-9 {
					t.Errorf("%s basis %v != oracle %v", id, basis, want.Basis)
				}
			}
		}
		// Every reference must resolve in the emitted compiled level.
		for _, ref := range append(append([]string{}, got.NavRefs...), got.ClearanceRefs...) {
			if !godotTestPointerExists(root, strings.TrimPrefix(ref, "#")) {
				t.Errorf("%s unresolved reference %s", id, ref)
			}
		}
	}
	if bytes.Contains(data, []byte("override_reason")) || bytes.Contains(data, []byte("source_transform")) {
		t.Fatal("visual-only pose leaked into manifest")
	}
	if len(actual.Instances["island.C1"].Bindings.Storefronts) != 5 || len(actual.Instances["crane.bridge"].Bindings.Lights) != 2 || len(actual.Instances["gate.entry"].Bindings.Gates) != 1 {
		t.Fatal("authored bindings lost")
	}
	if bytes.Contains(data, []byte("prefab_guid")) || bytes.Contains(data, []byte("res://")) {
		t.Fatal("client resources leaked into neutral manifest")
	}
	firstScene, err := os.ReadFile(filepath.Join(out, "location.tscn"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(firstScene), "[ext_resource ") != 58 || strings.Count(string(firstScene), " instance=ExtResource(") != 70 {
		t.Fatal("oracle kit composition changed")
	}
	if err = runGodot(args, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	secondScene, _ := os.ReadFile(filepath.Join(out, "location.tscn"))
	secondManifest, _ := os.ReadFile(filepath.Join(out, "placements.json"))
	if !bytes.Equal(firstScene, secondScene) || !bytes.Equal(data, secondManifest) {
		t.Fatal("regeneration changed output")
	}
}

func godotTestPointerExists(value any, ptr string) bool {
	for _, part := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		switch v := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = v[part]
			if !ok {
				return false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(v) {
				return false
			}
			value = v[index]
		default:
			return false
		}
	}
	return true
}

func TestGodotCUEEditAndFailurePreservesOutput(t *testing.T) {
	dir, out := t.TempDir(), t.TempDir()
	input := locationSDKTestInput()
	writeSDKTestCUE(t, dir, input)
	modelPath := filepath.Join(dir, "models.json")
	if err := os.WriteFile(modelPath, []byte(`{"counter":"res://kit/counter.glb"}`), 0644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"-layout", "clinic", "-models", modelPath, "-out", out, dir}
	if err := runGodot(args, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(out, "placements.json"))
	input.Layouts[0].Instances[0].At.X++
	writeSDKTestCUE(t, dir, input)
	if err := runGodot(args, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(out, "placements.json"))
	if bytes.Equal(first, second) {
		t.Fatal("CUE move did not regenerate placement")
	}
	if err := os.WriteFile(modelPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runGodot(args, &stdout, &stderr); err == nil {
		t.Fatal("missing client model accepted")
	}
	after, _ := os.ReadFile(filepath.Join(out, "placements.json"))
	if !bytes.Equal(second, after) {
		t.Fatal("invalid model mapping replaced valid output")
	}
	checkOut := filepath.Join(dir, "absent")
	if err := os.WriteFile(modelPath, []byte(`{"counter":"res://kit/counter.glb"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := runGodot([]string{"-check", "-layout", "clinic", "-models", modelPath, "-out", checkOut, dir}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(checkOut); !os.IsNotExist(err) {
		t.Fatal("check wrote files")
	}
}

// Importing authoring must observe edits in the source package on every run.
func TestGodotLiveImportsAndRejectsVisualPositionOverrides(t *testing.T) {
	dir, out := t.TempDir(), t.TempDir()
	for _, rel := range []string{"cue.mod", "source", "adapter"} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("cue.mod/module.cue", "module: \"live.test/game@v0\"\nlanguage: version: \"v0.17.0\"\n")
	write("adapter/main.cue", "package adapter\nimport source \"live.test/game/source:fixture\"\nsdk: source.sdk\n")
	input := locationSDKTestInput()
	writeSDKTestCUE(t, filepath.Join(dir, "source"), input)
	write("models.json", `{"counter":"res://kit/counter.glb"}`)
	var stdout, stderr bytes.Buffer
	args := []string{"-layout", "clinic", "-models", filepath.Join(dir, "models.json"), "-out", out, filepath.Join(dir, "adapter")}
	if err := runGodot(args, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(out, "placements.json"))
	input.Layouts[0].Instances[0].At.X++
	writeSDKTestCUE(t, filepath.Join(dir, "source"), input)
	if err := runGodot(args, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(out, "placements.json"))
	if bytes.Equal(first, second) {
		t.Fatal("imported live layout edit was frozen")
	}
	write("adapter/placement.cue", `package adapter
placement: instances: desk: pose_override: {
 position_plan_m: [99,0,99], rotation_degrees: 0, reason: "visual shortcut"
}
`)
	if err := runGodot(args, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "pose_override") {
		t.Fatalf("visual-only position override was not rejected: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(out, "placements.json"))
	if !bytes.Equal(second, after) {
		t.Fatal("rejected override replaced valid output")
	}
}
