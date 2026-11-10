package godot

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

// The legacy consumer must receive complete CUE resources, including an
// arbitrary non-emissive StandardMaterial role as well as generated concrete.
// Neither the location name nor the role names are taught to the adapter.
func TestFinishingBindingsSubsceneTwoLayoutsAndArbitraryRoles(t *testing.T) {
	for _, fixture := range []struct{ level, tier, wall, paint string }{{"first.station", "bronze", "bronze.local.concrete", "first.tool.paint"}, {"next.dock", "copper", "copper.new.concrete", "another.nonemissive.role"}} {
		level := locationsdk.CompiledLevel{ID: fixture.level, UnitMM: 500, Width: 6, Depth: 6, StructuralHash: strings.Repeat("a", 64), Navigation: &locationsdk.Navigation{BaseSurface: "floor", Surfaces: []locationsdk.NavSurface{{ID: "floor", SlabMM: 250, Slab: []locationsdk.Rect{{W: 6, H: 6}}}}}}
		manifest, err := placement.Build(level, placement.Authoring{})
		if err != nil {
			t.Fatal(err)
		}
		finish := &Finishing{Materials: map[string]Material{
			fixture.wall:       {SurfaceKind: "concrete", SurfaceCaste: fixture.tier, Provenance: "Existing concrete maps resolved for this location.", Concrete: &ConcreteMaterial{Albedo: "res://existing/albedo.jpg", Normal: "res://existing/normal.jpg", Roughness: "res://existing/roughness.jpg", Tint: [3]float64{.6, .5, .4}, TextureScale: .5, RoughnessBase: .6, PoreContrast: .3, NormalStrength: .2}},
			fixture.paint:      {Color: [4]float64{.25, .1, .04, 1}, Roughness: .7, Metallic: .2, Provenance: "An arbitrary CUE-owned plain paint, with no legacy texture/emission exemption."},
			"existing.texture": {Resource: "res://existing/unchanged.tres", Provenance: "An unchanged authored materialresource."},
		}, SurfaceRoles: map[string]string{"floor": "existing.texture", "wall": fixture.wall}, MaterialBindings: map[string]string{"ExistingSourcePaint": fixture.paint}}
		before, _ := json.Marshal(struct {
			Level    locationsdk.CompiledLevel
			Manifest placement.Manifest
			Finish   *Finishing
		}{level, manifest, finish})
		result, err := RenderFinishingBindings(level, manifest, finish)
		if err != nil {
			t.Fatal(err)
		}
		repeat, err := RenderFinishingBindings(level, manifest, finish)
		if err != nil || !bytes.Equal(result, repeat) {
			t.Fatal("nondeterministic materialsubscene", err)
		}
		normal, err := Render(level, manifest, Options{Finishing: finish})
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := ResolveArchitectureForLevel(finish, level)
		if err != nil {
			t.Fatal(err)
		}
		header, bindings, _, _, _, err := resolved.render(manifest, proceduralSurfaces(level, resolved))
		if err != nil {
			t.Fatal(err)
		}
		// Equality with the real common scene prevents a second shader/resource
		// implementation from drifting away from the normal authoring route.
		for _, text := range []string{header, bindings} {
			if !strings.Contains(string(result), text) || !strings.Contains(string(normal), text) {
				t.Fatal("legacy subscene changed common material resources/bindings")
			}
		}
		for _, text := range []string{`[node name="Finish" type="Node"]`, `path="res://location_finish.gd"`, `type="ShaderMaterial"`, `type="StandardMaterial3D"`, `"ANG_ROLE__` + fixture.wall + `":`, `"ANG_ROLE__` + fixture.paint + `":`, `"ExistingSourcePaint":`, `path="res://existing/unchanged.tres"`} {
			if !strings.Contains(string(result), text) {
				t.Fatalf("%s missing CUE role/resource %s", fixture.level, text)
			}
		}
		for _, forbidden := range []string{`type="MeshInstance3D"`, `type="Camera3D"`, `type="OmniLight3D"`, `type="DirectionalLight3D"`} {
			if strings.Contains(string(result), forbidden) {
				t.Fatal("material subscene duplicated location content", forbidden)
			}
		}
		after, _ := json.Marshal(struct {
			Level    locationsdk.CompiledLevel
			Manifest placement.Manifest
			Finish   *Finishing
		}{level, manifest, finish})
		if !bytes.Equal(before, after) {
			t.Fatal("material subscene mutated authoring/navigation/placements")
		}
	}
}

func TestFinishingBindingsEmptySceneAndStrictManifest(t *testing.T) {
	level := locationsdk.CompiledLevel{ID: "empty.location", UnitMM: 500, Width: 4, Depth: 4}
	manifest, err := placement.Build(level, placement.Authoring{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := RenderFinishingBindings(level, manifest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `[node name="Finish" type="Node"]`) || strings.Contains(string(result), `script =`) {
		t.Fatal("empty material catalog needs a valid empty subscene")
	}
	manifest.LayoutID = "another.location"
	if _, err = RenderFinishingBindings(level, manifest, nil); err == nil {
		t.Fatal("material subscene accepted another layout's manifest")
	}
}
