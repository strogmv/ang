package gamepoc

import (
	"encoding/json"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
	"strings"
	"testing"
)

func TestInfrastructurePaintHasIndependentPhysicalAxesAndApproaches(t *testing.T) {
	c, life, texts := wayfindingShapeFixture()
	d := &c.Destinations[0]
	d.GuideFrame = &WayfindingFrame{U: WayfindingPoint{X: 1}, V: WayfindingPoint{Y: 1}}
	d.GuideRoute = []WayfindingPoint{{X: 1.2, Y: 1}, {X: 1.2, Y: 4.7}, {X: 7, Y: 4.7}}
	if err := c.validate(life, map[string]bool{".": true}, texts); err != nil {
		t.Fatal(err)
	}
	original := append([]WayfindingPoint(nil), d.GuideRoute...)
	d.GuideRoute[1].X += .1
	if err := c.validate(life, map[string]bool{".": true}, texts); err == nil || !strings.Contains(err.Error(), "not aligned") {
		t.Fatalf("arbitrary shortcut accepted as engineered paint: %v", err)
	}
	d.GuideRoute = original
	d.GuideFrame.V = WayfindingPoint{X: 1, Y: .000001}
	if err := c.validate(life, map[string]bool{".": true}, texts); err == nil {
		t.Fatal("degenerate floor frame accepted")
	}
	d.GuideFrame.V = WayfindingPoint{Y: 1}
	d.GuideRoute[0] = WayfindingPoint{X: 2.2, Y: 1}
	if err := c.validate(life, map[string]bool{".": true}, texts); err == nil || !strings.Contains(err.Error(), "within one cell") {
		t.Fatalf("unrelated terminal accepted: %v", err)
	}
}

func TestInfrastructureSubstrateOnlyRestoresActualPropFootprints(t *testing.T) {
	c, _, texts := wayfindingShapeFixture()
	rows := []string{"#########", "#.......#", "#.......#", "#.#.....#", "#.......#", "#.......#", "#########"}
	encoded, _ := json.Marshal(rows)
	life := json.RawMessage(`{"locations":{"hall":{"terrain":` + string(encoded) + `}},"edges":[]}`)
	c.Substrates = map[string]WayfindingSubstrate{"hall": {Terrain: []string{"#########", "#.......#", "#.......#", "#.......#", "#.......#", "#.......#", "#########"}}}
	d := &c.Destinations[0]
	d.GuideRoute = []WayfindingPoint{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 5}, {X: 7, Y: 5}}
	d.GuideFrame = &WayfindingFrame{U: WayfindingPoint{X: 1}, V: WayfindingPoint{Y: 1}}
	if err := c.validate(life, map[string]bool{".": true}, texts); err == nil || !strings.Contains(err.Error(), "cannot turn architecture") {
		t.Fatalf("arbitrary collision bypass accepted: %v", err)
	}
	anchors := map[string]map[string]inv.Cell{"hall": {"storage.known": {X: 2, Y: 3}}}
	if err := c.validate(life, map[string]bool{".": true}, texts, anchors); err != nil {
		t.Fatal(err)
	}
	if len(d.GuideUnderlays) != 1 || d.GuideUnderlays[0] != (WayfindingCell{X: 2, Y: 3}) {
		t.Fatalf("wrong derived underlay: %v", d.GuideUnderlays)
	}
	if rows[3][2] != '#' {
		t.Fatal("paint changed movement terrain")
	}
	// Cannot claim the neighbouring wall as floor using an unrelated prop.
	c.Substrates["hall"].Terrain[3] = "........#"
	if err := c.validate(life, map[string]bool{".": true}, texts, anchors); err == nil {
		t.Fatal("solid edge converted into paint floor")
	}
}
