package gamepoc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
)

// Document 80 + demo-hub: a multi-deck layout binds to a story location with its
// navigation plane, stair links and gates; floor guides and SDK stand residents
// stay rejected.
func TestModularBindingRejectsMultideckGuidesAndStandResidents(t *testing.T) {
	raw := []byte(`{"locations":{"level.x":{"modular_layout":"stand"}}}`)
	for name, level := range map[string]locationsdk.RuntimeLevel{
		"floor guides":                   {ID: "stand", Depth: 1, Terrain: []string{"."}, InitialLayoutKnown: true, Navigation: &locationsdk.Navigation{}, ReservedPaths: []locationsdk.RuntimePath{{ID: "lane"}}},
		"floor guides with deck":         {ID: "stand", Depth: 1, Terrain: []string{"."}, InitialLayoutKnown: true, Navigation: &locationsdk.Navigation{BaseSurface: "floor", Surfaces: []locationsdk.NavSurface{{ID: "floor", Walkable: true}, {ID: "upper", Walkable: true, NavRow: 2}}}, ReservedPaths: []locationsdk.RuntimePath{{ID: "lane"}}},
		"floor guides with roof":         {ID: "stand", Depth: 1, Terrain: []string{"."}, InitialLayoutKnown: true, Navigation: &locationsdk.Navigation{BaseSurface: "floor", Surfaces: []locationsdk.NavSurface{{ID: "floor", Walkable: true}, {ID: "roof", Walkable: false, NavRow: -1}}}, ReservedPaths: []locationsdk.RuntimePath{{ID: "lane"}}},
		"floor guides with missing base": {ID: "stand", Depth: 1, Terrain: []string{"."}, InitialLayoutKnown: true, Navigation: &locationsdk.Navigation{Surfaces: []locationsdk.NavSurface{{Walkable: true}}}, ReservedPaths: []locationsdk.RuntimePath{{ID: "lane"}}},
		"test-stand":                     {ID: "stand", Depth: 1, Terrain: []string{"."}, InitialLayoutKnown: true, Residents: []locationsdk.CompiledResident{{ID: "npc"}}},
	} {
		bundle := &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{level}}
		if _, _, err := compileModularLocations(raw, bundle); err == nil || !strings.Contains(err.Error(), strings.Split(name, " with ")[0]) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestModularBindingCarriesMultideckLinksGatesAndPublicPopulation(t *testing.T) {
	// Row 0: hall (entry at 1,0); row 1: wall with a gate opening at (2,1); row 2: corridor beyond the gate.
	// Row 4: an upper band cell joined to the hall by a link; row 3 separates the bands.
	terrain := []string{"....", "##.#", "....", "####", "..##"}
	nav := &locationsdk.Navigation{Links: []locationsdk.NavLink{{A: locationsdk.Cell{X: 0, Y: 0}, B: locationsdk.Cell{X: 0, Y: 4}}}}
	level := locationsdk.RuntimeLevel{ID: "stand", Width: 4, Depth: 5, Terrain: terrain, InitialLayoutKnown: true, Navigation: nav, Entries: []locationsdk.Cell{{X: 1, Y: 0}},
		Gates: []locationsdk.RuntimeGate{{ID: "gate", Opening: []locationsdk.Cell{{X: 2, Y: 1}}, TravelTicks: 10}}}
	raw := []byte(`{"locations":{"level.x":{"modular_layout":"stand"}},"actors":[],"edges":[],"population":{"anchors":{}}}`)
	out, bindings, err := compileModularLocations(raw, &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{level}})
	if err != nil || len(bindings) != 1 {
		t.Fatal(bindings, err)
	}
	var life struct {
		Locations  map[string]struct{ Links, Gates []json.RawMessage }
		Population struct{ Anchors map[string][]Point }
	}
	if err := json.Unmarshal(out, &life); err != nil {
		t.Fatal(err)
	}
	if loc := life.Locations["level.x"]; len(loc.Links) != 1 || len(loc.Gates) != 1 {
		t.Fatalf("links/gates not bound: %+v", loc)
	}
	got := map[Point]bool{}
	for _, p := range life.Population.Anchors["level.x"] {
		got[p] = true
	}
	if !got[Point{X: 0, Y: 4}] || !got[Point{X: 3, Y: 0}] || got[Point{X: 2, Y: 1}] || got[Point{X: 0, Y: 2}] {
		t.Fatalf("population must stay on the public side incl. the upper band: %v", life.Population.Anchors["level.x"])
	}
}

