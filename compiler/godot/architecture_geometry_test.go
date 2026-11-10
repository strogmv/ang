package godot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func TestArchitectureGeometrySubsceneSharesMaterialsBoxesAndCameraSkins(t *testing.T) {
	for _, id := range []string{"first.station", "second.copper.dock"} {
		level := locationsdk.CompiledLevel{ID: id, UnitMM: 500, Width: 8, Depth: 8, StructuralHash: strings.Repeat("a", 64), Navigation: &locationsdk.Navigation{BaseSurface: "floor", Surfaces: []locationsdk.NavSurface{{ID: "floor", SlabMM: 250, Slab: []locationsdk.Rect{{W: 8, H: 8}}}}}}
		manifest, err := placement.Build(level, placement.Authoring{})
		if err != nil {
			t.Fatal(err)
		}
		f := concretePolicyFixture()
		f.Materials["concrete.copper"] = Material{SurfaceKind: "concrete", SurfaceCaste: "copper", Provenance: "Existing Copper maps", Concrete: &ConcreteMaterial{Albedo: "res://existing/a.jpg", Normal: "res://existing/n.jpg", Roughness: "res://existing/r.jpg", Tint: [3]float64{.4, .3, .2}, TextureScale: .25, RoughnessBase: .92, PoreContrast: 1, NormalStrength: 1}}
		f.Materials["other.nonemissive.paint"] = Material{Color: [4]float64{.2, .3, .4, 1}, Roughness: .6, Metallic: .1, Provenance: "Arbitrary CUE plain material"}
		f.Architecture.WallZones = map[string]WallZone{"lower.shell": {Caste: "copper", Reason: "Below the Bronze hall", MaterialRole: "concrete.copper"}}
		f.Architecture.Draws = []ArchitectureDraw{
			{ID: "lower.shell", Kind: "wall", Zone: "lower.shell", MaterialRole: "concrete.copper", Position: [3]float64{4, -4, 6}, Size: [3]float64{.33, 8.8, 13.36}, Reason: "Original concrete backing"},
			{ID: "camera.wall", Kind: "wall", MaterialRole: "concrete.bronze", Position: [3]float64{1, 3, 2}, Size: [3]float64{.3, 6, 10}, Reason: "Full enclosure", NearCut: &ArchitectureNearCut{HeightM: 3.89, Reason: "Retain engineering supports"}},
			{ID: "painted.trim", Kind: "trim", MaterialRole: "other.nonemissive.paint", Position: [3]float64{2, 1, 3}, Size: [3]float64{.2, .3, .4}, Reason: "No legacy fallback may recolour it"},
		}
		before, _ := json.Marshal(f)
		result, err := RenderArchitectureGeometry(level, manifest, f)
		if err != nil {
			t.Fatal(err)
		}
		again, err := RenderArchitectureGeometry(level, manifest, f)
		if err != nil || !bytes.Equal(result, again) {
			t.Fatal("architecture output is not byte stable", err)
		}
		normal, err := Render(level, manifest, Options{Finishing: f})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(result), `type="MeshInstance3D"`) != 3 {
			t.Fatal("subscene must contain only authored architecture", string(result))
		}
		for _, token := range []string{`scale = Vector3(0.330000000000, 8.800000000000, 13.360000000000)`, `position = Vector3(1.000000000000, 1.945000000000, -2.000000000000)`, `scale = Vector3(0.300000000000, 3.890000000000, 10.000000000000)`, `metadata/source_id = "lower.shell"`, `metadata/source_id = "painted.trim"`, `type="ShaderMaterial"`, `resource_name = "other.nonemissive.paint"`} {
			if !strings.Contains(string(result), token) || !strings.Contains(string(normal), token) {
				t.Fatal("sidecar drifted from common scene", token)
			}
		}
		if !strings.Contains(string(result), `"other.nonemissive.paint": SubResource(`) {
			t.Fatal("plain CUE material needs the existing scoped Finish adapter")
		}
		for _, forbidden := range []string{`metadata/source_id = "floor"`, `type="Camera3D"`, `type="OmniLight3D"`, `type="DirectionalLight3D"`, `name="Placements"`} {
			if strings.Contains(string(result), forbidden) {
				t.Fatal("sidecar duplicates consumer content", forbidden)
			}
		}
		solids, err := ResolveProceduralSolids(level, manifest, f)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, s := range solids {
			if s.SourceID == "camera.wall" {
				found = true
				if s.Min[2] != 0 || s.Max[2] != 6 {
					t.Fatal("camera skin weakened full solid", s)
				}
			}
		}
		if !found {
			t.Fatal("full camera wall disappeared")
		}
		after, _ := json.Marshal(f)
		if !bytes.Equal(before, after) {
			t.Fatal("sidecar mutated CUE input")
		}
	}
}

func TestArchitectureGeometryEmptyAndStrictManifest(t *testing.T) {
	level := locationsdk.CompiledLevel{ID: "empty", UnitMM: 500, Width: 4, Depth: 4}
	manifest, err := placement.Build(level, placement.Authoring{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := RenderArchitectureGeometry(level, manifest, nil)
	if err != nil || strings.Contains(string(data), `type="MeshInstance3D"`) {
		t.Fatal("empty architecture must be a valid empty subscene", err)
	}
	manifest.LayoutID = "another"
	if _, err = RenderArchitectureGeometry(level, manifest, nil); err == nil {
		t.Fatal("mismatched manifest accepted")
	}
}

func TestLiftingDeckCopperShellSecondLayoutByData(t *testing.T) {
	root := os.Getenv("ANG_GAME_ROOT")
	if root == "" {
		t.Skip("set ANG_GAME_ROOT for the shared Copper shell CUE definition")
	}
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		size, aperture, centre string
		depth                  float64
		northSize              [3]float64
	}{{"[12,8]", "[12.16,8.16]", "[49,18,6]", 6.55, [3]float64{13.36, 8.8, .33}}, {"[4,4]", "[4.16,4.16]", "[20,7,40]", 3.65, [3]float64{5.36, 5.9, .33}}} {
		expression := fmt.Sprintf(`#LiftingDeckCopperShell & {id:"unrelated.location.shell",centre_plan_m:%s,deck_size_m:%s,aperture_m:%s,depth_m:%g}`, fixture.centre, fixture.size, fixture.aperture, fixture.depth)
		cmd := exec.Command(cue, "export", ".", "-e", expression)
		cmd.Dir = filepath.Join(root, "content/world/godot_finishing")
		raw, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("CUE shell declaration: %v\n%s", err, raw)
		}
		var result struct {
			Draws []ArchitectureDraw  `json:"draws"`
			Zones map[string]WallZone `json:"wall_zones"`
		}
		if err = json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Draws) != 8 || len(result.Zones) != 8 {
			t.Fatal("four walls/four foundation posts must come from data", string(raw))
		}
		for _, d := range result.Draws {
			if d.MaterialRole != "concrete.copper" || result.Zones[d.ID].Caste != "copper" {
				t.Fatal("lower concrete lost its explicit caste", d)
			}
		}
		for axis, want := range fixture.northSize {
			got := result.Draws[0].Size[axis]
			if got < want-1e-9 || got > want+1e-9 {
				t.Fatal("second layout retained another model's dimensions", got, want)
			}
		}
	}
}
