package godot

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func concretePolicyFixture() *Finishing {
	return &Finishing{Architecture: &Architecture{Setting: "dome_interior", WallCaste: "bronze"}, Materials: map[string]Material{
		"concrete.bronze": {SurfaceKind: "concrete", SurfaceCaste: "bronze", Resource: "res://existing/approved.tres", Provenance: "approved unchanged material"},
		"concrete.silver": {SurfaceKind: "concrete", SurfaceCaste: "silver", Resource: "res://existing/clean.tres", Provenance: "existing concrete"},
		"steel":           {SurfaceKind: "steel", Resource: "res://existing/steel.tres", Provenance: "existing mechanical steel"},
	}, SurfaceRoles: map[string]string{}}
}

func TestArchitectureConcreteCasteDefaultAndReasonedZones(t *testing.T) {
	f := concretePolicyFixture()
	before, _ := json.Marshal(f)
	r, err := ResolveArchitecture(f)
	if err != nil {
		t.Fatal(err)
	}
	if r.SurfaceRoles["wall"] != "concrete.bronze" || r.Materials["concrete.bronze"].Resource != "res://existing/approved.tres" {
		t.Fatal("approved concrete reference changed")
	}
	after, _ := json.Marshal(f)
	if string(before) != string(after) {
		t.Fatal("normalization mutated source material data")
	}
	if _, err = ValidateWallMaterial(f, "a", "concrete.silver"); err == nil {
		t.Fatal("wrong caste wall accepted")
	}
	f.Architecture.WallZones = map[string]WallZone{"a": {Caste: "silver", Reason: "Existing controlled specialist threshold inside Bronze public area"}}
	if _, err = ValidateWallMaterial(f, "a", "concrete.silver"); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateWallMaterial(f, "a", "steel"); err == nil {
		t.Fatal("a caste zone accepted a nonconcrete wall")
	}
	f.Architecture.WallOverrides = map[string]WallOverride{"a": {MaterialRole: "steel", Reason: "Removable specialist cabin partition, not the concrete building enclosure"}}
	if used, err := ValidateWallMaterial(f, "a", "steel"); err != nil || used == nil {
		t.Fatal("written exception not surfaced", err)
	}
	f.Architecture.WallOverrides["a"] = WallOverride{}
	if _, err = ResolveArchitecture(f); err == nil {
		t.Fatal("silent nonconcrete override accepted")
	}
}

func TestArchitectureDrawPreservesNavigationAndRejectsWrongMaterial(t *testing.T) {
	level := locationsdk.CompiledLevel{ID: "room", UnitMM: 500, Width: 4, Depth: 4, StructuralHash: strings.Repeat("a", 64)}
	manifest, err := placement.Build(level, placement.Authoring{})
	if err != nil {
		t.Fatal(err)
	}
	f := concretePolicyFixture()
	f.Architecture.Draws = []ArchitectureDraw{{ID: "side.wall", Kind: "wall", MaterialRole: "concrete.bronze", Position: [3]float64{-.1, 1.2, 1}, Size: [3]float64{.2, 2.4, 2}, Reason: "Known side enclosure outside existing floor"}}
	f.ObjectRoles = map[string]string{"side.wall": "concrete.bronze"}
	before, _ := json.Marshal(level)
	scene, err := Render(level, manifest, Options{Finishing: f})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(scene), `metadata/source_id = "side.wall"`) || !strings.Contains(string(scene), `ANG_ROLE__concrete.bronze`) {
		t.Fatal("source identity or material role binding lost")
	}
	// Camera cutaway v1: the draw carries its structure kind, which the client matches against the level's occluder kinds.
	if !strings.Contains(string(scene), "metadata/source_id = \"side.wall\"\nmetadata/structure_kind = \"wall\"") {
		t.Fatal("architecture draw lost its structure kind")
	}
	after, _ := json.Marshal(level)
	if string(before) != string(after) {
		t.Fatal("architecture changed source navigation")
	}
	f.Architecture.Draws[0].MaterialRole = "steel"
	f.ObjectRoles["side.wall"] = "steel"
	if _, err = Render(level, manifest, Options{Finishing: f}); err == nil {
		t.Fatal("nonconcrete architectural draw accepted")
	}
}

