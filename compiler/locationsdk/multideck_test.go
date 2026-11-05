package locationsdk

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func cellp(x, y int) *Cell { return &Cell{x, y} }

// testMultideck: 20×22 plan, north wall, a 3 m gallery at +3.5 m on wall
// consoles and a front beam on two columns, one 1.5 m stair, a railing.
func testMultideck() Layout {
	return Layout{
		ID: "test.multideck", UnitMM: UnitMM, Seed: 1, Width: 20, Depth: 22, Grounded: true, BaseSurfaceID: "floor",
		Floor:         []FloorArea{{ID: "slab", Rect: Rect{0, 0, 20, 22}, Reason: "ground"}},
		Entries:       []Cell{{16, 20}},
		CutawayGroups: []CutawayGroup{{ID: "gallery", Policy: "occluding", Reason: "fade when hiding the hero"}},
		Decks: []Deck{{ID: "gallery", Kind: "deck", ElevationMM: 3500, SlabMM: 250, Walkable: true, CutawayGroup: "gallery",
			Floor: []FloorArea{{ID: "g", Rect: Rect{1, 1, 18, 6}, Reason: "gallery"}}, Reason: "upper walkway"}},
		Connectors: []Connector{{ID: "stair", Kind: "stair", From: SurfaceRef{Surface: "floor", Cell: Cell{4, 18}}, To: SurfaceRef{Surface: "gallery", Cell: Cell{4, 6}}, Direction: North, Width: 3,
			Flights: []StairFlight{{Risers: 20, Length: 11}}, Balustrades: Balustrades{Left: true, Right: true, HeightMM: 1100}, Reason: "public stair"}},
		Supports: []Support{
			{ID: "console.north", Kind: "wall_console", Surface: "gallery", From: cellp(1, 1), To: cellp(18, 1), Reason: "wall consoles"},
			{ID: "col.a", Kind: "column", Surface: "gallery", At: cellp(1, 6), RestsOn: "floor", Reason: "column"},
			{ID: "col.b", Kind: "column", Surface: "gallery", At: cellp(10, 6), RestsOn: "floor", Reason: "column"},
			{ID: "col.c", Kind: "column", Surface: "gallery", At: cellp(18, 6), RestsOn: "floor", Reason: "column"},
			{ID: "beam.front", Kind: "beam", Surface: "gallery", From: cellp(1, 6), To: cellp(18, 6), Reason: "front beam"},
		},
		Railings: []Railing{
			{ID: "rail.front.w", Surface: "gallery", From: Cell{1, 6}, To: Cell{3, 6}, Side: South, HeightMM: 1100, BlocksSight: true, Reason: "front"},
			{ID: "rail.front.e", Surface: "gallery", From: Cell{7, 6}, To: Cell{18, 6}, Side: South, HeightMM: 1100, BlocksSight: true, Reason: "front"},
			{ID: "rail.west", Surface: "gallery", From: Cell{1, 1}, To: Cell{1, 6}, Side: West, HeightMM: 1100, Reason: "end"},
			{ID: "rail.east", Surface: "gallery", From: Cell{18, 1}, To: Cell{18, 6}, Side: East, HeightMM: 1100, Reason: "end"},
		},
		Obstacles: []Obstacle{{ID: "wall.north", Surface: "floor", Rect: Rect{0, 0, 20, 1}, TopMM: 8000, BlocksMovement: true, BlocksSight: true, Style: "wall", Reason: "north wall"}},
		Residents: []Resident{{ID: "npc.watch", Name: "Watch", Reason: "post on the gallery", Steps: []ResidentStep{
			{AtMinute: 0, At: Cell{12, 11}, Activity: "rest", Label: "below"},
			{AtMinute: 2, Surface: "gallery", At: Cell{12, 3}, Activity: "post", Label: "gallery post"},
		}}},
	}
}

func multideckCatalog() Catalog {
	c := testCatalog()
	return c
}

