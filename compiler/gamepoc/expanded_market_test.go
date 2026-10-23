package gamepoc

import (
	"os"
	"testing"

	"cuelang.org/go/cue"
)

// This opt-in check loads the author's actual CUE, rather than a copied fixture.
// ANG stays independent of the external game checkout in ordinary CI.
// ANG_EXPANDED_MARKET_CUE=/path/to/content/world/painted go test ./compiler/gamepoc -run ExpandedMarket
func TestExpandedMarketActualAuthoring(t *testing.T) {
	dir := os.Getenv("ANG_EXPANDED_MARKET_CUE")
	if dir == "" {
		t.Skip("set ANG_EXPANDED_MARKET_CUE to inspect an authored expanded market")
	}
	v, err := cuePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	catalog := v.LookupPath(cue.ParsePath("catalog"))
	if err := catalog.Validate(cue.Concrete(true)); err != nil {
		t.Fatal(err)
	}
	var c struct {
		Levels []struct {
			ID             string           `json:"id"`
			Terrain        []string         `json:"terrain"`
			Anchors        map[string]Point `json:"anchors"`
			Stores         map[string]Point `json:"store_anchors"`
			ShopApproaches map[string]Point `json:"shop_approaches"`
			Landmarks      map[string]Point `json:"landmarks"`
			Exit           struct {
				At      Point `json:"at"`
				Landing Point `json:"landing"`
			} `json:"exit"`
			LoopProbe *struct {
				From Point `json:"from"`
				To   Point `json:"to"`
			} `json:"loop_probe"`
			ProofWaypoints []Point `json:"proof_waypoints"`
			ProofBlocked   Point   `json:"proof_blocked"`
		} `json:"levels"`
	}
	if err := catalog.Decode(&c); err != nil {
		t.Fatal(err)
	}
	if len(c.Levels) != 1 || c.Levels[0].ID != "level.c7_market" {
		t.Fatalf("expected one explicit market authoring record, got %d", len(c.Levels))
	}
	l := c.Levels[0]
	floor := expandedAuditFloor(l.Terrain)
	if len(floor) <= 128 {
		t.Fatalf("market has not expanded beyond the previous 128-cell aisle: %d", len(floor))
	}
	if !floor[l.Exit.At] || !floor[l.Exit.Landing] {
		t.Fatal("real transit exit or landing lies outside the playable floor")
	}
	reached := expandedAuditReach(floor, l.Exit.Landing)
	if len(reached) != len(floor) {
		t.Errorf("disconnected painted floor: reachable=%d authored=%d", len(reached), len(floor))
	}
	if len(l.ShopApproaches) != 8 {
		t.Fatalf("expected eight authored shop approaches, got %d", len(l.ShopApproaches))
	}
	for id, at := range l.ShopApproaches {
		if !reached[at] {
			t.Errorf("shop %s has no walkable customer approach: %+v", id, at)
		}
	}
	for _, id := range []string{"rest", "loading", "sanitary", "firstaid"} {
		at, exists := l.Landmarks[id]
		if !exists || !reached[at] {
			t.Errorf("missing/unreachable physical landmark %s: %+v", id, at)
		}
	}
	initial := map[Point]string{}
	for _, id := range []string{"hero_start", "varya", "repair_worker", "senya_start", "inspector", "egor"} {
		at, exists := l.Anchors[id]
		if !exists || !reached[at] {
			t.Errorf("actor anchor %s is missing or outside floor: %+v", id, at)
		}
		if other, occupied := initial[at]; occupied {
			t.Errorf("initial actor anchors overlap: %s and %s at %+v", id, other, at)
		}
		initial[at] = id
	}
	for id, at := range l.Anchors {
		if !reached[at] {
			t.Errorf("routine anchor %s is not reachable: %+v", id, at)
		}
	}
	if len(l.Stores) != 4 {
		t.Fatalf("expected four preserved fixed stores, got %d", len(l.Stores))
	}
	for id, at := range l.Stores {
		if floor[at] {
			t.Errorf("physical store %s can be walked through: %+v", id, at)
		}
		accessible := false
		for _, delta := range expandedAuditDirections {
			accessible = accessible || reached[Point{at.X + delta.X, at.Y + delta.Y}]
		}
		if !accessible {
			t.Errorf("physical store %s cannot be reached from an adjacent floor cell", id)
		}
	}
	if l.LoopProbe == nil {
		t.Fatal("expanded scene must author representative loop_probe endpoints")
	}
	circulationPaths := expandedAuditDisjoint(floor, l.LoopProbe.From, l.LoopProbe.To)
	if circulationPaths < 2 {
		t.Errorf("circulation has a single-cell chokepoint between loop probes: paths=%d", circulationPaths)
	}
	for _, at := range l.ProofWaypoints {
		if !reached[at] {
			t.Errorf("native proof target is not reachable: %+v", at)
		}
	}
	if floor[l.ProofBlocked] {
		t.Error("native blocked-target fixture is actually walkable")
	}
	t.Logf("expanded painted market: reached=%d floor=%d shop_approaches=%d fixed_stores=%d independent_circulation_paths=%d", len(reached), len(floor), len(l.ShopApproaches), len(l.Stores), circulationPaths)
}

