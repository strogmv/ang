package gamepoc

import (
	"fmt"
	"sort"
)

// Baseline fixes (21.11.2026, design/world-life/baseline-fixes/BRIEF.md, plan 4): places in a line in a dead end one
// cell wide. In a work area a co-worker standing at his own place steps into a free cell of the area beside it for a
// co-worker whose place lies beyond him (behavior, life_traffic.go: lifeCoworkerGiveWay). Where such a place has no free
// cell beside it — the only way to the other place runs through it — nobody can ever pass: the shawarma stall F4 kept its
// cook behind his daughter for good (his place (68,59) beyond hers (66,59) in a pass one cell wide). The build stops on
// such a place: the data needs a pocket beside it (or the places in another order).

// workLinePosts: the places of one worker inside one area (routine steps, counter posts, his own brain places).
type workLinePosts map[string][]Point

// checkWorkLines: for every two workers of an area, a place P of one that cuts the way from the area's doors to a place
// Q of the other must have a pocket — a cell of the area beside P with Q still reachable round it.
func checkWorkLines(level string, area lifeWorkArea, posts workLinePosts, floor func(Point) bool) error {
	if len(area.Workers) < 2 {
		return nil
	}
	cells := map[Point]bool{}
	for _, p := range area.Cells {
		cells[p] = true
	}
	steps := []Point{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}
	var doors []Point
	for _, p := range area.Cells {
		for _, d := range steps {
			n := Point{X: p.X + d.X, Y: p.Y + d.Y}
			if !cells[n] && floor(n) {
				doors = append(doors, p)
				break
			}
		}
	}
	reach := func(blocked map[Point]bool) map[Point]bool {
		seen := map[Point]bool{}
		var queue []Point
		for _, d := range doors {
			if !blocked[d] && !seen[d] {
				seen[d] = true
				queue = append(queue, d)
			}
		}
		for head := 0; head < len(queue); head++ {
			at := queue[head]
			for _, d := range steps {
				n := Point{X: at.X + d.X, Y: at.Y + d.Y}
				if cells[n] && !blocked[n] && !seen[n] {
					seen[n] = true
					queue = append(queue, n)
				}
			}
		}
		return seen
	}
	workers := append([]string(nil), area.Workers...)
	sort.Strings(workers)
	for _, w1 := range workers {
		for _, p := range posts[w1] {
			if !cells[p] {
				continue
			}
			cut := reach(map[Point]bool{p: true})
			for _, w2 := range workers {
				if w2 == w1 {
					continue
				}
				for _, q := range posts[w2] {
					if q == p || !cells[q] || cut[q] {
						continue
					}
					pocket := false
					for _, d := range steps {
						c := Point{X: p.X + d.X, Y: p.Y + d.Y}
						if c != q && cells[c] && reach(map[Point]bool{c: true})[q] {
							pocket = true
							break
						}
					}
					if !pocket {
						return fmt.Errorf("world.fixtures: work area %s on %s: the place (%d,%d) of %s stands in a line one cell wide on the only way to the place (%d,%d) of %s and has no free cell of the area beside it — nobody can pass; give it a pocket (or put the places in another order)", area.ID, level, p.X, p.Y, w1, q.X, q.Y, w2)
					}
				}
			}
		}
	}
	return nil
}
