package gamepoc

import (
	"encoding/json"
	"fmt"
)

// Baseline fixes (21.11.2026, design/world-life/baseline-fixes/BRIEF.md, plan 1, the housing block): the standing cells of
// a brain place (its slots: «the population anchors within four cells of the washroom door») are where people stand while
// they use it, for a quarter of an hour. Taken all at once in the morning they made a wall: the slots of the washroom of
// the housing block filled the north room from wall to wall and cut every home from the exit to the monorail (and the
// slots of the bunk corner did the same to the corridor). A slot is kept only while all the slots kept so far, all taken,
// still leave every way of its level open — between the level's exits and entries and the doors of its homes. The data
// stays a plain «near the door»; the generator keeps the ways: no slot on a shortest way between two doors of the level
// (exits, arrivals, homes' doors) or in a passage three cells wide or narrower (and long along it: the Life's lanes), and
// of the rest, in their order, the first that would close a way is left out.
// slotPassageWidth: a passage up to this wide has no place to stand (a person standing there takes a lane of it).
const slotPassageWidth = 3

func keepSlotWays(c *Compiled) error {
	if len(c.Life) == 0 {
		return nil
	}
	var life map[string]json.RawMessage
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	if len(life["brain"]) == 0 {
		return nil
	}
	var brain map[string]json.RawMessage
	if err := json.Unmarshal(life["brain"], &brain); err != nil {
		return err
	}
	var places []json.RawMessage
	if len(brain["places"]) == 0 {
		return nil
	}
	if err := json.Unmarshal(brain["places"], &places); err != nil {
		return err
	}
	var locations map[string]struct {
		Terrain []string       `json:"terrain"`
		Work    []lifeWorkArea `json:"work"`
		Links   []struct {
			A Point `json:"a"`
			B Point `json:"b"`
		} `json:"links"`
	}
	if err := json.Unmarshal(life["locations"], &locations); err != nil {
		return err
	}
	var edges []struct {
		From, To    string
		Exit, Entry Point
	}
	if err := json.Unmarshal(life["edges"], &edges); err != nil {
		return err
	}
	var residences []struct {
		Level    string `json:"level"`
		Entry    Point  `json:"entry"`
		ReturnAt Point  `json:"return_at"`
	}
	if len(life["residences"]) > 0 {
		if err := json.Unmarshal(life["residences"], &residences); err != nil {
			return err
		}
	}
	doors := map[string][]Point{}
	for _, e := range edges {
		doors[e.From] = append(doors[e.From], e.Exit)
		doors[e.To] = append(doors[e.To], e.Entry)
	}
	for _, r := range residences {
		doors[r.Level] = append(doors[r.Level], r.Entry, r.ReturnAt)
	}
	changed := false
	for i, raw := range places {
		var place struct {
			ID    string  `json:"id"`
			Level string  `json:"level"`
			Slots []Point `json:"slots"`
		}
		if err := json.Unmarshal(raw, &place); err != nil {
			return err
		}
		loc, ok := locations[place.Level]
		if len(place.Slots) == 0 || !ok || len(doors[place.Level]) < 2 {
			continue
		}
		links := map[Point][]Point{}
		for _, k := range loc.Links {
			links[k.A] = append(links[k.A], k.B)
			links[k.B] = append(links[k.B], k.A)
		}
		work := map[Point]bool{}
		for _, a := range loc.Work {
			for _, p := range a.Cells {
				work[p] = true
			}
		}
		// public floor: the ways of the level's people (the work cells are their workers' only)
		floor := func(p Point) bool {
			return p.Y >= 0 && p.Y < len(loc.Terrain) && p.X >= 0 && p.X < len(loc.Terrain[p.Y]) && c.Walkable[string(loc.Terrain[p.Y][p.X])] && !work[p]
		}
		// the doors that are connected on the bare floor must stay connected
		open := func(blocked map[Point]bool) bool {
			var keys []Point
			for _, d := range doors[place.Level] {
				if floor(d) && !blocked[d] {
					keys = append(keys, d)
				}
			}
			if len(keys) < 2 {
				return true
			}
			seen := map[Point]bool{keys[0]: true}
			queue := []Point{keys[0]}
			for head := 0; head < len(queue); head++ {
				at := queue[head]
				near := []Point{{X: at.X + 1, Y: at.Y}, {X: at.X, Y: at.Y + 1}, {X: at.X - 1, Y: at.Y}, {X: at.X, Y: at.Y - 1}}
				for _, n := range append(near, links[at]...) {
					if !seen[n] && !blocked[n] && floor(n) {
						seen[n] = true
						queue = append(queue, n)
					}
				}
			}
			for _, k := range keys {
				if !seen[k] {
					return false
				}
			}
			return true
		}
		if !open(nil) {
			continue // the level is cut on its own (an authored barrier): nothing for the slots to keep
		}
		// a passage (narrow across, long along: the lanes of the Life, behavior life_traffic.go) is a way, never a place to stand
		run := func(p Point, dx, dy int) int {
			n := 0
			for n < 16 && floor(Point{X: p.X + dx*(n+1), Y: p.Y + dy*(n+1)}) {
				n++
			}
			return n
		}
		passage := func(p Point) bool {
			across, along := run(p, -1, 0)+run(p, 1, 0)+1, run(p, 0, -1)+run(p, 0, 1)+1
			if across <= slotPassageWidth && along >= 2*across+2 {
				return true
			}
			across, along = along, across
			return across <= slotPassageWidth && along >= 2*across+2
		}
		// the main ways between the level's doors (a shortest way for every two of them, and the doors) stay free
		mainWay := map[Point]bool{}
		ds := doors[place.Level]
		for i := range ds {
			mainWay[ds[i]] = true
			for j := i + 1; j < len(ds); j++ {
				for _, c := range floorPath(ds[i], ds[j], floor, links) {
					mainWay[c] = true
				}
			}
		}
		kept := make([]Point, 0, len(place.Slots))
		blocked := map[Point]bool{}
		for _, s := range place.Slots {
			if passage(s) || mainWay[s] {
				continue
			}
			blocked[s] = true
			if open(blocked) {
				kept = append(kept, s)
				continue
			}
			delete(blocked, s)
		}
		if len(kept) == len(place.Slots) {
			continue
		}
		if len(kept) == 0 {
			return fmt.Errorf("world.life.brain: place %s on %s: every standing cell of it stands in a passage or closes a way of the level", place.ID, place.Level)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		fields["slots"], _ = json.Marshal(kept)
		out, err := json.Marshal(fields) // a map: its keys in order
		if err != nil {
			return err
		}
		places[i] = out
		changed = true
	}
	if !changed {
		return nil
	}
	var err error
	if brain["places"], err = json.Marshal(places); err != nil {
		return err
	}
	if life["brain"], err = json.Marshal(brain); err != nil {
		return err
	}
	raw, err := json.Marshal(life)
	if err != nil {
		return err
	}
	c.Life = raw
	return nil
}

// floorPath: a shortest way over the floor between two cells (breadth first, the neighbours in a fixed order), nil when
// there is none.
func floorPath(from, to Point, floor func(Point) bool, links map[Point][]Point) []Point {
	if from == to {
		return nil
	}
	prev := map[Point]Point{from: from}
	queue := []Point{from}
	for head := 0; head < len(queue); head++ {
		at := queue[head]
		near := []Point{{X: at.X + 1, Y: at.Y}, {X: at.X, Y: at.Y + 1}, {X: at.X - 1, Y: at.Y}, {X: at.X, Y: at.Y - 1}}
		for _, n := range append(near, links[at]...) {
			if _, seen := prev[n]; seen || !floor(n) {
				continue
			}
			prev[n] = at
			if n == to {
				var path []Point
				for c := to; c != from; c = prev[c] {
					path = append(path, c)
				}
				return path
			}
			queue = append(queue, n)
		}
	}
	return nil
}
