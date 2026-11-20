package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"

	"cuelang.org/go/cue"
)

// DialogueRules are the shared rules of a conversation like Disco Elysium (owner 20.11, design/dialogue/de-v1): the chance
// table of a skill check, the names of the difficulties, the colours and names of the skills and of the castes (on the
// dark conversation column, owner 20.11 «Облик диалога»), the texts of the log. The chance formula lives in CUE (world.dialogue_rules.chance is the evaluated table); the runtime
// (templates/dialogue.go.tmpl) only reads the table, so the chance an answer shows is the one the server rolls with.
type DialogueRules struct {
	ChanceRule struct {
		BasePercent     int `json:"base_percent"`
		PerPointPercent int `json:"per_point_percent"`
		MinPercent      int `json:"min_percent"`
		MaxPercent      int `json:"max_percent"`
	} `json:"chance_rule"`
	Difficulties []DialogueDifficulty        `json:"difficulties"`
	Chance       map[string][]int            `json:"chance"`
	Skills       map[string]DialogueSkillTag `json:"skills"`
	CasteColours map[string]string           `json:"caste_colours"`
	Ground       string                      `json:"ground"`
	MinContrast  float64                     `json:"min_contrast"`
	MaxLines     int                         `json:"max_lines"`
	Text         struct {
		You     string `json:"you"`
		Success string `json:"success"`
		Failure string `json:"failure"`
		Check   string `json:"check"`
		Locked  string `json:"locked"`
		Result  string `json:"result"`
		Once    string `json:"once"`
		Retry   string `json:"retry"`
	} `json:"text"`
}
type DialogueDifficulty struct {
	ID    string `json:"id"`
	Value int    `json:"value"`
	Text  string `json:"text"`
}
type DialogueSkillTag struct {
	Name   string `json:"name"`
	Colour string `json:"colour"`
}

// DialogueLine is a line of a conversation: who speaks and the localization key.
type DialogueLine struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

// DialogueCheck is a skill check in an answer.
type DialogueCheck struct {
	Skill      string `json:"skill"`
	Difficulty string `json:"difficulty"`
	Kind       string `json:"kind"`
}

// DialogueVoice is an inner voice: the hero's skill speaks when it is at least AtLeast.
type DialogueVoice struct {
	Skill   string `json:"skill"`
	AtLeast int    `json:"at_least"`
	Text    string `json:"text"`
}

var dialogueColour = regexp.MustCompile(`^#[0-9a-f]{6}$`)