func TestMultideckCompilesNavigationPlane(t *testing.T) {
	out := requireBuilt(t, multideckCatalog(), testMultideck())
	n := out.Navigation
	if n == nil || n.Schema != NavigationSchema || n.PlanWidth != 20 || n.PlanDepth != 22 {
		t.Fatalf("navigation missing: %+v", n)
	}
	// Bands: base row 0, gallery row 15; stair block after; links at both ends.
	var gallery NavSurface
	for _, s := range n.Surfaces {
		if s.ID == "gallery" {
			gallery = s
		}
	}
	if gallery.NavRow != 23 || n.Surfaces[0].ID != "floor" || n.Surfaces[0].NavRow != 0 {
		t.Fatalf("unexpected bands: %+v", n.Surfaces)
	}
	if len(n.Connectors) != 1 || len(n.Links) != 6 {
		t.Fatalf("expected one stair with 3 lanes × 2 links: %+v", n.Links)
	}
	c := n.Connectors[0]
	if c.Length != 11 || len(c.Nodes) != 33 || c.TopMM-c.BottomMM != 3500 {
		t.Fatalf("stair geometry: %+v", c)
	}
	for _, node := range c.Nodes {
		if out.Terrain[node.Nav.Y][node.Nav.X] != '.' || out.Terrain[node.Plan.Y][node.Plan.X] != '#' {
			t.Fatal("stair node must be walkable in its block and its body blocked on the base")
		}
		if node.HeightMM <= 0 || node.HeightMM >= 3500 {
			t.Fatalf("node height %d", node.HeightMM)
		}
	}
	if out.Terrain[23+3][12] != '.' || out.Terrain[23+8][12] != ' ' {
		t.Fatal("gallery band must carry slab cells and void elsewhere")
	}
	if len(out.Residents) != 1 || out.Residents[0].Steps[1].Nav != (Cell{12, 26}) {
		t.Fatalf("resident gallery post must map into the gallery band: %+v", out.Residents)
	}
	again := requireBuilt(t, multideckCatalog(), testMultideck())
	if again.StructuralHash != out.StructuralHash {
		t.Fatal("multideck hash must be deterministic")
	}
	edited := testMultideck()
	edited.Decks[0].Reason = "different prose"
	if requireBuilt(t, multideckCatalog(), edited).StructuralHash != out.StructuralHash {
		t.Fatal("prose must stay outside structural identity")
	}
	edited.Decks[0].ElevationMM = 3400
	edited.Connectors[0].Flights[0].Risers = 20
	if built, err := Build(multideckCatalog(), edited); err == nil && built.StructuralHash == out.StructuralHash {
		t.Fatal("elevation must change structural identity")
	}
}

func TestPlanarLayoutsKeepTheirStructuralBytes(t *testing.T) {
	c := testCatalog()
	for _, l := range testLayouts() {
		out := requireBuilt(t, c, l)
		if out.Navigation != nil || out.Residents != nil {
			t.Fatal("planar layout must not gain navigation")
		}
		b, _ := json.Marshal(l)
		if strings.Contains(string(b), "connectors") || strings.Contains(string(b), "residents") || strings.Contains(string(b), "base_surface_id") {
			t.Fatal("planar layout JSON must not change: multi-deck fields are omitempty")
		}
	}
}

