package locationintent

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/godot"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func load(t *testing.T) (map[string]Intent, locationsdk.Catalog, Defaults) {
	t.Helper()
	read := func(name string, out any) {
		raw, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		d := json.NewDecoder(strings.NewReader(string(raw)))
		d.DisallowUnknownFields()
		if err = d.Decode(out); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	var intents map[string]Intent
	var catalog locationsdk.Catalog
	var defaults Defaults
	read("locations.json", &intents)
	read("catalog.json", &catalog)
	read("defaults.json", &defaults)
	return intents, catalog, defaults
}

// compile runs the derived level through the real SDK compiler, the placement
// manifest and the Godot scene emitter: the same path as a hand-written layout.
func compile(t *testing.T, d *Derived) (locationsdk.CompiledLevel, []byte) {
	t.Helper()
	level, err := locationsdk.Build(d.Catalog, d.Layout)
	if err != nil {
		if v, ok := err.(*locationsdk.ValidationError); ok {
			t.Fatalf("SDK build: %+v", v.Diagnostics)
		}
		t.Fatalf("SDK build: %v", err)
	}
	manifest, err := placement.Build(level, d.Authoring)
	if err != nil {
		t.Fatalf("placement: %v", err)
	}
	scene, err := godot.Render(level, manifest, godot.Options{Finishing: d.Finishing, Models: d.Models, Camera: d.Authoring.Camera})
	if err != nil {
		t.Fatalf("scene: %v", err)
	}
	return level, scene
}

func TestCopperDockDerivesAndCompiles(t *testing.T) {
	intents, catalog, defaults := load(t)
	d, diags := Derive(intents["level.c7_copper_dock"], catalog, defaults)
	if d == nil || len(diags) != 0 {
		t.Fatalf("dock: %+v", diags)
	}
	level, scene := compile(t, d)
	if level.Width != 66 || level.Depth != 46 || len(level.Instances) != 4 {
		t.Fatalf("size %dx%d instances %d", level.Width, level.Depth, len(level.Instances))
	}
	if len(d.Exits) != 2 || len(d.Layout.Entries) != 2 || len(d.World.LifeEdges) != 4 {
		t.Fatalf("exits %d entries %d edges %d", len(d.Exits), len(d.Layout.Entries), len(d.World.LifeEdges))
	}
	if len(d.Missing) != 1 || d.Missing[0].ID != "kit.geo.dock.scale" || len(d.Greybox) != 3 || d.Models["kit.geo.counter"] == "" {
		// The manifest desk is a real block now (the market workbench bound to
		// its passport ID); scales, cart dock and bench are still greyboxes.
		t.Fatalf("missing %+v greybox %v", d.Missing, d.Greybox)
	}
	// The installed dock's identities survive: plan ID, region node, edge IDs,
	// the closed ward exit with its server reason, the deck pair.
	if d.Layout.ID != "example.c7_copper_dock" || d.World.RegionNode.ID != "hd.copper_dock" || d.World.NameKey != "ui.deck.dock.name" {
		t.Fatalf("identities: %s %s %s", d.Layout.ID, d.World.RegionNode.ID, d.World.NameKey)
	}
	closed := map[string]string{}
	for _, e := range d.World.LifeEdges {
		closed[e.ID] = e.ClosedReasonKey
	}
	if closed["dock_copper_walk"] != "error.deck.ward_unbuilt" || closed["copper_dock_walk"] != "error.deck.ward_unbuilt" || closed["hub_copper_deck"] != "" || len(closed) != 4 {
		t.Fatalf("edges %+v", closed)
	}
	if d.Layout.Interior == nil || d.Layout.Setting != "dome_interior" || len(d.Layout.Obstacles) != 0 {
		t.Fatal("interior envelope missing or walls are still obstacles")
	}
	cut, capped, leaf := 0, 0, false
	for _, w := range d.Finishing.Architecture.Draws {
		if w.NearCut != nil {
			cut++
		}
		if strings.Contains(w.ID, ".mass.") {
			capped++
		}
		leaf = leaf || strings.HasSuffix(w.ID, ".exit.ward.leaf")
	}
	if cut == 0 || capped != 2 || !leaf {
		t.Fatalf("near-cut walls %d, capped masses %d, closed leaf %t", cut, capped, leaf)
	}
	if len(d.Lamps) < 12 || len(d.World.Markers) != 1 || len(d.World.Inspections) != 1 || len(d.World.People) != 1 {
		t.Fatalf("lamps %d markers %d inspections %d people %d", len(d.Lamps), len(d.World.Markers), len(d.World.Inspections), len(d.World.People))
	}
	for _, want := range []string{"scale.greybox", "concrete", "OmniLight3D", "fixture.hall_lamp"} {
		if !strings.Contains(string(scene), want) {
			t.Errorf("scene lacks %q", want)
		}
	}
	// The derived approach of every fixture is free floor of the compiled level.
	for _, f := range d.Fixtures {
		if row := level.Terrain[f.Approach.Y]; row[f.Approach.X] != '.' {
			t.Errorf("%s approach (%d,%d) is %q", f.ID, f.Approach.X, f.Approach.Y, row[f.Approach.X])
		}
		// Our rotation agrees with the compiler's.
		for _, i := range level.Instances {
			if i.ID != f.ID {
				continue
			}
			got := map[locationsdk.Cell]bool{}
			for _, c := range i.Blocked {
				got[c] = true
			}
			for _, c := range f.Cells {
				if !got[c] {
					t.Errorf("%s: derived cell %+v is not blocked by the compiler", f.ID, c)
				}
			}
		}
	}
}

