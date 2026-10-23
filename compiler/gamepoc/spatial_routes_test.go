package gamepoc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func routeFixture() SpatialCatalog {
	c := spatialFixture()
	c.Levels[0].Routes = []SpatialRoute{{ID: "trade-route", From: SpatialEndpoint{Ref: "exit"}, To: SpatialEndpoint{Ref: "trade"}, StyleID: "industrial", Symbol: "M"}}
	return c
}
func TestSpatialRoutesAvoidPropsAndPreserveGeometry(t *testing.T) {
	c := routeFixture()
	terrain := append([]string(nil), c.Levels[0].Terrain...)
	if e := c.compile(spatialLegend()); e != nil {
		t.Fatal(e)
	}
	l := c.Levels[0]
	if !reflect.DeepEqual(terrain, l.Terrain) {
		t.Fatal("changed navigation")
	}
	cells := map[Point]bool{}
	symbols := 0
	for _, d := range l.Decorations {
		if d.RouteID == "" {
			continue
		}
		if !spatialLegend()[l.Terrain[d.At.Y][d.At.X:d.At.X+1]] {
			t.Fatal("paint through obstacle")
		}
		if d.Symbol != "" {
			symbols++
		}
		cells[d.At] = true
		if d.Surface != "floor" {
			t.Fatal("paint not flat")
		}
	}
	if !cells[Point{X: 7, Y: 3}] || !cells[Point{X: 3, Y: 1}] || symbols < 2 {
		t.Fatal("missing endpoints")
	}
	before := append([]SpatialDecoration(nil), l.Decorations...)
	if e := c.compile(spatialLegend()); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, c.Levels[0].Decorations) {
		t.Fatal("recompile changed marks")
	}
}
func TestSpatialRouteBFSCardinalShortestAndUnreachable(t *testing.T) {
	l := SpatialLevel{Width: 5, Height: 5}
	floor := make([]bool, 25)
	for i := range floor {
		floor[i] = true
	}
	floor[2*5+2] = false
	budget := 1000
	path, e := l.routePath(floor, Point{X: 1, Y: 2}, Point{X: 3, Y: 2}, &budget)
	if e != nil {
		t.Fatal(e)
	}
	if len(path) != 5 {
		t.Fatal(path)
	}
	for i, p := range path {
		if i > 0 && spatialDistance(path[i-1], p) != 1 {
			t.Fatal("noncardinal")
		}
		if !floor[p.Y*5+p.X] {
			t.Fatal("obstacle")
		}
	}
	for y := 0; y < 5; y++ {
		floor[y*5+2] = false
	}
	budget = 1000
	if _, e = l.routePath(floor, Point{X: 1, Y: 2}, Point{X: 3, Y: 2}, &budget); e == nil || !strings.Contains(e.Error(), "unreachable") {
		t.Fatal(e)
	}
	budget = 0
	floor[12] = true
	if _, e = l.routePath(floor, Point{X: 1, Y: 2}, Point{X: 3, Y: 2}, &budget); e == nil {
		t.Fatal("unbounded search")
	}
}
func TestSpatialRoutesOrderAndProjection(t *testing.T) {
	a := routeFixture()
	r := a.Levels[0].Routes[0]
	r.ID = "second"
	r.To = SpatialEndpoint{At: &Point{X: 1, Y: 5}}
	a.Levels[0].Routes = append(a.Levels[0].Routes, r)
	b := routeFixture()
	b.Levels[0].Routes = []SpatialRoute{r, b.Levels[0].Routes[0]}
	if e := a.compile(spatialLegend()); e != nil {
		t.Fatal(e)
	}
	if e := b.compile(spatialLegend()); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(a.Levels[0].Decorations, b.Levels[0].Decorations) {
		t.Fatal("input order changes paint")
	}
}
func TestSpatialRoutesClientProjectionContainsPaintOnly(t *testing.T) {
	c := routeFixture()
	if e := c.compile(spatialLegend()); e != nil {
		t.Fatal(e)
	}
	data, e := json.Marshal(clientSpatial(c))
	if e != nil {
		t.Fatal(e)
	}
	var v struct{ Levels []map[string]json.RawMessage }
	if e = json.Unmarshal(data, &v); e != nil {
		t.Fatal(e)
	}
	if _, ok := v.Levels[0]["routes"]; ok {
		t.Fatal("source endpoints leaked to view")
	}
	var marks []SpatialDecoration
	if e = json.Unmarshal(v.Levels[0]["decorations"], &marks); e != nil {
		t.Fatal(e)
	}
	found := false
	for _, m := range marks {
		if m.RouteID == "trade-route" && m.Symbol == "M" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing route symbol projection")
	}
}

func TestSpatialRoutesRejectInvalidEndpointsAndPolicy(t *testing.T) {
	for _, name := range []string{"missing", "blocked", "both", "neither", "same", "style", "symbol", "duplicate", "many"} {
		t.Run(name, func(t *testing.T) {
			c := routeFixture()
			r := &c.Levels[0].Routes[0]
			switch name {
			case "missing":
				r.To.Ref = "absent"
			case "blocked":
				r.To = SpatialEndpoint{At: &Point{X: 3, Y: 2}}
			case "both":
				r.To.At = &Point{X: 1, Y: 1}
			case "neither":
				r.To = SpatialEndpoint{}
			case "same":
				r.To = r.From
			case "style":
				r.StyleID = "absent"
			case "symbol":
				r.Symbol = "toolong"
			case "duplicate":
				c.Levels[0].Routes = append(c.Levels[0].Routes, *r)
			case "many":
				for len(c.Levels[0].Routes) < 65 {
					c.Levels[0].Routes = append(c.Levels[0].Routes, *r)
				}
			}
			if e := c.compile(spatialLegend()); e == nil {
				t.Fatal("accepted")
			}
		})
	}
}
