package gamepoc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
)

// A stand of one level bound to a layout: a counter loop (rect 6 x 5 at 2,2) with its door on the west, a lamp, the floor
// round them, a seller whose post is inside the loop, a buyer outside, a public brain place and a population anchor.
func fixturesStand(t *testing.T, mutate func(terrain []string, life map[string]any, bundle *locationsdk.RuntimeBundle, in *FixtureCatalog)) (*Compiled, *FixtureCatalog) {
	t.Helper()
	terrain := []string{
		"..........",
		"..........",
		"..++++++..",
		".......+..",
		"..+....+..",
		"..++++++..",
		"..........",
	}
	life := map[string]any{
		"locations": map[string]any{"level.hall": map[string]any{"terrain": terrain, "markers": []any{}}},
		"actors": []any{
			map[string]any{"id": "char.seller", "steps": []any{map[string]any{"level": "level.hall", "at": map[string]int{"x": 4, "y": 3}}}},
			map[string]any{"id": "char.buyer", "steps": []any{map[string]any{"level": "level.hall", "at": map[string]int{"x": 4, "y": 6}}}},
		},
		"edges":      []any{},
		"brain":      map[string]any{"places": []any{map[string]any{"id": "place.counter", "level": "level.hall", "at": map[string]int{"x": 4, "y": 1}}}},
		"population": map[string]any{"anchors": map[string]any{"level.hall": []any{map[string]int{"x": 0, "y": 0}, map[string]int{"x": 5, "y": 4}}}},
	}
	bundle := &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{{
		ID: "layout.hall",
		Instances: []locationsdk.CompiledInstance{
			{ID: "loop.1", ModuleID: "kit.loop", At: locationsdk.Cell{X: 2, Y: 2}, Width: 6, Depth: 4,
				Footprint: []locationsdk.Cell{{X: 2, Y: 2}, {X: 3, Y: 2}, {X: 4, Y: 2}, {X: 5, Y: 2}, {X: 6, Y: 2}, {X: 7, Y: 2}, {X: 7, Y: 3}, {X: 2, Y: 4}, {X: 7, Y: 4}, {X: 2, Y: 5}, {X: 3, Y: 5}, {X: 4, Y: 5}, {X: 5, Y: 5}, {X: 6, Y: 5}, {X: 7, Y: 5}}},
			{ID: "lamp.1", ModuleID: "kit.lamp", At: locationsdk.Cell{X: 0, Y: 6}, Width: 1, Depth: 1},
		},
		Ports: []locationsdk.CompiledPort{{ID: "gate", InstanceID: "loop.1", Kind: "door", At: locationsdk.Cell{X: 2, Y: 3}, Opening: []locationsdk.Cell{{X: 2, Y: 3}}}},
	}}}
	in := &FixtureCatalog{Rules: map[string]FixtureRule{
		"kit.loop": {Mount: "floor", Inside: "work", Reason: "sellers inside"},
		"kit.lamp": {Mount: "overhead", Reason: "hangs"},
	}}
	if mutate != nil {
		mutate(terrain, life, bundle, in)
		life["locations"].(map[string]any)["level.hall"].(map[string]any)["terrain"] = terrain
	}
	raw, err := json.Marshal(life)
	if err != nil {
		t.Fatal(err)
	}
	return &Compiled{Life: raw, Walkable: map[string]bool{".": true, "#": false, "+": false}, LocationSDK: bundle, ModularLocations: []ModularLocation{{LevelID: "level.hall", LayoutID: "layout.hall"}}}, in
}

