package gamepoc

import (
	"encoding/json"
	"testing"
)

// Plan 1 of the baseline fixes (the housing block): the standing cells of a place, all taken, never close a way of the
// level; the first slot that would is left out, the rest are kept in their order.
func TestSlotsOfAPlaceNeverCloseAWayOfTheLevel(t *testing.T) {
	// a hall three cells high between a home door (0,1) and an exit (6,1); the washroom's slots fill the column x=3
	life := `{"locations":{"hall":{"terrain":["#######",".......","#######"]},"room":{"terrain":["...","...","..."]}},` +
		`"edges":[{"id":"out","from":"hall","to":"room","exit":{"x":6,"y":1},"entry":{"x":0,"y":0}},{"id":"in","from":"room","to":"hall","exit":{"x":2,"y":2},"entry":{"x":6,"y":1}}],` +
		`"residences":[{"id":"home","level":"hall","entry":{"x":0,"y":1},"return_at":{"x":1,"y":1}}],` +
		`"brain":{"places":[{"id":"washroom","kind":"hygiene","level":"hall","at":{"x":3,"y":1},"slots":[{"x":3,"y":1},{"x":4,"y":1}]},` +
		`{"id":"corner","kind":"social","level":"room","at":{"x":1,"y":1},"slots":[{"x":1,"y":1},{"x":1,"y":0}]}]}}`
	c := Compiled{Life: []byte(life), Walkable: map[string]bool{".": true}}
	if err := keepSlotWays(&c); err == nil {
		t.Fatal("a place whose every slot closes the only way was accepted")
	}
	// a room four cells high with doors in its four corners and a corridor two cells wide under it: the slots on the
	// rows between the doors (the main ways) and in the corridor are left out, the middle of the room keeps its slots
	c.Life = []byte(`{"locations":{"hall":{"terrain":["########","........","........","........","........","#..#####","#..#####","#..#####","#..#####","#..#####","#..#####","########"]}},` +
		`"edges":[{"id":"out","from":"hall","to":"hall","exit":{"x":7,"y":1},"entry":{"x":7,"y":4}}],` +
		`"residences":[{"id":"home","level":"hall","entry":{"x":0,"y":1},"return_at":{"x":0,"y":4}},{"id":"back","level":"hall","entry":{"x":1,"y":10},"return_at":{"x":2,"y":10}}],` +
		`"brain":{"places":[{"id":"washroom","kind":"hygiene","level":"hall","at":{"x":3,"y":1},"slots":[{"x":3,"y":1},{"x":3,"y":2},{"x":3,"y":3},{"x":3,"y":4},{"x":4,"y":1},{"x":1,"y":7}]}]}}`)
	if err := keepSlotWays(&c); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Brain struct {
			Places []struct {
				ID    string  `json:"id"`
				Kind  string  `json:"kind"`
				Slots []Point `json:"slots"`
			} `json:"places"`
		} `json:"brain"`
	}
	if err := json.Unmarshal(c.Life, &got); err != nil {
		t.Fatal(err)
	}
	p := got.Brain.Places[0]
	want := []Point{{X: 3, Y: 2}, {X: 3, Y: 3}}
	if p.ID != "washroom" || p.Kind != "hygiene" || len(p.Slots) != len(want) || p.Slots[0] != want[0] || p.Slots[1] != want[1] {
		t.Fatalf("expected the slots that leave the room's way open: %+v", p)
	}
}
