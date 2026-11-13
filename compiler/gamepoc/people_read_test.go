package gamepoc

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func peopleReadFixture() (PeopleRead, func(string) bool) {
	level := PeopleReadLevel{
		LevelID: "level.market",
		Why:     "the people melt into the rust floor",
		Surroundings: PeopleReadSurroundings{Saturation: 0.55, SaturationBright: 0.9, BrightLuma: [2]float64{0.15, 0.6},
			Value: 0.9, ShadowTint: [3]float64{0.96, 0.99, 1.06}, Detail: 0.75},
		People: PeopleReadPeople{Neutral: 0.4, Contrast: 1.5, Gain: [3]float64{1.15, 1.15, 1.2}, ShadowFill: 0.3, Saturation: 1.05,
			RimColor: [3]float64{0.6, 0.7, 0.9}, RimStrength: 0.05, RimPower: 3},
	}
	hub := level
	hub.LevelID, hub.Why = "level.hub", "the same air"
	p := PeopleRead{Levels: []PeopleReadLevel{level, hub}}
	known := func(id string) bool { return id == "level.market" || id == "level.hub" }
	return p, known
}

func TestPeopleReadAcceptedAndProjected(t *testing.T) {
	p, known := peopleReadFixture()
	if err := checkPeopleRead(p, known); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(clientPeopleRead(&p))
	for _, want := range []string{`"levels":{"level.hub":`, `"level.market":{"people":{`,
		`"surroundings":{"saturation":0.55,"saturation_bright":0.9,"bright_luma":[0.15,0.6],"value":0.9,"shadow_tint":[0.96,0.99,1.06],"detail":0.75}`,
		`"people":{"neutral":0.4,"contrast":1.5,"gain":[1.15,1.15,1.2],"shadow_fill":0.3,"saturation":1.05,"rim_color":[0.6,0.7,0.9],"rim_strength":0.05,"rim_power":3}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("client people read lacks %s:\n%s", want, raw)
		}
	}
	if strings.Contains(string(raw), "why") || strings.Contains(string(raw), "level_id") {
		t.Fatalf("the reasons stay in CUE: %s", raw)
	}
	// Edges that still step back without turning grey, and people lifted to exactly double their light at most.
	for _, edge := range []func(p *PeopleRead){
		func(p *PeopleRead) { p.Levels[0].Surroundings.Saturation, p.Levels[0].People.Saturation = 0.2, 0.8 },
		func(p *PeopleRead) { p.Levels[0].Surroundings.SaturationBright = 1 },
		func(p *PeopleRead) { p.Levels[0].Surroundings.BrightLuma = [2]float64{0, 8} },
		func(p *PeopleRead) { p.Levels[0].Surroundings.Value = 0.6 },
		func(p *PeopleRead) { p.Levels[0].People.Gain[2], p.Levels[0].People.ShadowFill = 1.6, 0.25 },
		func(p *PeopleRead) { p.Levels[0].People.RimStrength, p.Levels[0].People.RimPower = 0, 1 },
		func(p *PeopleRead) { p.Levels[0].Surroundings.Detail = 0.4 },
		func(p *PeopleRead) { p.Levels[0].Surroundings.Detail = 1 },
		func(p *PeopleRead) { p.Levels[0].People.Neutral, p.Levels[0].People.Contrast = 0, 1 },
		func(p *PeopleRead) { p.Levels[0].People.Neutral, p.Levels[0].People.Contrast = 0.8, 2.5 },
	} {
		p, known := peopleReadFixture()
		edge(&p)
		if err := checkPeopleRead(p, known); err != nil {
			t.Fatalf("%+v: %v", p.Levels[0], err)
		}
	}
}

func TestPeopleReadRejected(t *testing.T) {
	cases := map[string]func(p *PeopleRead){
		"no_levels":         func(p *PeopleRead) { p.Levels = nil },
		"unknown_level":     func(p *PeopleRead) { p.Levels[1].LevelID = "level.nowhere" },
		"level_twice":       func(p *PeopleRead) { p.Levels[1].LevelID = "level.market" },
		"no_reason":         func(p *PeopleRead) { p.Levels[0].Why = "" },
		"grey_world":        func(p *PeopleRead) { p.Levels[0].Surroundings.Saturation = 0.1 },
		"louder_world":      func(p *PeopleRead) { p.Levels[0].Surroundings.SaturationBright = 1.2 },
		"bright_reversed":   func(p *PeopleRead) { p.Levels[0].Surroundings.BrightLuma = [2]float64{0.6, 0.15} },
		"bright_negative":   func(p *PeopleRead) { p.Levels[0].Surroundings.BrightLuma[0] = -0.1 },
		"value_dark":        func(p *PeopleRead) { p.Levels[0].Surroundings.Value = 0.5 },
		"value_nan":         func(p *PeopleRead) { p.Levels[0].Surroundings.Value = math.NaN() },
		"tint_colour":       func(p *PeopleRead) { p.Levels[0].Surroundings.ShadowTint[2] = 1.4 },
		"gain_low":          func(p *PeopleRead) { p.Levels[0].People.Gain[0] = 0.5 },
		"gain_bleach":       func(p *PeopleRead) { p.Levels[0].People.Gain[1], p.Levels[0].People.ShadowFill = 1.6, 0.4 },
		"fill_high":         func(p *PeopleRead) { p.Levels[0].People.ShadowFill = 1.2 },
		"people_grey":       func(p *PeopleRead) { p.Levels[0].People.Saturation = 0.7 },
		"people_loud":       func(p *PeopleRead) { p.Levels[0].People.Saturation = 1.6 },
		"people_below_room": func(p *PeopleRead) { p.Levels[0].Surroundings.Saturation, p.Levels[0].People.Saturation = 0.95, 0.9 },
		"rim_strong":        func(p *PeopleRead) { p.Levels[0].People.RimStrength = 0.8 },
		"rim_power":         func(p *PeopleRead) { p.Levels[0].People.RimPower = 0.5 },
		"rim_colour":        func(p *PeopleRead) { p.Levels[0].People.RimColor[0] = 1.5 },
		"gain_nan":          func(p *PeopleRead) { p.Levels[0].People.Gain[2] = math.NaN() },
		"detail_blur":       func(p *PeopleRead) { p.Levels[0].Surroundings.Detail = 0.3 },
		"detail_sharper":    func(p *PeopleRead) { p.Levels[0].Surroundings.Detail = 1.2 },
		"detail_nan":        func(p *PeopleRead) { p.Levels[0].Surroundings.Detail = math.NaN() },
		"neutral_full":      func(p *PeopleRead) { p.Levels[0].People.Neutral = 0.9 },
		"neutral_negative":  func(p *PeopleRead) { p.Levels[0].People.Neutral = -0.1 },
		"contrast_flatter":  func(p *PeopleRead) { p.Levels[0].People.Contrast = 0.8 },
		"contrast_harsh":    func(p *PeopleRead) { p.Levels[0].People.Contrast = 3 },
		"contrast_inf":      func(p *PeopleRead) { p.Levels[0].People.Contrast = math.Inf(1) },
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			p, known := peopleReadFixture()
			spoil(&p)
			if err := checkPeopleRead(p, known); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestPeopleReadRead(t *testing.T) {
	ctx := cuecontext.New()
	c := Compiled{Life: json.RawMessage(`{"locations":{"level.market":{},"level.hub":{}}}`)}
	level := `{level_id: "level.market", why: "melts", surroundings: {saturation: 0.55, saturation_bright: 0.9, bright_luma: [0.15, 0.6], value: 0.9, shadow_tint: [0.96, 0.99, 1.06], detail: 0.75},
		people: {neutral: 0.4, contrast: 1.5, gain: [1.15, 1.15, 1.2], shadow_fill: 0.3, saturation: 1.05, rim_color: [0.6, 0.7, 0.9], rim_strength: 0.05, rim_power: 3%s}}`
	p, err := readPeopleRead(ctx.CompileString(`people_read: levels: [`+strings.Replace(level, "%s", "", 1)+`]`), c)
	want, _ := peopleReadFixture()
	if err != nil || p == nil || len(p.Levels) != 1 || p.Levels[0].Surroundings != want.Levels[0].Surroundings ||
		p.Levels[0].People != want.Levels[0].People {
		t.Fatalf("read %+v, %v", p, err)
	}
	if p, err := readPeopleRead(ctx.CompileString(`other: 1`), c); err != nil || p != nil {
		t.Fatalf("a world without the block: %+v, %v", p, err)
	}
	for name, src := range map[string]string{
		"unknown_field": `people_read: levels: [` + strings.Replace(level, "%s", ", glow: 1", 1) + `]`,
		"string_value":  `people_read: levels: [` + strings.Replace(strings.Replace(level, "%s", "", 1), "value: 0.9", `value: "dim"`, 1) + `]`,
		"unknown_level": `people_read: levels: [` + strings.Replace(strings.Replace(level, "%s", "", 1), "level.market", "level.moon", 1) + `]`,
		"short_gain":    `people_read: levels: [` + strings.Replace(strings.Replace(level, "%s", "", 1), "gain: [1.15, 1.15, 1.2]", "gain: [1.15]", 1) + `]`,
		"no_levels":     `people_read: levels: []`,
		"short_rim":     `people_read: levels: [` + strings.Replace(strings.Replace(level, "%s", "", 1), "rim_color: [0.6, 0.7, 0.9]", "rim_color: [0.6, 0.7]", 1) + `]`,
		"haze_moved":    `people_read: levels: [` + strings.Replace(level, "%s", ", haze_share: 0.5", 1) + `]`,
		"long_tint":     `people_read: levels: [` + strings.Replace(strings.Replace(level, "%s", "", 1), "shadow_tint: [0.96, 0.99, 1.06]", "shadow_tint: [0.96, 0.99, 1.06, 1]", 1) + `]`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readPeopleRead(ctx.CompileString(src), c); err == nil || !strings.Contains(err.Error(), "world.people_read") {
				t.Fatalf("accepted or unnamed: %v", err)
			}
		})
	}
}
