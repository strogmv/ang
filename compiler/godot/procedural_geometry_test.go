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