func TestMultideckRejectsUnsafeOrUnsupportedGeometry(t *testing.T) {
	tests := []struct {
		name, code string
		edit       func(*Layout)
	}{
		{"floating slab", "unsupported_slab", func(l *Layout) { l.Supports = nil }},
		{"far from support", "unsupported_slab", func(l *Layout) { l.Supports = l.Supports[:1] }},
		{"beam span", "beam_span", func(l *Layout) { l.Supports = append(l.Supports[:2], l.Supports[3:]...) }},
		{"beam end unsupported", "unsupported_beam", func(l *Layout) { l.Supports[4].To = cellp(17, 6) }},
		{"column outside slab", "unsupported_column", func(l *Layout) { l.Supports[2].At = cellp(10, 8) }},
		{"obstacle on stair", "stair_overlap", func(l *Layout) {
			l.Obstacles = append(l.Obstacles, Obstacle{ID: "crate", Surface: "floor", Rect: Rect{5, 10, 1, 1}, TopMM: 900, BlocksMovement: true, Style: "panel", Reason: "crate"})
		}},
		{"hanger reserved", "unsupported_support_kind", func(l *Layout) { l.Supports[1].Kind = "hanger" }},
		{"riser too high", "stair_riser", func(l *Layout) { l.Connectors[0].Flights[0].Risers = 18 }},
		{"going too short", "stair_going", func(l *Layout) {
			l.Connectors[0].To.Cell = Cell{4, 7}
			l.Connectors[0].Flights[0].Length = 10
		}},
		{"stair too narrow", "stair_width", func(l *Layout) { l.Connectors[0].Width = 1 }},
		{"stair descends", "stair_direction", func(l *Layout) {
			c := &l.Connectors[0]
			c.From, c.To, c.Direction = c.To, c.From, South
		}},
		{"misaligned landings", "connector_geometry", func(l *Layout) { l.Connectors[0].To.Cell = Cell{5, 6} }},
		{"open stair side", "stair_unprotected_side", func(l *Layout) { l.Connectors[0].Balustrades.Left = false }},
		{"landing blocked", "connector_landing", func(l *Layout) {
			l.Obstacles = append(l.Obstacles, Obstacle{ID: "crate", Surface: "floor", Rect: Rect{4, 18, 1, 1}, TopMM: 900, BlocksMovement: true, Style: "panel", Reason: "crate"})
		}},
		{"ladder reserved", "unsupported_connector_kind", func(l *Layout) { l.Connectors[0].Kind = "ladder" }},
		{"lift is transport", "transport_connector_reserved", func(l *Layout) { l.Connectors[0].Kind = "lift" }},
		{"other location", "cross_location_connector", func(l *Layout) { l.Connectors[0].To.Location = "elsewhere" }},
		{"missing railing", "unprotected_deck_edge", func(l *Layout) { l.Railings = l.Railings[1:] }},
		{"railing across stair head", "railing_across_connector", func(l *Layout) { l.Railings[0].To = Cell{4, 6} }},
		{"railing off edge", "railing_not_at_edge", func(l *Layout) { l.Railings[2].Side = East }},
		{"low railing", "railing_definition", func(l *Layout) { l.Railings[0].HeightMM = 900 }},
		{"low headroom under beam", "insufficient_headroom", func(l *Layout) { l.Supports[4].DepthMM = 1200 }},
		{"stair under slab", "insufficient_headroom", func(l *Layout) {
			l.Decks = append(l.Decks, Deck{ID: "canopy", Kind: "roof", ElevationMM: 4500, SlabMM: 250, CutawayGroup: "gallery", Floor: []FloorArea{{ID: "c", Rect: Rect{4, 7, 4, 3}, Reason: "canopy"}}, Reason: "low canopy over the stair head"})
			l.Supports = append(l.Supports, Support{ID: "col.canopy", Kind: "column", Surface: "canopy", At: cellp(7, 8), RestsOn: "floor", Reason: "canopy post"})
		}},
		{"unreachable deck", "surface_unreachable", func(l *Layout) {
			l.Connectors = nil
			l.Railings = append(l.Railings, Railing{ID: "rail.mid", Surface: "gallery", From: Cell{4, 6}, To: Cell{6, 6}, Side: South, HeightMM: 1100, Reason: "closed"})
		}},
		{"heightfield reserved", "unsupported_surface_geometry", func(l *Layout) { l.Decks[0].Geometry = "heightfield" }},
		{"ground reserved", "unsupported_ground_surface", func(l *Layout) { l.Decks[0].Kind = "ground" }},
		{"no cutaway group", "deck_cutaway_group", func(l *Layout) { l.Decks[0].CutawayGroup = "absent" }},
		{"guides rejected", "unsupported_multideck_guides", func(l *Layout) {
			l.ReservedPaths = []ReservedPath{{ID: "p", Points: []Cell{{15, 20}, {18, 20}}, Width: 1, Reason: "lane"}}
		}},
		{"obstacle through slab", "obstacle_crosses_slab", func(l *Layout) {
			l.Obstacles = append(l.Obstacles, Obstacle{ID: "pole", Surface: "floor", Rect: Rect{12, 4, 1, 1}, TopMM: 5000, BlocksMovement: true, BlocksSight: true, Style: "wall", Reason: "pole"})
		}},
		{"obstacle without effect", "obstacle_without_effect", func(l *Layout) { l.Obstacles[0].BlocksMovement, l.Obstacles[0].BlocksSight = false, false }},
		{"duplicate element id", "duplicate_element_id", func(l *Layout) { l.Railings[0].ID = "stair" }},
		{"slab overlap", "slab_overlap", func(l *Layout) {
			l.Decks = append(l.Decks, Deck{ID: "mezz", Kind: "roof", ElevationMM: 3400, SlabMM: 250, CutawayGroup: "gallery", Floor: []FloorArea{{ID: "m", Rect: Rect{12, 2, 2, 2}, Reason: "m"}}, Reason: "bad"})
		}},
		{"resident on void", "resident_post_blocked", func(l *Layout) { l.Residents[0].Steps[1].At = Cell{12, 10} }},
		{"resident at spawn", "resident_overlap", func(l *Layout) { l.Residents[0].Steps[0].At = Cell{16, 20} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := testMultideck()
			tt.edit(&l)
			_, err := Build(multideckCatalog(), l)
			var invalid *ValidationError
			if !errors.As(err, &invalid) || invalid.Diagnostics[0].Code != tt.code {
				t.Fatalf("expected %s, got %v", tt.code, err)
			}
		})
	}
}

