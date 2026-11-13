package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"

	"cuelang.org/go/cue"
)

// PeopleRead is world.people_read (people read v1, the game's design/people-read/v1/BRIEF.md): at the fixed 12 m frame the
// people must stand out of the level. Per level, presentation only: the surroundings step back (a grade of the level's
// picture: less chroma in its darks and mids, kept in the bright — the lamps' warm pools, the light guides' beam — a little
// less value, cooler darks, calmer fine detail), the people step forward (drawn again from their own lit colour: pushed away
// from the light of the ground beside them, a gain, a lift on the side away from the key light, a thin rim). Lights, fog and
// haze are not part of it (light v2 owns them). The client (client-godot/people_read.gd) reads the numbers from
// generated/presentation.json people_read.levels[<level>]; a level without them is drawn as before.
type PeopleRead struct {
	Levels []PeopleReadLevel `json:"levels"`
}

// PeopleReadLevel: one level's numbers and why the level needs them (the reason stays in CUE).
type PeopleReadLevel struct {
	LevelID      string                 `json:"level_id"`
	Why          string                 `json:"why"`
	Surroundings PeopleReadSurroundings `json:"surroundings"`
	People       PeopleReadPeople       `json:"people"`
}

// PeopleReadSurroundings: the grade of everything that is not a person. Colours are the scene's linear light before
// tonemapping. Saturation is the chroma kept in the darks and mids, SaturationBright in the bright; BrightLuma the scene
// luminance where the one becomes the other. Value multiplies the surroundings; ShadowTint multiplies their darks (cooler
// with blue above red). Detail is the share of their fine light-and-dark detail kept (the owner's rule: the surroundings a
// little less detailed than the people).
type PeopleReadSurroundings struct {
	Saturation       float64    `json:"saturation"`
	SaturationBright float64    `json:"saturation_bright"`
	BrightLuma       [2]float64 `json:"bright_luma"`
	Value            float64    `json:"value"`
	ShadowTint       [3]float64 `json:"shadow_tint"`
	Detail           float64    `json:"detail"`
}

// PeopleReadPeople: the people's own pass. Neutral takes that share of the key light's colour cast off them (their own
// colourway shows, not the hall's warm light); Contrast pushes their light away from the light around them on the screen (a
// dark figure on a lit floor darker, a lit one on a dark floor lighter; 1: none); Gain multiplies their lit colour (day /
// dusk / night), ShadowFill adds up to that share on the side away from the key light, Saturation is their chroma (1: as
// lit), the rim adds RimColor x RimStrength on the silhouette (sharper with RimPower).
type PeopleReadPeople struct {
	Neutral     float64    `json:"neutral"`
	Contrast    float64    `json:"contrast"`
	Gain        [3]float64 `json:"gain"`
	ShadowFill  float64    `json:"shadow_fill"`
	Saturation  float64    `json:"saturation"`
	RimColor    [3]float64 `json:"rim_color"`
	RimStrength float64    `json:"rim_strength"`
	RimPower    float64    `json:"rim_power"`
}