// Compile real vertical geometry, then bind it; synthetic Navigation metadata
// alone cannot establish that obstacle/headroom checks preserved the route.
func TestModularBindingSingleSurfaceVerticalReservations(t *testing.T) {
	c := locationsdk.Catalog{UnitMM: 500,
		Geometries: []locationsdk.Geometry{{ID: "marker", Width: 1, Depth: 1, Height: 1}},
		Modules:    []locationsdk.Module{{ID: "kit.marker", GeometryID: "marker", Variants: []locationsdk.Variant{{ID: "v1", GeometryID: "marker"}}}},
		Times:      []locationsdk.TimeState{{ID: "day", Brightness: 1}}, Events: []locationsdk.EventState{{ID: "normal"}}}
	l := locationsdk.Layout{ID: "single.floor", UnitMM: 500, Width: 8, Depth: 8, Grounded: true, BaseSurfaceID: "floor",
		Floor: []locationsdk.FloorArea{{ID: "slab", Rect: locationsdk.Rect{W: 8, H: 8}, Reason: "supported slab"}}, Entries: []locationsdk.Cell{{X: 1, Y: 4}},
		Obstacles: []locationsdk.Obstacle{{ID: "wall", Surface: "floor", Rect: locationsdk.Rect{X: 3, Y: 2, W: 1, H: 1}, TopMM: 6000, BlocksMovement: true, BlocksSight: true, Style: "wall", Reason: "solid wall"},
			{ID: "header", Surface: "floor", Rect: locationsdk.Rect{X: 3, Y: 3, W: 1, H: 2}, BottomMM: 2400, TopMM: 6000, BlocksSight: true, Style: "wall", Reason: "open passage header"}},
		ReservedPaths: []locationsdk.ReservedPath{{ID: "public", Points: []locationsdk.Cell{{X: 1, Y: 4}, {X: 6, Y: 4}}, Width: 1, Reason: "public passage"}}}
	bundle, err := locationsdk.CompileRuntime(locationsdk.Input{Catalog: c, Layouts: []locationsdk.Layout{l}}, locationsdk.RuntimeOptions{TickHz: 20, MoveEveryTicks: 2})
	if err != nil {
		t.Fatal(err)
	}
	level := bundle.Levels[0]
	if len(level.Navigation.Surfaces) != 1 || len(level.ReservedPaths) != 1 || len(level.Navigation.Obstacles) != 2 {
		t.Fatal("vertical geometry or reservations dropped")
	}
	out, bindings, err := compileModularLocations([]byte(`{"locations":{"story":{"modular_layout":"single.floor"}},"actors":[],"edges":[],"population":{"anchors":{}}}`), &bundle)
	if err != nil {
		t.Fatal(err)
	}
	var life struct {
		Locations map[string]struct {
			Terrain []string
			CellMM  int `json:"cell_mm"`
		}
		Population struct{ Anchors map[string][]Point }
	}
	if err = json.Unmarshal(out, &life); err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 || !reflect.DeepEqual(life.Locations["story"].Terrain, level.Terrain) || life.Locations["story"].CellMM != 500 {
		t.Fatal("compiled binding was not materialized")
	}
	reserved := map[Point]bool{}
	for _, at := range level.ReservedPaths[0].Cells {
		reserved[Point{X: at.X, Y: at.Y}] = true
	}
	for _, at := range life.Population.Anchors["story"] {
		if reserved[at] {
			t.Fatal("idle population blocks reserved corridor", at)
		}
	}
	// Re-check against final obstacle geometry, including low headroom.
	for _, bottom := range []int{0, 1700} {
		l.Obstacles[1].BottomMM = bottom
		l.Obstacles[1].BlocksMovement = true
		if _, err = locationsdk.CompileRuntime(locationsdk.Input{Catalog: c, Layouts: []locationsdk.Layout{l}}, locationsdk.RuntimeOptions{TickHz: 20, MoveEveryTicks: 2}); err == nil || !strings.Contains(err.Error(), "reserved_path_obstructed") {
			t.Fatalf("obstructed reserved path accepted: %v", err)
		}
	}
}