func TestMultideckRuntimeCarriesNavigationPlane(t *testing.T) {
	bundle, err := CompileRuntime(Input{Catalog: multideckCatalog(), Layouts: []Layout{testMultideck()}}, RuntimeOptions{TickHz: 10, MoveEveryTicks: 2})
	if err != nil {
		t.Fatal(err)
	}
	level := bundle.Levels[0]
	if !level.NavigationAuthored || level.Navigation == nil || level.Width != level.Navigation.NavWidth || level.Depth != level.Navigation.NavDepth || len(level.Terrain) != level.Depth || len(level.Residents) != 1 {
		t.Fatalf("runtime level must expose the whole navigation plane: %dx%d", level.Width, level.Depth)
	}
}

// demo-hub: the near cut of the camera lists existing base-surface obstacles only.
func TestCameraNearCutRejectsUnknownAndUpperObstacles(t *testing.T) {
	base := testMultideck()
	if len(base.Obstacles) == 0 {
		t.Fatal("fixture has no obstacle")
	}
	var floor, upper string
	for _, o := range base.Obstacles {
		if o.Surface == base.BaseSurfaceID && floor == "" {
			floor = o.ID
		}
		if o.Surface != base.BaseSurfaceID && upper == "" {
			upper = o.ID
		}
	}
	ok := base
	ok.Camera = &LayoutCamera{Projection: "orthographic", ElevationDegrees: 35, AzimuthDegrees: 37, NearCut: &LayoutNearCut{HeightMM: 1200, Obstacles: []string{floor}}}
	level, err := Build(multideckCatalog(), ok)
	if floor != "" && (err != nil || level.Camera == nil || level.Camera.NearCut == nil || level.Camera.NearCut.HeightMM != 1200) {
		t.Fatalf("valid near cut rejected or dropped: %v", err)
	}
	for name, cut := range map[string]*LayoutNearCut{"unknown": {HeightMM: 1200, Obstacles: []string{"missing"}}, "low": {HeightMM: 100, Obstacles: []string{floor}}, "duplicate": {HeightMM: 1200, Obstacles: []string{floor, floor}}, "upper": {HeightMM: 1200, Obstacles: []string{upper}}} {
		if name == "upper" && upper == "" || name != "unknown" && floor == "" {
			continue
		}
		bad := base
		bad.Camera = &LayoutCamera{Projection: "orthographic", ElevationDegrees: 35, AzimuthDegrees: 37, NearCut: cut}
		if _, err := Build(multideckCatalog(), bad); err == nil || !strings.Contains(err.Error(), "camera_near_cut") {
			t.Fatalf("%s near cut accepted: %v", name, err)
		}
	}
}

