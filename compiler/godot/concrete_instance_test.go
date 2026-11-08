package godot

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

func TestConcreteHistoryNeedsPhysicalSourcesAndLocalizedMicrotags(t *testing.T) {
	anchors := []ConcreteAnchor{{ID: "pipe", Kind: "pipe", SourceRef: "real.pipe", Position: placement.Vec3{1, 2, 0}, Strength: 1, RadiusM: .5}}
	source, err := concreteSourceTable(anchors)
	if err != nil {
		t.Fatal(err)
	}
	spec := ConcreteInstance{Tier: "copper", Age: .7, Exposure: "public", Function: []string{"water"}, Anchors: []string{"pipe"}}
	if err = validateConcreteInstance("wall", spec, source); err != nil {
		t.Fatal(err)
	}
	spec.Anchors = nil
	if err = validateConcreteInstance("wall", spec, source); err == nil {
		t.Fatal("unanchored stain accepted")
	}
	spec.Anchors = []string{"pipe"}
	spec.Patches = []ConcretePatch{{ID: "paid", Anchor: "pipe", Size: [2]float64{.6, .5}, Atlas: [4]float64{0, 0, .5, .5}, Tagged: true}}
	if err = validateConcreteInstance("wall", spec, source); err == nil {
		t.Fatal("paid patch without localized number/date accepted")
	}
	spec.Patches[0].Number = "017"
	spec.Patches[0].Date = "08.11.26"
	spec.Patches[0].TextKey = "arch.repair.tag.number_date"
	if err = validateConcreteInstance("wall", spec, source); err != nil {
		t.Fatal(err)
	}
	spec.Enamel = true
	if err = validateConcreteInstance("wall", spec, source); err == nil {
		t.Fatal("Copper enamel accepted")
	}
}

func TestConcreteGoldResinUsesOpeningCornersAndNoMachineGhosts(t *testing.T) {
	hint := ConcreteAnchor{ID: "hidden", Kind: "steel", SourceRef: "real.closed.cover", Position: placement.Vec3{1, .05, 0}, Strength: .2, RadiusM: .05, Hidden: true}
	spec := ConcreteInstance{Tier: "gold", Age: .7, Exposure: "public", Resin: true, Caries: &ConcreteCaries{Strength: .2, Hints: []ConcreteAnchor{hint}}}
	if err := validateConcreteInstance("wall", spec, nil); err == nil {
		t.Fatal("arbitrary resin without opening accepted")
	}
	spec.Openings = []ConcreteOpening{{ID: "door", SourceRef: "real.door", Center: placement.Vec3{4.5, 1.05, .105}, WidthM: 1, HeightM: 2.1, Tangent: placement.Vec3{1, 0, 0}, LoadDirection: "diagonal_from_top_corners"}}
	if err := validateConcreteInstance("wall", spec, nil); err != nil {
		t.Fatal(err)
	}
	params := concreteInstanceParameters(&ConcreteMaterial{Instance: &spec})
	if !strings.Contains(params, "shader_parameter/resin_count = 2") || !strings.Contains(params, "Vector3(4.000000000000,2.100000000000,-0.105000000000)") {
		t.Fatal("resin did not start at the actual top opening corner")
	}
	spec.Ghosts = []ConcreteGhost{{ID: "old", SourceRef: "wall", Size: [2]float64{1, 1}, AnchorColumns: 2, AnchorRows: 2}}
	if err := validateConcreteInstance("wall", spec, nil); err == nil {
		t.Fatal("Gold machine ghost accepted")
	}
}

func TestConcreteTouchRoutesComeFromFixtureNavInsteadOfOtherLevel(t *testing.T) {
	fixture := locationsdk.CompiledLevel{UnitMM: 500, Terrain: []string{"....", "....", "...."}, ReservedPaths: []locationsdk.CompiledPath{{ID: "queue", Cells: []locationsdk.Cell{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}}}}}
	routes := buildConcreteRoutes(fixture, 2)
	if len(routes) == 0 || routes[0].From[1] != 2 {
		t.Fatal("NAV routes lost physical floor")
	}
	p := &ConcreteMaterial{Instance: &ConcreteInstance{Tier: "bronze", Exposure: "public", Age: .7, Handwear: true}, ReviewNavigation: &fixture}
	f := &Finishing{Materials: map[string]Material{"review": {Concrete: p}}}
	other := locationsdk.CompiledLevel{UnitMM: 1000, Terrain: []string{"."}}
	out, err := ResolveArchitectureForLevel(f, other)
	if err != nil || len(out.Materials["review"].Concrete.RouteData) != 0 {
		t.Fatal("actual level NAV contaminated review fixture", err)
	}
}

