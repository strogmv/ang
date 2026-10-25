package gamepoc

import (
	"encoding/json"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func wayfindingShapeFixture() (WayfindingCatalog, json.RawMessage, LocalizationCatalog) {
	d := WayfindingDestination{ID: "paint.hall", NameKey: "wayfinding.paint", Level: "hall", Color: "#83A7BD", Kind: "landmark", At: WayfindingCell{X: 7, Y: 5}, GuideFrom: &WayfindingCell{X: 1, Y: 1}, GuideRoute: []WayfindingPoint{{X: 1, Y: 1}, {X: 1.45, Y: 1}, {X: 1.45, Y: 5}, {X: 7, Y: 5}}}
	return WayfindingCatalog{Destinations: []WayfindingDestination{d}}, json.RawMessage(`{"locations":{"hall":{"terrain":["#########","#.......#","#.......#","#.......#","#.......#","#.......#","#########"]}},"edges":[]}`), LocalizationCatalog{Messages: []LocalizationMessage{{Key: d.NameKey}}}
}

func TestWayfindingAuthoredShapeCompilesExactBufferedSupport(t *testing.T) {
	c, life, texts := wayfindingShapeFixture()
	before, _ := json.Marshal(c.Destinations[0].GuideRoute)
	if err := c.validate(life, map[string]bool{".": true}, texts); err != nil {
		t.Fatal(err)
	}
	d := c.Destinations[0]
	want := []WayfindingCell{}
	for y := 1; y <= 5; y++ {
		for x := 1; x <= 2; x++ {
			want = append(want, WayfindingCell{X: x, Y: y})
		}
	}
	for x := 3; x <= 7; x++ {
		want = append(want, WayfindingCell{X: x, Y: 5})
	}
	if !reflect.DeepEqual(d.GuideSupport, want) {
		t.Fatalf("paint support loses .10-cell side buffer: got %v want %v", d.GuideSupport, want)
	}
	after, _ := json.Marshal(d.GuideRoute)
	if string(before) != string(after) {
		t.Fatal("compiler rewrote authored centreline")
	}
	encoded, _ := json.Marshal(c)
	if err := c.validate(life, map[string]bool{".": true}, texts); err != nil {
		t.Fatal(err)
	}
	encodedAgain, _ := json.Marshal(c)
	if string(encoded) != string(encodedAgain) {
		t.Fatal("support is not deterministic across repeat validation")
	}
	// Private authoring, including the derived mask, must never be a client asset.
	compiled := paintedFixture()
	compiled.Wayfinding = c
	public, err := clientResource(compiled)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"guide_route", "guide_support", "paint.hall"} {
		if strings.Contains(string(public), secret) {
			t.Fatalf("private paint geometry leaked in Resource: %s", secret)
		}
	}
	// A former materialized support field cannot survive removal of its route.
	c.Destinations[0].GuideRoute = nil
	if err := c.validate(life, map[string]bool{".": true}, texts); err != nil {
		t.Fatal(err)
	}
	legacy, _ := json.Marshal(c)
	if strings.Contains(string(legacy), "guide_route") || strings.Contains(string(legacy), "guide_support") {
		t.Fatal("legacy destination retained stale optional shape geometry")
	}
}

