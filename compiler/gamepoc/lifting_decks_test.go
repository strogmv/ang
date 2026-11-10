package gamepoc

import (
	"cuelang.org/go/cue/cuecontext"
	"encoding/json"
	"strings"
	"testing"
)

func TestLiftingDeckDataCanDeclareSecondLocation(t *testing.T) {
	raw := []byte(`{"decks":[{"id":"deck.second","level_id":"second","instance_id":"fixture","name_key":"ui.deck.second","access":"open","manual":false,"closed_walkable":true,"rect":{"x":0,"y":0,"w":2,"h":2},"barrier_rect":{"x":0,"y":2,"w":2,"h":1},"approach":{"x":2,"y":1},"plan_m":[0,0,0],"control_plan_m":[0,0,0],"depth_m":3.65,"chamber_speed_mps":0.4,"lockdown_speed_mps":0.8,"power_speed_mps":0.6,"shaft_depth_m":500,"shaft_speed_mps":2,"travel_edge":"","return_edge":"","runs":{"period_seconds":600,"day_start_second":21600,"day_end_second":79200,"extra_seconds":[16800]},"signal_before_open_s":10,"signal_before_descend_s":10,"timing_ms":{"unlock":1000,"unseat":1000,"reseat":600,"lock":500,"beacon_period":250,"equalise":6000,"barrier":2800,"open":6000,"loading":20000,"close":6000,"lockdown_warning":1200,"lockdown_barrier":800,"lockdown_close":1600,"retry":5000},"nodes":[{"name":"deck.small","channel":"deck","axis":"y","min":-3.65,"max":0,"rotation":false}]}]}`)
	var c LiftingDeckCatalog
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	life := json.RawMessage(`{"locations":{"second":{"terrain":["....","....","...."]}},"edges":[]}`)
	if err := c.validate(life); err != nil {
		t.Fatal(err)
	}

	c.Decks[0].SignalBeforeOpenS = 9
	if err := c.validate(life); err == nil || !strings.Contains(err.Error(), "ten physical seconds") {
		t.Fatal("short normal signal accepted")
	}
	c.Decks[0].SignalBeforeOpenS = 10
	// The second placement is compiled from CUE data through the same reader.
	declaration := cuecontext.New().CompileString("lifting_decks: " + string(raw) + `
localization:{messages:[{key:"ui.deck.second",context:"Second location fixture.",texts:{ru:"Вторая платформа",en:"Second deck"}}]}`)
	decoded, err := readLiftingDecks(declaration, life)
	if err != nil || len(decoded.Decks) != 1 || decoded.Decks[0].LevelID != "second" {
		t.Fatalf("second CUE deck declaration: %v", err)
	}
	if len(decoded.Decks[0].BarrierFloor) != 1 || decoded.Decks[0].BarrierFloor[0] != ".." {
		t.Fatal("second layout lost its authored barrier floor")
	}
	blockedBase := json.RawMessage(`{"locations":{"second":{"terrain":["....","....","++.."]}},"edges":[]}`)
	blocked, err := readLiftingDecks(declaration, blockedBase)
	if err != nil || blocked.Decks[0].BarrierFloor[0] != "++" {
		t.Fatalf("legacy blocked barrier footprint changed: %v", err)
	}
	c.Decks[0].Sounds = []LiftingDeckSound{{Node: "second.control", Channel: "horn", Resource: "res://assets/audio/deck_horn.wav", Offset: []float64{1, 2, 3}, VolumeDB: -20, Loop: true}}
	if err := c.validate(life); err != nil {
		t.Fatalf("second location sound binding: %v", err)
	}
	c.Decks[0].Sounds[0].Channel = "local_cycle"
	if err := c.validate(life); err == nil || !strings.Contains(err.Error(), "sound") {
		t.Fatal("client-owned sound phase accepted")
	}
	c.Decks[0].Sounds[0].Channel = "horn"
	c.Decks[0].Access = "invented"
	if err := c.validate(life); err == nil || !strings.Contains(err.Error(), "access") {
		t.Fatal("unknown access mode accepted")
	}
	c.Decks[0].Access = "permit"
	if err := c.validate(life); err != nil {
		t.Fatal(err)
	}
	c.Decks[0].BarrierRect.Y = 1
	if err := c.validate(life); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatal("barrier could restore ordinary floor inside an open shaft")
	}
	c.Decks[0].BarrierRect.Y = 2
	c.Decks[0].Approach.X = 1
	if err := c.validate(life); err == nil || !strings.Contains(err.Error(), "waiting approach") {
		t.Fatal("waiting target could block its own opening cycle")
	}
	c.Decks[0].Approach.X = 2
	c.Decks[0].Rect.W = 5
	if err := c.validate(life); err == nil {
		t.Fatal("shaft outside floor accepted")
	}
}

func TestLiftingDeckPopulationAnchorsExcludeOnlyMovingApertures(t *testing.T) {
	var c LiftingDeckCatalog
	if err := json.Unmarshal([]byte(`{"decks":[{"level_id":"hub","rect":{"x":1,"y":1,"w":2,"h":2}},{"level_id":"second","rect":{"x":0,"y":0,"w":1,"h":1}}]}`), &c); err != nil {
		t.Fatal(err)
	}
	life := json.RawMessage(`{"population":{"max_actors":100,"anchors":{"hub":[{"x":0,"y":0},{"x":1,"y":1},{"x":2,"y":2},{"x":3,"y":3}],"second":[{"x":0,"y":0},{"x":1,"y":0}],"untouched":[{"x":0,"y":0}]}},"marker":"keep"}`)
	got, err := filterLiftingDeckPopulationAnchors(life, &c)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Population struct {
			MaxActors int `json:"max_actors"`
			Anchors   map[string][]struct{ X, Y int }
		}
		Marker string
	}
	if err = json.Unmarshal(got, &result); err != nil {
		t.Fatal(err)
	}
	if result.Population.MaxActors != 100 || result.Marker != "keep" || len(result.Population.Anchors["hub"]) != 2 || result.Population.Anchors["hub"][1].X != 3 || len(result.Population.Anchors["second"]) != 1 || len(result.Population.Anchors["untouched"]) != 1 {
		t.Fatalf("wrong population anchor reservation: %s", got)
	}
	again, err := filterLiftingDeckPopulationAnchors(got, &c)
	if err != nil || string(again) != string(got) {
		t.Fatal("moving-floor reservation is not byte stable")
	}
}