func TestFixturesKeepTheInsideForItsWorkersAndCheckEveryFixture(t *testing.T) {
	c, in := fixturesStand(t, nil)
	if err := applyFixtures(c, in); err != nil {
		t.Fatalf("a valid stand was refused: %v", err)
	}
	var life struct {
		Locations map[string]struct {
			Work []lifeWorkArea `json:"work"`
		} `json:"locations"`
		Population struct {
			Anchors map[string][]Point `json:"anchors"`
		} `json:"population"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		t.Fatal(err)
	}
	work := life.Locations["level.hall"].Work
	inside := []Point{{2, 3}, {3, 3}, {4, 3}, {5, 3}, {6, 3}, {3, 4}, {4, 4}, {5, 4}, {6, 4}}
	if len(work) != 1 || work[0].ID != "loop.1" || !reflect.DeepEqual(work[0].Workers, []string{"char.seller"}) || !reflect.DeepEqual(work[0].Cells, inside) {
		t.Fatalf("work areas %+v", work)
	}
	if anchors := life.Population.Anchors["level.hall"]; len(anchors) != 1 || anchors[0] != (Point{0, 0}) {
		t.Fatalf("population anchors kept in the work cells: %v", anchors)
	}
	refusals := []struct {
		name, want string
		mutate     func([]string, map[string]any, *locationsdk.RuntimeBundle, *FixtureCatalog)
	}{
		{"undeclared inside", "open to everybody", func(_ []string, _ map[string]any, _ *locationsdk.RuntimeBundle, in *FixtureCatalog) {
			in.Rules["kit.loop"] = FixtureRule{Mount: "floor", Reason: "r"}
		}},
		{"a floor fixture without a footprint", "has no footprint", func(_ []string, _ map[string]any, _ *locationsdk.RuntimeBundle, in *FixtureCatalog) {
			delete(in.Rules, "kit.lamp")
		}},
		{"a mounted fixture on the floor", "declared overhead but stands", func(_ []string, _ map[string]any, b *locationsdk.RuntimeBundle, _ *FixtureCatalog) {
			b.Levels[0].Instances[1].Footprint = []locationsdk.Cell{{X: 0, Y: 6}}
		}},
		{"a footprint on open floor", "is open floor", func(terrain []string, _ map[string]any, _ *locationsdk.RuntimeBundle, _ *FixtureCatalog) {
			terrain[2] = "..+.++++.."
		}},
		{"an inside declared but open", "declares an inside", func(terrain []string, _ map[string]any, b *locationsdk.RuntimeBundle, _ *FixtureCatalog) {
			terrain[5] = "..++..++.."
			var fp []locationsdk.Cell
			for _, c := range b.Levels[0].Instances[0].Footprint {
				if c.Y != 5 || (c.X != 4 && c.X != 5) {
					fp = append(fp, c)
				}
			}
			b.Levels[0].Instances[0].Footprint = fp
		}},
		{"a public place in the work cells", "lies in the work cells", func(_ []string, life map[string]any, _ *locationsdk.RuntimeBundle, _ *FixtureCatalog) {
			life["brain"] = map[string]any{"places": []any{map[string]any{"id": "place.inside", "level": "level.hall", "at": map[string]int{"x": 5, "y": 3}}}}
		}},
		{"an unknown mount", "unknown mount", func(_ []string, _ map[string]any, _ *locationsdk.RuntimeBundle, in *FixtureCatalog) {
			in.Rules["kit.lamp"] = FixtureRule{Mount: "ceiling", Reason: "r"}
		}},
		{"a placed fixture on an unknown level", "unknown level", func(_ []string, _ map[string]any, _ *locationsdk.RuntimeBundle, in *FixtureCatalog) {
			in.Placed = []PlacedFixture{{Level: "level.none", ID: "x", Kit: "kit.loop", Rect: FixtureRect{W: 1, H: 1}, Footprint: []Point{{0, 0}}, Reason: "r"}}
		}},
	}
	for _, r := range refusals {
		c, in := fixturesStand(t, r.mutate)
		err := applyFixtures(c, in)
		if err == nil || !strings.Contains(err.Error(), r.want) {
			t.Fatalf("%s: got %v, want %q", r.name, err, r.want)
		}
	}
	// a placed fixture (not an SDK module) is checked and kept by the same rules
	c, in = fixturesStand(t, func(_ []string, _ map[string]any, b *locationsdk.RuntimeBundle, in *FixtureCatalog) {
		loop := b.Levels[0].Instances[0]
		b.Levels[0].Instances = b.Levels[0].Instances[1:]
		b.Levels[0].Ports = nil
		var fp []Point
		for _, p := range loop.Footprint {
			fp = append(fp, Point{p.X, p.Y})
		}
		in.Placed = []PlacedFixture{{Level: "level.hall", ID: "loop.placed", Kit: "kit.loop", Rect: FixtureRect{X: 2, Y: 2, W: 6, H: 4}, Footprint: fp, Doors: []Point{{2, 3}}, Reason: "r"}}
	})
	if err := applyFixtures(c, in); err != nil || !strings.Contains(string(c.Life), `"loop.placed"`) {
		t.Fatalf("placed fixture: %v", err)
	}
}
