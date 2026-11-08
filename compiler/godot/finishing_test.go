package godot

import (
	"bytes"
	"encoding/json"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
	"math"
	"strings"
	"testing"
)

func TestFinishingIsGenericAndPreservesCompiledSemantics(t *testing.T) {
	l := locationsdk.CompiledLevel{ID: "synthetic-room", UnitMM: 500, Width: 10, Depth: 10, Navigation: &locationsdk.Navigation{Surfaces: []locationsdk.NavSurface{{ID: "floor", SlabMM: 250, Slab: []locationsdk.Rect{{W: 10, H: 10}}}}, Obstacles: []locationsdk.NavObstacle{{ID: "facade", Rect: locationsdk.Rect{W: 3, H: 1}, BottomMM: 0, TopMM: 3000, BlocksMovement: true, BlocksSight: true}}}}
	m, _ := placement.Build(l, placement.Authoring{Extras: []placement.Extra{{InstanceID: "finish", ModelRef: "panel", SourceKind: "art", Reason: "source panel"}}})
	before, _ := json.Marshal(l)
	manifestBefore, _ := json.Marshal(m)
	f := &Finishing{Materials: map[string]Material{"tile": {Resource: "res://materials/tile.tres", Provenance: "existing tile"}, "red": {Color: [4]float64{.25, .001, .0005, 1}, Roughness: .38, Metallic: .05, Emission: [3]float64{.8, .001, .0005}, EmissionEnergy: 3, Provenance: "existing red eye"}}, SurfaceRoles: map[string]string{"floor": "tile"}, MaterialBindings: map[string]string{"BlenderEye": "red"}, Replacements: []Replacement{{SurfaceID: "facade", InstanceID: "finish", Reason: "art panel"}}, Lights: []Light{{ID: "fixture", At: placement.Vec3{2, 4, 3}, Aim: placement.Vec3{2, 0, 3}, Color: [3]float64{1, .44, .12}, Energy: 3, Range: 8, Angle: 120, Mode: [3]float64{1, .5, .25}, Source: "source.blend#lamp"}}}
	o := Options{Finishing: f, Models: map[string]string{"panel": "res://panel.glb"}}
	a, err := Render(l, m, o)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(l, m, o)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("nondeterministic finish", err)
	}
	s := string(a)
	for _, part := range []string{`"BlenderEye": SubResource(`, `material_override = ExtResource(`, `metadata/mode = Vector3(1.000000000000, 0.500000000000, 0.250000000000)`, `type="SpotLight3D"`, `emission_enabled = true`, `metadata/source = "source.blend#lamp"`} {
		if !strings.Contains(s, part) {
			t.Fatal("missing", part)
		}
	}
	if strings.Contains(s, `metadata/source_id = "facade"`) {
		t.Fatal("art-covered SDK facade still rendered")
	}
	after, _ := json.Marshal(l)
	manifestAfter, _ := json.Marshal(m)
	if !bytes.Equal(before, after) || !bytes.Equal(manifestBefore, manifestAfter) {
		t.Fatal("finishing mutated navigation/placements")
	}
	for _, bad := range []Replacement{{SurfaceID: "typo", InstanceID: "finish", Reason: "x"}, {SurfaceID: "facade", InstanceID: "unbound", Reason: "x"}, {SurfaceID: "facade", InstanceID: "finish"}} {
		f.Replacements = []Replacement{bad}
		if _, err = Render(l, m, o); err == nil {
			t.Fatal("invalid replacement accepted", bad)
		}
	}
	f.Replacements = nil
	f.Lights[0].Energy = math.NaN()
	if _, err = Render(l, m, o); err == nil {
		t.Fatal("nonfinite light accepted")
	}
	f.Lights = nil
	f.Materials["tile"] = Material{Resource: "res://../escape.tres", Provenance: "x"}
	if _, err = Render(l, m, o); err == nil {
		t.Fatal("escaping material path accepted")
	}
}

func TestFinishingPreservesLinearColourRectangleAndSourceCulling(t *testing.T) {
	f := &Finishing{Materials: map[string]Material{"brass": {Color: [4]float64{.5, .25, .01, 1}, Roughness: .5, DoubleSided: true, Provenance: "source linear material"}}, MaterialBindings: map[string]string{"original": "brass"}, Lights: []Light{{ID: "rectangle", Shadows: true, At: placement.Vec3{1, 3, 2}, Aim: placement.Vec3{1, 0, 2}, Color: [3]float64{1, .5, .01}, Energy: 3, Range: 8, AreaSize: [2]float64{.8, .08}, Mode: [3]float64{1, 1, 1}, Source: "source rectangle"}}, Environment: &EnvironmentProfile{AmbientColor: [3]float64{.19, .23, .30}, AmbientEnergy: [3]float64{.4, .25, .12}, Exposure: 1.74, Provenance: "source world"}}
	h, _, lights, _, _, err := f.render(placement.Manifest{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"0.735356983052", "cull_mode = 2"} {
		if !strings.Contains(h, part) {
			t.Fatal("lost source colour/culling", part, h)
		}
	}
	if !strings.Contains(lights, "shadow_enabled = true") || !strings.Contains(lights, `type="AreaLight3D"`) || !strings.Contains(lights, "area_size = Vector2(0.800000000000, 0.080000000000)") {
		t.Fatal("lost rectangle", lights)
	}
	f.Lights[0].AreaSize[1] = 0
	if _, _, _, _, _, err = f.render(placement.Manifest{}, nil); err == nil {
		t.Fatal("partial rectangle accepted")
	}
	if _, err = f.Environment.render(); err != nil {
		t.Fatal(err)
	}
}
