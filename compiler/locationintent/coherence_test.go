package locationintent

import (
	"strings"
	"testing"
)

// Every coherence rule has an intent that breaks exactly it. The message must
// name the authored ID, a cell where there is one, and a fix hint.
func TestCoherenceRulesStopBrokenIntents(t *testing.T) {
	two := 2.0
	cases := []struct {
		name, code, id string
		cell           bool
		edit           func(*Intent)
	}{
		{"exit without edges", "exit_unbound", "corridor", false, func(in *Intent) { in.Exits[0].ReverseEdge = "" }},
		{"exit into a neighbouring zone", "exit_not_on_outer_wall", "corridor", true, func(in *Intent) { in.Exits[0].Zone, in.Exits[0].Wall = "hall", "east"; in.Exits[0].AtM = nil }},
		{"marker on bare floor", "marker_on_bare_floor", "way.intent_bronze_room.nowhere", true, func(in *Intent) {
			in.Markers = []FreeMarker{{ID: "nowhere", Number: 5, Key: "wayfinding.test.nowhere", Zone: "hall", AtM: [2]float64{6, 5}}}
		}},
		{"inspection on bare floor", "inspection_on_bare_floor", "prop.intent_bronze_room.nothing", true, func(in *Intent) {
			in.Inspections = []FreeInspection{{ID: "nothing", Zone: "hall", AtM: [2]float64{6, 5}, Text: Text{Key: "inspect.test.nothing", RU: "Ничего.", EN: "Nothing.", Context: "test"}}}
		}},
		{"lamp on a side without a wall", "lamp_unmounted", "open.0", true, func(in *Intent) {
			in.Lights = []LightRule{{ID: "open", Rule: "wall_every_m", Zone: "lobby", On: "west", StepM: 4}}
		}},
		{"zone cut off by a fixture", "zone_unreachable", "hall", true, func(in *Intent) {
			// A bench across the 4 m doorway between lobby and hall, twice.
			in.Fixtures = append(in.Fixtures,
				Fixture{ID: "block.a", Kit: "kit.geo.bench", Zone: "lobby", On: "floor", AtM: &[2]float64{0, 0}, Rotation: 90, Purpose: "test"},
				Fixture{ID: "block.b", Kit: "kit.geo.bench", Zone: "lobby", On: "floor", AtM: &[2]float64{0, 2}, Rotation: 90, Purpose: "test"})
		}},
		{"fixture on a side without a wall", "fixture_without_wall", "bench.a", true, func(in *Intent) { in.Fixtures[0].Zone, in.Fixtures[0].On = "lobby", "west" }},
		{"fixture does not fit", "fixture_outside_zone", "bench.a", true, func(in *Intent) { in.Fixtures[0].AlignM = &[]float64{11}[0] }},
		{"zones overlap", "zone_overlap", "lobby", true, func(in *Intent) { in.Zones[1].Rect.X = 11 }},
		{"kit without passport or size", "kit_without_passport", "reader", false, func(in *Intent) { in.Fixtures[2].SizeM = nil }},
		{"no way in", "no_live_exit", "level.intent_bronze_room", false, func(in *Intent) { in.Exits[0].Status = "planned" }},
		{"exit does not fit its wall", "exit_does_not_fit", "corridor", false, func(in *Intent) { in.Exits[0].AtM = &two; in.Exits[0].WidthM = 6 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			intents, catalog, defaults := load(t)
			in := intents["level.intent_bronze_room"]
			c.edit(&in)
			_, diags := Derive(in, catalog, defaults)
			for _, d := range diags {
				if d.Code == c.code && d.ID == c.id {
					if d.Hint == "" || d.Message == "" || (c.cell && d.Cell == nil) {
						t.Fatalf("diagnostic lacks cell/message/hint: %+v", d)
					}
					if !strings.Contains((&Error{diags}).Error(), c.code) {
						t.Fatal("error text lacks the code")
					}
					return
				}
			}
			t.Fatalf("want %s for %s, got %+v", c.code, c.id, diags)
		})
	}
}

// The hub's known false alarms stay quiet: a marker at a doorway, an
// inspection at a door interface without footprint, mass behind a wall.
func TestCoherenceAcceptsDoorwaysAndDeclaredMass(t *testing.T) {
	intents, catalog, defaults := load(t)
	in := intents["level.intent_bronze_room"]
	in.Markers = []FreeMarker{{ID: "door", Number: 3, Key: "wayfinding.test.door", Zone: "lobby", AtM: [2]float64{3.5, 2}}}
	if _, diags := Derive(in, catalog, defaults); len(diags) != 0 {
		t.Fatalf("a marker beside its doorway must pass: %+v", diags)
	}
	dock := intents["level.c7_copper_dock"]
	dock.Markers = []FreeMarker{{ID: "deck", Number: 4, Key: "wayfinding.test.deck", Zone: "landing", AtM: [2]float64{7, 9}}}
	if _, diags := Derive(dock, catalog, defaults); len(diags) != 0 {
		t.Fatalf("a marker at a mechanism without footprint must pass: %+v", diags)
	}
}

// A pocket outside the walkable area is never left as a dark pit:
// it is capped at the wall top by default and reported.
func TestEnclosedPocketIsCapped(t *testing.T) {
	intents, catalog, defaults := load(t)
	in := intents["level.intent_bronze_room"]
	// A ring of floor around an undeclared 2x2 m hole in the hall.
	in.Zones = []Zone{
		{ID: "hall", Rect: RectM{X: 0, Y: 0, W: 12, H: 3}, Purpose: "north", Public: true},
		{ID: "west", Rect: RectM{X: 0, Y: 3, W: 5, H: 2}, Purpose: "west", Public: true},
		{ID: "east", Rect: RectM{X: 7, Y: 3, W: 5, H: 2}, Purpose: "east", Public: true},
		{ID: "south", Rect: RectM{X: 0, Y: 5, W: 12, H: 3}, Purpose: "south", Public: true},
		in.Zones[1],
	}
	in.Fixtures = nil
	d, diags := Derive(in, catalog, defaults)
	if d == nil || len(diags) != 0 {
		t.Fatalf("%+v", diags)
	}
	if len(d.Capped) != 1 || d.Capped[0].Cells != 16 {
		t.Fatalf("capped %+v", d.Capped)
	}
	found := false
	for _, w := range d.Finishing.Architecture.Draws {
		found = found || strings.Contains(w.ID, ".cap.0.")
	}
	if !found {
		t.Fatal("capped pocket has no wall-top mass")
	}
	compile(t, d)
	// The same rule on a hand-written layout reports the pocket with its place.
	layout := d.Layout
	layout.Obstacles = nil
	got := LayoutPockets(layout)
	if len(got) != 3 || got[1].Code != "enclosed_pocket" || got[1].Cell == nil || got[1].Hint == "" || !strings.Contains(got[1].Message, "16 cells") {
		t.Fatalf("layout rule: %+v", got)
	}
	// A notch between two rooms' outer walls is a pit too, even though it is
	// open to the outside: the bronze room without its declared piers gets caps.
	plain := intents["level.intent_bronze_room"]
	plain.Solids = nil
	d, diags = Derive(plain, catalog, defaults)
	if len(diags) != 0 || len(d.Capped) != 2 {
		t.Fatalf("notches beside the lobby must be capped: %+v %+v", diags, d.Capped)
	}
}
