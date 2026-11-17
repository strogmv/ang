package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func edgeFogFixture() (EdgeFog, func(string) bool) {
	wide, why := 8.0, "the hub's deck ends are its two exits and the rail lies 6 m below"
	e := EdgeFog{EdgeFogNumbers: EdgeFogNumbers{StartM: 4, WidthM: 6, ExitM: 3, DepthStartM: 7, DepthM: 12, Color: "#17130F"},
		Levels: []EdgeFogLevel{{LevelID: "level.hub", Why: why, WidthM: &wide}}}
	known := func(id string) bool { return id == "level.market" || id == "level.hub" }
	return e, known
}

func TestEdgeFogAcceptedAndProjected(t *testing.T) {
	e, known := edgeFogFixture()
	if err := checkEdgeFog(e, known); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(clientEdgeFog(&e))
	for _, want := range []string{
		`"default":{"color":"#17130F","depth_m":12,"depth_start_m":7,"exit_m":3,"start_m":4,"width_m":6}`,
		// a level's own number over the shared ones; the rest stay shared
		`"level.hub":{"color":"#17130F","depth_m":12,"depth_start_m":7,"exit_m":3,"start_m":4,"width_m":8}`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("client fog lacks %s: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "why") || strings.Contains(string(raw), "level.market") {
		t.Fatalf("the reasons stay in CUE and a level without its own numbers takes the default: %s", raw)
	}
}

func TestEdgeFogRejected(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := map[string]func(e *EdgeFog){
		"start_negative":   func(e *EdgeFog) { e.StartM = -1 },
		"start_far":        func(e *EdgeFog) { e.StartM = 9 },
		"width_hard":       func(e *EdgeFog) { e.WidthM = 0.5 },
		"width_far":        func(e *EdgeFog) { e.WidthM = 20 },
		"exit_hard":        func(e *EdgeFog) { e.ExitM = 0.5 },
		"exit_long":        func(e *EdgeFog) { e.ExitM = 9 },
		"depth_start_deep": func(e *EdgeFog) { e.DepthStartM = 13 },
		"depth_thin":       func(e *EdgeFog) { e.DepthM = e.DepthStartM + 0.5 },
		"depth_far":        func(e *EdgeFog) { e.DepthM = 31 },
		"colour_form":      func(e *EdgeFog) { e.Color = "black" },
		"colour_bright":    func(e *EdgeFog) { e.Color = "#404040" },
		"colour_cold":      func(e *EdgeFog) { e.Color = "#0A0F18" },
		"unknown_level":    func(e *EdgeFog) { e.Levels[0].LevelID = "level.nowhere" },
		"level_twice":      func(e *EdgeFog) { e.Levels = append(e.Levels, e.Levels[0]) },
		"no_reason":        func(e *EdgeFog) { e.Levels[0].Why = "" },
		"empty_override":   func(e *EdgeFog) { e.Levels[0].WidthM = nil },
		"override_bad":     func(e *EdgeFog) { e.Levels[0].ExitM = f(0.2) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e, known := edgeFogFixture()
			mutate(&e)
			if err := checkEdgeFog(e, known); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

// A world with levels has fog, and every level has floor for its edge.
func TestEdgeFogRequiredAndMeasurable(t *testing.T) {
	life := []byte(`{"locations":{"level.a":{"terrain":["#.#","###"]},"level.b":{"terrain":["###"]}}}`)
	walk := map[string]bool{".": true}
	c := Compiled{Life: life, Walkable: walk}
	world := fogWorld(t, `edge_fog: {start_m: 4, width_m: 6, exit_m: 3, depth_start_m: 7, depth_m: 12, color: "#17130F", levels: []}`)
	if _, err := readEdgeFog(world, c); err == nil || !strings.Contains(err.Error(), "level.b has no walkable cell") {
		t.Fatalf("a level without floor passed: %v", err)
	}
	c.Life = []byte(`{"locations":{"level.a":{"terrain":["#.#"]}}}`)
	if fog, err := readEdgeFog(world, c); err != nil || fog == nil {
		t.Fatalf("fog of a measurable level: %v", err)
	}
	if _, err := readEdgeFog(fogWorld(t, `other: 1`), c); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("a world with levels and no fog passed: %v", err)
	}
	if fog, err := readEdgeFog(fogWorld(t, `other: 1`), Compiled{}); err != nil || fog != nil {
		t.Fatalf("a world without levels needs no fog: %v", err)
	}
}

func fogWorld(t *testing.T, source string) cue.Value {
	t.Helper()
	v := cuecontext.New().CompileString(source)
	if v.Err() != nil {
		t.Fatal(v.Err())
	}
	return v
}
