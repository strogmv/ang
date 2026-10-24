package locationsdk

import (
	"fmt"
	"reflect"
	"testing"
)

func budgetInput(width, depth int) Input {
	return Input{Catalog: testCatalog(), Layouts: []Layout{{ID: "bounded.runtime", UnitMM: 500, Width: width, Depth: depth, Grounded: true, Floor: []FloorArea{{ID: "support", Rect: Rect{W: width, H: depth}, Reason: "authored supported floor"}}, Entries: []Cell{{0, 0}}}}}
}

func budgetOptions() RuntimeOptions { return RuntimeOptions{TickHz: 10, MoveEveryTicks: 3} }

func TestRuntimeAreaBudgetPrecedesBuildAndKeepsLargerAuthoringSupport(t *testing.T) {
	in := budgetInput(256, 256)
	out := requireRuntime(t, in, budgetOptions())
	if len(out.Levels[0].Floor) != RuntimeCellBudget {
		t.Fatal("exact renderer cell boundary was rejected or changed")
	}
	in = budgetInput(256, 257)
	if _, err := Build(in.Catalog, in.Layouts[0]); err != nil {
		t.Fatalf("generic authoring compiler lost its larger supported grid: %v", err)
	}
	// Even an invalid empty catalog must not get as far as Build/prepare when
	// dimensions alone exceed the runtime allocation contract.
	in.Catalog = Catalog{}
	bundle, err := CompileRuntime(in, budgetOptions())
	requireCode(t, err, "runtime_area_budget")
	if !reflect.DeepEqual(bundle, RuntimeBundle{}) {
		t.Fatal("oversized runtime returned a partial payload")
	}
	in.Layouts[0].Width = int(^uint(0) >> 1)
	in.Layouts[0].Depth = 2
	_, err = CompileRuntime(in, budgetOptions())
	requireCode(t, err, "runtime_area_budget")
}

func TestRuntimeRenderedCellBudgetIncludesFootprints(t *testing.T) {
	makeInput := func(physical int) Input {
		in := budgetInput(256, 255)
		in.Layouts[0].Entries = []Cell{{1, 1}}
		g := Geometry{ID: "edge.material", Width: 256, Depth: 2, Height: 1}
		for n := 0; n < physical; n++ {
			at := Cell{n % 256, n / 256}
			g.Footprint = append(g.Footprint, at)
			g.Blocked = append(g.Blocked, at)
		}
		in.Catalog.Geometries = []Geometry{g}
		in.Catalog.Modules = []Module{{ID: "edge", GeometryID: g.ID, Variants: []Variant{{ID: "base", GeometryID: g.ID}}}}
		in.Layouts[0].Instances = []Instance{{ID: "edge.instance", ModuleID: "edge", Reason: "material at the exterior floor edge"}}
		return in
	}
	out := requireRuntime(t, makeInput(256), budgetOptions())
	if len(out.Levels[0].Floor)+len(out.Levels[0].Instances[0].Footprint) != RuntimeCellBudget {
		t.Fatal("test did not reach exact rendered-cell boundary")
	}
	_, err := CompileRuntime(makeInput(257), budgetOptions())
	requireCode(t, err, "runtime_geometry_budget")
}

func TestRuntimeInstanceAndGuideBudgets(t *testing.T) {
	in := budgetInput(2, 2)
	for i := 0; i < RuntimeInstanceBudget; i++ {
		in.Layouts[0].Instances = append(in.Layouts[0].Instances, Instance{ID: fmt.Sprintf("door.%d", i), ModuleID: "module.shape.opening", Reason: "nonphysical connection marker"})
	}
	if got := len(requireRuntime(t, in, budgetOptions()).Levels[0].Instances); got != RuntimeInstanceBudget {
		t.Fatal("exact instance budget changed")
	}
	in.Layouts[0].Instances = append(in.Layouts[0].Instances, Instance{ID: "one.too.many", ModuleID: "module.shape.opening", Reason: "excess marker"})
	_, err := CompileRuntime(in, budgetOptions())
	requireCode(t, err, "runtime_instance_budget")
	in = budgetInput(2, 2)
	for i := 0; i < 4; i++ {
		in.Layouts[0].ReservedPaths = append(in.Layouts[0].ReservedPaths, ReservedPath{ID: fmt.Sprintf("guide.%d", i), Points: make([]Cell, 4096), Width: 1, Reason: "authored ordered guide points"})
	}
	if len(requireRuntime(t, in, budgetOptions()).Levels[0].ReservedPaths) != 4 {
		t.Fatal("exact ordered guide point boundary changed")
	}
	in.Layouts[0].ReservedPaths = append(in.Layouts[0].ReservedPaths, ReservedPath{ID: "excess", Points: []Cell{{0, 0}}, Width: 1, Reason: "excess point"})
	_, err = CompileRuntime(in, budgetOptions())
	requireCode(t, err, "runtime_guide_budget")
}

