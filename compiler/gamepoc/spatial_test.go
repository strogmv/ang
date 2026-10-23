package gamepoc

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func spatialFixture() SpatialCatalog {
	return SpatialCatalog{
		Seed:   72,
		Styles: []SpatialStyle{{ID: "industrial", AssetStyle: "floor", FloorColor: "#556677", AccentColor: "#8899AA", Variants: 17}},
		Levels: []SpatialLevel{{
			ID: "market", Width: 9, Height: 7, CellMeters: 1, CoverAllFloor: true,
			Terrain: []string{"#########", "#.......#", "#..CC...#", "#.......D", "#.......#", "#.......#", "#########"},
			Zones: []SpatialZone{
				{ID: "west", Kind: "market", Rect: SpatialRect{X: 1, Y: 1, W: 4, H: 5}, StyleID: "industrial", Purpose: "Stalls"},
				{ID: "east", Kind: "service", Rect: SpatialRect{X: 5, Y: 1, W: 3, H: 5}, StyleID: "industrial", Purpose: "Services"},
			},
			Corridors: []SpatialCorridor{{ID: "main", Rect: SpatialRect{X: 1, Y: 3, W: 7, H: 2}, WidthMeters: 2}},
			Doors:     []SpatialDoor{{ID: "door", At: Point{X: 8, Y: 3}, Purpose: "Station"}},
			Exits:     []SpatialExit{{ID: "exit", At: Point{X: 8, Y: 3}, Landing: Point{X: 7, Y: 3}, Destination: "monorail"}},
			Props:     []SpatialProp{{ID: "stall", Rect: SpatialRect{X: 3, Y: 2, W: 2, H: 1}, Glyph: "C", StyleID: "industrial", Owner: "market", Purpose: "Counter"}},
			Decorations: []SpatialDecoration{
				{ID: "scuff", At: Point{X: 2, Y: 3}, StyleID: "industrial", Variant: -1},
				{ID: "mark", At: Point{X: 3, Y: 3}, StyleID: "industrial", Variant: 5},
			},
			InteractionSpots: []SpatialInteraction{{ID: "trade", At: Point{X: 3, Y: 1}}},
		}},
	}
}

func spatialLegend() map[string]bool {
	return map[string]bool{"#": false, ".": true, "C": false, "D": true}
}
func spatialValue(t *testing.T, catalog SpatialCatalog) cue.Value {
	t.Helper()
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return cuecontext.New().CompileBytes(data)
}

func TestCompileSpatialSharedGeometryAndBindings(t *testing.T) {
	input := spatialFixture()
	compiled, err := compileSpatial(spatialValue(t, input), spatialLegend())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(compiled.Levels[0].Terrain, input.Levels[0].Terrain) {
		t.Fatal("compiler changed authoritative terrain")
	}
	if compiled.Levels[0].Decorations[1].ResolvedVariant != 5 {
		t.Fatal("manual variant lost")
	}
	for _, mark := range compiled.Levels[0].Decorations {
		if mark.Surface != "floor" || mark.ResolvedVariant < 0 || mark.ResolvedVariant >= 17 {
			t.Fatalf("invalid flattened binding: %+v", mark)
		}
	}
	// The public wire catalog is JSON, not an engine-specific asset description.
	data, err := json.Marshal(compiled)
	if err != nil {
		t.Fatal(err)
	}
	var restored SpatialCatalog
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(compiled, restored) {
		t.Fatal("catalog JSON roundtrip differs")
	}
	again, err := compileSpatial(spatialValue(t, input), spatialLegend())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(compiled, again) {
		t.Fatal("compilation is not deterministic")
	}
	// IDs, not slice positions or PRNG consumption, determine automatic variants.
	input.Levels[0].Decorations[0], input.Levels[0].Decorations[1] = input.Levels[0].Decorations[1], input.Levels[0].Decorations[0]
	input.Levels[0].Zones[0], input.Levels[0].Zones[1] = input.Levels[0].Zones[1], input.Levels[0].Zones[0]
	reordered, err := compileSpatial(spatialValue(t, input), spatialLegend())
	if err != nil {
		t.Fatal(err)
	}
	if reordered.Levels[0].Decorations[1].ResolvedVariant != compiled.Levels[0].Decorations[0].ResolvedVariant || reordered.Levels[0].Zones[1].ResolvedVariant != compiled.Levels[0].Zones[0].ResolvedVariant {
		t.Fatal("catalog reordering changed bindings")
	}
	changed := false
	for seed := int64(1); seed < 20; seed++ {
		if spatialVariant(seed, "market", "decoration", "scuff", 17) != compiled.Levels[0].Decorations[0].ResolvedVariant {
			changed = true
		}
	}
	if !changed {
		t.Fatal("seed has no influence")
	}
}

