package gamepoc

import (
	"math"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func validDialogueRules() DialogueRules {
	var d DialogueRules
	d.ChanceRule.BasePercent, d.ChanceRule.PerPointPercent, d.ChanceRule.MinPercent, d.ChanceRule.MaxPercent = 50, 2, 3, 97
	d.Difficulties = []DialogueDifficulty{{ID: "easy", Value: 10, Text: "d.easy"}, {ID: "medium", Value: 25, Text: "d.medium"}}
	d.Chance = map[string][]int{}
	for _, level := range d.Difficulties {
		row := make([]int, 101)
		for skill := range row {
			row[skill] = min(97, max(3, 50+2*(skill-level.Value)))
		}
		d.Chance[level.ID] = row
	}
	d.Skills = map[string]DialogueSkillTag{"speech": {Name: "skill.speech", Colour: "#e8a0a8"}}
	d.CasteColours = map[string]string{"copper": "#c49368", "none": "#d8d2c4"}
	d.Ground, d.MinContrast, d.MaxLines = "#0f1311", 4.5, 96
	d.Text.You, d.Text.Success, d.Text.Failure, d.Text.Check, d.Text.Locked, d.Text.Result = "you", "ok", "fail", "check", "locked", "result"
	d.Text.Once, d.Text.Retry = "once", "retry"
	return d
}

func TestDialogueRulesAreTheFormulaAndReadable(t *testing.T) {
	if err := validDialogueRules().validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*DialogueRules){
		"a chance off the formula": func(d *DialogueRules) {
			row := append([]int(nil), d.Chance["medium"]...)
			row[20]++
			d.Chance = map[string][]int{"easy": d.Chance["easy"], "medium": row}
		},
		"a short row": func(d *DialogueRules) {
			d.Chance = map[string][]int{"easy": d.Chance["easy"], "medium": d.Chance["medium"][:100]}
		},
		"difficulties not rising": func(d *DialogueRules) { d.Difficulties = []DialogueDifficulty{d.Difficulties[1], d.Difficulties[0]} },
		"a dark skill on the ground": func(d *DialogueRules) {
			d.Skills = map[string]DialogueSkillTag{"speech": {Name: "s", Colour: "#3a2a2a"}}
		},
		"a dark caste on the ground": func(d *DialogueRules) { d.CasteColours = map[string]string{"copper": "#673125"} },
		"a check without its text":   func(d *DialogueRules) { d.Text.Check = "" },
		"a chance rule out of range": func(d *DialogueRules) { d.ChanceRule.MaxPercent = 101 },
	} {
		d := validDialogueRules()
		mutate(&d)
		if d.validate() == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestColourContrastIsWCAG(t *testing.T) {
	if c := colourContrast("#000000", "#ffffff"); math.Abs(c-21) > 1e-9 {
		t.Fatalf("black on white %.3f", c)
	}
	if c := colourContrast("#777777", "#777777"); math.Abs(c-1) > 1e-9 {
		t.Fatalf("a colour on itself %.3f", c)
	}
}

func TestDialogueChecksNameKnownSkillsAndDifficulties(t *testing.T) {
	d := validDialogueRules()
	if err := d.skillCheck("x", DialogueCheck{Skill: "speech", Difficulty: "medium", Kind: "once"}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []DialogueCheck{{Skill: "sword", Difficulty: "medium", Kind: "once"}, {Skill: "speech", Difficulty: "legendary", Kind: "once"}, {Skill: "speech", Difficulty: "medium", Kind: "twice"}} {
		if d.skillCheck("x", k) == nil {
			t.Fatalf("%+v accepted", k)
		}
	}
	var none *DialogueRules
	if none.skillCheck("x", DialogueCheck{Skill: "speech", Difficulty: "medium", Kind: "once"}) == nil {
		t.Fatal("a check without rules accepted")
	}
}

func TestLevelDialoguesRefuseChecksAndVoicesWithoutARuntime(t *testing.T) {
	ctx := cuecontext.New()
	plain := ctx.CompileString(`levels: market: dialogues: "dlg.x": nodes: intro: {voices: [], options: [{id: "bye"}]}`)
	if err := rejectUnrunDialogueFields(plain); err != nil {
		t.Fatalf("a plain dialogue refused: %v", err)
	}
	for name, src := range map[string]string{
		"a voice":     `levels: market: dialogues: "dlg.x": nodes: intro: {voices: [{skill: "observation", at_least: 1, text: "k"}], options: []}`,
		"a check":     `levels: market: dialogues: "dlg.x": nodes: intro: {options: [{id: "lie", check: {skill: "speech", difficulty: "medium", kind: "once"}}]}`,
		"a fail node": `levels: market: dialogues: "dlg.x": nodes: intro: {options: [{id: "lie", fail_next: "intro"}]}`,
	} {
		err := rejectUnrunDialogueFields(ctx.CompileString(src))
		if err == nil || !strings.Contains(err.Error(), "no server runtime") {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
}