func TestRuntimePartsBudgetIncludesBarriers(t *testing.T) {
	in := budgetInput(1024, 64)
	for i := 0; i < RuntimePartBudget; i++ {
		in.Layouts[0].Drops = append(in.Layouts[0].Drops, DropAnnotation{ID: fmt.Sprintf("north.%d", i), At: Cell{i, 0}, Side: North, LowerElevation: -1, Barrier: true, Reason: "authored lower edge"})
	}
	out := requireRuntime(t, in, budgetOptions())
	if len(out.Levels[0].Barriers) != RuntimePartBudget {
		t.Fatal("exact rendered-part boundary changed")
	}
	in.Layouts[0].Drops = append(in.Layouts[0].Drops, DropAnnotation{ID: "excess.south", At: Cell{0, 63}, Side: South, LowerElevation: -1, Barrier: true, Reason: "one excess barrier mesh"})
	_, err := CompileRuntime(in, budgetOptions())
	requireCode(t, err, "runtime_part_budget")
}

func TestRuntimeUniqueDecorationBudgetAndBoxCells(t *testing.T) {
	makeInput := func(bindings int) Input {
		in := budgetInput(17, 2)
		in.Layouts[0].Entries = []Cell{{0, 1}}
		g := Geometry{ID: "attachment.support", Width: 17, Depth: 1, Height: 3}
		for i := 0; i < 17; i++ {
			g.Sockets = append(g.Sockets, Socket{ID: fmt.Sprintf("socket.%d", i), At: Cell{i, 0}, Z: 2, Width: 1, Depth: 1, Height: 1})
		}
		in.Catalog.Geometries = []Geometry{g}
		in.Catalog.Modules = []Module{{ID: "support", GeometryID: g.ID, Variants: []Variant{{ID: "base", GeometryID: g.ID}}}}
		in.Catalog.Events = nil
		for i := 0; i < 32; i++ {
			in.Catalog.Events = append(in.Catalog.Events, EventState{ID: fmt.Sprintf("event.%d", i)})
		}
		instance := Instance{ID: "attachment.instance", ModuleID: "support", Reason: "nonblocking attachment sockets"}
		for i := 0; i < bindings; i++ {
			instance.Decorations = append(instance.Decorations, DecorationBinding{ID: fmt.Sprintf("panel.%d", i), EventID: fmt.Sprintf("event.%d", i%32), SocketID: fmt.Sprintf("socket.%d", i/32), DecorationID: "small.panel", Reason: "removable presentation box"})
		}
		in.Layouts[0].Instances = []Instance{instance}
		return in
	}
	out := requireRuntime(t, makeInput(RuntimeDecorationBudget), budgetOptions())
	unique := map[string]bool{}
	for _, view := range out.Levels[0].Views {
		for _, item := range view.Decorations {
			unique[item.ID] = true
		}
	}
	if len(unique) != RuntimeDecorationBudget {
		t.Fatal("logical bindings were counted once per time state instead of once per ID")
	}
	_, err := CompileRuntime(makeInput(RuntimeDecorationBudget+1), budgetOptions())
	requireCode(t, err, "runtime_decoration_budget")
	// Valid dimensions by themselves are insufficient if a decorative box would
	// exceed the renderer's aggregate cell budget. Overflow cannot wrap it small.
	in := runtimeTestInput()
	in.Catalog.Decorations[0].Width = int(^uint(0) >> 1)
	in.Catalog.Decorations[0].Depth = int(^uint(0) >> 1)
	_, err = CompileRuntime(in, budgetOptions())
	requireCode(t, err, "runtime_geometry_budget")
}
