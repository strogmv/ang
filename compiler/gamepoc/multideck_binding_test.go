package gamepoc

import (
	"encoding/json"
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
		"floor guides": {ID: "stand", Depth: 1, Terrain: []string{"."}, InitialLayoutKnown: true, Navigation: &locationsdk.Navigation{}, ReservedPaths: []locationsdk.RuntimePath{{ID: "lane"}}},
		"test-stand":   {ID: "stand", Depth: 1, Terrain: []string{"."}, InitialLayoutKnown: true, Residents: []locationsdk.CompiledResident{{ID: "npc"}}},
	} {
		bundle := &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{level}}
		if _, _, err := compileModularLocations(raw, bundle); err == nil || !strings.Contains(err.Error(), name) {
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