func TestConcreteCariesCreatesConcealedSourceBoundPhysicalSign(t *testing.T) {
	hint := ConcreteAnchor{ID: "hidden.cover.damp", Kind: "steel", Effect: "damp", SourceRef: "actual.cover", Position: placement.Vec3{2, .07, 1}, Strength: .3, RadiusM: .05, Hidden: true}
	instance := &ConcreteInstance{Tier: "silver", Age: .5, Exposure: "public", Enamel: true, Caries: &ConcreteCaries{Strength: .2, Hints: []ConcreteAnchor{hint}}}
	f := &Finishing{Architecture: &Architecture{Setting: "dome_interior", WallCaste: "silver", ConcreteRole: "concrete.silver", Concrete: instance, Draws: []ArchitectureDraw{{ID: "actual.cover", Kind: "trim", MaterialRole: "steel", Position: [3]float64{2, .1, 1.01}, Size: [3]float64{.3, .2, .02}, Reason: "Actual closed kit cover conceals the damp sign"}}}, Materials: map[string]Material{
		"concrete.silver": {SurfaceKind: "concrete", SurfaceCaste: "silver", Concrete: &ConcreteMaterial{}},
		"caries.damp":     {SurfaceKind: "deposit", Provenance: "Source-bound physical damp film"},
		"steel":           {SurfaceKind: "steel"},
	}}
	out, err := ResolveArchitectureForLevel(f, locationsdk.CompiledLevel{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, draw := range out.Architecture.Draws {
		if draw.ID == "caries."+hint.ID {
			found = true
			if draw.Position != [3]float64(hint.Position) || draw.Size[2] != .001 {
				t.Fatal("hint source pose or physical film lost")
			}
		}
	}
	if !found {
		t.Fatal("caries remained validation-only metadata")
	}
	profile := out.Materials[out.Architecture.ConcreteRole].Concrete
	params := concreteInstanceParameters(profile)
	if !strings.Contains(params, "shader_parameter/caries_count = 1") {
		t.Fatal("caries did not reach shader")
	}
	f.Architecture.Draws = nil
	if _, err = ResolveArchitectureForLevel(f, locationsdk.CompiledLevel{}); err == nil || !strings.Contains(err.Error(), "actual.cover") {
		t.Fatal("nonexistent physical hint source accepted", err)
	}
}

func TestConcreteTouchDensityAccumulatesAndProjectsUpperNavSurface(t *testing.T) {
	level := locationsdk.CompiledLevel{UnitMM: 500, Terrain: []string{"..", "..", "..", ".."}, Navigation: &locationsdk.Navigation{PlanWidth: 2, PlanDepth: 2, Surfaces: []locationsdk.NavSurface{{ID: "lower", Walkable: true, NavRow: 0, ElevationMM: 0}, {ID: "upper", Walkable: true, NavRow: 2, ElevationMM: 3000}}}, ReservedPaths: []locationsdk.CompiledPath{{ID: "a", Cells: []locationsdk.Cell{{X: 0, Y: 2}}}}}
	before := buildConcreteRoutes(level, 3)
	var first float64
	for _, route := range before {
		if route.From[2] > 1 || route.To[2] > 1 || route.From[1] != 3 {
			t.Fatal("upper NAV band was not projected to physical plan", route)
		}
		if route.From[0] == .25 && route.From[2] == .25 {
			first = route.Weight
		}
	}
	level.ReservedPaths = append(level.ReservedPaths, locationsdk.CompiledPath{ID: "b", Cells: []locationsdk.Cell{{X: 0, Y: 2}}})
	after := buildConcreteRoutes(level, 3)
	found := false
	for _, route := range after {
		if route.From[0] == .25 && route.From[2] == .25 {
			found = true
			if route.Weight <= first {
				t.Fatal("overlapping route density was reduced to max instead of accumulated")
			}
		}
	}
	if !found {
		t.Fatal("actual upper walk cell lost")
	}
}
