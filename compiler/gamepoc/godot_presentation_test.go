package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
)

func TestGodotRenderOnlySDKKeepsServerGrid(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "client"), 0755); err != nil {
		t.Fatal(err)
	}
	level := locationsdk.RuntimeLevel{ID: "author.view", UnitMM: 500, Width: 4, Depth: 4, Terrain: []string{"..##", "..##", "....", "...."}, StructuralHash: strings.Repeat("a", 64)}
	c := Compiled{GodotTargets: []string{"client"}, GodotScenes: []GodotScene{{LevelID: "room", Scene: "res://finish.tscn", LayoutID: level.ID}}, Walkable: map[string]bool{".": true}, Life: json.RawMessage(`{"locations":{"room":{"terrain":[".#",".."],"cell_mm":1000}},"edges":[]}`), LocationSDK: &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{level}}}
	before := string(c.Life)
	if err := writeGodotLocations(root, c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "client/generated/locations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index map[string]godotLocation
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	row := index["room"]
	if row.Kind != "authored_sdk" || row.UnitMM != 1000 || row.LayoutID != "room" || row.PresentationLayoutID != level.ID || row.Navigation != nil || !reflect.DeepEqual(row.Terrain, []string{".#", ".."}) || before != string(c.Life) {
		t.Fatalf("presentation changed server grid: %+v", row)
	}
	c.LocationSDK.Levels[0].Terrain[0] = "...#"
	if err := writeGodotLocations(root, c); err == nil || !strings.Contains(err.Error(), "server cell (1,0)") {
		t.Fatal("expanded art floor accepted", err)
	}
}

func TestGodotRenderOnlySDKRejectsMetricDrift(t *testing.T) {
	valid := locationsdk.RuntimeLevel{UnitMM: 500, Width: 2, Depth: 2, Terrain: []string{"..", ".."}, Elevation: 36}
	row := godotLocation{UnitMM: 1000, ElevationMM: 18000, Terrain: []string{"."}}
	for name, mutate := range map[string]func(*locationsdk.RuntimeLevel){
		"units":                 func(l *locationsdk.RuntimeLevel) { l.UnitMM = 600 },
		"dimensions":            func(l *locationsdk.RuntimeLevel) { l.Width = 3 },
		"elevation":             func(l *locationsdk.RuntimeLevel) { l.Elevation = 35 },
		"blocked source square": func(l *locationsdk.RuntimeLevel) { l.Terrain = []string{".#", ".."} },
		"ragged":                func(l *locationsdk.RuntimeLevel) { l.Terrain = []string{".", ".."} },
	} {
		t.Run(name, func(t *testing.T) {
			v := valid
			mutate(&v)
			if validateGodotPresentationFloor(v, row, map[string]bool{".": true}) == nil {
				t.Fatal("metric drift accepted")
			}
		})
	}
	if err := validateGodotPresentationFloor(valid, row, map[string]bool{".": true}); err != nil {
		t.Fatal(err)
	}
}
