package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func cameraCutawayFixture() (CameraCutaway, func(string) bool, map[string][]string) {
	c := CameraCutaway{StubM: 1.2, ReachM: 10, Seconds: 0.4, HoldSeconds: 1, CapColor: "#16130F", Levels: []CameraCutawayLevel{
		{LevelID: "level.market", OccluderTerrains: []string{"wall", "pillar", "rock"}, Why: "the hall's camera-side ring wall"},
		{LevelID: "level.hub", OccluderTerrains: []string{"wall"}, Why: "the hub's bay walls"},
	}}
	known := func(id string) bool { return id == "level.market" || id == "level.hub" }
	// The market is also a world level with the view hint (in another order); the hub is not.
	views := map[string][]string{"level.market": {"rock", "wall", "pillar"}}
	return c, known, views
}

func TestCameraCutawayAcceptedAndProjected(t *testing.T) {
	c, known, views := cameraCutawayFixture()
	if err := checkCameraCutaway(c, known, views); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(clientCameraCutaway(&c))
	for _, want := range []string{`"stub_m":1.2`, `"reach_m":10`, `"seconds":0.4`, `"hold_seconds":1`, `"cap_color":"#16130F"`,
		`"level.hub":{"occluder_terrains":["wall"]}`, `"level.market":{"occluder_terrains":["wall","pillar","rock"]}`} {
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
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c, known, views := cameraCutawayFixture()
			c.Levels = append([]CameraCutawayLevel{}, c.Levels...)
			mutate(&c, views)
			if err := checkCameraCutaway(c, known, views); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
