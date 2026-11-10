package godot

import (
	"encoding/json"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestFloorOpeningTwoLayoutsPreserveNavigation(t *testing.T) {
	for _, origin := range []int{0, 20} {
		level := locationsdk.CompiledLevel{ID: "floor.fixture", UnitMM: 500, Width: 64, Depth: 64, Navigation: &locationsdk.Navigation{Surfaces: []locationsdk.NavSurface{{ID: "floor", SlabMM: 250, ElevationMM: 18000, Slab: []locationsdk.Rect{{X: origin, Y: 0, W: 20, H: 20}}}}}}
		before, _ := json.Marshal(level.Navigation)
		manifest := placement.Manifest{Instances: map[string]placement.Instance{"deck": {ModelRef: "reviewed.deck"}}}
		opening := FloorOpening{ID: "shaft", SurfaceID: "floor", OwnerInstance: "deck", Rect: [4]float64{float64(origin)/2 + 2.92, 2.92, float64(origin)/2 + 7.08, 7.08}, Reason: "deck leaves cover the visual aperture"}
		finish := &Finishing{FloorOpenings: []FloorOpening{opening}}
		boxes, err := proceduralBoxes(level, manifest, finish, nil, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		after, _ := json.Marshal(level.Navigation)
		if string(before) != string(after) {
			t.Fatal("render cutout changed authoritative support/nav")
		}
		if len(boxes) != 4 {
			t.Fatalf("expected four remaining slab pieces, got%d", len(boxes))
		}
		parts := cutFloorRect([4]float64{float64(origin) / 2, 0, float64(origin)/2 + 10, 10}, [][4]float64{opening.Rect})
		area := 0.
		for _, r := range parts {
			area += (r[2] - r[0]) * (r[3] - r[1])
			if math.Min(r[2], opening.Rect[2]) > math.Max(r[0], opening.Rect[0]) && math.Min(r[3], opening.Rect[3]) > math.Max(r[1], opening.Rect[1]) {
				t.Fatal("floor still covers open shaft")
			}
		}
		if math.Abs(area-(100-4.16*4.16)) > 1e-8 {
			t.Fatalf("slab lost unrelated floor: %.8f", area)
		}
		finish.FloorOpenings[0].OwnerInstance = "missing"
		if _, err = proceduralBoxes(level, manifest, finish, nil, nil, false); err == nil {
			t.Fatal("unowned floor opening accepted")
		}
	}
}

func TestFloorRecessTwoLayoutsKeepsSupportAndOpening(t *testing.T) {
	for _, offset := range []float64{0, 20} {
		opening := [4]float64{offset + 3, 3, offset + 7, 7}
		parts := floorSlabParts([4]float64{offset, 0, offset + 10, 10}, 18, .25, [][4]float64{opening}, []FloorRecess{{Rect: [4]float64{offset + 1, 2, offset + 9, 8}, CoverThicknessM: .07}})
		area, capArea := 0., 0.
		for _, p := range parts {
			r := p.Rect
			a := (r[2] - r[0]) * (r[3] - r[1])
			area += a
			if p.Top != 18 {
				t.Fatal("recess moved the walking surface")
			}
			if p.Thickness == .07 {
				capArea += a
			} else if p.Thickness != .25 {
				t.Fatal("unrelated slab thickness changed")
			}
			if math.Min(r[2], opening[2]) > math.Max(r[0], opening[0]) && math.Min(r[3], opening[3]) > math.Max(r[1], opening[1]) {
				t.Fatal("recess filled the open shaft")
			}
		}
		if math.Abs(area-84) > 1e-8 || math.Abs(capArea-32) > 1e-8 {
			t.Fatalf("lost supported floor or cap area: %g/%g", area, capArea)
		}
		level := locationsdk.CompiledLevel{UnitMM: 500, Navigation: &locationsdk.Navigation{Surfaces: []locationsdk.NavSurface{{ID: "floor", SlabMM: 250, ElevationMM: 18000, Slab: []locationsdk.Rect{{X: int(offset * 2), W: 20, H: 20}}}}}}
		before, _ := json.Marshal(level.Navigation)
		manifest := placement.Manifest{Instances: map[string]placement.Instance{"deck": {ModelRef: "deck"}}}
		f := &Finishing{FloorRecesses: []FloorRecess{{ID: "pocket", SurfaceID: "floor", OwnerInstance: "deck", Rect: [4]float64{offset + 1, 2, offset + 9, 8}, CoverThicknessM: .07, Reason: "reviewed equipment clearance"}}}
		draws, err := ResolveFloorDraws(level, manifest, f)
		if err != nil {
			t.Fatal(err)
		}
		boxes, err := proceduralBoxes(level, manifest, f, nil, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(draws) != len(boxes) || len(draws) != 5 {
			t.Fatalf("legacy and ordinary renderers disagree: %d/%d", len(draws), len(boxes))
		}
		after, _ := json.Marshal(level.Navigation)
		if string(before) != string(after) {
			t.Fatal("recess changed navigation")
		}
		f.FloorRecesses[0].CoverThicknessM = 0
		if _, err = ResolveFloorDraws(level, manifest, f); err == nil {
			t.Fatal("zero-thickness floor cover accepted")
		}
	}
}

func TestChamferedFrameCornersTwoLayouts(t *testing.T) {
	for _, offset := range []float64{0, 20} {
		level := locationsdk.CompiledLevel{ID: "chamfered-room", UnitMM: 500, Width: 80, Depth: 40, Navigation: &locationsdk.Navigation{Surfaces: []locationsdk.NavSurface{{ID: "floor", SlabMM: 250, ElevationMM: 18000, Slab: []locationsdk.Rect{{X: int(offset * 2), W: 40, H: 40}}}}}}
		before, _ := json.Marshal(level.Navigation)
		manifest, err := placement.Build(level, placement.Authoring{})
		if err != nil {
			t.Fatal(err)
		}
		manifest.Instances["deck"] = placement.Instance{ModelRef: "real-deck"}
		f := &Finishing{FloorOpenings: []FloorOpening{{ID: "octagonal-frame", SurfaceID: "floor", OwnerInstance: "deck", Rect: [4]float64{offset + 2, 2, offset + 16, 12}, CornerCutM: 1.05, Reason: "real octagonal coaming replaces its floor"}}, FloorRecesses: []FloorRecess{{ID: "pocket", SurfaceID: "floor", OwnerInstance: "deck", Rect: [4]float64{offset + 1, 1, offset + 17, 13}, CoverThicknessM: .03, Reason: "measured equipment clearance"}}}
		draws, err := ResolveFloorDraws(level, manifest, f)
		if err != nil {
			t.Fatal(err)
		}
		area, cornerArea := 0., 0.
		corners := 0
		for _, draw := range draws {
			if draw.TopM != 18 {
				t.Fatal("corner changed the walking top")
			}
			if draw.Triangle == nil {
				r := draw.Rect
				area += (r[2] - r[0]) * (r[3] - r[1])
				continue
			}
			a := floorTriangleArea(*draw.Triangle)
			cornerArea += a
			area += a
			corners++
			if draw.ThicknessM != .03 {
				t.Fatal("corner ignored the pocket roof")
			}
			// Parse the actual serialized affine transform; every top vertex must
			// match its plan triangle, not a mirrored/shifted wedge above a hole.
			raw := strings.TrimSuffix(strings.TrimPrefix(draw.GodotTransform, "Transform3D("), ")")
			fields := strings.Split(raw, ",")
			v := []float64{}
			for _, field := range fields {
				number, e := strconv.ParseFloat(strings.TrimSpace(field), 64)
				if e != nil {
					t.Fatal(e)
				}
				v = append(v, number)
			}
			for i, local := range [][3]float64{{-.5, -.5, .5}, {.5, -.5, .5}, {-.5, .5, .5}} {
				world := [3]float64{}
				for row := range 3 {
					world[row] = v[9+row]
					for col := range 3 {
						world[row] += v[row*3+col] * local[col]
					}
				}
				want := (*draw.Triangle)[i]
				if math.Abs(world[0]-want[0]) > 1e-9 || math.Abs(world[1]-18) > 1e-9 || math.Abs(-world[2]-want[1]) > 1e-9 {
					t.Fatalf("corner pose mismatch: %v vs %v", world, want)
				}
			}
		}
		if corners != 4 || math.Abs(cornerArea-2*1.05*1.05) > 1e-9 || math.Abs(area-(400-140+2*1.05*1.05)) > 1e-9 {
			t.Fatalf("frame corner floor coverage: corners%d area%g/%g", corners, cornerArea, area)
		}
		after, _ := json.Marshal(level.Navigation)
		if string(before) != string(after) {
			t.Fatal("chamfer changed support/navigation")
		}
		scene, err := Render(level, manifest, Options{Finishing: f, Models: map[string]string{"real-deck": "res://deck.tscn"}})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(scene), `mesh = SubResource("floor_corner")`) != 4 {
			t.Fatal("resolved floor corner missing from the actual scene")
		}
	}
}