func readDialogueRules(world cue.Value) (*DialogueRules, error) {
	v := world.LookupPath(cue.ParsePath("dialogue_rules"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.dialogue_rules: %w", err)
	}
	var d DialogueRules
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("world.dialogue_rules: %w", err)
	}
	if err = d.validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

// validate: the table is the formula's (clamped, never decreasing in skill), the difficulties rise, every colour reads on
// the conversation column's ground.
func (d DialogueRules) validate() error {
	fail := func(f string, a ...any) error { return fmt.Errorf("world.dialogue_rules: "+f, a...) }
	r := d.ChanceRule
	if r.PerPointPercent < 1 || r.MinPercent < 0 || r.MaxPercent > 100 || r.MinPercent > r.MaxPercent || r.BasePercent < r.MinPercent || r.BasePercent > r.MaxPercent {
		return fail("invalid chance rule")
	}
	if len(d.Difficulties) == 0 || len(d.Chance) != len(d.Difficulties) {
		return fail("every difficulty needs its chance row")
	}
	for i, level := range d.Difficulties {
		if level.ID == "" || level.Text == "" || (i > 0 && level.Value <= d.Difficulties[i-1].Value) {
			return fail("difficulty %d: empty or not above the one before", i)
		}
		row := d.Chance[level.ID]
		if len(row) != 101 {
			return fail("difficulty %s: the chance row must cover skill 0..100", level.ID)
		}
		for skill, p := range row {
			want := min(r.MaxPercent, max(r.MinPercent, r.BasePercent+r.PerPointPercent*(skill-level.Value)))
			if p != want {
				return fail("difficulty %s, skill %d: chance %d is not the rule's %d", level.ID, skill, p, want)
			}
		}
	}
	if len(d.Skills) == 0 || !dialogueColour.MatchString(d.Ground) || d.MinContrast < 3 || d.MaxLines < 8 || d.MaxLines > 256 {
		return fail("missing skills, ground, contrast or log length")
	}
	for id, s := range d.Skills {
		if s.Name == "" || !dialogueColour.MatchString(s.Colour) {
			return fail("skill %s: name and #rrggbb colour", id)
		}
		if c := colourContrast(s.Colour, d.Ground); c < d.MinContrast {
			return fail("skill %s: colour %s reads %.2f:1 on the ground %s, below %.1f:1", id, s.Colour, c, d.Ground, d.MinContrast)
		}
	}
	for caste, colour := range d.CasteColours {
		if !dialogueColour.MatchString(colour) {
			return fail("caste %s: #rrggbb colour", caste)
		}
		if c := colourContrast(colour, d.Ground); c < d.MinContrast {
			return fail("caste %s: colour %s reads %.2f:1 on the ground %s, below %.1f:1", caste, colour, c, d.Ground, d.MinContrast)
		}
	}
	for _, key := range []string{d.Text.You, d.Text.Success, d.Text.Failure, d.Text.Check, d.Text.Locked, d.Text.Result, d.Text.Once, d.Text.Retry} {
		if key == "" {
			return fail("missing log text")
		}
	}
	return nil
}

// colourContrast is the WCAG 2 contrast ratio of two #rrggbb colours.
func colourContrast(a, b string) float64 {
	lum := func(hex string) float64 {
		var c [3]float64
		for i := range c {
			v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
			x := float64(v) / 255
			if x <= 0.03928 {
				c[i] = x / 12.92
			} else {
				c[i] = math.Pow((x+0.055)/1.055, 2.4)
			}
		}
		return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2]
	}
	la, lb := lum(a), lum(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// check: every skill of the hero's development has a tag and no other; every text is in the catalog.
func (d *DialogueRules) check(c *Compiled) error {
	if d == nil {
		return nil
	}
	var life struct {
		Development struct {
			Skills []struct {
				ID string `json:"id"`
			} `json:"skills"`
		} `json:"development"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	if len(life.Development.Skills) != len(d.Skills) {
		return fmt.Errorf("world.dialogue_rules: %d skill tags for %d developed skills", len(d.Skills), len(life.Development.Skills))
	}
	for _, s := range life.Development.Skills {
		if _, ok := d.Skills[s.ID]; !ok {
			return fmt.Errorf("world.dialogue_rules: developed skill %s has no tag", s.ID)
		}
	}
	keys := []string{d.Text.You, d.Text.Success, d.Text.Failure, d.Text.Check, d.Text.Locked, d.Text.Result, d.Text.Once, d.Text.Retry}
	for _, level := range d.Difficulties {
		keys = append(keys, level.Text)
	}
	for _, s := range d.Skills {
		keys = append(keys, s.Name)
	}
	return requireLocalized(c, "world.dialogue_rules", keys)
}

// lines: speakers among the allowed ids, texts localized.
func (d *DialogueRules) lines(c *Compiled, where string, lines []DialogueLine, speakers map[string]bool) error {
	if len(lines) == 0 {
		return fmt.Errorf("%s: no lines", where)
	}
	keys := make([]string, 0, len(lines))
	for _, l := range lines {
		if !speakers[l.Speaker] {
			return fmt.Errorf("%s: %s does not take part in this conversation", where, l.Speaker)
		}
		keys = append(keys, l.Text)
	}
	return requireLocalized(c, where, keys)
}

// skillCheck: a check names a developed skill and a known difficulty.
func (d *DialogueRules) skillCheck(where string, k DialogueCheck) error {
	if d == nil {
		return fmt.Errorf("%s: a skill check needs world.dialogue_rules", where)
	}
	if _, ok := d.Skills[k.Skill]; !ok {
		return fmt.Errorf("%s: unknown skill %s", where, k.Skill)
	}
	if _, ok := d.Chance[k.Difficulty]; !ok {
		return fmt.Errorf("%s: unknown difficulty %s", where, k.Difficulty)
	}
	if k.Kind != "once" && k.Kind != "retry" {
		return fmt.Errorf("%s: check kind %q is neither once nor retry", where, k.Kind)
	}
	return nil
}

// voices: each names a developed skill, a threshold 0..100 and a localized line.
func (d *DialogueRules) voices(c *Compiled, where string, voices []DialogueVoice) error {
	keys := []string{}
	for _, v := range voices {
		if d == nil {
			return fmt.Errorf("%s: an inner voice needs world.dialogue_rules", where)
		}
		if _, ok := d.Skills[v.Skill]; !ok || v.AtLeast < 0 || v.AtLeast > 100 {
			return fmt.Errorf("%s: voice of unknown skill %s or threshold %d", where, v.Skill, v.AtLeast)
		}
		keys = append(keys, v.Text)
	}
	return requireLocalized(c, where, keys)
}

func requireLocalized(c *Compiled, where string, keys []string) error {
	messages := map[string]bool{}
	for _, m := range c.Localization.Messages {
		messages[m.Key] = true
	}
	for _, key := range keys {
		if key == "" || !messages[key] {
			return fmt.Errorf("%s: missing localization %q", where, key)
		}
	}
	return nil
}

// rejectUnrunDialogueFields: the level dialogues (#Dialogue, world.levels.*.dialogues) have no server runtime yet, so a
// skill check, its failure's node or an inner voice there would generate and never roll. Until #Dialogue runs on the
// server they belong to a conversation that does: today the calendar encounter.
func rejectUnrunDialogueFields(world cue.Value) error {
	levels := world.LookupPath(cue.ParsePath("levels"))
	if !levels.Exists() {
		return nil
	}
	it, err := levels.Fields()
	if err != nil {
		return err
	}
	for it.Next() {
		dialogues := it.Value().LookupPath(cue.ParsePath("dialogues"))
		if !dialogues.Exists() {
			continue
		}
		raw, err := dialogues.MarshalJSON()
		if err != nil {
			return fmt.Errorf("world.levels.%s.dialogues: %w", it.Selector(), err)
		}
		var all map[string]struct {
			Nodes map[string]struct {
				Voices  []json.RawMessage `json:"voices"`
				Options []struct {
					ID       string          `json:"id"`
					Check    json.RawMessage `json:"check"`
					FailNext *string         `json:"fail_next"`
				} `json:"options"`
			} `json:"nodes"`
		}
		if err := json.Unmarshal(raw, &all); err != nil {
			return fmt.Errorf("world.levels.%s.dialogues: %w", it.Selector(), err)
		}
		for id, d := range all {
			for node, n := range d.Nodes {
				where := fmt.Sprintf("world.levels.%s.dialogues.%s.%s", it.Selector(), id, node)
				if len(n.Voices) > 0 {
					return fmt.Errorf("%s: inner voices run only in a calendar encounter for now (#Dialogue has no server runtime)", where)
				}
				for _, o := range n.Options {
					if len(o.Check) > 0 || o.FailNext != nil {
						return fmt.Errorf("%s.%s: skill checks run only in a calendar encounter for now (#Dialogue has no server runtime)", where, o.ID)
					}
				}
			}
		}
	}
	return nil
}