var expandedAuditDirections = []Point{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

func expandedAuditFloor(terrain []string) map[Point]bool {
	floor := map[Point]bool{}
	for y, row := range terrain {
		for x, glyph := range []byte(row) {
			if glyph == '.' {
				floor[Point{x, y}] = true
			}
		}
	}
	return floor
}

func expandedAuditReach(floor map[Point]bool, from Point) map[Point]bool {
	reached := map[Point]bool{}
	if !floor[from] {
		return reached
	}
	reached[from] = true
	queue := []Point{from}
	for head := 0; head < len(queue); head++ {
		at := queue[head]
		for _, d := range expandedAuditDirections {
			next := Point{at.X + d.X, at.Y + d.Y}
			if floor[next] && !reached[next] {
				reached[next] = true
				queue = append(queue, next)
			}
		}
	}
	return reached
}

// Vertex splitting catches a single-cell bottleneck even when both large halves
// of a hall are connected. Flow is capped at two; it does not solve NPC movement.
func expandedAuditDisjoint(floor map[Point]bool, from, to Point) int {
	if !floor[from] || !floor[to] || from == to {
		return 0
	}
	ids := map[Point]int{}
	for at := range floor {
		ids[at] = len(ids)
	}
	type edge struct{ from, to int }
	capacity := map[edge]int{}
	adjacent := make([][]int, len(ids)*2)
	add := func(a, b, value int) {
		key, reverse := edge{a, b}, edge{b, a}
		if _, exists := capacity[key]; !exists {
			adjacent[a] = append(adjacent[a], b)
			adjacent[b] = append(adjacent[b], a)
			capacity[reverse] = 0
		}
		capacity[key] += value
	}
	for at, id := range ids {
		cap := 1
		if at == from || at == to {
			cap = 2
		}
		add(id*2, id*2+1, cap)
		for _, d := range expandedAuditDirections {
			if neighbor, exists := ids[Point{at.X + d.X, at.Y + d.Y}]; exists {
				add(id*2+1, neighbor*2, 1)
			}
		}
	}
	source, sink, flow := ids[from]*2, ids[to]*2+1, 0
	for flow < 2 {
		parent := make([]int, len(adjacent))
		for i := range parent {
			parent[i] = -1
		}
		parent[source] = source
		queue := []int{source}
		for head := 0; head < len(queue) && parent[sink] == -1; head++ {
			at := queue[head]
			for _, next := range adjacent[at] {
				if parent[next] == -1 && capacity[edge{at, next}] > 0 {
					parent[next] = at
					queue = append(queue, next)
				}
			}
		}
		if parent[sink] == -1 {
			break
		}
		for at := sink; at != source; at = parent[at] {
			previous := parent[at]
			capacity[edge{previous, at}]--
			capacity[edge{at, previous}]++
		}
		flow++
	}
	return flow
}

func TestExpandedMarketAuditDistinguishesConnectivityFromCirculation(t *testing.T) {
	for _, test := range []struct {
		name    string
		terrain []string
		from    Point
		to      Point
		paths   int
	}{
		{"one narrow lane", []string{"#######", "#.....#", "#######"}, Point{1, 1}, Point{5, 1}, 1},
		{"ring around shop", []string{"#######", "#.....#", "#.###.#", "#.....#", "#######"}, Point{1, 2}, Point{5, 2}, 2},
		{"two halls with one bridge", []string{"#########", "#...#...#", "#.......#", "#...#...#", "#########"}, Point{1, 2}, Point{7, 2}, 1},
		{"isolated destination", []string{"#######", "#..#..#", "#######"}, Point{1, 1}, Point{5, 1}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if n := expandedAuditDisjoint(expandedAuditFloor(test.terrain), test.from, test.to); n != test.paths {
				t.Fatalf("independent circulation paths=%d want %d", n, test.paths)
			}
		})
	}
}
