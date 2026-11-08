package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGodotLocationsAuthoredFloorAndPrivateNeighbours(t *testing.T) {
	root := t.TempDir()
	client := filepath.Join(root, "client")
	if err := os.Mkdir(client, 0755); err != nil {
		t.Fatal(err)
	}
	c := Compiled{GodotTargets: []string{"client"}, Walkable: map[string]bool{".": true}, Life: json.RawMessage(`{"locations":{"a":{"terrain":["###","#.#","###"]},"b":{"terrain":[".."]}},"edges":[{"from":"a","to":"b"}]}`)}
	if err := writeGodotLocations(root, c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(client, "generated/locations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index map[string]godotLocation
	if err = json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	if index["a"].Kind != "bare_floor" || index["a"].UnitMM != 1000 || len(index["a"].Neighbours) != 1 || index["a"].Neighbours[0] != "b" {
		t.Fatal(index)
	}
	scene, err := os.ReadFile(filepath.Join(client, "generated/locations/a/location.tscn"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(scene)
	if strings.Count(text, `type="MeshInstance3D"`) != 1 || !strings.Contains(text, `type="PlaneMesh"`) || !strings.Contains(text, "Vector3(1.500000000000, 0.000000000000, -1.500000000000)") {
		t.Fatal(text)
	}
	if strings.Contains(text, "Heading") || strings.Contains(text, "exit") {
		t.Fatal("shell invented exit presentation")
	}
	c.GodotScenes = []GodotScene{{LevelID: "absent", Scene: "res://finish.tscn"}}
	if writeGodotLocations(root, c) == nil {
		t.Fatal("unknown finishing accepted")
	}
	for _, bad := range []string{"res://../escape.tscn", "res:///root.tscn", "res://a/../b.tscn", "file.tscn", "res://a.glb"} {
		if godotScenePath(bad) {
			t.Fatal("unsafe finishing", bad)
		}
	}
}
