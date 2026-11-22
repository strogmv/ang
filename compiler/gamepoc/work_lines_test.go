package gamepoc

import (
	"strings"
	"testing"
)

// Plan 4 of the baseline fixes (21.11.2026): places in a line in a dead end one cell wide need a pocket beside them.
func TestWorkLinesNeedAPocketBesideAPlaceOnTheOnlyWay(t *testing.T) {
	// row 2 x 2..8 is the line, entered from (1,2); row 3 is the aisle under a counter (not the area's floor)
	terrain := []string{"##########", "##########", ".........#", "#.........", "##########"}
	floor := func(p Point) bool {
		return p.Y >= 0 && p.Y < len(terrain) && p.X >= 0 && p.X < len(terrain[p.Y]) && terrain[p.Y][p.X] == '.'
	}
	line := lifeWorkArea{ID: "grill", Workers: []string{"cook", "helper"}}
	for x := 2; x <= 8; x++ {
		line.Cells = append(line.Cells, Point{X: x, Y: 2})
	}
	// the area's door is (2,2): the aisle (1,2) and the floor under the counter row are outside it
	terrain[3] = "##########"
	posts := workLinePosts{"cook": {{X: 6, Y: 2}}, "helper": {{X: 4, Y: 2}}}
	err := checkWorkLines("level.test", line, posts, floor)
	if err == nil || !strings.Contains(err.Error(), "(4,2) of helper") || !strings.Contains(err.Error(), "(6,2) of cook") {
		t.Fatalf("a place on the only way with no pocket was accepted: %v", err)
	}
	// a pocket beside her place (the island F5 shape): accepted
	terrain[3] = "####.#####"
	pocket := line
	pocket.Cells = append(append([]Point(nil), line.Cells...), Point{X: 4, Y: 3})
	if err := checkWorkLines("level.test", pocket, posts, floor); err != nil {
		t.Fatalf("a place with a pocket beside it was refused: %v", err)
	}
	// his place deeper than hers is fine the other way round, and one worker alone has nobody to let pass
	if err := checkWorkLines("level.test", line, workLinePosts{"cook": {{X: 4, Y: 2}}, "helper": {{X: 3, Y: 4}}}, floor); err != nil {
		t.Fatalf("places that cut nothing were refused: %v", err)
	}
	alone := line
	alone.Workers = []string{"cook"}
	if err := checkWorkLines("level.test", alone, workLinePosts{"cook": {{X: 6, Y: 2}, {X: 4, Y: 2}}}, floor); err != nil {
		t.Fatalf("one worker's own places were refused: %v", err)
	}
}
