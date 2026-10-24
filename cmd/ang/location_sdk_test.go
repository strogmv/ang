package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
)

func locationSDKTestInput() locationsdk.Input {
	return locationsdk.Input{
		Catalog: locationsdk.Catalog{
			UnitMM: 500,
			Geometries: []locationsdk.Geometry{{ID: "counter.geometry", Width: 1, Depth: 1, Height: 2,
				Footprint: []locationsdk.Cell{{X: 0, Y: 0}}, Blocked: []locationsdk.Cell{{X: 0, Y: 0}},
				UseClearance: []locationsdk.Cell{{X: 0, Y: 1}}, MaintenanceClearance: []locationsdk.Cell{{X: 1, Y: 0}}}},
			Modules: []locationsdk.Module{{ID: "counter", GeometryID: "counter.geometry", Variants: []locationsdk.Variant{{ID: "plain", GeometryID: "counter.geometry"}}}},
			Times:   []locationsdk.TimeState{{ID: "day", Brightness: 1}, {ID: "night", Brightness: .6}},
			Events:  []locationsdk.EventState{{ID: "ordinary"}, {ID: "holiday", AccentColor: "#bbaa77"}},
		},
		Layouts: []locationsdk.Layout{{ID: "clinic", UnitMM: 500, Width: 8, Depth: 6, Grounded: true,
			Floor:     []locationsdk.FloorArea{{ID: "room", Rect: locationsdk.Rect{W: 8, H: 6}, Reason: "grounded clinic floor"}},
			Entries:   []locationsdk.Cell{{X: 0, Y: 0}},
			Instances: []locationsdk.Instance{{ID: "counter.01", ModuleID: "counter", At: locationsdk.Cell{X: 3, Y: 2}, Reason: "service counter with a clear front and maintenance side"}},
		}},
	}
}

func writeSDKTestCUE(t *testing.T, dir string, input locationsdk.Input) {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "layout.cue"), append([]byte("package fixture\nsdk: "), data...), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLocationSDKDeterministicOutputAndInvalidEditPreservesPreview(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "inspection")
	input := locationSDKTestInput()
	writeSDKTestCUE(t, dir, input)
	var stdout, stderr bytes.Buffer
	if err := runLocationSDK([]string{"-out", out, dir}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(out, "compiled.json")
	first, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = runLocationSDK([]string{"-out", out, dir}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(jsonPath)
	if !bytes.Equal(first, second) {
		t.Fatal("identical source produced a different SDK bundle")
	}
	html, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil || bytes.Contains(html, []byte("__LOCATION_SDK_DATA__")) {
		t.Fatalf("preview was not assembled: %v", err)
	}
	// An invalid later layout cannot leave the earlier layout published alone.
	broken := input.Layouts[0]
	broken.ID = "second.invalid"
	broken.Instances = []locationsdk.Instance{{ID: "bad", ModuleID: "missing.module", Reason: "invalid edit"}}
	input.Layouts = append(input.Layouts, broken)
	writeSDKTestCUE(t, dir, input)
	if err = runLocationSDK([]string{"-out", out, dir}, &stdout, &stderr); err == nil {
		t.Fatal("unknown module accepted")
	}
	after, _ := os.ReadFile(jsonPath)
	afterHTML, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if !bytes.Equal(first, after) || !bytes.Equal(html, afterHTML) {
		t.Fatal("invalid source replaced valid outputs")
	}
}

func TestLocationSDKCheckDoesNotWriteAndEscapesAuthoredText(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "absent")
	input := locationSDKTestInput()
	input.Layouts[0].Instances[0].Reason = "</script><script>throw Error('injected')</script>"
	writeSDKTestCUE(t, dir, input)
	var stdout, stderr bytes.Buffer
	if err := runLocationSDK([]string{"-check", "-out", out, dir}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("check created output")
	}
	if err := runLocationSDK([]string{"-out", out, dir}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	html, _ := os.ReadFile(filepath.Join(out, "index.html"))
	if strings.Contains(string(html), "</script><script>throw") {
		t.Fatal("authored reason became executable HTML")
	}
	var result locationSDKBundle
	data, _ := os.ReadFile(filepath.Join(out, "compiled.json"))
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.RuntimeConnected {
		t.Fatal("authoring output claimed runtime integration")
	}
	if result.Levels[0].Instances[0].Reason != input.Layouts[0].Instances[0].Reason {
		t.Fatal("inspection lost original authored reason")
	}
}
