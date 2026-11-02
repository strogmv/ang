package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/barks"
	rt "github.com/strogmv/ang/compiler/barks/runtime"
)

// Barks (doc 94, world.barks): the world catalog of the lines over the head. The places name existing world data; here
// they become cells (brain place stools and posts, a gate and its side, the vending lanes, a crafting master's counter),
// the texts are measured from the localization, and compiler/barks checks every content rule — a violation fails generate.
type barksAnchor struct {
	BrainPlace  string   `json:"brain_place"`
	Gate        string   `json:"gate"`
	Side        string   `json:"side"`
	DepthCells  int      `json:"depth_cells"`
	Vending     bool     `json:"vending"`
	Master      string   `json:"master"`
	MarginCells int      `json:"margin_cells"`
	Rect        *rt.Rect `json:"rect"`
}
type barksPlaceIn struct {
	ID       string      `json:"id"`
	Kind     string      `json:"kind"`
	Level    string      `json:"level"`
	Anchor   barksAnchor `json:"anchor"`
	Subjects []string    `json:"subjects"`
}
type barksLineIn struct {
	ID              string                            `json:"id"`
	Speaker         string                            `json:"speaker"`
	Caste           string                            `json:"caste"`
	To              string                            `json:"to"`
	When            []string                          `json:"when"`
	Requires        rt.Requires                       `json:"requires"`
	React           []rt.React                        `json:"react"`
	Weight          int                               `json:"weight"`
	CooldownSeconds int64                             `json:"cooldown_seconds"`
	Reply           string                            `json:"reply"`
	Voice           string                            `json:"voice"`
	Fix             map[string]struct{ From, To int } `json:"fix"`
	Sfx             string                            `json:"sfx"`
}
type barksReplyIn struct {
	ID           string `json:"id"`
	Speaker      string `json:"speaker"`
	Kind         string `json:"kind"`
	Gesture      string `json:"gesture"`
	Key          string `json:"key"`
	DelaySeconds int64  `json:"delay_seconds"`
}
type barksPackIn struct {
	Place           string         `json:"place"`
	Periods         []rt.Period    `json:"periods"`
	HeroWithinCells int            `json:"hero_within_cells"`
	GapSeconds      int64          `json:"gap_seconds"`
	MaxPerHour      int            `json:"max_per_hour"`
	MeanWaitSeconds int64          `json:"mean_wait_seconds"`
	Lines           []barksLineIn  `json:"lines"`
	Replies         []barksReplyIn `json:"replies"`
}
type barksCombatIn struct {
	HeroWithinCells int               `json:"hero_within_cells"`
	RollMax         int               `json:"roll_max"`
	CunningRoles    []string          `json:"cunning_roles"`
	Intents         []string          `json:"intents"`
	Voices          []rt.Voice        `json:"voices"`
	Speakers        map[string]string `json:"speakers"`
	Archetypes      map[string]string `json:"archetypes"`
	Lines           []struct {
		ID              string   `json:"id"`
		Voice           string   `json:"voice"`
		Intent          string   `json:"intent"`
		Requires        []string `json:"requires"`
		Weight          int      `json:"weight"`
		CooldownSeconds int64    `json:"cooldown_seconds"`
		Sign            string   `json:"sign"`
		Sfx             string   `json:"sfx"`
	} `json:"lines"`
	Deceptions []struct {
		Declared string `json:"declared"`
		Truth    string `json:"truth"`
		Tells    []struct {
			Key       string `json:"key"`
			Skill     string `json:"skill"`
			Threshold int    `json:"threshold"`
		} `json:"tells"`
	} `json:"deceptions"`
}
type barksInput struct {
	MaxActive        int               `json:"max_active"`
	MovingTicks      int64             `json:"moving_ticks"`
	Timing           rt.Timing         `json:"timing"`
	Layout           rt.Layout         `json:"layout"`
	Fillers          []string          `json:"fillers"`
	Faces            []rt.Face         `json:"faces"`
	Castes           map[string]string `json:"castes"`
	PopulationCastes map[string]string `json:"population_castes"`
	Hidden           []struct {
		Person string   `json:"person"`
		Secret []string `json:"secret"`
		Public []string `json:"public"`
	} `json:"hidden"`
	Events []rt.Event     `json:"events"`
	Places []barksPlaceIn `json:"places"`
	Packs  []barksPackIn  `json:"packs"`
	Combat barksCombatIn  `json:"combat"`
}

