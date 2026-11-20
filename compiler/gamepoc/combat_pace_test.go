package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func paceOK() CombatPace {
	return CombatPace{
		Default: "turn_based",
		Modes: []CombatPaceMode{
			{ID: "turn_based", TextKey: "ui.combat.pace.turn_based", Move: "walk", WalkMPS: 1.3, BeatShare: 1, PauseS: .35},
			{ID: "fast_turn_based", TextKey: "ui.combat.pace.fast_turn_based", Move: "slide", SlideS: .15, BeatShare: .35, PauseS: .05},
		},
		Setting: "combat_pace", Key: "Q",
		Beats: map[string]float64{"attack": .7, "wound": 0}, BeatDefaultS: .45,
		Shifts:    map[string]float64{"roll": .5},
		HandoverS: .12, CameraLagS: .18, CameraSettle: 1.2, AfterFight: "fast_turn_based",
	}
}

func paceCompiled() Compiled {
	p := paceOK()
	c := Compiled{CombatPace: &p}
	c.Localization.Messages = []LocalizationMessage{
		{Key: "ui.combat.pace.turn_based", Texts: map[string]string{"ru": "Пошаговый", "en": "Turn-based"}},
		{Key: "ui.combat.pace.fast_turn_based", Texts: map[string]string{"ru": "Быстрый пошаговый", "en": "Fast turn-based"}},
	}
	c.GodotSettings = &GodotSettings{Entries: []GodotSetting{{ID: "combat_pace", Kind: "choice", Default: "turn_based",
		Options: []SettingOption{{Value: "turn_based", Key: "ui.combat.pace.turn_based"}, {Value: "fast_turn_based", Key: "ui.combat.pace.fast_turn_based"}}}}}
	return c
}

// The two modes of the owner's decision, turn-based by default; the fast one slides and never waits longer.
func TestCombatPaceModesAndDefault(t *testing.T) {
	if err := paceOK().validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*CombatPace){
		"default":    func(p *CombatPace) { p.Default = "fast_turn_based" },
		"modes":      func(p *CombatPace) { p.Modes = p.Modes[:1] },
		"order":      func(p *CombatPace) { p.Modes[0], p.Modes[1] = p.Modes[1], p.Modes[0] },
		"walk":       func(p *CombatPace) { p.Modes[1].Move, p.Modes[1].WalkMPS, p.Modes[1].SlideS = "walk", 1.3, 0 },
		"slide":      func(p *CombatPace) { p.Modes[1].SlideS = .9 },
		"pause":      func(p *CombatPace) { p.Modes[1].PauseS = .5 },
		"beat_share": func(p *CombatPace) { p.Modes[1].BeatShare = 1.2 },
		"move":       func(p *CombatPace) { p.Beats["move"] = .2 },
		"shift":      func(p *CombatPace) { p.Shifts["attack"] = .2 },
		"handover":   func(p *CombatPace) { p.HandoverS = 0 },
		"settle":     func(p *CombatPace) { p.CameraSettle = 5 },
		"after":      func(p *CombatPace) { p.AfterFight = "real_time" },
		"beat type":  func(p *CombatPace) { p.Beats["atack"] = .7 },
		"shift type": func(p *CombatPace) { p.Shifts["rol"] = .5 },
		"key":        func(p *CombatPace) { p.Key = "" },
	} {
		p := paceOK()
		p.Beats = map[string]float64{"attack": .7, "wound": 0}
		p.Shifts = map[string]float64{"roll": .5}
		change(&p)
		if err := p.validate(); err == nil {
			t.Fatalf("%s: want a failure", name)
		}
	}
}

// The setting offers exactly the modes, in order, with the pace's own default; every mode has its text.
func TestCombatPaceSettingAndTexts(t *testing.T) {
	if err := validateCombatPace(paceCompiled()); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Compiled){
		"setting default": func(c *Compiled) { c.GodotSettings.Entries[0].Default = "fast_turn_based" },
		"setting options": func(c *Compiled) { c.GodotSettings.Entries[0].Options = c.GodotSettings.Entries[0].Options[:1] },
		"option order": func(c *Compiled) {
			o := c.GodotSettings.Entries[0].Options
			o[0], o[1] = o[1], o[0]
		},
		"no entry":   func(c *Compiled) { c.GodotSettings.Entries[0].ID = "other" },
		"no english": func(c *Compiled) { delete(c.Localization.Messages[1].Texts, "en") },
	} {
		c := paceCompiled()
		change(&c)
		if err := validateCombatPace(c); err == nil {
			t.Fatalf("%s: want a failure", name)
		}
	}
}

// Same seed, same fight in both modes: the pace reaches the client only. The server's world (world.json is the
// marshalled Compiled) is byte for byte the same whatever the pace's default and numbers, and has no pace in it; only
// the texts of the setting are there, as every text is. The client's presentation carries the pace whole.
func TestCombatPaceStaysOutOfTheServerWorld(t *testing.T) {
	turn, fast := paceCompiled(), paceCompiled()
	fast.CombatPace.Default = "fast_turn_based"
	fast.CombatPace.Modes[0].WalkMPS, fast.CombatPace.Modes[1].SlideS = 3, .01
	a, err := json.Marshal(turn)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(fast)
	if err != nil {
		t.Fatal(err)
	}
	var world map[string]json.RawMessage
	if err := json.Unmarshal(a, &world); err != nil {
		t.Fatal(err)
	}
	_, leaked := world["combat_pace"]
	if string(a) != string(b) || leaked || strings.Contains(string(a), `"slide_s"`) || strings.Contains(string(a), `"walk_mps"`) {
		t.Fatalf("the server's world must not see the pace: %s", a)
	}
	view, err := clientResource(turn)
	if err != nil {
		t.Fatal(err)
	}
	var client struct {
		CombatPace CombatPace `json:"combat_pace"`
	}
	if err := json.Unmarshal(view, &client); err != nil {
		t.Fatal(err)
	}
	if client.CombatPace.Default != "turn_based" || len(client.CombatPace.Modes) != 2 || client.CombatPace.Modes[1].SlideS != .15 {
		t.Fatalf("the client's presentation lacks the pace: %+v", client.CombatPace)
	}
}

// Every event type a server source publishes is known to the pace's check: scan the fight's sources for the types they
// set and compare (a new type in a template must be added to combatEventTypes, or a beat for it would be refused).
func TestCombatEventTypesCoverTheServer(t *testing.T) {
	sources := []string{}
	for _, pattern := range []string{"../behavior/templates/combat*.go.tmpl", "templates/combat*.go.tmpl", "../wounds/runtime/*.go"} {
		found, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range found {
			if !strings.Contains(f, "_test") {
				sources = append(sources, f)
			}
		}
	}
	if len(sources) < 5 {
		t.Fatalf("the fight's sources moved: %v", sources)
	}
	typed := regexp.MustCompile(`Type: *"([a-z_]+)"`)
	seen := 0
	for _, f := range sources {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range typed.FindAllStringSubmatch(string(raw), -1) {
			seen++
			if !combatEventTypes[m[1]] {
				t.Errorf("%s publishes event type %q unknown to combatEventTypes", f, m[1])
			}
		}
	}
	if seen < 20 {
		t.Fatalf("only %d event types found in %d sources", seen, len(sources))
	}
}
