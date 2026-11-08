package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
)

// The live CUE package is loaded through an in-memory file overlay. No source
// file or world snapshot is copied/frozen; B replaces only the geometry file.
func TestStationBlenderLocatorsDriveAuthoritativeExitsAnchorsAndPaint(t *testing.T) {
	root := os.Getenv("ANG_STATION_POSE_WORLD")
	if root == "" {
		t.Skip("set ANG_STATION_POSE_WORLD to the live content/world CUE module")
	}
	build := func(pkg string, geometry map[string]any) cue.Value {
		t.Helper()
		cfg := &load.Config{Dir: root}
		if geometry != nil {
			raw, err := json.Marshal(geometry)
			if err != nil {
				t.Fatal(err)
			}
			cfg.Overlay = map[string]load.Source{filepath.Join(root, "station_sdk", "poses.cue"): load.FromBytes(append([]byte("package station_sdk\ngenerated_geometry: "), raw...))}
		}
		instances := load.Instances([]string{pkg}, cfg)
		if len(instances) != 1 {
			t.Fatal("expected one live CUE package")
		}
		value := cuecontext.New().BuildInstance(instances[0])
		if err := value.Err(); err != nil {
			t.Fatal(err)
		}
		return value
	}
	decode := func(value cue.Value, path string) any {
		t.Helper()
		raw, err := value.LookupPath(cue.ParsePath(path)).MarshalJSON()
		if err != nil {
			t.Fatal(path, cueerrors.Details(err, nil))
		}
		var out any
		if err = json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	source := build("./station_sdk", nil)
	if !reflect.DeepEqual(decode(source, "catalog.level"), decode(source, "catalog.source_level")) {
		t.Fatal("A station source changed")
	}
	geometry := decode(source, "generated_geometry").(map[string]any)
	geometry["source_kind"] = "blend"
	untouched := build("./station_sdk", geometry)
	for _, field := range []string{"exits", "anchors", "landmarks"} {
		if !reflect.DeepEqual(decode(source, "catalog.level."+field), decode(untouched, "catalog.level."+field)) {
			t.Fatal("unchanged Blender round trip moved ", field)
		}
	}
	baseline := build(".", nil)
	roundTrip := build(".", geometry)
	for _, path := range []string{"world.life.edges", "world.wayfinding.destinations", "world.life.actors"} {
		if !reflect.DeepEqual(decode(baseline, path), decode(roundTrip, path)) {
			t.Fatal("unchanged Blender locator round trip changed authoritative data", path)
		}
	}
	moved := map[string]bool{"station_home": false, "waiting_safe": false, "paint.station.public_origin": false, "paint.station.public_west": false}
	for _, value := range geometry["locators"].([]any) {
		locator := value.(map[string]any)
		name := locator["name"].(string)
		if _, ok := moved[name]; ok {
			position := locator["position_plan_m"].([]any)
			position[0] = position[0].(float64) + 1
			moved[name] = true
		}
	}
	for id, found := range moved {
		if !found {
			t.Fatal("missing authoring locator", id)
		}
	}
	changed := build(".", geometry)
	edgesA := decode(baseline, "world.life.edges").([]any)
	edgesB := decode(changed, "world.life.edges").([]any)
	if len(edgesA) != len(edgesB) {
		t.Fatal("geometry changed semantic edge count")
	}
	exitChanged := false
	for i, value := range edgesA {
		a := value.(map[string]any)
		b := edgesB[i].(map[string]any)
		if a["id"] == "station_home" {
			expected := a["exit"].(map[string]any)["x"].(float64) + 1
			if b["exit"].(map[string]any)["x"] != expected {
				t.Fatal("Blender exit did not reach authoritative Life edge")
			}
			exitChanged = true
			for key, v := range a {
				if key != "exit" && !reflect.DeepEqual(v, b[key]) {
					t.Fatal("exit movement changed edge semantics", key)
				}
			}
		}
	}
	if !exitChanged {
		t.Fatal("missing actual station_home server edge")
	}
	markerChanged := false
	guideChanged := false
	for _, value := range decode(changed, "world.wayfinding.destinations").([]any) {
		d := value.(map[string]any)
		if d["id"] == "way.station.home" {
			if d["at"].(map[string]any)["x"] != float64(1) {
				t.Fatal("wayfinding kept original exit")
			}
			if d["guide_from"].(map[string]any)["x"] != float64(19) {
				t.Fatal("wayfinding kept original waiting anchor")
			}
			markerChanged = true
			route := d["guide_route"].([]any)
			if route[0].(map[string]any)["x"] != float64(19) || route[1].(map[string]any)["x"] != float64(1.25) {
				t.Fatal("paint kept hand poses instead of imported locators")
			}
			guideChanged = true
		}
	}
	if !markerChanged || !guideChanged {
		t.Fatal("missing actual station marker/paint contribution")
	}
	actorChanged := false
	for _, value := range decode(changed, "world.life.actors").([]any) {
		actor := value.(map[string]any)
		if actor["id"] == "char.poc_repair_worker" {
			for _, row := range actor["steps"].([]any) {
				step := row.(map[string]any)
				if step["level"] == "level.poc_monorail" {
					if step["at"].(map[string]any)["x"] != float64(19) {
						t.Fatal("commute kept original waiting pose")
					}
					actorChanged = true
				}
			}
		}
	}
	if !actorChanged {
		t.Fatal("no authoritative waiting routine checked")
	}
	t.Log("A byte-equivalent; unchanged B locators preserve authoritative data; isolated moved exit, waiting anchor and paint datums reach Life edges, commute routines and wayfinding without changing IDs/modes/keys")
}
