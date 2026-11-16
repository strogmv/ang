package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func cameraCutawayFixture() (CameraCutaway, func(string) bool, map[string][]string, map[string]map[string]bool) {
	c := CameraCutaway{StubM: 1.2, ReachM: 10, Seconds: 0.4, HoldSeconds: 1, CapColor: "#16130F", Levels: []CameraCutawayLevel{
		{LevelID: "level.market", OccluderTerrains: []string{"wall", "pillar", "rock"}, Why: "the hall's camera-side ring wall",
			Parts: []CameraCutawayPart{{ModuleID: "kit.geo.stall", Kind: "wall", Why: "the stall's back hides the half-island",
				RegionM: CutawayRegion{X: [2]float64{0.16, 5.9}, Up: [2]float64{1.25, 2.7}, South: [2]float64{0, 0.5}}}}},
		{LevelID: "level.hub", OccluderTerrains: []string{"wall"}, Why: "the hub's bay walls"},
	}, SeeThrough: &CutawaySeeThrough{RadiusM: 1.7, CentreUpM: 0.9, FadeM: 0.35, EdgeM: 0.05, EdgeColor: "#17130F", NearM: 0.25, FloorKeepM: 0.06}}
	known := func(id string) bool { return id == "level.market" || id == "level.hub" }
	// The market is also a world level with the view hint (in another order); the hub is not.
	views := map[string][]string{"level.market": {"rock", "wall", "pillar"}}
	placed := map[string]map[string]bool{"level.market": {"kit.geo.stall": true, "kit.geo.island": true}}
	return c, known, views, placed
}

func TestCameraCutawayAcceptedAndProjected(t *testing.T) {
	c, known, views, placed := cameraCutawayFixture()
	if err := checkCameraCutaway(c, known, views, placed); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(clientCameraCutaway(&c))
	for _, want := range []string{`"stub_m":1.2`, `"reach_m":10`, `"seconds":0.4`, `"hold_seconds":1`, `"cap_color":"#16130F"`,
		`"level.hub":{"occluder_terrains":["wall"]}`, `"occluder_terrains":["wall","pillar","rock"]`,
		`"parts":[{"kind":"wall","module_id":"kit.geo.stall","region_m":{"x":[0.16,5.9],"up":[1.25,2.7],"south":[0,0.5]}}]`,
		`"see_through":{"radius_m":1.7,"centre_up_m":0.9,"fade_m":0.35,"edge_m":0.05,"edge_color":"#17130F","near_m":0.25,"floor_keep_m":0.06}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("client cutaway lacks %s: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "why") {
		t.Fatalf("the reasons stay in CUE: %s", raw)
	}
}

func TestCameraCutawayRejected(t *testing.T) {
	cases := map[string]func(c *CameraCutaway, views map[string][]string){
		"stub_low":      func(c *CameraCutaway, _ map[string][]string) { c.StubM = 0.1 },
		"stub_high":     func(c *CameraCutaway, _ map[string][]string) { c.StubM = 4 },
		"reach_low":     func(c *CameraCutaway, _ map[string][]string) { c.ReachM = 1 },
		"reach_high":    func(c *CameraCutaway, _ map[string][]string) { c.ReachM = 40 },
		"seconds_zero":  func(c *CameraCutaway, _ map[string][]string) { c.Seconds = 0 },
		"seconds_long":  func(c *CameraCutaway, _ map[string][]string) { c.Seconds = 5 },
		"hold_negative": func(c *CameraCutaway, _ map[string][]string) { c.HoldSeconds = -1 },
		"colour":        func(c *CameraCutaway, _ map[string][]string) { c.CapColor = "black" },
		"no_levels":     func(c *CameraCutaway, _ map[string][]string) { c.Levels = nil },
		"unknown_level": func(c *CameraCutaway, _ map[string][]string) { c.Levels[1].LevelID = "level.nowhere" },
		"level_twice":   func(c *CameraCutaway, _ map[string][]string) { c.Levels[1].LevelID = "level.market" },
		"no_reason":     func(c *CameraCutaway, _ map[string][]string) { c.Levels[1].Why = "" },
		"no_kinds":      func(c *CameraCutaway, _ map[string][]string) { c.Levels[1].OccluderTerrains = nil },
		"kind_twice":    func(c *CameraCutaway, _ map[string][]string) { c.Levels[1].OccluderTerrains = []string{"wall", "wall"} },
		"floor_occluder": func(c *CameraCutaway, _ map[string][]string) {
			c.Levels[1].OccluderTerrains = []string{"wall", "floor"}
		},
		"view_differs": func(_ *CameraCutaway, views map[string][]string) { views["level.market"] = []string{"wall"} },
		// the cut parts of a module (the see-through zone)
		"part_unplaced_module": func(c *CameraCutaway, _ map[string][]string) { c.Levels[0].Parts[0].ModuleID = "kit.geo.nowhere" },
		"part_no_module":       func(c *CameraCutaway, _ map[string][]string) { c.Levels[0].Parts[0].ModuleID = "" },
		"part_twice": func(c *CameraCutaway, _ map[string][]string) {
			c.Levels[0].Parts = append(c.Levels[0].Parts, c.Levels[0].Parts[0])
		},
		"part_not_occluder": func(c *CameraCutaway, _ map[string][]string) { c.Levels[0].Parts[0].Kind = "counter" },
		"part_no_reason":    func(c *CameraCutaway, _ map[string][]string) { c.Levels[0].Parts[0].Why = "" },
		"part_below_stub":   func(c *CameraCutaway, _ map[string][]string) { c.Levels[0].Parts[0].RegionM.Up[0] = 0.9 },
		"part_empty_region": func(c *CameraCutaway, _ map[string][]string) {
			c.Levels[0].Parts[0].RegionM.South = [2]float64{0.5, 0.5}
		},
		"part_on_level_without_module": func(c *CameraCutaway, _ map[string][]string) {
			c.Levels[1].Parts = append([]CameraCutawayPart{}, c.Levels[0].Parts...)
		},
		// the see-through circle
		"see_radius_small":  func(c *CameraCutaway, _ map[string][]string) { c.SeeThrough.RadiusM = 0.2 },
		"see_radius_screen": func(c *CameraCutaway, _ map[string][]string) { c.SeeThrough.RadiusM = 9 },
		"see_fade_zero":     func(c *CameraCutaway, _ map[string][]string) { c.SeeThrough.FadeM = 0 },
		"see_fade_wide":     func(c *CameraCutaway, _ map[string][]string) { c.SeeThrough.FadeM = 2 },
		"see_edge_thick":    func(c *CameraCutaway, _ map[string][]string) { c.SeeThrough.EdgeM = 0.5 },
		"see_edge_colour":   func(c *CameraCutaway, _ map[string][]string) { c.SeeThrough.EdgeColor = "dark" },
		"see_near_far":      func(c *CameraCutaway, _ map[string][]string) { c.SeeThrough.NearM = 3 },
		"see_centre_high":   func(c *CameraCutaway, _ map[string][]string) { c.SeeThrough.CentreUpM = 2.5 },
		"see_floor_cut":     func(c *CameraCutaway, _ map[string][]string) { c.SeeThrough.FloorKeepM = -0.1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c, known, views, placed := cameraCutawayFixture()
			c.Levels = append([]CameraCutawayLevel{}, c.Levels...)
			c.Levels[0].Parts = append([]CameraCutawayPart{}, c.Levels[0].Parts...)
			see := *c.SeeThrough
			c.SeeThrough = &see
			mutate(&c, views)
			if err := checkCameraCutaway(c, known, views, placed); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
