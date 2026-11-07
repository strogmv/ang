package placement

import (
	"math"
	"reflect"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
)

func TestQuarterTurnPivotsAndHandedness(t *testing.T) {
	// A non-square passport distinguishes the compensated NW pivot from a
	// cell-centre rotation. Width/depth here are already rotated by locationsdk.
	for _, tt := range []struct {
		rotation, width, depth int
		position               Vec3
		localX                 Vec3
	}{
		{0, 6, 2, Vec3{5, 1.25, -10}, Vec3{1, 0, 0}},
		{90, 2, 6, Vec3{6, 1.25, -10}, Vec3{0, 0, -1}},
		{180, 6, 2, Vec3{8, 1.25, -11}, Vec3{-1, 0, 0}},
		{270, 2, 6, Vec3{5, 1.25, -13}, Vec3{0, 0, 1}},
	} {
		pose := Pose(locationsdk.Cell{X: 10, Y: 20}, tt.width, tt.depth, 1.25, tt.rotation)
		if pose.Position != tt.position {
			t.Fatalf("rotation %d: got pivot %v, want %v", tt.rotation, pose.Position, tt.position)
		}
		basis := pose.BasisColumns()
		for j := range tt.localX {
			if math.Abs(basis[0][j]-tt.localX[j]) > 1e-12 {
				t.Fatalf("rotation %d: wrong model X basis %v", tt.rotation, basis)
			}
		}
		if determinant(basis) != -1 {
			t.Fatalf("model lost handedness: %v", basis)
		}
	}
}

func TestCameraKeepsWestCraneScreenRight(t *testing.T) {
	b := CameraBasis(35, 37)
	if math.Abs(determinant(b)-1) > 1e-12 {
		t.Fatalf("camera basis is not right handed: %v", b)
	}
	// Crane relative to oracle focus. Screen right is dot(world delta,right).
	crane, focus := FromPlan(Vec3{3.5, 8.55, 4}), FromPlan(Vec3{28.5, 4.6, 22})
	screenX := 0.0
	for j := range crane {
		screenX += (crane[j] - focus[j]) * b[0][j]
	}
	if screenX <= 0 {
		t.Fatalf("west crane mirrored to screen left: %g", screenX)
	}
	if b[2][0] <= 0 || b[2][1] <= 0 || b[2][2] >= 0 {
		t.Fatalf("wrong camera offset quadrant: %v", b[2])
	}
}

func determinant(b [3]Vec3) float64 {
	return b[0][0]*(b[1][1]*b[2][2]-b[1][2]*b[2][1]) - b[1][0]*(b[0][1]*b[2][2]-b[0][2]*b[2][1]) + b[2][0]*(b[0][1]*b[1][2]-b[0][2]*b[1][1])
}

func TestManifestBindingsReferencesAndOverrides(t *testing.T) {
	l := locationsdk.CompiledLevel{ID: "room", UnitMM: 500, Elevation: 2,
		Instances:  []locationsdk.CompiledInstance{{ID: "desk", ModuleID: "passport.desk", At: locationsdk.Cell{X: 4, Y: 6}, Width: 2, Depth: 1}},
		Clearances: []locationsdk.CompiledClearance{{InstanceID: "desk", Kind: "use"}},
		Ports:      []locationsdk.CompiledPort{{ID: "front", InstanceID: "desk"}},
		Navigation: &locationsdk.Navigation{BaseSurface: "floor", Surfaces: []locationsdk.NavSurface{{ID: "floor"}, {ID: "roof", ElevationMM: 4000, SlabMM: 200, Slab: []locationsdk.Rect{{X: 4, Y: 6, W: 2, H: 1}}}}}}
	ref := "desk.stocked"
	spec := Authoring{LayoutID: "room", Instances: map[string]InstanceSpec{"desk": {ModelRef: &ref, Bindings: Bindings{Gates: []string{"gate.desk"}}, Override: &PoseOverride{PositionPlanM: Vec3{9, 1, 12}, RotationDegrees: 180, Reason: "approved service band"}}}}
	m, err := Build(l, spec)
	if err != nil {
		t.Fatal(err)
	}
	i := m.Instances["desk"]
	if i.ModelRef != ref || i.SourceTransform == nil || i.SourceTransform.Position != (Vec3{2, 1, -3}) || i.WorldTransform.Position != (Vec3{9, 1, -12}) {
		t.Fatalf("wrong resolved placement: %+v", i)
	}
	if !reflect.DeepEqual(i.ClearanceRefs, []string{"#/clearances/0"}) || !reflect.DeepEqual(i.NavRefs, []string{"#/instances/0/blocked", "#/instances/0/footprint", "#/ports/0"}) || len(i.Bindings.Gates) != 1 {
		t.Fatalf("lost binding/references: %+v", i)
	}
	if m.Instances["roof"].WorldTransform.Position != (Vec3{2, 3.8, -3}) {
		t.Fatal("surface must use slab underside and first slab origin")
	}
	spec.Instances["typo"] = InstanceSpec{}
	if _, err = Build(l, spec); err == nil {
		t.Fatal("unknown override accepted")
	}
	delete(spec.Instances, "typo")
	spec.Instances["desk"].Override.Reason = ""
	if _, err = Build(l, spec); err == nil {
		t.Fatal("unexplained override accepted")
	}
	spec.Instances = nil
	spec.Extras = []Extra{{InstanceID: "desk", ModelRef: "train", SourceKind: "epoch", Reason: "epoch"}}
	if _, err = Build(l, spec); err == nil {
		t.Fatal("duplicate extra accepted")
	}
}
