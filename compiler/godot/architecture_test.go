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