// demo-hub: an open steel stair keeps the same navigation and physical body, but sight passes under its treads and landing;
// only the two channel stringers stay as sight edges. The default construction is the closed solid block.
func TestSteelOpenStairIsSightClearBelowTreads(t *testing.T) {
	solid := testMultideck()
	steel := testMultideck()
	for i := range steel.Connectors {
		steel.Connectors[i].Construction = "steel_open"
	}
	a, err := Build(multideckCatalog(), solid)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(multideckCatalog(), steel)
	if err != nil {
		t.Fatal(err)
	}
	id := solid.Connectors[0].ID
	count := func(level CompiledLevel) (solids, stringers int) {
		for _, s := range level.Navigation.Solids {
			if s.Source == id {
				solids++
			}
		}
		for _, e := range level.Navigation.Edges {
			if e.Source == id+".stringer" {
				stringers++
			}
		}
		return
	}
	if s, e := count(a); s == 0 || e != 0 {
		t.Fatalf("solid stair: %d solid spans, %d stringer edges", s, e)
	}
	if s, e := count(b); s != 0 || e == 0 {
		t.Fatalf("steel stair: %d solid spans, %d stringer edges", s, e)
	}
	if strings.Join(a.Terrain, "\n") != strings.Join(b.Terrain, "\n") || len(a.Navigation.Links) != len(b.Navigation.Links) {
		t.Fatal("construction changed navigation")
	}
	bad := testMultideck()
	bad.Connectors[0].Construction = "wooden"
	if _, err := Build(multideckCatalog(), bad); err == nil {
		t.Fatal("unknown stair construction accepted")
	}
}

// demo-hub v12-vending: a tall floor fixture may join an occluding cutaway group (it fades while it hides the hero); an
// unknown group or a building group is rejected.
func TestInstanceJoinsOnlyAnOccludingCutawayGroup(t *testing.T) {
	layout := testMultideck()
	layout.CutawayGroups = append(layout.CutawayGroups, CutawayGroup{ID: "fixture", Policy: "occluding", Reason: "tall block fades while it hides the hero"})
	layout.Instances = []Instance{{ID: "block", ModuleID: "module.shape.box", At: Cell{14, 13}, Reason: "tall fixture on the floor", CutawayGroup: "fixture"}}
	out := requireBuilt(t, multideckCatalog(), layout)
	found := false
	for _, g := range out.Navigation.CutawayGroups {
		if g.ID == "fixture" {
			found = len(g.Instances) == 1 && g.Instances[0] == "block" && len(g.Surfaces) == 0 && g.TopMM > 0
		}
		if g.ID == "gallery" && len(g.Instances) != 0 {
			t.Fatal("instance leaked into another group")
		}
	}
	if !found {
		t.Fatalf("occluding group lost its instance: %+v", out.Navigation.CutawayGroups)
	}
	for name, group := range map[string]string{"unknown": "missing", "building": "house"} {
		bad := testMultideck()
		bad.CutawayGroups = append(bad.CutawayGroups, CutawayGroup{ID: "house", Policy: "building", Interior: []Rect{{12, 10, 6, 6}}, Reason: "building"})
		bad.Instances = []Instance{{ID: "block", ModuleID: "module.shape.box", At: Cell{14, 13}, Reason: "tall fixture", CutawayGroup: group}}
		if _, err := Build(multideckCatalog(), bad); err == nil || !strings.Contains(err.Error(), "instance_cutaway_group") {
			t.Fatalf("%s group accepted for an instance: %v", name, err)
		}
	}
}

func TestObstacleAboveModuleUsesPhysicalHeightInterval(t *testing.T) {
	for _, elevation := range []int{0, 4} {
		layout := testMultideck()
		layout.Elevation = elevation
		layout.Decks[0].ElevationMM += elevation * UnitMM
		layout.Instances = []Instance{{ID: "low.module", ModuleID: "module.shape.box", At: Cell{12, 12}, Reason: "low enclosed service"}}
		layout.Obstacles = append(layout.Obstacles, Obstacle{ID: "upper.wall", Surface: "floor", Rect: Rect{12, 12, 2, 1}, BottomMM: 1500, TopMM: 3500, BlocksSight: true, Style: "wall", Reason: "structural wall continues above low module"})
		out := requireBuilt(t, testCatalog(), layout)
		if out.Navigation == nil {
			t.Fatal("missing vertical navigation")
		}
		found := false
		for _, solid := range out.Navigation.Solids {
			if solid.Source == "upper.wall" && solid.BottomMM == elevation*UnitMM+1500 && solid.TopMM == elevation*UnitMM+3500 {
				found = true
			}
		}
		if !found {
			t.Fatal("upper wall vanished or changed elevation")
		}
		layout.Obstacles[len(layout.Obstacles)-1].BottomMM = 1499
		if _, err := Build(testCatalog(), layout); err == nil || !strings.Contains(err.Error(), "physical_overlap") {
			t.Fatalf("one-mm physical intersection accepted: %v", err)
		}
	}
}
