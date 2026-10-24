package locationsdk

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func testCatalog() Catalog {
	box := Geometry{ID: "shape.box", Width: 2, Depth: 1, Height: 3, Footprint: []Cell{{0, 0}, {1, 0}}, Blocked: []Cell{{0, 0}, {1, 0}}, UseClearance: []Cell{{0, 1}, {1, 1}, {0, 2}, {1, 2}}, MaintenanceClearance: []Cell{{0, -1}, {1, -1}}, Ports: []Port{{ID: "front", Kind: "use", At: Cell{0, 1}, Side: South, Approach: []Cell{{0, 1}, {1, 1}, {0, 2}, {1, 2}}}}, Sockets: []Socket{{ID: "top", At: Cell{0, 0}, Z: 2, Width: 2, Depth: 1, Height: 1}}}
	wall := Geometry{ID: "shape.wall", Width: 1, Depth: 10, Height: 6}
	for y := 0; y < wall.Depth; y++ {
		wall.Footprint = append(wall.Footprint, Cell{0, y})
		wall.Blocked = append(wall.Blocked, Cell{0, y})
	}
	door := Geometry{ID: "shape.opening", Width: 2, Depth: 1, Height: 6, Ports: []Port{{ID: "front", Kind: "door", At: Cell{0, 0}, Opening: []Cell{{0, 0}, {1, 0}}, Side: South, Approach: []Cell{{0, 0}}}}}
	c := Catalog{UnitMM: UnitMM, Geometries: []Geometry{box, wall, door}, Times: []TimeState{{ID: "bright", Brightness: 1, ActivityTags: []string{"awake"}}, {ID: "dim", Brightness: .64, ActivityTags: []string{"quiet"}}}, Events: []EventState{{ID: "normal"}, {ID: "festival", ActivityTags: []string{"gather"}}}, Decorations: []Decoration{{ID: "small.panel", Width: 1, Depth: 1, Height: 1}}}
	for _, g := range c.Geometries {
		c.Modules = append(c.Modules, Module{ID: "module." + g.ID, GeometryID: g.ID, Variants: []Variant{{ID: "a", GeometryID: g.ID}, {ID: "b", GeometryID: g.ID}}})
	}
	return c
}

func testLayouts() []Layout {
	return []Layout{
		{ID: "author.strip", UnitMM: UnitMM, Seed: 77, Width: 14, Depth: 10, Grounded: true, Floor: []FloorArea{{ID: "all", Rect: Rect{0, 0, 14, 10}, Reason: "authored supported floor"}}, Entries: []Cell{{0, 5}}, Instances: []Instance{{ID: "cabinet.first", ModuleID: "module.shape.box", At: Cell{3, 2}, Reason: "equipment with front and rear service", OwnerRef: "operator", Fate: ItemFate{Policy: "keep"}, Decorations: []DecorationBinding{{ID: "panel.first", EventID: "festival", SocketID: "top", DecorationID: "small.panel", Reason: "removable event panel"}}}, {ID: "cabinet.second", ModuleID: "module.shape.box", At: Cell{9, 2}, Reason: "second equipment bay"}}, ReservedPaths: []ReservedPath{{ID: "public", Points: []Cell{{1, 5}, {11, 5}}, Width: 2, Reason: "clear walking route"}}},
		{ID: "author.clinic", UnitMM: UnitMM, Seed: 88, Width: 16, Depth: 12, Grounded: true, Floor: []FloorArea{{ID: "north", Rect: Rect{0, 0, 6, 12}, Reason: "supported front wing"}, {ID: "south", Rect: Rect{0, 6, 16, 6}, Reason: "supported side wing"}}, Entries: []Cell{{0, 2}}, Instances: []Instance{{ID: "care.terminal", ModuleID: "module.shape.box", At: Cell{10, 10}, Rotation: 180, Reason: "terminal faces north with rear clearance"}}, ReservedPaths: []ReservedPath{{ID: "care.route", Points: []Cell{{2, 1}, {2, 7}, {13, 7}}, Width: 2, Reason: "continuous care route"}}},
		{ID: "author.junction", UnitMM: UnitMM, Seed: 99, Width: 18, Depth: 14, Floor: []FloorArea{{ID: "cross", Rect: Rect{0, 5, 18, 4}, Reason: "cross corridor"}, {ID: "branch", Rect: Rect{6, 0, 5, 14}, Reason: "connected branch"}}, Entries: []Cell{{1, 6}}, Instances: []Instance{{ID: "rotated.terminal", ModuleID: "module.shape.box", At: Cell{8, 10}, Rotation: 90, Reason: "terminal fronts west into a clear branch bay"}}, ReservedPaths: []ReservedPath{{ID: "cross.route", Points: []Cell{{1, 6}, {16, 6}}, Width: 1, Reason: "clear cross route"}, {ID: "branch.route", Points: []Cell{{9, 1}, {9, 12}}, Width: 1, Reason: "clear branch route"}}, Drops: []DropAnnotation{{ID: "lower.ledge", At: Cell{17, 8}, Side: South, LowerElevation: -4, Barrier: true, Reason: "explicit ledge above a lower service deck"}}},
	}
}

