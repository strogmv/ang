package godot

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func TestWallTopMassCapsEveryPocketAndPreservesFloorAndOpening(t *testing.T) {
	for _, id := range []string{"first.hub", "second.dock"} {
		level := locationsdk.CompiledLevel{ID: id, UnitMM: 500, Width: 8, Depth: 8, Elevation: 36, StructuralHash: strings.Repeat("a", 64)}
		for y := 2; y < 6; y++ {
			for x := 2; x < 6; x++ {
				level.Preview.Floor = append(level.Preview.Floor, locationsdk.PreviewFloor{At: locationsdk.Cell{X: x, Y: y}, SourceID: "actual.floor"})
			}
		}
		f := concretePolicyFixture()
		f.Architecture.WallTopMass = []WallTopMass{{ID: "solid.mass", Bounds: locationsdk.Rect{W: 8, H: 8}, Openings: []locationsdk.Rect{{W: 4, H: 1}}, HeightM: 6, ThicknessM: .3, Reason: "Authored hatched solid mass"}}
		original, _ := json.Marshal(f)
		resolved, err := ResolveArchitectureForLevel(f, level)
		if err != nil {
			t.Fatal(err)
		}
		draws := resolved.Architecture.Draws
		if len(draws) > 8 {
			t.Fatal("cell-per-draw cap regression", len(draws))
		}
		area := 0.0
		covered := map[locationsdk.Cell]bool{}
		for _, d := range draws {
			if !strings.HasPrefix(d.ID, "solid.mass.cap.") {
				continue
			}
			if d.Kind != "wall" || d.MaterialRole != "concrete.bronze" || math.Abs(d.Position[1]+d.Size[1]/2-24) > 1e-9 {
				t.Fatal("cap must match full wall top and caste", d)
			}
			area += d.Size[0] * d.Size[2]
			x0 := int(math.Round((d.Position[0] - d.Size[0]/2) * 2))
			y0 := int(math.Round((d.Position[2] - d.Size[2]/2) * 2))
			for y := y0; y < y0+int(math.Round(d.Size[2]*2)); y++ {
				for x := x0; x < x0+int(math.Round(d.Size[0]*2)); x++ {
					c := locationsdk.Cell{X: x, Y: y}
					if covered[c] || (x >= 2 && x < 6 && y >= 2 && y < 6) || (y == 0 && x < 4) {
						t.Fatal("cap covers floor, opening or duplicate", c)
					}
					covered[c] = true
				}
			}
		}
		if area != 11 || len(covered) != 44 {
			t.Fatal("missing floorless pocket", area, len(covered))
		}
		again, err := ResolveArchitectureForLevel(resolved, level)
		if err != nil || len(again.Architecture.Draws) != len(draws) {
			t.Fatal("double resolution duplicated caps", err)
		}
		after, _ := json.Marshal(f)
		if string(original) != string(after) {
			t.Fatal("mass resolution mutated author input")
		}
		manifest, err := placement.Build(level, placement.Authoring{})
		if err != nil {
			t.Fatal(err)
		}
		scene, err := RenderArchitectureGeometry(level, manifest, f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(scene), `metadata/source_id = "solid.mass.cap.000"`) {
			t.Fatal("cap missing in actual common sidecar")
		}
		// No cap may change navigation: only the author's original Preview/Floor exists.
		if len(level.Preview.Floor) != 16 || level.Navigation != nil {
			t.Fatal("cap created walking floor")
		}
	}
}

func TestWallTopMassStrictEnvelope(t *testing.T) {
	level := locationsdk.CompiledLevel{ID: "negative", UnitMM: 500, Width: 4, Depth: 4}
	good := WallTopMass{ID: "mass", Bounds: locationsdk.Rect{W: 4, H: 4}, HeightM: 6, ThicknessM: .3, Reason: "Structural mass"}
	for _, mutate := range []func(*WallTopMass){func(m *WallTopMass) { m.Bounds.W = 5 }, func(m *WallTopMass) { m.Reason = "" }, func(m *WallTopMass) { m.ThicknessM = 7 }, func(m *WallTopMass) { m.HeightM = math.NaN() }, func(m *WallTopMass) { m.Openings = []locationsdk.Rect{{}} }} {
		m := good
		mutate(&m)
		f := concretePolicyFixture()
		f.Architecture.WallTopMass = []WallTopMass{m}
		if _, err := ResolveArchitectureForLevel(f, level); err == nil {
			t.Fatal("invalid mass accepted", m)
		}
	}
}