func readPeopleRead(world cue.Value, c Compiled) (*PeopleRead, error) {
	v := world.LookupPath(cue.ParsePath("people_read"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.people_read: %w", err)
	}
	var out PeopleRead
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.people_read: %w", err)
	}
	// Go fills a short list into a fixed array with zeros: every list must have exactly its length.
	if err := peopleReadLengths(raw); err != nil {
		return nil, fmt.Errorf("world.people_read: %w", err)
	}
	var life struct {
		Locations map[string]json.RawMessage `json:"locations"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return nil, err
	}
	known := func(id string) bool { _, ok := life.Locations[id]; return ok }
	if err := checkPeopleRead(out, known); err != nil {
		return nil, fmt.Errorf("world.people_read: %w", err)
	}
	return &out, nil
}

func peopleReadLengths(raw []byte) error {
	var doc struct {
		Levels []struct {
			LevelID      string                     `json:"level_id"`
			Surroundings map[string]json.RawMessage `json:"surroundings"`
			People       map[string]json.RawMessage `json:"people"`
		} `json:"levels"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	for _, l := range doc.Levels {
		want := []struct {
			block map[string]json.RawMessage
			name  string
			n     int
			inner int
		}{{l.Surroundings, "bright_luma", 2, 0}, {l.Surroundings, "shadow_tint", 3, 0}, {l.People, "gain", 3, 0},
			{l.People, "rim_color", 3, 0}}
		for _, w := range want {
			var list []json.RawMessage
			if err := json.Unmarshal(w.block[w.name], &list); err != nil || len(list) != w.n {
				return fmt.Errorf("level %s: %s needs exactly %d numbers", l.LevelID, w.name, w.n)
			}
			for _, inner := range list {
				var row []float64
				if w.inner > 0 && (json.Unmarshal(inner, &row) != nil || len(row) != w.inner) {
					return fmt.Errorf("level %s: each %s needs exactly %d numbers", l.LevelID, w.name, w.inner)
				}
			}
		}
	}
	return nil
}

func checkPeopleRead(p PeopleRead, known func(string) bool) error {
	if len(p.Levels) == 0 {
		return fmt.Errorf("no level: say where the people must read")
	}
	seen := map[string]bool{}
	for _, l := range p.Levels {
		if seen[l.LevelID] {
			return fmt.Errorf("level %s twice", l.LevelID)
		}
		seen[l.LevelID] = true
		if !known(l.LevelID) {
			return fmt.Errorf("unknown level %s", l.LevelID)
		}
		if l.Why == "" {
			return fmt.Errorf("level %s: say why its people need it (a measure of the frame)", l.LevelID)
		}
		if err := checkPeopleReadLevel(l); err != nil {
			return fmt.Errorf("level %s: %w", l.LevelID, err)
		}
	}
	return nil
}

func checkPeopleReadLevel(l PeopleReadLevel) error {
	finite := func(xs ...float64) bool {
		for _, x := range xs {
			if math.IsNaN(x) || math.IsInf(x, 0) {
				return false
			}
		}
		return true
	}
	in := func(x, lo, hi float64) bool { return x >= lo && x <= hi }
	s, p := l.Surroundings, l.People
	all := []float64{s.Saturation, s.SaturationBright, s.BrightLuma[0], s.BrightLuma[1], s.Value, s.Detail,
		p.Neutral, p.Contrast, p.ShadowFill, p.Saturation, p.RimStrength, p.RimPower}
	all = append(all, s.ShadowTint[:]...)
	all = append(all, p.Gain[:]...)
	all = append(all, p.RimColor[:]...)
	if !finite(all...) {
		return fmt.Errorf("non-finite number")
	}
	switch {
	case !in(s.Saturation, 0.2, 1) || !in(s.SaturationBright, 0.2, 1):
		return fmt.Errorf("surroundings saturation %v / bright %v: 0.2..1 (the surroundings step back, they do not turn grey or louder)", s.Saturation, s.SaturationBright)
	case !(s.BrightLuma[0] >= 0 && s.BrightLuma[0] < s.BrightLuma[1] && s.BrightLuma[1] <= 8):
		return fmt.Errorf("bright_luma %v: 0 <= from < to <= 8 (scene luminance)", s.BrightLuma)
	case !in(s.Value, 0.6, 1.1):
		return fmt.Errorf("surroundings value %v: 0.6..1.1", s.Value)
	case !in(s.Detail, 0.4, 1):
		return fmt.Errorf("surroundings detail %v: 0.4..1 (a little less detailed, not a blur)", s.Detail)
	case !in(p.ShadowFill, 0, 1):
		return fmt.Errorf("people shadow_fill %v: 0..1", p.ShadowFill)
	case !in(p.Neutral, 0, 0.8):
		return fmt.Errorf("people neutral %v: 0..0.8 (the hall's light still colours them)", p.Neutral)
	case !in(p.Contrast, 1, 2.5):
		return fmt.Errorf("people contrast %v: 1..2.5", p.Contrast)
	case !in(p.Saturation, 0.8, 1.5):
		return fmt.Errorf("people saturation %v: 0.8..1.5 (the colourway of a look stays its own)", p.Saturation)
	case p.Saturation < s.Saturation:
		return fmt.Errorf("people saturation %v below the surroundings' %v: the people would melt further into them", p.Saturation, s.Saturation)
	case !in(p.RimStrength, 0, 0.5) || !in(p.RimPower, 1, 8):
		return fmt.Errorf("rim strength %v (0..0.5) / power %v (1..8)", p.RimStrength, p.RimPower)
	}
	for i, t := range s.ShadowTint {
		if !in(t, 0.8, 1.2) {
			return fmt.Errorf("shadow_tint[%d] %v: 0.8..1.2 (a tint, not a colour)", i, t)
		}
	}
	for i := range 3 {
		// The painterly material of part A bleached the people: the brightest lift (gain on the side away from the key)
		// stays below double their own light.
		if !in(p.Gain[i], 0.8, 1.6) || p.Gain[i]*(1+p.ShadowFill) > 2 {
			return fmt.Errorf("people gain[%d] %v with shadow_fill %v: gain 0.8..1.6 and gain x (1 + fill) <= 2 (no bleaching)", i, p.Gain[i], p.ShadowFill)
		}
	}
	for _, v := range p.RimColor {
		if !in(v, 0, 1) {
			return fmt.Errorf("rim_color %v: linear 0..1", p.RimColor)
		}
	}
	return nil
}

// clientPeopleRead: the numbers per level, without the reasons.
func clientPeopleRead(p *PeopleRead) map[string]any {
	levels := map[string]any{}
	for _, l := range p.Levels {
		levels[l.LevelID] = map[string]any{"surroundings": l.Surroundings, "people": l.People}
	}
	return map[string]any{"levels": levels}
}