func TestArchitectureNearCutKeepsFullSolidAndOtherLocationsOptIn(t *testing.T) {
	for _, id := range []string{"hub.track.wall", "second.station.wall"} {
		level := locationsdk.CompiledLevel{ID: id, UnitMM: 500, Width: 4, Depth: 4, StructuralHash: strings.Repeat("a", 64)}
		manifest, err := placement.Build(level, placement.Authoring{})
		if err != nil {
			t.Fatal(err)
		}
		f := concretePolicyFixture()
		f.Architecture.Draws = []ArchitectureDraw{{ID: id, Kind: "wall", MaterialRole: "concrete.bronze", Position: [3]float64{1, 21, 30.2}, Size: [3]float64{64, 6, .4}, Reason: "Full enclosure", NearCut: &ArchitectureNearCut{HeightM: 1.2, Reason: "Camera-facing wall"}}}
		scene, err := Render(level, manifest, Options{Finishing: f})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(scene), "position = Vector3(1.000000000000, 18.600000000000, -30.200000000000)") || !strings.Contains(string(scene), "scale = Vector3(64.000000000000, 1.200000000000, 0.400000000000)") {
			t.Fatal("camera skin was not cut at 1.2 m above its foot")
		}
		solids, err := ResolveProceduralSolids(level, manifest, f)
		if err != nil {
			t.Fatal(err)
		}
		if len(solids) != 1 || solids[0].Min[2] != 18 || solids[0].Max[2] != 24 {
			t.Fatalf("camera cut weakened full geometry guard: %+v", solids)
		}
		// A tall wall may retain a higher camera skin for mounted engineering.
		// The bound is its actual full height, not an unrelated absolute3m cap.
		f.Architecture.Draws[0].NearCut = &ArchitectureNearCut{HeightM: 4, Reason: "Keep wall-mounted lamp and consoles supported"}
		tall, err := Render(level, manifest, Options{Finishing: f})
		if err != nil || !strings.Contains(string(tall), "scale = Vector3(64.000000000000, 4.000000000000, 0.400000000000)") {
			t.Fatal("valid high camera skin rejected", err)
		}
		solids, err = ResolveProceduralSolids(level, manifest, f)
		if err != nil || len(solids) != 1 || solids[0].Min[2] != 18 || solids[0].Max[2] != 24 {
			t.Fatalf("high camera skin weakened full geometry: %+v, %v", solids, err)
		}
		f.Architecture.Draws[0].NearCut = nil
		uncut, err := Render(level, manifest, Options{Finishing: f})
		if err != nil || !strings.Contains(string(uncut), "scale = Vector3(64.000000000000, 6.000000000000, 0.400000000000)") {
			t.Fatal("unselected wall was cut", err)
		}
		f.Architecture.Draws[0].NearCut = &ArchitectureNearCut{HeightM: 6, Reason: "Invalid"}
		if _, err = Render(level, manifest, Options{Finishing: f}); err == nil {
			t.Fatal("invalid cut accepted")
		}
	}
}

func TestConcreteFamilyUsesExistingMapsAndBoundedMasks(t *testing.T) {
	f := concretePolicyFixture()
	f.Materials["concrete.silver"] = Material{SurfaceKind: "concrete", SurfaceCaste: "silver", Provenance: "existing maps, controlled finish", Concrete: &ConcreteMaterial{Albedo: "res://a.jpg", Normal: "res://n.jpg", Roughness: "res://r.jpg", Tint: [3]float64{.6, .6, .6}, TextureScale: .5, RoughnessBase: .36, PoreContrast: .3, NormalStrength: .2, ProtocolStripe: .8}}
	header, _, _, _, _, err := f.render(placement.Manifest{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{`type="ShaderMaterial"`, `path="res://a.jpg"`, `shader_parameter/protocol_stripe = 0.800000000000`} {
		if !strings.Contains(header, token) {
			t.Fatal("missing concrete parameter", token)
		}
	}
	f.Materials["concrete.silver"].Concrete.Water = 1.01
	if _, _, _, _, _, err = f.render(placement.Manifest{}, nil); err == nil {
		t.Fatal("invalid wear parameter accepted")
	}
}