func copyValue[T any](t *testing.T, value T) T {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func requireBuilt(t *testing.T, c Catalog, l Layout) CompiledLevel {
	t.Helper()
	out, err := Build(c, l)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBuildThreeDifferentAuthoredLayoutsFromOneKit(t *testing.T) {
	c := testCatalog()
	hashes := map[string]bool{}
	for _, l := range testLayouts() {
		t.Run(l.ID, func(t *testing.T) {
			out := requireBuilt(t, c, l)
			if len(out.Terrain) != l.Depth || len(out.Terrain[0]) != l.Width || len(out.Instances) != len(l.Instances) || len(out.Views) != 4 || len(out.StructuralHash) != 64 || hashes[out.StructuralHash] {
				t.Fatal("missing or shared layout-specific compiled geometry")
			}
			hashes[out.StructuralHash] = true
			if l.Grounded && len(out.Barriers) != 0 {
				t.Fatal("grounded room acquired an automatic fence")
			}
			for _, view := range out.Views {
				if view.StructuralHash != out.StructuralHash {
					t.Fatal("presentation copied/changed geometry")
				}
				if view.TimeID == "dim" && view.Brightness != .64 {
					t.Fatal("festival erased the night axis")
				}
			}
			for _, item := range out.Instances {
				if item.Reason == "" || item.OwnerRef == "" || item.Fate.Policy == "" {
					t.Fatal("generated item lost authoring purpose or explicit unknown metadata")
				}
			}
			for _, floor := range out.Preview.Floor {
				if floor.SourceID == "" || floor.Reason == "" {
					t.Fatal("generated floor has no explanation")
				}
			}
			for _, b := range out.Barriers {
				if b.SourceID == "" || b.Reason == "" {
					t.Fatal("barrier was not explained by an authored drop")
				}
			}
		})
	}
}

func TestBuildRotationTransformsPhysicalAndFrontServiceGeometry(t *testing.T) {
	c := testCatalog()
	for _, rotation := range []int{0, 90, 180, 270} {
		l := testLayouts()[0]
		l.Instances = []Instance{{ID: "turn", ModuleID: "module.shape.box", At: Cell{6, 4}, Rotation: rotation, Reason: "rotation calibration"}}
		l.ReservedPaths = nil
		out := requireBuilt(t, c, l)
		want := map[int]struct {
			footprint, use, maintenance []Cell
			side                        Side
		}{
			0:   {[]Cell{{6, 4}, {7, 4}}, []Cell{{6, 5}, {7, 5}, {6, 6}, {7, 6}}, []Cell{{6, 3}, {7, 3}}, South},
			90:  {[]Cell{{6, 4}, {6, 5}}, []Cell{{4, 4}, {5, 4}, {4, 5}, {5, 5}}, []Cell{{7, 4}, {7, 5}}, West},
			180: {[]Cell{{6, 4}, {7, 4}}, []Cell{{6, 2}, {7, 2}, {6, 3}, {7, 3}}, []Cell{{6, 5}, {7, 5}}, North},
			270: {[]Cell{{6, 4}, {6, 5}}, []Cell{{7, 4}, {8, 4}, {7, 5}, {8, 5}}, []Cell{{5, 4}, {5, 5}}, East},
		}[rotation]
		if !reflect.DeepEqual(out.Instances[0].Footprint, want.footprint) || out.Ports[0].Side != want.side {
			t.Fatalf("rotation %d physical/port mismatch: %#v", rotation, out)
		}
		for _, clearance := range out.Clearances {
			expected := want.use
			if clearance.Kind == "maintenance" {
				expected = want.maintenance
			}
			if !reflect.DeepEqual(clearance.Cells, expected) {
				t.Fatalf("rotation%d %s got%v want%v", rotation, clearance.Kind, clearance.Cells, expected)
			}
		}
	}
}

func TestBuildIsOrderIndependentAndDoesNotAliasMutableInput(t *testing.T) {
	c, l := testCatalog(), testLayouts()[0]
	before, _ := json.Marshal(Input{c, []Layout{l}})
	baseline := requireBuilt(t, c, l)
	gold, _ := json.Marshal(baseline)
	reorderedC, reorderedL := copyValue(t, c), copyValue(t, l)
	slices.Reverse(reorderedC.Geometries)
	slices.Reverse(reorderedC.Modules)
	slices.Reverse(reorderedC.Times)
	slices.Reverse(reorderedC.Events)
	slices.Reverse(reorderedL.Instances)
	slices.Reverse(reorderedL.Floor)
	for i := range reorderedC.Modules {
		slices.Reverse(reorderedC.Modules[i].Variants)
	}
	for i := range reorderedC.Geometries {
		slices.Reverse(reorderedC.Geometries[i].Footprint)
		slices.Reverse(reorderedC.Geometries[i].Blocked)
		slices.Reverse(reorderedC.Geometries[i].Ports)
	}
	again := requireBuilt(t, reorderedC, reorderedL)
	bytesAgain, _ := json.Marshal(again)
	if string(gold) != string(bytesAgain) {
		t.Fatal("authoring record order changed canonical output/seed choices")
	}
	baseline.Instances[0].Footprint[0].X = 999
	baseline.Ports[0].Approach[0].X = 999
	baseline.Ports[0].Opening[0].X = 999
	baseline.Clearances[0].Cells[0].X = 999
	baseline.Views[0].ActivityTags[0] = "edited-output"
	after, _ := json.Marshal(Input{c, []Layout{l}})
	if string(before) != string(after) {
		t.Fatal("Build or returned slices mutated caller inputs")
	}
	rebuilt := requireBuilt(t, c, l)
	rebuiltJSON, _ := json.Marshal(rebuilt)
	if string(gold) != string(rebuiltJSON) {
		t.Fatal("mutating a prior output changed later builds")
	}
	for seed := int64(1); seed < 12; seed++ {
		l.Seed = seed
		out := requireBuilt(t, c, l)
		if out.StructuralHash != rebuilt.StructuralHash {
			t.Fatal("variant seed changed structure")
		}
	}
}

func TestInterchangeableVariantsComparePortsFootprintAndAccess(t *testing.T) {
	c, l := testCatalog(), testLayouts()[0]
	alias := copyValue(t, c.Geometries[0])
	alias.ID = "shape.equivalent"
	slices.Reverse(alias.Blocked)
	c.Geometries = append(c.Geometries, alias)
	c.Modules[0].Variants[1].GeometryID = alias.ID
	first := requireBuilt(t, c, l)
	l.Instances[0].VariantID = "b"
	second := requireBuilt(t, c, l)
	if first.StructuralHash != second.StructuralHash {
		t.Fatal("equivalent geometry reference changed structure")
	}
	for _, change := range []struct {
		name string
		edit func(*Geometry)
	}{
		{"dimensions", func(g *Geometry) { g.Width++ }},
		{"front access", func(g *Geometry) { g.UseClearance = append(g.UseClearance, Cell{0, 3}) }},
		{"maintenance access", func(g *Geometry) { g.MaintenanceClearance = append(g.MaintenanceClearance, Cell{0, -2}) }},
		{"port side", func(g *Geometry) { g.Ports[0].Side = East }},
		{"physical footprint", func(g *Geometry) { g.Footprint = []Cell{{0, 0}}; g.Blocked = []Cell{{0, 0}} }},
	} {
		t.Run(change.name, func(t *testing.T) {
			bad := copyValue(t, c)
			change.edit(&bad.Geometries[len(bad.Geometries)-1])
			_, err := Build(bad, l)
			requireCode(t, err, "variant_incompatible")
		})
	}
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var invalid *ValidationError
	if !errors.As(err, &invalid) || len(invalid.Diagnostics) == 0 || invalid.Diagnostics[0].Code != code || invalid.Diagnostics[0].Path == "" || invalid.Diagnostics[0].Message == "" {
		t.Fatalf("wanted machine-readable %s, got %#v (%v)", code, invalid, err)
	}
}

func TestBuildRejectsInvalidGeometryAccessAndUnsupportedFeatures(t *testing.T) {
	tests := []struct {
		name, code string
		edit       func(*Catalog, *Layout)
	}{
		{"units", "unsupported_units", func(c *Catalog, l *Layout) { l.UnitMM = 1000 }},
		{"decks", "unsupported_decks", func(c *Catalog, l *Layout) { l.Decks = []Deck{{ID: "other", Elevation: 6}} }},
		{"duplicate instances", "duplicate_or_invalid_instance", func(c *Catalog, l *Layout) { l.Instances = append(l.Instances, l.Instances[0]) }},
		{"overlap", "physical_overlap", func(c *Catalog, l *Layout) { l.Instances[1].At = l.Instances[0].At }},
		{"blocked use approach", "clearance_obstructed", func(c *Catalog, l *Layout) { l.Instances[1].At = Cell{3, 3} }},
		{"blocked maintenance approach", "clearance_obstructed", func(c *Catalog, l *Layout) { l.Instances[1].At = Cell{3, 1} }},
		{"reserved route", "reserved_path_obstructed", func(c *Catalog, l *Layout) { l.Instances[1].At = Cell{9, 5} }},
		{"unsupported rotation", "unsupported_rotation", func(c *Catalog, l *Layout) { l.Instances[0].Rotation = 45 }},
		{"no purpose", "missing_reason", func(c *Catalog, l *Layout) { l.Instances[0].Reason = " " }},
		{"unknown owner syntax", "owner_metadata", func(c *Catalog, l *Layout) { l.Instances[0].OwnerRef = "?" }},
		{"missing fate event", "fate_metadata", func(c *Catalog, l *Layout) { l.Instances[0].Fate = ItemFate{Policy: "until_event"} }},
		{"unknown module", "unknown_module", func(c *Catalog, l *Layout) { l.Instances[0].ModuleID = "absent" }},
		{"unknown variant", "unknown_variant", func(c *Catalog, l *Layout) { l.Instances[0].VariantID = "absent" }},
		{"out of bounds", "instance_out_of_bounds", func(c *Catalog, l *Layout) { l.Instances[0].At = Cell{l.Width - 1, 2} }},
		{"void support", "unsupported_instance", func(c *Catalog, l *Layout) { l.Floor[0].Rect = Rect{0, 0, 6, 10} }},
		{"diagonal route", "reserved_path_geometry", func(c *Catalog, l *Layout) { l.ReservedPaths[0].Points[1].Y++ }},
		{"route width outside floor", "reserved_path_geometry", func(c *Catalog, l *Layout) { l.ReservedPaths[0].Width = 8 }},
		{"entry occupied", "entry_obstructed", func(c *Catalog, l *Layout) { l.Entries[0] = l.Instances[0].At }},
		{"missing entry", "entry_required", func(c *Catalog, l *Layout) { l.Entries = nil }},
		{"disconnected floor", "floor_disconnected", func(c *Catalog, l *Layout) {
			l.Instances = nil
			l.ReservedPaths = nil
			l.Floor = []FloorArea{{ID: "one", Rect: Rect{0, 0, 3, 10}, Reason: "one wing"}, {ID: "two", Rect: Rect{8, 0, 3, 10}, Reason: "unconnected wing"}}
		}},
		{"interior drop", "drop_not_at_floor_edge", func(c *Catalog, l *Layout) {
			l.Drops = []DropAnnotation{{ID: "bad", At: Cell{5, 5}, Side: South, LowerElevation: -4, Barrier: true, Reason: "incorrect interior fence"}}
		}},
		{"unverified variant art", "unsupported_unverified_asset", func(c *Catalog, l *Layout) { c.Modules[0].Variants[0].ImageRef = "image.png" }},
		{"unverified decorative art", "unsupported_unverified_asset", func(c *Catalog, l *Layout) { c.Decorations[0].ImageRef = "image.png" }},
		{"blocking holiday decor", "decoration_policy", func(c *Catalog, l *Layout) { c.Decorations[0].Blocking = true }},
		{"oversized holiday decor", "decoration_does_not_fit", func(c *Catalog, l *Layout) { c.Decorations[0].Width = 3 }},
		{"unknown event socket", "unknown_socket", func(c *Catalog, l *Layout) { l.Instances[0].Decorations[0].SocketID = "absent" }},
		{"double booked socket", "socket_occupied", func(c *Catalog, l *Layout) {
			b := l.Instances[0].Decorations[0]
			b.ID = "second.panel"
			l.Instances[0].Decorations = append(l.Instances[0].Decorations, b)
		}},
		{"nonfinite night", "time_state", func(c *Catalog, l *Layout) { c.Times[1].Brightness = math.NaN() }},
		{"geometry blocker outside material", "blocked_without_footprint", func(c *Catalog, l *Layout) { c.Geometries[0].Footprint = c.Geometries[0].Footprint[:1] }},
		{"rear approach", "port_approach_not_front", func(c *Catalog, l *Layout) {
			c.Geometries[0].Ports[0].Approach = []Cell{{0, 2}}
			c.Geometries[0].Ports[0].At = Cell{0, 3}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, l := testCatalog(), testLayouts()[0]
			test.edit(&c, &l)
			out, err := Build(c, l)
			requireCode(t, err, test.code)
			if len(out.Diagnostics) == 0 || len(out.Terrain) != 0 || len(out.Instances) != 0 {
				t.Fatal("failed compilation leaked a usable partial level or lost diagnostics")
			}
		})
	}
}

func TestBarriersRequireActualAuthoredEdgesAndNeverCrossDoors(t *testing.T) {
	c, l := testCatalog(), testLayouts()[0]
	l.Instances = nil
	l.ReservedPaths = nil
	out := requireBuilt(t, c, l)
	if len(out.Barriers) != 0 {
		t.Fatal("unannotated grounded floor got fences")
	}
	l.Drops = []DropAnnotation{{ID: "south", At: Cell{3, 9}, Side: South, LowerElevation: -6, Barrier: true, Reason: "authored lower platform"}}
	out = requireBuilt(t, c, l)
	if len(out.Barriers) != 1 || len(out.Drops) != 1 || out.Barriers[0].At != (Cell{3, 9}) || out.Barriers[0].LowerElevation != -6 {
		t.Fatal("authored edge was lost or expanded into arbitrary fences")
	}
	l.Instances = []Instance{{ID: "entry.door", ModuleID: "module.shape.opening", At: Cell{3, 9}, Reason: "real free doorway"}}
	_, err := Build(c, l)
	requireCode(t, err, "barrier_across_door")
	l.Drops[0].Barrier = false
	out = requireBuilt(t, c, l)
	if len(out.Barriers) != 0 || len(out.Drops) != 1 {
		t.Fatal("explicit no-barrier edge was silently changed")
	}
	if !strings.Contains(out.Drops[0].Reason, "platform") {
		t.Fatal("drop provenance lost")
	}
}

func TestWideDoorRejectsBarrierOnEveryRotatedOpeningCell(t *testing.T) {
	for _, tc := range []struct {
		rotation          int
		origin, nonAnchor Cell
		side              Side
	}{
		{0, Cell{3, 9}, Cell{4, 9}, South},
		{90, Cell{0, 3}, Cell{0, 4}, West},
		{180, Cell{3, 0}, Cell{3, 0}, North},
		{270, Cell{13, 3}, Cell{13, 3}, East},
	} {
		c, l := testCatalog(), testLayouts()[0]
		l.ReservedPaths = nil
		l.Instances = []Instance{{ID: "wide.door", ModuleID: "module.shape.opening", At: tc.origin, Rotation: tc.rotation, Reason: "one-metre doorway"}}
		out := requireBuilt(t, c, l)
		if out.Ports[0].At == tc.nonAnchor || out.Ports[0].Side != tc.side || !slices.Contains(out.Ports[0].Opening, tc.nonAnchor) {
			t.Fatalf("rotation%d did not preserve the complete authored opening: %#v", tc.rotation, out.Ports)
		}
		l.Drops = []DropAnnotation{{ID: "illegal.rail", At: tc.nonAnchor, Side: tc.side, LowerElevation: -4, Barrier: true, Reason: "would cross second doorway cell"}}
		_, err := Build(c, l)
		requireCode(t, err, "barrier_across_door")
	}
}

func TestBuildStableInstanceSeedDoesNotDependOnUnrelatedAdditions(t *testing.T) {
	c, l := testCatalog(), testLayouts()[0]
	before := requireBuilt(t, c, l)
	l.Instances = append(l.Instances, Instance{ID: "unrelated", ModuleID: "module.shape.box", At: Cell{1, 7}, Rotation: 180, Reason: "separate service bay"})
	after := requireBuilt(t, c, l)
	for _, old := range before.Instances {
		for _, now := range after.Instances {
			if old.ID == now.ID && old.VariantID != now.VariantID {
				t.Fatal("adding another item changed prior deterministic variants")
			}
		}
	}
}

func TestSocketEntireBoxMustFitModuleAndRotatesWithAttachment(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Socket)
	}{
		{"x extent beyond wall", func(s *Socket) { s.At.X = 1 }},
		{"y extent beyond wall", func(s *Socket) { s.At.Y = 1; s.Depth = 2 }},
		{"overflowing x size", func(s *Socket) { s.Width = int(^uint(0) >> 1) }},
		{"overflowing y size", func(s *Socket) { s.Depth = int(^uint(0) >> 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, l := testCatalog(), testLayouts()[0]
			c.Geometries[0].Depth = 2
			tc.edit(&c.Geometries[0].Sockets[0])
			_, err := Build(c, l)
			requireCode(t, err, "socket_geometry")
		})
	}
	for _, tc := range []struct {
		rotation int
		at       Cell
		w, d     int
	}{
		{0, Cell{8, 5}, 4, 3},
		{90, Cell{7, 6}, 3, 4},
		{180, Cell{7, 5}, 4, 3},
		{270, Cell{7, 5}, 3, 4},
	} {
		c, l := testCatalog(), testLayouts()[0]
		c.Geometries[0].Width, c.Geometries[0].Depth = 4, 3
		c.Geometries[0].Sockets[0] = Socket{ID: "top", At: Cell{2, 1}, Z: 2, Width: 2, Depth: 2, Height: 1}
		c.Decorations[0].Width, c.Decorations[0].Depth = 2, 2
		l.Instances = l.Instances[:1]
		l.Instances[0].At = Cell{6, 4}
		l.Instances[0].Rotation = tc.rotation
		l.ReservedPaths = nil
		out := requireBuilt(t, c, l)
		if out.Instances[0].Width != tc.w || out.Instances[0].Depth != tc.d {
			t.Fatalf("rotation%d changed enclosing module bounds", tc.rotation)
		}
		for _, view := range out.Views {
			if view.EventID != "festival" {
				if len(view.Decorations) != 0 {
					t.Fatal("ordinary event gained an attachment")
				}
				continue
			}
			if len(view.Decorations) != 1 || view.Decorations[0].At != tc.at || view.Decorations[0].Z != 2 {
				t.Fatalf("exact-fitting attachment did not follow rotation%d: %#v", tc.rotation, view.Decorations)
			}
		}
	}
}