func TestCompileSpatialRejectsInvalidGeometry(t *testing.T) {
	cases := []struct {
		name, want string
		mutate     func(*SpatialCatalog)
	}{
		{"cell_size", "cell_meters", func(c *SpatialCatalog) { c.Levels[0].CellMeters = 0.5 }},
		{"overflow", "dimensions", func(c *SpatialCatalog) { c.Levels[0].Width = math.MaxInt }},
		{"height", "height", func(c *SpatialCatalog) { c.Levels[0].Height = 8 }},
		{"row_width", "row", func(c *SpatialCatalog) { c.Levels[0].Terrain[0] = "###" }},
		{"unknown_glyph", "glyph", func(c *SpatialCatalog) { c.Levels[0].Terrain[0] = "####?####" }},
		{"empty_floor", "no walkable", func(c *SpatialCatalog) {
			for i := range c.Levels[0].Terrain {
				c.Levels[0].Terrain[i] = "#########"
			}
		}},
		{"duplicate_level", "duplicate level", func(c *SpatialCatalog) { c.Levels = append(c.Levels, c.Levels[0]) }},
		{"duplicate_style", "duplicate style", func(c *SpatialCatalog) { c.Styles = append(c.Styles, c.Styles[0]) }},
		{"bad_color", "colors", func(c *SpatialCatalog) { c.Styles[0].FloorColor = "#ZZ0000" }},
		{"bad_variant_count", "variants", func(c *SpatialCatalog) { c.Styles[0].Variants = 0 }},
		{"zone_bounds", "invalid rect", func(c *SpatialCatalog) { c.Levels[0].Zones[0].Rect.X = -1 }},
		{"zone_overlap", "overlaps", func(c *SpatialCatalog) { c.Levels[0].Zones[1].Rect.X = 4 }},
		{"zone_gap", "no zone", func(c *SpatialCatalog) { c.Levels[0].Zones[0].Rect.W = 3 }},
		{"unknown_style", "unknown style", func(c *SpatialCatalog) { c.Levels[0].Zones[0].StyleID = "missing" }},
		{"duplicate_id", "duplicate", func(c *SpatialCatalog) { c.Levels[0].Doors[0].ID = "west" }},
		{"width_m", "width_m", func(c *SpatialCatalog) { c.Levels[0].Corridors[0].WidthMeters = 3 }},
		{"corridor_blocked", "corridor", func(c *SpatialCatalog) { c.Levels[0].Corridors[0].Rect.Y = 2 }},
		{"door_blocked", "door", func(c *SpatialCatalog) { c.Levels[0].Doors[0].At = Point{X: 0, Y: 0} }},
		{"exit_blocked", "exit", func(c *SpatialCatalog) { c.Levels[0].Exits[0].At = Point{X: 0, Y: 0} }},
		{"landing_remote", "adjacent", func(c *SpatialCatalog) { c.Levels[0].Exits[0].Landing = Point{X: 1, Y: 1} }},
		{"landing_blocked", "exit", func(c *SpatialCatalog) { c.Levels[0].Exits[0].Landing = Point{X: 8, Y: 2} }},
		{"prop_bounds", "bounds", func(c *SpatialCatalog) { c.Levels[0].Props[0].Rect.W = 100 }},
		{"prop_mismatch", "mismatches", func(c *SpatialCatalog) { c.Levels[0].Props[0].Rect.X = 4 }},
		{"prop_walkable", "nonwalkable", func(c *SpatialCatalog) { c.Levels[0].Props[0].Glyph = "." }},
		{"prop_overlap", "overlaps", func(c *SpatialCatalog) {
			p := c.Levels[0].Props[0]
			p.ID = "second"
			c.Levels[0].Props = append(c.Levels[0].Props, p)
		}},
		{"mark_on_prop", "walkable floor", func(c *SpatialCatalog) { c.Levels[0].Decorations[0].At = Point{X: 3, Y: 2} }},
		{"mark_tall", "flat floor", func(c *SpatialCatalog) { c.Levels[0].Decorations[0].Surface = "wall" }},
		{"variant_negative", "variant", func(c *SpatialCatalog) { c.Levels[0].Decorations[0].Variant = -2 }},
		{"variant_high", "variant", func(c *SpatialCatalog) { c.Levels[0].Decorations[0].Variant = 17 }},
		{"interaction_blocked", "interaction", func(c *SpatialCatalog) { c.Levels[0].InteractionSpots[0].At = Point{X: 3, Y: 2} }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			catalog := spatialFixture()
			tt.mutate(&catalog)
			err := catalog.compile(spatialLegend())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestCompileSpatialDoorsConnectRooms(t *testing.T) {
	catalog := spatialFixture()
	level := &catalog.Levels[0]
	level.Corridors = nil
	level.Props = nil
	level.Decorations = nil
	level.InteractionSpots = nil
	level.Terrain = []string{"#########", "#...#...#", "#...#...#", "#...#...D", "#...#...#", "#...#...#", "#########"}
	if err := catalog.compile(spatialLegend()); err == nil || !strings.Contains(err.Error(), "disconnected") {
		t.Fatalf("isolated room accepted: %v", err)
	}
	level.Terrain[3] = "#...D...D"
	level.Doors = append(level.Doors, SpatialDoor{ID: "joining_door", At: Point{X: 4, Y: 3}, Purpose: "Link rooms"})
	if err := catalog.compile(spatialLegend()); err != nil {
		t.Fatal(err)
	}
	// A diagonal corner cannot serve as an invisible doorway.
	level.Terrain = []string{"#########", "#.......#", "#.......#", "#.......D", "######..#", "#.#####.#", "#########"}
	level.Doors = level.Doors[:1]
	if err := catalog.compile(spatialLegend()); err == nil || !strings.Contains(err.Error(), "disconnected") {
		t.Fatalf("isolated diagonal floor accepted: %v", err)
	}
}

func TestCompileSpatialOptionalAndDimensionsFromContent(t *testing.T) {
	absent := cuecontext.New().CompileString("{}").LookupPath(cue.ParsePath("spatial"))
	got, err := compileSpatial(absent, spatialLegend())
	if err != nil || len(got.Levels) != 0 {
		t.Fatalf("absent legacy catalog: %+v %v", got, err)
	}
	if _, err := compileSpatial(cuecontext.New().CompileString(`{levels: "wrong"}`), spatialLegend()); err == nil {
		t.Fatal("bad shape accepted")
	}
	for _, size := range [][2]int{{5, 5}, {48, 32}, {64, 18}} {
		catalog := spatialFixture()
		catalog.Levels = catalog.Levels[:1]
		w, h := size[0], size[1]
		level := SpatialLevel{ID: "arbitrary", Width: w, Height: h, CellMeters: 1, CoverAllFloor: true}
		for y := 0; y < h; y++ {
			row := strings.Repeat("#", w)
			if y > 0 && y < h-1 {
				row = "#" + strings.Repeat(".", w-2) + "#"
			}
			level.Terrain = append(level.Terrain, row)
		}
		level.Zones = []SpatialZone{{ID: "whole", Kind: "common", Rect: SpatialRect{X: 1, Y: 1, W: w - 2, H: h - 2}, StyleID: "industrial", Purpose: "Open hall"}}
		catalog.Levels[0] = level
		if err := catalog.compile(spatialLegend()); err != nil {
			t.Fatalf("%dx%d: %v", w, h, err)
		}
	}
	catalog := spatialFixture()
	catalog.Levels[0].CoverAllFloor = false
	catalog.Levels[0].Zones = nil
	if err := catalog.compile(spatialLegend()); err != nil {
		t.Fatal("optional partial zone coverage rejected:", err)
	}
}

func spatialModuleFixture() SpatialCatalog {
	catalog := spatialFixture()
	catalog.ModuleSnapMeters = 2
	capsule := SpatialModule{ID: "shop", Kind: "shop_capsule", Footprint: SpatialFootprint{W: 6, H: 4}, HeightMM: 3000, SnapMeters: 2, Placement: "ground",
		Ports:      []SpatialModulePort{{ID: "entry", At: Point{X: 2, Y: 3}, Side: "south", Kind: "entry"}},
		Clearances: []SpatialModuleClearance{{ID: "queue", Side: "south", DepthMeters: 2, Purpose: "Customers wait outside the corridor"}},
	}
	living := capsule
	living.ID = "living"
	living.Kind = "living_capsule"
	living.Placement = "definition_only"
	catalog.Modules = []SpatialModule{capsule, living,
		{ID: "gallery", Kind: "gallery", Footprint: SpatialFootprint{W: 6, H: 2}, HeightMM: 200, ElevationMM: 3000, Level: 1, SnapMeters: 2, Placement: "definition_only"},
		{ID: "landing", Kind: "landing", Footprint: SpatialFootprint{W: 2, H: 2}, HeightMM: 200, ElevationMM: 3000, Level: 1, SnapMeters: 2, Placement: "definition_only"},
		{ID: "stair", Kind: "stair", Footprint: SpatialFootprint{W: 4, H: 2}, HeightMM: 3000, RiseMM: 3000, SnapMeters: 2, Placement: "definition_only", LandingModuleID: "landing", Ports: []SpatialModulePort{{ID: "bottom", At: Point{X: 0, Y: 0}, Side: "west", Kind: "connection"}, {ID: "top", At: Point{X: 3, Y: 0}, Side: "east", Kind: "connection"}}},
		{ID: "cassette", Kind: "cassette", Footprint: SpatialFootprint{W: 1, H: 1}, HeightMM: 800, SnapMeters: 1, Placement: "definition_only"},
	}
	catalog.Levels = []SpatialLevel{{ID: "market", Width: 20, Height: 18, CellMeters: 1, CoverAllFloor: true,
		Zones:     []SpatialZone{{ID: "hall", Kind: "market", Rect: SpatialRect{X: 1, Y: 1, W: 18, H: 16}, StyleID: "industrial", Purpose: "Market"}},
		Corridors: []SpatialCorridor{{ID: "main", Rect: SpatialRect{X: 1, Y: 14, W: 18, H: 2}, WidthMeters: 2}},
		Props: []SpatialProp{
			{ID: "capsule.first", Rect: SpatialRect{X: 2, Y: 2, W: 6, H: 4}, Glyph: "C", StyleID: "industrial", Owner: "market", Purpose: "Shop", ModuleID: "shop", Clearances: []SpatialPlacedClearance{{ID: "queue", Rect: SpatialRect{X: 2, Y: 6, W: 6, H: 2}}}},
			{ID: "capsule.second", Rect: SpatialRect{X: 2, Y: 8, W: 6, H: 4}, Glyph: "C", StyleID: "industrial", Owner: "market", Purpose: "Shop", ModuleID: "shop", Rotation: 180, Clearances: []SpatialPlacedClearance{{ID: "queue", Rect: SpatialRect{X: 2, Y: 6, W: 6, H: 2}}}},
			{ID: "legacy.odd", Rect: SpatialRect{X: 15, Y: 3, W: 1, H: 1}, Glyph: "C", StyleID: "industrial", Owner: "market", Purpose: "Repaired old cabinet"},
		},
	}}
	spatialModuleTerrain(&catalog.Levels[0])
	return catalog
}

func spatialModuleTerrain(level *SpatialLevel) {
	level.Terrain = nil
	for y := 0; y < level.Height; y++ {
		row := []byte(strings.Repeat(".", level.Width))
		for x := range row {
			if x == 0 || y == 0 || x == level.Width-1 || y == level.Height-1 {
				row[x] = '#'
				continue
			}
			for _, prop := range level.Props {
				if x >= prop.Rect.X && x < prop.Rect.X+prop.Rect.W && y >= prop.Rect.Y && y < prop.Rect.Y+prop.Rect.H {
					row[x] = prop.Glyph[0]
				}
			}
		}
		level.Terrain = append(level.Terrain, string(row))
	}
}

func TestCompileSpatialModulesGroundAndDefinitions(t *testing.T) {
	catalog := spatialModuleFixture()
	got, err := compileSpatial(spatialValue(t, catalog), spatialLegend())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Modules) != 6 || len(got.Levels[0].Props) != 3 {
		t.Fatal("definitions created physical objects")
	}
	if got.Levels[0].Props[2].Rect.X != 15 {
		t.Fatal("legacy odd cell changed")
	}
	if got.Modules[5].SnapMeters != 1 {
		t.Fatal("cassette forced onto large module grid")
	}
	if !reflect.DeepEqual(got.Modules, catalog.Modules) {
		t.Fatal("prototype metadata lost")
	}
	// Facing shops may share an apron. It is a free access zone, not two private
	// occupancy claims or automatic inventory slots.
	if got.Levels[0].Props[0].Clearances[0].Rect != got.Levels[0].Props[1].Clearances[0].Rect {
		t.Fatal("shared approach changed")
	}
	for _, rotation := range []int{0, 90, 180, 270} {
		t.Run(string(rune('a'+rotation/90)), func(t *testing.T) {
			c := spatialModuleFixture()
			level := &c.Levels[0]
			level.Props = level.Props[:1]
			p := &level.Props[0]
			p.Rect = SpatialRect{X: 4, Y: 4, W: 6, H: 4}
			p.Rotation = rotation
			if rotation == 90 || rotation == 270 {
				p.Rect.W, p.Rect.H = p.Rect.H, p.Rect.W
			}
			rect := spatialClearanceRect(c.Modules[0], c.Modules[0].Clearances[0], rotation)
			rect.X += p.Rect.X
			rect.Y += p.Rect.Y
			p.Clearances[0].Rect = rect
			spatialModuleTerrain(level)
			if err := c.compile(spatialLegend()); err != nil {
				t.Fatalf("rotation %d: %v", rotation, err)
			}
		})
	}
}

func TestCompileSpatialModulePortAndApronMayShareLocalID(t *testing.T) {
	catalog := spatialModuleFixture()
	catalog.Modules[0].Clearances[0].ID = "entry"
	for i := 0; i < 2; i++ {
		catalog.Levels[0].Props[i].Clearances[0].ID = "entry"
	}
	if err := catalog.compile(spatialLegend()); err != nil {
		t.Fatal(err)
	}
}

func TestCompileSpatialModulesInvalid(t *testing.T) {
	cases := []struct {
		name, want string
		change     func(*SpatialCatalog)
	}{
		{"grid", "module_snap_m", func(c *SpatialCatalog) { c.ModuleSnapMeters = 0 }},
		{"duplicate", "duplicate module", func(c *SpatialCatalog) { c.Modules = append(c.Modules, c.Modules[0]) }},
		{"capsule_size", "footprint", func(c *SpatialCatalog) { c.Modules[0].Footprint.W = 0 }},
		{"capsule_height", "height", func(c *SpatialCatalog) { c.Modules[0].HeightMM = 0 }},
		{"gallery_elevation", "elevation", func(c *SpatialCatalog) { c.Modules[2].ElevationMM = -1 }},
		{"stair_rise", "stair", func(c *SpatialCatalog) { c.Modules[4].RiseMM = 0 }},
		{"landing_size", "footprint", func(c *SpatialCatalog) { c.Modules[3].Footprint.W = 0 }},
		{"stair_missing_landing", "landing_module_id", func(c *SpatialCatalog) { c.Modules[4].LandingModuleID = "missing" }},
		{"stair_missing_endpoint", "two connection", func(c *SpatialCatalog) { c.Modules[4].Ports = c.Modules[4].Ports[:1] }},
		{"cassette_large_grid", "snap_m", func(c *SpatialCatalog) { c.Modules[5].SnapMeters = 2 }},
		{"port_outside", "boundary", func(c *SpatialCatalog) { c.Modules[0].Ports[0].At.X = 6 }},
		{"port_wrong_side", "boundary", func(c *SpatialCatalog) { c.Modules[0].Ports[0].Side = "north" }},
		{"port_duplicate", "duplicate port", func(c *SpatialCatalog) { c.Modules[0].Ports = append(c.Modules[0].Ports, c.Modules[0].Ports[0]) }},
		{"entry_without_apron", "clearance", func(c *SpatialCatalog) { c.Modules[0].Clearances = nil }},
		{"unknown_prototype", "unknown module", func(c *SpatialCatalog) { c.Levels[0].Props[0].ModuleID = "missing" }},
		{"definition_instantiated", "definition_only", func(c *SpatialCatalog) { c.Levels[0].Props[0].ModuleID = "living" }},
		{"upper_instance", "upper-level", func(c *SpatialCatalog) { c.Levels[0].Props[0].Level = 1 }},
		{"bad_rotation", "rotation", func(c *SpatialCatalog) { c.Levels[0].Props[0].Rotation = 45 }},
		{"instance_size", "footprint", func(c *SpatialCatalog) { c.Levels[0].Props[0].Rect.W = 4 }},
		{"instance_offgrid", "snap grid", func(c *SpatialCatalog) { c.Levels[0].Props[0].Rect.X = 3 }},
		{"missing_apron", "clearances", func(c *SpatialCatalog) { c.Levels[0].Props[0].Clearances = nil }},
		{"wrong_apron", "outside module apron", func(c *SpatialCatalog) { c.Levels[0].Props[0].Clearances[0].Rect.Y = 5 }},
		{"apron_on_corridor", "main corridor", func(c *SpatialCatalog) { c.Levels[0].Corridors[0].Rect = SpatialRect{X: 1, Y: 6, W: 18, H: 2} }},
		{"blocked_apron", "clearance", func(c *SpatialCatalog) { c.Levels[0].Props[2].Rect.X = 3; c.Levels[0].Props[2].Rect.Y = 6 }},
		{"legacy_upper", "require module_id", func(c *SpatialCatalog) { c.Levels[0].Props[2].Level = 1 }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := spatialModuleFixture()
			tt.change(&c)
			spatialModuleTerrain(&c.Levels[0])
			err := c.compile(spatialLegend())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
}

func TestCompileSpatialFloorPaintShapes(t *testing.T) {
	for _, shape := range []string{"", "mark", "line_x", "line_y", "floating_wall"} {
		input := spatialFixture()
		input.Levels[0].Decorations[0].Shape = shape
		compiled, err := compileSpatial(spatialValue(t, input), spatialLegend())
		if shape == "floating_wall" {
			if err == nil {
				t.Fatal("unknown floor shape accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		want := shape
		if want == "" {
			want = "mark"
		}
		if compiled.Levels[0].Decorations[0].Shape != want {
			t.Fatal("shape lost")
		}
		if !reflect.DeepEqual(input.Levels[0].Terrain, compiled.Levels[0].Terrain) {
			t.Fatal("paint changed collision")
		}
	}
}

func TestCompileSpatialModuleDimensionsAreAuthoredData(t *testing.T) {
	catalog := spatialModuleFixture()
	// The catalog default is not a kind-specific law. Every prototype declares
	// its own spacing, including fine-grained cassettes and larger structural bays.
	catalog.ModuleSnapMeters = 3
	shop := &catalog.Modules[0]
	shop.Footprint = SpatialFootprint{W: 8, H: 4}
	shop.HeightMM = 3500
	for i := 0; i < 2; i++ {
		catalog.Levels[0].Props[i].Rect.W = 8
		catalog.Levels[0].Props[i].Clearances[0].Rect.W = 8
	}
	gallery := &catalog.Modules[2]
	gallery.Footprint = SpatialFootprint{W: 10, H: 4}
	gallery.ElevationMM = 7500
	gallery.Level = 3
	landing := &catalog.Modules[3]
	landing.Footprint = SpatialFootprint{W: 4, H: 6}
	landing.ElevationMM = 4200
	landing.Level = 2
	stair := &catalog.Modules[4]
	stair.Footprint = SpatialFootprint{W: 8, H: 4}
	stair.HeightMM = 4500
	stair.RiseMM = 4200
	stair.Ports[1].At = Point{X: 7, Y: 0}
	cassette := &catalog.Modules[5]
	cassette.Footprint = SpatialFootprint{W: 2, H: 2}
	cassette.SnapMeters = 2
	cassette.HeightMM = 1200
	spatialModuleTerrain(&catalog.Levels[0])
	compiled, err := compileSpatial(spatialValue(t, catalog), spatialLegend())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(compiled.Modules, catalog.Modules) || compiled.ModuleSnapMeters != 3 {
		t.Fatal("authored dimensions or construction grids were changed")
	}
	if compiled.Levels[0].Props[0].Rect.W != 8 {
		t.Fatal("instance geometry was forced back to a POC prototype")
	}
}

func TestCompileSpatialModuleUsesItsOwnConstructionGrid(t *testing.T) {
	catalog := spatialModuleFixture()
	catalog.ModuleSnapMeters = 4
	shop := &catalog.Modules[0]
	shop.Footprint = SpatialFootprint{W: 8, H: 4}
	shop.SnapMeters = 4
	shop.HeightMM = 3500
	shop.Clearances[0].DepthMeters = 4
	level := &catalog.Levels[0]
	level.Props = level.Props[:1]
	prop := &level.Props[0]
	prop.Rect = SpatialRect{X: 4, Y: 4, W: 8, H: 4}
	prop.Clearances[0].Rect = SpatialRect{X: 4, Y: 8, W: 8, H: 4}
	spatialModuleTerrain(level)
	if err := catalog.compile(spatialLegend()); err != nil {
		t.Fatal(err)
	}
	prop.Rect.X = 6 // Still aligned to the old POC's 2m, but not this module's 4m.
	prop.Clearances[0].Rect.X = 6
	spatialModuleTerrain(level)
	if err := catalog.compile(spatialLegend()); err == nil || !strings.Contains(err.Error(), "snap grid") {
		t.Fatalf("module's declared grid ignored: %v", err)
	}
}

func TestCompileSpatialModuleGenericGeometryGuards(t *testing.T) {
	cases := []struct {
		name, want string
		change     func(*SpatialCatalog)
	}{
		{"catalog_negative_grid", "module_snap_m", func(c *SpatialCatalog) { c.ModuleSnapMeters = -1 }},
		{"catalog_grid_bound", "module_snap_m", func(c *SpatialCatalog) { c.ModuleSnapMeters = 4097 }},
		{"module_zero_grid", "snap_m", func(c *SpatialCatalog) { c.Modules[0].SnapMeters = 0 }},
		{"module_grid_bound", "snap_m", func(c *SpatialCatalog) { c.Modules[0].SnapMeters = 4097 }},
		{"misaligned_dimension", "snap_m", func(c *SpatialCatalog) { c.Modules[0].Footprint.W = 7 }},
		{"nonstair_rise", "non-stair", func(c *SpatialCatalog) { c.Modules[2].RiseMM = 100 }},
		{"rise_exceeds_volume", "height", func(c *SpatialCatalog) { c.Modules[4].RiseMM = 3500 }},
		{"wrong_landing_type", "landing prototype", func(c *SpatialCatalog) { c.Modules[4].LandingModuleID = "gallery" }},
		{"footprint_bound", "footprint", func(c *SpatialCatalog) { c.Modules[3].Footprint.W = 4098 }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			catalog := spatialModuleFixture()
			tt.change(&catalog)
			if err := catalog.compile(spatialLegend()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
}
