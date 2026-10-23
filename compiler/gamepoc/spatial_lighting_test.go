package gamepoc

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func lightingFixture() SpatialCatalog {
	c := spatialFixture()
	c.Levels[0].Lighting = &SpatialLighting{Rooms: SpatialLightRule{FixtureID: "ceiling.room", SpacingCells: 4, RangeMeters: 3, Intensity: .25, Color: "#AABBCC"}, Corridors: SpatialLightRule{FixtureID: "ceiling.strip", SpacingCells: 3, RangeMeters: 2, Intensity: .35, Color: "#CCDDEE"}, Circuits: []string{"light.a", "light.b"}, EmergencyEvery: 3, MaxFixtures: 32, RequireServiceAccess: true}
	return c
}
func TestSpatialLightingCoverageServiceAndProjection(t *testing.T) {
	c := lightingFixture()
	terrain := append([]string(nil), c.Levels[0].Terrain...)
	if e := c.compile(spatialLegend()); e != nil {
		t.Fatal(e)
	}
	l := c.Levels[0]
	if !reflect.DeepEqual(terrain, l.Terrain) {
		t.Fatal("lighting changed navigation")
	}
	if len(l.Lights) == 0 || len(l.Lights) > 32 {
		t.Fatal(len(l.Lights))
	}
	floor := make([]bool, l.Width*l.Height)
	for y, row := range l.Terrain {
		for x := range row {
			floor[y*l.Width+x] = spatialLegend()[row[x:x+1]]
		}
	}
	covered := make([]bool, len(floor))
	kinds := map[string]bool{}
	for i, v := range l.Lights {
		p := v.At
		if !floor[p.Y*l.Width+p.X] || p == l.Exits[0].At || p == l.Exits[0].Landing {
			t.Fatal("invalid anchor")
		}
		if v.ServiceAt == nil || spatialDistance(p, *v.ServiceAt) != 1 || !floor[v.ServiceAt.Y*l.Width+v.ServiceAt.X] {
			t.Fatal("missing service access")
		}
		if v.Circuit != l.Lighting.Circuits[i%2] || v.Emergency != (i%3 == 0) {
			t.Fatal("circuit/emergency mapping")
		}
		kinds[v.Kind] = true
		b := 10000
		if e := l.lightFlood(floor, p.Y*l.Width+p.X, int(v.RangeMeters), &b, func(cell, d int) bool { covered[cell] = true; return false }); e != nil {
			t.Fatal(e)
		}
	}
	if !kinds["room"] || !kinds["corridor"] {
		t.Fatal(kinds)
	}
	for i, v := range floor {
		if v && !covered[i] {
			t.Fatalf("uncovered%d", i)
		}
	}
	before := append([]SpatialLight(nil), l.Lights...)
	if e := c.compile(spatialLegend()); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, c.Levels[0].Lights) {
		t.Fatal("recompile nondeterminism")
	}
	data, e := json.Marshal(clientSpatial(c))
	if e != nil {
		t.Fatal(e)
	}
	var wire struct {
		Levels []struct {
			Lights []SpatialLight `json:"lights"`
		}
	}
	if e = json.Unmarshal(data, &wire); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(wire.Levels[0].Lights, before) {
		t.Fatal("projection lost lights")
	}
}
func TestSpatialLightingWallsAndBoundedSearch(t *testing.T) {
	l := SpatialLevel{Width: 5, Height: 3}
	floor := []bool{true, true, false, true, true, true, true, false, true, true, true, true, true, true, true}
	b := 100
	seen := map[int]bool{}
	if e := l.lightFlood(floor, 1, 3, &b, func(i, d int) bool { seen[i] = true; return false }); e != nil {
		t.Fatal(e)
	}
	if seen[3] {
		t.Fatal("light coverage crossed wall")
	}
	b = 0
	if e := l.lightFlood(floor, 1, 3, &b, func(i, d int) bool { return false }); e == nil {
		t.Fatal("unbounded work")
	}
}
func TestSpatialLightingInvalidPoliciesFail(t *testing.T) {
	for _, name := range []string{"range", "infinite", "nan", "intensity", "spacing", "color", "circuits", "emergency", "budget", "fixture"} {
		t.Run(name, func(t *testing.T) {
			c := lightingFixture()
			p := c.Levels[0].Lighting
			switch name {
			case "range":
				p.Rooms.RangeMeters = 0
			case "infinite":
				p.Rooms.RangeMeters = math.Inf(1)
			case "nan":
				p.Rooms.Intensity = math.NaN()
			case "intensity":
				p.Corridors.Intensity = 2
			case "spacing":
				p.Rooms.SpacingCells = 0
			case "color":
				p.Rooms.Color = "oops"
			case "circuits":
				p.Circuits = []string{"same", "same"}
			case "emergency":
				p.EmergencyEvery = 0
			case "budget":
				p.MaxFixtures = 1
				p.Rooms.RangeMeters = 1
				p.Corridors.RangeMeters = 1
			case "fixture":
				p.Rooms.FixtureID = ""
			}
			if e := c.compile(spatialLegend()); e == nil {
				t.Fatal("accepted")
			}
		})
	}
}
func TestSpatialLightingOptionalAndOrderStable(t *testing.T) {
	c := spatialFixture()
	if e := c.compile(spatialLegend()); e != nil {
		t.Fatal(e)
	}
	if len(c.Levels[0].Lights) != 0 {
		t.Fatal("legacy got lights")
	}
	a, b := lightingFixture(), lightingFixture()
	b.Levels[0].Zones[0], b.Levels[0].Zones[1] = b.Levels[0].Zones[1], b.Levels[0].Zones[0]
	if e := a.compile(spatialLegend()); e != nil {
		t.Fatal(e)
	}
	if e := b.compile(spatialLegend()); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(a.Levels[0].Lights, b.Levels[0].Lights) {
		t.Fatal("zone input order changes lights")
	}
}

func TestSpatialLightingRequiresActualServiceCell(t *testing.T) {
	cfg := lightingFixture().Levels[0].Lighting
	level := SpatialLevel{Width: 3, Height: 3, Lighting: cfg}
	floor := make([]bool, 9)
	floor[4] = true
	unique := func(string, string) error { return nil }
	if err := level.compileLights(floor, unique); err == nil {
		t.Fatal("fixture without adjacent service access accepted")
	}
	cfg.RequireServiceAccess = false
	if err := level.compileLights(floor, unique); err != nil {
		t.Fatal(err)
	}
	if len(level.Lights) != 1 || level.Lights[0].ServiceAt != nil {
		t.Fatal("optional service access changed placement contract")
	}
}
