package gamepoc

import (
	"encoding/json"
	"github.com/strogmv/ang/compiler/locationsdk"
	"strings"
	"testing"
)

func TestModularLocationMaterializesOnlyTheBoundTerrain(t *testing.T) {
	raw := json.RawMessage(`{"actors":[{"id":"same-person"}],"edges":[{"id":"same-link"}],"locations":{"normal":{"name":"shop","markers":[],"modular_layout":"geometry"},"other":{"terrain":[".."]}}}`)
	bundle := &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{{ID: "geometry", Width: 3, Depth: 2, InitialLayoutKnown: true, Terrain: []string{"#..", "..."}}}}
	resolved, bindings, err := compileModularLocations(raw, bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 || bindings[0].LevelID != "normal" || bindings[0].LayoutID != "geometry" {
		t.Fatal(bindings)
	}
	var got map[string]json.RawMessage
	_ = json.Unmarshal(resolved, &got)
	if string(got["actors"]) != `[{"id":"same-person"}]` || string(got["edges"]) != `[{"id":"same-link"}]` {
		t.Fatal("modular binding erased story state")
	}
	var loc map[string]map[string]json.RawMessage
	_ = json.Unmarshal(got["locations"], &loc)
	if string(loc["normal"]["terrain"]) != `["#..","..."]` || string(loc["other"]["terrain"]) != `[".."]` {
		t.Fatal(string(resolved))
	}
	if _, ok := loc["normal"]["modular_layout"]; ok {
		t.Fatal("unresolved authoring field leaked into runtime")
	}
}
func TestModularLocationRejectsAmbiguousAndUnknownGeometry(t *testing.T) {
	bundle := &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{{ID: "g", Depth: 1, InitialLayoutKnown: true, Terrain: []string{"."}}}}
	for _, text := range []string{`{"locations":{"x":{"modular_layout":"missing"}}}`, `{"locations":{"x":{"modular_layout":"g","terrain":["."]}}}`, `{"locations":{"x":{"modular_layout":"g","initial_layout_known":false}}}`, `{"locations":{"x":{"modular_layout":"g"},"y":{"modular_layout":"g"}}}`} {
		if _, _, err := compileModularLocations(json.RawMessage(text), bundle); err == nil || !strings.Contains(err.Error(), "modular") {
			t.Fatal(text, err)
		}
	}
}

func TestModularPopulationUsesCompiledPublicIdleFloor(t *testing.T) {
	raw := json.RawMessage(`{"actors":[{"steps":[{"level":"s","at":{"x":0,"y":1}}]}],"edges":[{"from":"s","exit":{"x":2,"y":0}}],"population":{"anchors":{}},"locations":{"s":{"modular_layout":"g"}}}`)
	bundle := &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{{ID: "g", Depth: 2, InitialLayoutKnown: true, Terrain: []string{"#..", "..."}, Clearances: []locationsdk.CompiledClearance{{Kind: "maintenance", Cells: []locationsdk.Cell{{X: 1, Y: 0}}}}, ReservedPaths: []locationsdk.RuntimePath{{Cells: []locationsdk.Cell{{X: 2, Y: 1}}}}}}}
	got, _, err := compileModularLocations(raw, bundle)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Population struct{ Anchors map[string][]Point }
	}
	if err = json.Unmarshal(got, &result); err != nil {
		t.Fatal(err)
	}
	a := result.Population.Anchors["s"]
	if len(a) != 1 || a[0] != (Point{X: 1, Y: 1}) {
		t.Fatalf("idle population occupied a staff/circulation/post cell: %+v", a)
	}
}