// A second location of another caste, with no light rules at all, needs no code.
func TestBronzeRoomReusesEverything(t *testing.T) {
	intents, catalog, defaults := load(t)
	d, diags := Derive(intents["level.intent_bronze_room"], catalog, defaults)
	if d == nil || len(diags) != 0 {
		t.Fatalf("bronze room: %+v", diags)
	}
	_, scene := compile(t, d)
	if d.Finishing.Architecture.WallCaste != "bronze" || d.Finishing.SurfaceRoles["wall"] != "concrete.bronze" {
		t.Fatalf("caste %+v", d.Finishing.SurfaceRoles)
	}
	if len(d.Lamps) < 3 {
		t.Fatalf("caste default light gave %d lamps", len(d.Lamps))
	}
	if !strings.Contains(string(scene), "reader.greybox") {
		t.Error("missing block has no greybox with its ID")
	}
	// Placeholder swap: registering a model under the same ang_id removes the
	// greybox and binds the model, with no other change.
	defaults.Models["kit.geo.station.fare_reader"] = "res://location-kit/kit.geo.station.fare_reader.tscn"
	d, diags = Derive(intents["level.intent_bronze_room"], catalog, defaults)
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	_, scene = compile(t, d)
	if strings.Contains(string(scene), "reader.greybox") || !strings.Contains(string(scene), "kit.geo.station.fare_reader.tscn") {
		t.Error("model with the same ang_id did not replace the greybox")
	}
}

func TestDeterministic(t *testing.T) {
	intents, catalog, defaults := load(t)
	a, _ := Derive(intents["level.c7_copper_dock"], catalog, defaults)
	b, _ := Derive(intents["level.c7_copper_dock"], catalog, defaults)
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	if string(x) != string(y) {
		t.Fatal("two derivations differ")
	}
}

// Every caste has defaults: the same bare room in Silver and Gold compiles and
// carries the hidden decay those castes require.
func TestSilverAndGoldDefaults(t *testing.T) {
	intents, catalog, defaults := load(t)
	for _, tier := range []string{"silver", "gold"} {
		in := intents["level.intent_bronze_room"]
		in.Tier = tier
		d, diags := Derive(in, catalog, defaults)
		if d == nil || len(diags) != 0 {
			t.Fatalf("%s: %+v", tier, diags)
		}
		compile(t, d)
		if d.Finishing.SurfaceRoles["wall"] != "concrete."+tier || d.Finishing.Architecture.Concrete.Caries == nil {
			t.Fatalf("%s: caste defaults incomplete", tier)
		}
	}
}

// A caste that names a wall kit gets its walls from panels, with lamps on the
// panels' embed plates; a caste without one keeps boxes and a flush housing.
func TestWallKitAndHousings(t *testing.T) {
	intents, catalog, defaults := load(t)
	bronze, diags := Derive(intents["level.intent_bronze_room"], catalog, defaults)
	if len(diags) > 0 {
		t.Fatalf("bronze: %+v", diags)
	}
	if bronze.WallPanels == 0 || bronze.WallTriangles == 0 || len(bronze.Finishing.MaterialBindings) != 2 {
		t.Fatalf("bronze walls are not laid from the kit: %d panels, %d triangles", bronze.WallPanels, bronze.WallTriangles)
	}
	conduit, kerbed := false, false
	for _, d := range bronze.Finishing.Architecture.Draws {
		conduit = conduit || strings.Contains(d.ID, ".conduit.")
		kerbed = kerbed || (strings.Contains(d.ID, ".mass.") && d.NearCut != nil && d.NearCut.HeightM < 0.5)
	}
	if !conduit || !kerbed {
		t.Fatalf("conduit=%v, mass by the doorway cut to a kerb=%v", conduit, kerbed)
	}
	compile(t, bronze)
	silver := intents["level.intent_bronze_room"]
	silver.Tier = "silver"
	d, diags := Derive(silver, catalog, defaults)
	if len(diags) > 0 {
		t.Fatalf("silver: %+v", diags)
	}
	frames := 0
	for _, x := range d.Finishing.Architecture.Draws {
		if strings.HasSuffix(x.ID, ".frame") {
			frames++
		}
	}
	if d.WallPanels != 0 || frames == 0 {
		t.Fatalf("silver: %d panels, %d housings", d.WallPanels, frames)
	}
	compile(t, d)
}
