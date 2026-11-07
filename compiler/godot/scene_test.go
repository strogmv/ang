package godot

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func TestSceneReusesModelsAndEmitsCompiledGeometry(t *testing.T) {
	l := locationsdk.CompiledLevel{ID: "room", UnitMM: 500, Width: 20, Depth: 20,
		Instances: []locationsdk.CompiledInstance{{ID: "a", ModuleID: "desk", Width: 2, Depth: 1}, {ID: "b", ModuleID: "desk", Width: 2, Depth: 1, At: locationsdk.Cell{X: 5, Y: 3}, Rotation: 180}},
		Navigation: &locationsdk.Navigation{BaseSurface: "floor", Surfaces: []locationsdk.NavSurface{{ID: "floor", SlabMM: 250, ElevationMM: 1000, Slab: []locationsdk.Rect{{X: 2, Y: 4, W: 8, H: 6}}}},
			Obstacles: []locationsdk.NavObstacle{{ID: "wall", Rect: locationsdk.Rect{X: 2, Y: 4, W: 8, H: 1}, BottomMM: 1000, TopMM: 4000}}}}
	m, err := placement.Build(l, placement.Authoring{})
	if err != nil {
		t.Fatal(err)
	}
	options := Options{Models: map[string]string{"desk": "res://kit/desk.glb"}}
	first, err := Render(l, m, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(l, m, options)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("non-deterministic scene", err)
	}
	s := string(first)
	if strings.Count(s, "[ext_resource ") != 1 || strings.Count(s, `instance=ExtResource("model_000")`) != 2 {
		t.Fatal("model resource was not reused")
	}
	if !strings.Contains(s, "position = Vector3(3.000000000000, 0.875000000000, -3.500000000000)") || !strings.Contains(s, "scale = Vector3(4.000000000000, 0.250000000000, 3.000000000000)") {
		t.Fatal("slab top/underside/metric bounds lost")
	}
	if !strings.Contains(s, "position = Vector3(3.000000000000, 2.500000000000, -2.250000000000)") {
		t.Fatal("obstacle interval not emitted")
	}
	if !strings.Contains(s, "projection = 1") || !strings.Contains(s, "metadata/elevation_degrees = 35") || !strings.Contains(s, "metadata/azimuth_degrees = 37") {
		t.Fatal("camera contract lost")
	}
	if _, err = Render(l, m, Options{}); err == nil {
		t.Fatal("missing model silently omitted")
	}
	for _, p := range []string{"res://../desk.glb", "res://kit/../desk.glb", "file:///desk.glb", "res://desk.png", "res://desk.glb\n[node name=\"bad\"]"} {
		if _, err = Render(l, m, Options{Models: map[string]string{"desk": p}}); err == nil {
			t.Fatalf("invalid path accepted: %q", p)
		}
	}
	m.StructuralHash = "stale"
	if _, err = Render(l, m, options); err == nil {
		t.Fatal("stale manifest accepted")
	}
}

func TestSerializedCameraLooksAtAuthoredFocus(t *testing.T) {
	l := locationsdk.CompiledLevel{ID: "camera", UnitMM: 500, Camera: &locationsdk.LayoutCamera{ElevationDegrees: 35, AzimuthDegrees: 37}}
	m, err := placement.Build(l, placement.Authoring{})
	if err != nil {
		t.Fatal(err)
	}
	framing := placement.Camera{FocusPlanM: placement.Vec3{28.5, 4.6, 22}, SizeM: 49, DistanceM: 120}
	data, err := Render(l, m, Options{Camera: &framing})
	if err != nil {
		t.Fatal(err)
	}
	_, camera, ok := strings.Cut(string(data), `[node name="Camera3D"`)
	if !ok {
		t.Fatal("camera missing")
	}
	_, camera, ok = strings.Cut(camera, "transform = Transform3D(")
	if !ok {
		t.Fatal("camera transform missing")
	}
	values, _, _ := strings.Cut(camera, ")")
	parts := strings.Split(values, ", ")
	if len(parts) != 12 {
		t.Fatal("invalid transform arity")
	}
	numbers := make([]float64, 12)
	for j, part := range parts {
		numbers[j], err = strconv.ParseFloat(part, 64)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Godot text stores matrix rows, followed by origin. Test the physical
	// camera ray instead of reproducing the emitter's matrix construction.
	focus := placement.FromPlan(framing.FocusPlanM)
	for j := range 3 {
		direction := (numbers[9+j] - focus[j]) / framing.DistanceM
		if math.Abs(numbers[j*3+2]-direction) > 1e-10 {
			t.Fatalf("serialized camera points away from focus: %v", numbers)
		}
	}
	if numbers[0] >= 0 || numbers[6] >= 0 || math.Abs(numbers[3]) > 1e-10 {
		t.Fatal("camera right must face west/north and remain horizontal")
	}
}