func readBarks(world cue.Value) (*barksInput, error) {
	v := world.LookupPath(cue.ParsePath("barks"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.barks: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var in barksInput
	if err := d.Decode(&in); err != nil {
		return nil, fmt.Errorf("world.barks: %w", err)
	}
	return &in, nil
}

// barksLife is the part of the compiled Life the places resolve against.
type barksLife struct {
	Locations map[string]struct {
		Terrain []string `json:"terrain"`
		Gates   []struct {
			ID      string    `json:"id"`
			Opening []rt.Cell `json:"opening"`
		} `json:"gates"`
	} `json:"locations"`
	Actors []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"actors"`
	Population struct {
		Profiles []struct {
			ID string `json:"id"`
		} `json:"profiles"`
	} `json:"population"`
	Brain struct {
		Places []struct {
			ID      string  `json:"id"`
			Level   string  `json:"level"`
			At      rt.Cell `json:"at"`
			Seating *struct {
				Seats []struct{ At rt.Cell } `json:"seats"`
				Queue []struct{ At rt.Cell } `json:"queue"`
				Posts []struct{ At rt.Cell } `json:"posts"`
			} `json:"seating"`
		} `json:"places"`
	} `json:"brain"`
}

func bounds(cells []rt.Cell, margin int) rt.Rect {
	r := rt.Rect{X0: cells[0].X, Y0: cells[0].Y, X1: cells[0].X, Y1: cells[0].Y}
	for _, c := range cells[1:] {
		r.X0, r.Y0, r.X1, r.Y1 = min(r.X0, c.X), min(r.Y0, c.Y), max(r.X1, c.X), max(r.Y1, c.Y)
	}
	return rt.Rect{X0: max(0, r.X0-margin), Y0: max(0, r.Y0-margin), X1: r.X1 + margin, Y1: r.Y1 + margin}
}

// resolveBarkPlace turns an anchor into cells of the compiled world.
func resolveBarkPlace(p barksPlaceIn, life barksLife, c Compiled) (rt.Place, error) {
	out := rt.Place{ID: p.ID, Kind: p.Kind, Level: p.Level, Subjects: append([]string(nil), p.Subjects...)}
	a := p.Anchor
	loc, ok := life.Locations[p.Level]
	if !ok {
		return out, fmt.Errorf("barks place %s: unknown level %s", p.ID, p.Level)
	}
	switch {
	case a.BrainPlace != "":
		found := false
		for _, bp := range life.Brain.Places {
			if bp.ID != a.BrainPlace || bp.Level != p.Level || bp.Seating == nil {
				continue
			}
			found = true
			for _, group := range [][]struct{ At rt.Cell }{bp.Seating.Seats, bp.Seating.Queue, bp.Seating.Posts} {
				for _, s := range group {
					out.Spots = append(out.Spots, s.At)
				}
			}
		}
		if !found {
			return out, fmt.Errorf("barks place %s: no seated brain place %s on %s", p.ID, a.BrainPlace, p.Level)
		}
	case a.Gate != "":
		var opening []rt.Cell
		for _, g := range loc.Gates {
			if g.ID == a.Gate {
				opening = g.Opening
			}
		}
		if len(opening) == 0 {
			return out, fmt.Errorf("barks place %s: unknown gate %s on %s", p.ID, a.Gate, p.Level)
		}
		r := bounds(opening, 0)
		depth := max(a.DepthCells, 1)
		// the opening is a row (gates 6/7, beyond = north) or a column (gate 8, beyond = west); the hall is the other side
		horizontal := r.Y0 == r.Y1
		hall, beyond := r, r
		if horizontal {
			hall.Y1, beyond.Y0 = r.Y1+depth, max(0, r.Y0-depth)
		} else {
			hall.X1, beyond.X0 = r.X1+depth, max(0, r.X0-depth)
		}
		switch a.Side {
		case "beyond":
			out.Rects = []rt.Rect{beyond}
		case "hall":
			out.Rects = []rt.Rect{hall}
		default:
			out.Rects = []rt.Rect{hall, beyond}
		}
		if a.MarginCells > 0 {
			for i := range out.Rects {
				if horizontal {
					out.Rects[i].X0, out.Rects[i].X1 = max(0, out.Rects[i].X0-a.MarginCells), out.Rects[i].X1+a.MarginCells
				} else {
					out.Rects[i].Y0, out.Rects[i].Y1 = max(0, out.Rects[i].Y0-a.MarginCells), out.Rects[i].Y1+a.MarginCells
				}
			}
		}
		out.Subjects = append(out.Subjects, a.Gate)
	case a.Vending:
		if c.Vending == nil || c.Vending.Level != p.Level {
			return out, fmt.Errorf("barks place %s: no vending block on %s", p.ID, p.Level)
		}
		var cells []rt.Cell
		for _, u := range c.Vending.Units {
			for _, l := range u.Lane {
				cells = append(cells, rt.Cell{X: l.X, Y: l.Y})
			}
			out.Subjects = append(out.Subjects, u.ID)
		}
		out.Rects = []rt.Rect{bounds(cells, a.MarginCells)}
	case a.Master != "":
		if c.Crafting == nil {
			return out, fmt.Errorf("barks place %s: no crafting masters", p.ID)
		}
		found := false
		for _, ms := range c.Crafting.Masters {
			if ms.Actor == a.Master && ms.Level == p.Level {
				found = true
				out.Rects = []rt.Rect{bounds([]rt.Cell{{X: ms.Approach.X, Y: ms.Approach.Y}}, max(a.MarginCells, ms.Reach))}
				out.Subjects = append(out.Subjects, ms.Actor)
			}
		}
		if !found {
			return out, fmt.Errorf("barks place %s: %s is not a crafting master on %s", p.ID, a.Master, p.Level)
		}
	case a.Rect != nil:
		out.Rects = []rt.Rect{*a.Rect}
	default:
		return out, fmt.Errorf("barks place %s: the anchor names nothing", p.ID)
	}
	h := len(loc.Terrain)
	for i := range out.Rects {
		r := &out.Rects[i]
		if h > 0 {
			r.X1, r.Y1 = min(r.X1, len(loc.Terrain[0])-1), min(r.Y1, h-1)
		}
		if r.X1 < r.X0 || r.Y1 < r.Y0 {
			return out, fmt.Errorf("barks place %s: an empty rectangle", p.ID)
		}
	}
	return out, nil
}

// compileBarks builds the engine model, checks the content and renders the report rows; nil without world.barks.
func compileBarks(unity string, c *Compiled) error {
	in := c.barksIn
	if in == nil {
		return nil
	}
	var life barksLife
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return fmt.Errorf("barks: life: %w", err)
	}
	texts := map[string]map[string]string{}
	contexts := map[string]string{}
	for _, msg := range c.Localization.Messages {
		texts[msg.Key] = msg.Texts
		contexts[msg.Key] = msg.Context
	}
	measure := func(key string) map[string]rt.Measure {
		out := map[string]rt.Measure{}
		for _, loc := range c.Localization.Locales {
			out[loc] = rt.MeasureText(texts[key][loc])
		}
		return out
	}
	m := rt.Model{TickHz: int64(c.Config.TickHz), MaxActive: in.MaxActive, MovingTicks: in.MovingTicks, Locales: append([]string(nil), c.Localization.Locales...),
		Timing: in.Timing, Layout: in.Layout, Faces: in.Faces, Castes: in.Castes, PopulationCastes: in.PopulationCastes, Events: in.Events}
	profiles := map[string]bool{}
	for _, p := range life.Population.Profiles {
		profiles[p.ID] = true
	}
	for id := range in.PopulationCastes {
		if !profiles[id] {
			return fmt.Errorf("barks: population caste of unknown profile %s", id)
		}
	}
	for _, p := range in.Places {
		place, err := resolveBarkPlace(p, life, *c)
		if err != nil {
			return err
		}
		m.Places = append(m.Places, place)
	}
	for _, p := range in.Packs {
		pack := rt.Pack{Place: p.Place, Periods: p.Periods, HeroWithinCells: p.HeroWithinCells, GapSeconds: p.GapSeconds, MaxPerHour: p.MaxPerHour, MeanWaitSeconds: p.MeanWaitSeconds}
		sort.Slice(pack.Periods, func(i, j int) bool { return pack.Periods[i].ID < pack.Periods[j].ID })
		for _, l := range p.Lines {
			caste := l.Caste
			if l.Speaker != rt.Anyone {
				caste = in.Castes[l.Speaker]
			}
			line := rt.Line{ID: l.ID, Speaker: l.Speaker, Caste: caste, To: l.To, When: l.When, Requires: l.Requires, React: l.React, Weight: l.Weight,
				CooldownSeconds: l.CooldownSeconds, Reply: l.Reply, Voice: l.Voice, Sfx: l.Sfx, Measure: measure(l.ID)}
			if len(l.Fix) > 0 {
				line.Fix = map[string]rt.Fix{}
				for loc, f := range l.Fix {
					key := l.ID + ".fix"
					line.Fix[loc] = rt.Fix{From: f.From, To: f.To, Key: key, Measure: rt.MeasureText(texts[key][loc])}
				}
			}
			pack.Lines = append(pack.Lines, line)
		}
		for _, r := range p.Replies {
			reply := rt.Reply{ID: r.ID, Speaker: r.Speaker, Caste: in.Castes[r.Speaker], Kind: r.Kind, Gesture: r.Gesture, Key: r.Key, DelaySeconds: r.DelaySeconds}
			if r.Kind == "line" {
				reply.Measure = measure(r.Key)
			}
			pack.Replies = append(pack.Replies, reply)
		}
		m.Packs = append(m.Packs, pack)
	}
	cb := in.Combat
	m.Combat = rt.Combat{HeroWithinCells: cb.HeroWithinCells, RollMax: cb.RollMax, CunningRoles: cb.CunningRoles, Intents: cb.Intents, Voices: cb.Voices, Speakers: cb.Speakers, Archetypes: cb.Archetypes}
	for _, l := range cb.Lines {
		cl := rt.CombatLine{ID: l.ID, Voice: l.Voice, Intent: l.Intent, Requires: l.Requires, Weight: l.Weight, CooldownSeconds: l.CooldownSeconds, Sign: l.Sign, Sfx: l.Sfx}
		if l.Sign == "" {
			cl.Measure = measure(l.ID)
		}
		m.Combat.Lines = append(m.Combat.Lines, cl)
	}
	for _, d := range cb.Deceptions {
		dc := rt.Deception{Declared: d.Declared, Truth: d.Truth}
		for _, t := range d.Tells {
			dc.Tells = append(dc.Tells, rt.Tell{Key: t.Key, Skill: t.Skill, Threshold: t.Threshold, Measure: measure(t.Key)})
		}
		m.Combat.Deceptions = append(m.Combat.Deceptions, dc)
	}
	m = m.Sorted()
	// the fit check uses the very fonts the client draws with (WorldText metrics)
	fonts := map[string]barks.Font{}
	loaded, err := loadWorldTextFonts(unity, c.WorldText)
	if err != nil {
		return fmt.Errorf("barks: %w", err)
	}
	for style, f := range loaded {
		fonts[style] = barks.Font{Advance: f.advance, Tracking: f.ref.TrackingEM}
	}
	world := barks.World{
		Texts:   func(key string) map[string]string { return texts[key] },
		Fonts:   fonts,
		Fillers: in.Fillers,
		Walkable: func(level string, at rt.Cell) bool {
			loc, ok := life.Locations[level]
			if !ok || at.Y < 0 || at.Y >= len(loc.Terrain) || at.X < 0 || at.X >= len(loc.Terrain[at.Y]) {
				return false
			}
			return c.Walkable[string(loc.Terrain[at.Y][at.X])]
		},
		Warn: func(s string) { c.barksWarnings = append(c.barksWarnings, s) },
	}
	for _, msg := range c.Localization.Messages {
		if strings.HasPrefix(msg.Key, "bark.") || strings.HasPrefix(msg.Key, "reply.") {
			continue
		}
		for _, t := range msg.Texts {
			world.Corpus = append(world.Corpus, t)
		}
	}
	for _, h := range in.Hidden {
		world.Hidden = append(world.Hidden, barks.Hidden{Person: h.Person, Secret: h.Secret, Public: h.Public})
	}
	if err := barks.Check(m, world); err != nil {
		return err
	}
	c.Barks = &m
	names := func(id string) string {
		if t := texts["actor."+id+".name"]; t["ru"] != "" {
			return t["ru"]
		}
		for _, a := range life.Actors {
			if a.ID == id && a.Name != "" {
				return a.Name
			}
		}
		switch id {
		case "hall":
			return "в зал"
		case "crew":
			return "своим"
		case "stall":
			return "у прилавка"
		}
		return strings.TrimPrefix(id, "char.")
	}
	c.barksReport = barks.Report(m, func(k string) map[string]string { return texts[k] }, func(k string) string { return contexts[k] }, names, c.barksWarnings)
	return nil
}

// silentBarks: a world without world.barks still gets the engine and its host adapter, with nothing to say.
func silentBarks(c Compiled) *rt.Model {
	m := rt.Model{TickHz: int64(c.Config.TickHz), MaxActive: 1, MovingTicks: 1, Locales: []string{"ru"},
		Timing: rt.Timing{SetMS: 1, StampMinMS: 1, HoldMinMS: 1, WordsPerMinute: 1, FadeMS: 1, OffscreenFadeMS: 1, GestureMS: 1},
		Layout: rt.Layout{EMPx: 18, MaxWidthEM: 17, MaxLines: 2, LineLimitRunes: 52}}
	return &m
}