func TestWayfindingAuthoredShapeRejectsUnsafeGeometry(t *testing.T) {
	for name, change := range map[string]func(*WayfindingDestination){
		"needs guide_from": func(d *WayfindingDestination) { d.GuideFrom = nil },
		"too short":        func(d *WayfindingDestination) { d.GuideRoute = d.GuideRoute[:1] },
		"wrong first":      func(d *WayfindingDestination) { d.GuideRoute[0].X += .01 },
		"wrong last":       func(d *WayfindingDestination) { d.GuideRoute[len(d.GuideRoute)-1].Y -= .01 },
		"NaN":              func(d *WayfindingDestination) { d.GuideRoute[1].X = math.NaN() },
		"infinity":         func(d *WayfindingDestination) { d.GuideRoute[1].Y = math.Inf(1) },
		"negative":         func(d *WayfindingDestination) { d.GuideRoute[1].X = -.01 },
		"out of floor":     func(d *WayfindingDestination) { d.GuideRoute[1].Y = 7 },
		"duplicate point":  func(d *WayfindingDestination) { d.GuideRoute[1] = d.GuideRoute[0] },
		"65 points":        func(d *WayfindingDestination) { d.GuideRoute = make([]WayfindingPoint, 65) },
	} {
		t.Run(name, func(t *testing.T) {
			c, life, texts := wayfindingShapeFixture()
			change(&c.Destinations[0])
			if err := c.validate(life, map[string]bool{".": true}, texts); err == nil || !strings.Contains(err.Error(), "guide_route paint.hall") {
				t.Fatalf("invalid authored shape accepted or missing diagnostic: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name, rows string
		route      []WayfindingPoint
	}{
		{"middle wall despite connected fallback", `["#########","#.......#","#.......#","#...#...#","#.......#","#.......#","#########"]`, []WayfindingPoint{{X: 1, Y: 1}, {X: 7, Y: 5}}},
		{"side buffer clips wall", `["#########","#.......#","#.......#","#.#.....#","#.......#","#.......#","#########"]`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, texts := wayfindingShapeFixture()
			if tc.route != nil {
				c.Destinations[0].GuideRoute = tc.route
			}
			life := json.RawMessage(`{"locations":{"hall":{"terrain":` + tc.rows + `}},"edges":[]}`)
			if err := c.validate(life, map[string]bool{".": true}, texts); err == nil || !strings.Contains(err.Error(), "buffered paint crosses blocked cell") {
				t.Fatalf("unsafe buffered segment accepted: %v", err)
			}
		})
	}
}

func TestWayfindingShapeDDAAgreesWithFullFloorCapsuleEnumeration(t *testing.T) {
	const size = 20
	rows := make([]string, size)
	for i := range rows {
		rows[i] = strings.Repeat(".", size)
	}
	rng := rand.New(rand.NewSource(29))
	for n := 0; n < 150; n++ {
		d := WayfindingDestination{GuideFrom: &WayfindingCell{X: 1, Y: 1}, At: WayfindingCell{X: 18, Y: 18}, GuideRoute: []WayfindingPoint{{X: 1, Y: 1}, {X: 1 + rng.Float64()*17, Y: 1 + rng.Float64()*17}, {X: 18, Y: 18}}}
		got, err := wayfindingGuideSupport(d, rows, func(p WayfindingCell) bool { return p.X >= 0 && p.Y >= 0 && p.X < size && p.Y < size })
		if err != nil {
			t.Fatal(err)
		}
		var want []WayfindingCell
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				p := WayfindingCell{X: x, Y: y}
				for i := 1; i < len(d.GuideRoute); i++ {
					if wayfindingSegmentSquareDistanceSquared(d.GuideRoute[i-1], d.GuideRoute[i], p) <= .01+1e-12 {
						want = append(want, p)
						break
					}
				}
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("DDA dropped capsule cells for %v: got%v want%v", d.GuideRoute, got, want)
		}
	}
	for _, tc := range []struct {
		a, b WayfindingPoint
		cell WayfindingCell
		want float64
	}{
		{WayfindingPoint{X: 1, Y: 1}, WayfindingPoint{X: 4, Y: 1}, WayfindingCell{X: 2, Y: 2}, .25},
		{WayfindingPoint{X: 1, Y: 1.41}, WayfindingPoint{X: 4, Y: 1.41}, WayfindingCell{X: 2, Y: 2}, .0081},
		{WayfindingPoint{X: 1, Y: 1}, WayfindingPoint{X: 3, Y: 3}, WayfindingCell{X: 2, Y: 1}, 0},
		{WayfindingPoint{X: 1, Y: 1}, WayfindingPoint{X: 2, Y: 1}, WayfindingCell{X: 3, Y: 2}, .5},
	} {
		if got := wayfindingSegmentSquareDistanceSquared(tc.a, tc.b, tc.cell); math.Abs(got-tc.want) > 1e-10 {
			t.Fatalf("analytic segment/square distance got%g want%g", got, tc.want)
		}
	}
}

func TestWayfindingAuthoredShapePointAndSupportBudgets(t *testing.T) {
	c, life, texts := wayfindingShapeFixture()
	c.Destinations[0].GuideRoute = make([]WayfindingPoint, 64)
	for i := range c.Destinations[0].GuideRoute {
		fraction := float64(i) / 63
		c.Destinations[0].GuideRoute[i] = WayfindingPoint{X: 1 + 6*fraction, Y: 1 + 4*fraction}
	}
	if err := c.validate(life, map[string]bool{".": true}, texts); err != nil {
		t.Fatalf("64-point supported route rejected: %v", err)
	}
	for _, tc := range []struct {
		width, count int
		diagnostic   string
	}{{17000, 1, "buffered support exceeds"}, {9001, 2, "guide_route support exceeds"}} {
		base := WayfindingDestination{ID: "long", NameKey: "wayfinding.paint", Level: "hall", Color: "#83A7BD", Kind: "landmark", At: WayfindingCell{X: 1}, GuideFrom: &WayfindingCell{}, GuideRoute: []WayfindingPoint{{}, {X: float64(tc.width - 1)}, {X: 1}}}
		catalog := WayfindingCatalog{}
		for i := 0; i < tc.count; i++ {
			d := base
			d.ID += strings.Repeat("x", i)
			catalog.Destinations = append(catalog.Destinations, d)
		}
		world := json.RawMessage(`{"locations":{"hall":{"terrain":["` + strings.Repeat(".", tc.width) + `"]}},"edges":[]}`)
		if err := catalog.validate(world, map[string]bool{".": true}, texts); err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
			t.Fatalf("unbounded derived support accepted: %v", err)
		}
	}
}
