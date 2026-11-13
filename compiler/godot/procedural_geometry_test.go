package godot

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func TestPrimitiveBudgetUsesEmittedCellsAndModelSuppression(t *testing.T) {
	level := locationsdk.CompiledLevel{ID: "budget-room", UnitMM: 500, Width: 2, Depth: 2, Preview: locationsdk.PreviewData{}}
	// Four rendered half-metre cells are not the author's one 1m blockout box.
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			level.Preview.Floor = append(level.Preview.Floor, locationsdk.PreviewFloor{SourceID: "slab", At: locationsdk.Cell{X: x, Y: y}})
		}
	}
	manifest, err := placement.Build(level, placement.Authoring{})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanProceduralGeometry(level, manifest, nil, false)
	if err != nil || plan.Triangles != 48 || len(plan.Primitives) != 4 {
		t.Fatalf("emitted cell budget: %+v, %v", plan, err)
	}
	scene, err := Render(level, manifest, Options{})
	if err != nil || strings.Count(string(scene), `metadata/source_id = "slab"`) != 4 {
		t.Fatalf("draw plan differs from scene: %v", err)
	}
	// Camera cutaway v1: every procedural box names its structure kind (a slab is a floor, never an occluder).
	if strings.Count(string(scene), "metadata/source_id = \"slab\"\nmetadata/structure_kind = \"floor\"") != 4 {
		t.Fatalf("slab boxes lost their structure kind:\n%s", scene)
	}
	plane, err := PlanProceduralGeometry(level, manifest, nil, true)
	if err != nil || plane.Triangles != 8 {
		t.Fatalf("FloorOnly plane topology: %+v, %v", plane, err)
	}
	// Bound floor and art replacement both suppress procedural geometry, so an
	// art subtotal never receives a second invisible slab/obstacle charge.
	level.Navigation = &locationsdk.Navigation{Surfaces: []locationsdk.NavSurface{{ID: "slab", SlabMM: 200, Slab: []locationsdk.Rect{{W: 2, H: 2}}}}, Obstacles: []locationsdk.NavObstacle{{ID: "wall", Rect: locationsdk.Rect{W: 2, H: 1}, BottomMM: 0, TopMM: 2000}}}
	manifest.Instances = map[string]placement.Instance{"slab": {ModelRef: "floor-model"}, "cladding": {ModelRef: "wall-model"}}
	finish := &Finishing{Replacements: []Replacement{{SurfaceID: "wall", InstanceID: "cladding", Reason: "bound art replaces the wall"}}}
	plan, err = PlanProceduralGeometry(level, manifest, finish, false)
	if err != nil || plan.Triangles != 0 || len(plan.Primitives) != 0 {
		t.Fatalf("suppressed primitive counted: %+v, %v", plan, err)
	}
}

// Camera cutaway v1: a CUE obstacle box carries its style as the structure kind, a slab "floor"; the client cuts only the
// level's occluder kinds, so a panel or glazing obstacle is never mistaken for a wall.
func TestProceduralBoxesNameTheirStructureKind(t *testing.T) {
	level := locationsdk.CompiledLevel{ID: "kind-room", UnitMM: 500, Width: 4, Depth: 4}
	level.Navigation = &locationsdk.Navigation{
		Surfaces: []locationsdk.NavSurface{{ID: "deck", SlabMM: 250, Walkable: true, Slab: []locationsdk.Rect{{W: 4, H: 4}}}},
		Obstacles: []locationsdk.NavObstacle{
			{ID: "near.wall", Style: "wall", Rect: locationsdk.Rect{X: 0, Y: 3, W: 4, H: 1}, TopMM: 6000},
			{ID: "shop.panel", Style: "panel", Rect: locationsdk.Rect{X: 0, Y: 0, W: 1, H: 1}, TopMM: 2400},
		},
	}
	manifest, err := placement.Build(level, placement.Authoring{})
	if err != nil {
		t.Fatal(err)
	}
	scene, err := Render(level, manifest, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"metadata/source_id = \"deck\"\nmetadata/structure_kind = \"floor\"",
		"metadata/source_id = \"near.wall\"\nmetadata/structure_kind = \"wall\"",
		"metadata/source_id = \"shop.panel\"\nmetadata/structure_kind = \"panel\"",
	} {
		if !strings.Contains(string(scene), want) {
			t.Fatalf("missing %q in\n%s", want, scene)
		}
	}
}
