// Package barks is the engine of the lines spoken over the head (doc 94): which line a person of a place says, when,
// to whom and how long it stays on screen. It is deterministic (seed + counters), owns no world state and reads the
// world only through the Frame the host builds: the people of the hero's level, where they stand and the world facts.
// Texts are localization keys; the host resolves them per locale. The caste of the speaker decides the face.
package barks

import (
	"fmt"
	"regexp"
	"sort"
)

type Cell struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type Rect struct {
	X0 int `json:"x0"`
	Y0 int `json:"y0"`
	X1 int `json:"x1"`
	Y1 int `json:"y1"`
}

func (r Rect) Has(c Cell) bool { return c.X >= r.X0 && c.X <= r.X1 && c.Y >= r.Y0 && c.Y <= r.Y1 }

// Face is the presentation of a caste (doc 94 §1, §13): the font, how the text appears and how a mistake is treated.
type Face struct {
	Caste       string `json:"caste"`
	Face        string `json:"face"`       // stencil | chalk | matrix
	Reveal      string `json:"reveal"`     // written | stamped | set
	Correction  string `json:"correction"` // cross_out | restamp | reprint | none
	CPS         int    `json:"cps"`
	Capitals    bool   `json:"capitals"`
	Punctuation bool   `json:"punctuation"`
	Colour      string `json:"colour"`
	Speaks      bool   `json:"speaks_in_act1"`
	WordsMax    int    `json:"words_max"`
	Undecided   bool   `json:"undecided"`
}

// Timing of doc 94 §3 and the layout of §5, shared by the server (how long a line lives) and the client (how it is drawn).
type Timing struct {
	CommaMS         int64 `json:"comma_ms"`
	StopMS          int64 `json:"stop_ms"`
	FixPauseMS      int64 `json:"fix_pause_ms"`
	StrikeMS        int64 `json:"strike_ms"`
	SetMS           int64 `json:"set_ms"`
	StampMinMS      int64 `json:"stamp_min_ms"`
	HoldMinMS       int64 `json:"hold_min_ms"`
	HoldBaseMS      int64 `json:"hold_base_ms"`
	WordsPerMinute  int64 `json:"words_per_minute"` // reading speed of the hold (2.6 words/s = 156)
	FadeMS          int64 `json:"fade_ms"`
	OffscreenFadeMS int64 `json:"offscreen_fade_ms"`
	GestureMS       int64 `json:"gesture_ms"`
}
type Layout struct {
	EMPx           int     `json:"em_px"` // glyph size at 1280x800 (doc 86: >= 12 px)
	MaxWidthEM     float64 `json:"max_width_em"`
	MaxLines       int     `json:"max_lines"`
	LineLimitRunes int     `json:"line_limit_runes"`
}

// Event is a world fact a line may react to. Scope: world (any on the speaker's level), place (a subject of the pack's
// place: its gate, its vending unit), speaker (the speaker's own: his order, his queue). A stub is authored ahead of the
// system and never fires.
type Event struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Scope  string `json:"scope"`
}

// Place is where a pack binds: the seats and posts of a brain place, or areas of the level (a gate, a corridor, a lane).
type Place struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Level    string   `json:"level"`
	Spots    []Cell   `json:"spots,omitempty"`
	Rects    []Rect   `json:"rects,omitempty"`
	Subjects []string `json:"subjects,omitempty"`
}

func (p Place) Has(level string, at Cell) bool {
	if level != p.Level {
		return false
	}
	for _, s := range p.Spots {
		if s == at {
			return true
		}
	}
	for _, r := range p.Rects {
		if r.Has(at) {
			return true
		}
	}
	return false
}

// Area: spots win over areas, a smaller area over a larger one (the yatai stool over the hall around it).
func (p Place) Area() int {
	if len(p.Spots) > 0 {
		return 0
	}
	n := 0
	for _, r := range p.Rects {
		n += (r.X1 - r.X0 + 1) * (r.Y1 - r.Y0 + 1)
	}
	return n
}

// Measure of a text in one locale: what the timing needs (the text itself stays in the localization).
type Measure struct {
	Runes  int `json:"runes"`
	Words  int `json:"words"`
	Commas int `json:"commas"`
	Stops  int `json:"stops"`
}
type Fix struct {
	From int    `json:"from"`
	To   int    `json:"to"`
	Key  string `json:"key"`
	// the correction text measured, for its own reveal
	Measure Measure `json:"measure"`
}
type Requires struct {
	// Target: the person whose post and distance count (default: `to` when it names a person).
	Target            string `json:"target,omitempty"`
	SpeakerSeated     bool   `json:"speaker_seated,omitempty"`
	SpeakerOnPost     bool   `json:"speaker_on_post,omitempty"`
	TargetOnPost      bool   `json:"target_on_post,omitempty"`
	TargetWithinCells int    `json:"target_within_cells,omitempty"`
	HeroPassingCells  int    `json:"hero_passing_cells,omitempty"`
	// somebody on his way passes within these cells of the speaker (the guard hurries those still in the hall)
	WalkersWithinCells int `json:"walkers_within_cells,omitempty"`
}
type React struct {
	Event string `json:"event"`
	// Subject pins the fact to one subject (gate 6 of the hall, whatever the place); empty = by the event's scope.
	Subject       string `json:"subject,omitempty"`
	WithinSeconds int64  `json:"within_seconds"`
	Negate        bool   `json:"negate,omitempty"`
}

// Anyone is the speaker of a line said by whoever of the line's caste stands at the place (a passer-by, a buyer).
const Anyone = "anyone"

type Line struct {
	ID              string             `json:"id"`
	Speaker         string             `json:"speaker"`
	Caste           string             `json:"caste"`
	To              string             `json:"to"`
	When            []string           `json:"when"`
	Requires        Requires           `json:"requires"`
	React           []React            `json:"react,omitempty"`
	Weight          int                `json:"weight"`
	CooldownSeconds int64              `json:"cooldown_seconds"`
	Reply           string             `json:"reply,omitempty"`
	Voice           string             `json:"voice"`
	Fix             map[string]Fix     `json:"fix,omitempty"`
	Sfx             string             `json:"sfx,omitempty"`
	Measure         map[string]Measure `json:"measure"`
}
type Reply struct {
	ID           string             `json:"id"`
	Speaker      string             `json:"speaker"`
	Caste        string             `json:"caste"`
	Kind         string             `json:"kind"` // line | gesture
	Gesture      string             `json:"gesture,omitempty"`
	Key          string             `json:"key,omitempty"`
	DelaySeconds int64              `json:"delay_seconds"`
	Measure      map[string]Measure `json:"measure,omitempty"`
}
type Period struct {
	ID   string `json:"id"`
	From int64  `json:"from"`
	To   int64  `json:"to"`
}

func (p Period) Has(day int64) bool {
	if p.From <= p.To {
		return day >= p.From && day < p.To
	}
	return day >= p.From || day < p.To
}

type Pack struct {
	Place           string   `json:"place"`
	Periods         []Period `json:"periods"`
	HeroWithinCells int      `json:"hero_within_cells"`
	GapSeconds      int64    `json:"gap_seconds"`
	MaxPerHour      int      `json:"max_per_hour"`
	// mean silence after the gap before the pack speaks again (a hazard, not a metronome)
	MeanWaitSeconds int64   `json:"mean_wait_seconds"`
	Lines           []Line  `json:"lines"`
	Replies         []Reply `json:"replies,omitempty"`
}

// ---------------------------------------------------------------- combat (owner 02.11: «реплики во время боя тоже нужны»)

// Voice is how a side speaks in a fight: the bronze guard (by the book), the bandits and techi-mafia (rough, «пак»,
// «долг», «слепая зона» — canon speech_registers.md), the free folk; a beast voice has no words, only a sound and a sign.
type Voice struct {
	ID    string `json:"id"`
	Caste string `json:"caste"`
	Beast bool   `json:"beast,omitempty"`
}
type CombatLine struct {
	ID              string             `json:"id"`
	Voice           string             `json:"voice"`
	Intent          string             `json:"intent"`
	Requires        []string           `json:"requires,omitempty"` // cue facts that must be present
	Weight          int                `json:"weight"`
	CooldownSeconds int64              `json:"cooldown_seconds"`
	Sign            string             `json:"sign,omitempty"` // beast: growl | bark | howl
	Sfx             string             `json:"sfx,omitempty"`
	Measure         map[string]Measure `json:"measure,omitempty"`
}

// Tell is how the hero may see through a declared intent: a skill of the hero against a threshold, shown as his thought.
type Tell struct {
	Key       string             `json:"key"`
	Skill     string             `json:"skill"`
	Threshold int                `json:"threshold"`
	Measure   map[string]Measure `json:"measure,omitempty"`
}
type Deception struct {
	Declared string `json:"declared"`
	Truth    string `json:"truth"`
	Tells    []Tell `json:"tells"`
}
type Combat struct {
	HeroWithinCells int               `json:"hero_within_cells"`
	Intents         []string          `json:"intents"`
	Voices          []Voice           `json:"voices"`
	Speakers        map[string]string `json:"speakers,omitempty"`   // actor -> voice
	Archetypes      map[string]string `json:"archetypes,omitempty"` // archetype -> voice
	CunningRoles    []string          `json:"cunning_roles"`
	RollMax         int               `json:"roll_max"`
	Lines           []CombatLine      `json:"lines,omitempty"`
	Deceptions      []Deception       `json:"deceptions,omitempty"`
}

// ---------------------------------------------------------------- exchanges (demo-hub v26-alive): people talk to each other

// Party is one side of an exchange: a person of the world, or "anyone" of a caste who stands by the first side.
type Party struct {
	Who     string `json:"who"`
	Caste   string `json:"caste"`             // his caste (named: from Castes)
	Posture string `json:"posture,omitempty"` // "" any | post | seated | standing: where he must be while the exchange lasts
}

// Turn is one line of an exchange. By = which side says it (0 or 1); Gesture = a talk gesture id of the view (empty: none).
type Turn struct {
	ID      string             `json:"id"`
	By      int                `json:"by"`
	Gesture string             `json:"gesture,omitempty"`
	Measure map[string]Measure `json:"measure"`
}

// Exchange is two people talking: 2-3 turns, the next only after the last one is over and only while both are in place and heard.
type Exchange struct {
	ID              string   `json:"id"`
	A               Party    `json:"a"`
	B               Party    `json:"b"`
	WithinCells     int      `json:"within_cells"`
	HeroWithinCells int      `json:"hero_within_cells"`
	Periods         []Period `json:"periods"`
	Weight          int      `json:"weight"`
	CooldownSeconds int64    `json:"cooldown_seconds"`
	Turns           []Turn   `json:"turns"`
}

// TalkRules: the clock of all exchanges of a level: silence between two, the hourly cap, the mean wait after the silence
// (a hazard), the pause between two turns, how long a turn may wait for its speaker to finish another line.
type TalkRules struct {
	GapSeconds      int64 `json:"gap_seconds"`
	MaxPerHour      int   `json:"max_per_hour"`
	MeanWaitSeconds int64 `json:"mean_wait_seconds"`
	PauseMS         int64 `json:"pause_ms"`
	GiveUpMS        int64 `json:"give_up_ms"`
}

// Model is the compiled catalog (world.barks + the resolved places).
type Model struct {
	TickHz      int64             `json:"tick_hz"`
	MaxActive   int               `json:"max_active"`
	MovingTicks int64             `json:"moving_ticks"` // the hero counts as passing this long after a step
	Locales     []string          `json:"locales"`
	Timing      Timing            `json:"timing"`
	Layout      Layout            `json:"layout"`
	Faces       []Face            `json:"faces"`
	Castes      map[string]string `json:"castes"`
	// the anonymous people of the population, by profile (they speak only the lines of "anyone" of their caste)
	PopulationCastes map[string]string `json:"population_castes,omitempty"`
	Events           []Event           `json:"events"`
	Places           []Place           `json:"places"`
	Packs            []Pack            `json:"packs"`
	Combat           Combat            `json:"combat"`
	Exchanges        []Exchange        `json:"exchanges,omitempty"`
	Talk             TalkRules         `json:"talk"`
}

func (m Model) Exchange(id string) (*Exchange, bool) {
	for i := range m.Exchanges {
		if m.Exchanges[i].ID == id {
			return &m.Exchanges[i], true
		}
	}
	return nil, false
}

// TurnOf finds the exchange a spoken turn belongs to.
func (m Model) TurnOf(line string) (*Exchange, *Turn, bool) {
	for i := range m.Exchanges {
		for j := range m.Exchanges[i].Turns {
			if m.Exchanges[i].Turns[j].ID == line {
				return &m.Exchanges[i], &m.Exchanges[i].Turns[j], true
			}
		}
	}
	return nil, nil, false
}

func (m Model) Face(caste string) (Face, bool) {
	for _, f := range m.Faces {
		if f.Caste == caste {
			return f, true
		}
	}
	return Face{}, false
}
func (m Model) Place(id string) (Place, bool) {
	for _, p := range m.Places {
		if p.ID == id {
			return p, true
		}
	}
	return Place{}, false
}
func (m Model) Event(id string) (Event, bool) {
	for _, e := range m.Events {
		if e.ID == id {
			return e, true
		}
	}
	return Event{}, false
}
func (m Model) Pack(place string) (*Pack, bool) {
	for i := range m.Packs {
		if m.Packs[i].Place == place {
			return &m.Packs[i], true
		}
	}
	return nil, false
}

// Line finds an ambient line or reply by id (both are spoken keys of a pack).
func (m Model) Line(id string) (*Line, *Pack, bool) {
	for i := range m.Packs {
		for j := range m.Packs[i].Lines {
			if m.Packs[i].Lines[j].ID == id {
				return &m.Packs[i].Lines[j], &m.Packs[i], true
			}
		}
	}
	return nil, nil, false
}
func (m Model) Reply(id string) (*Reply, bool) {
	for i := range m.Packs {
		for j := range m.Packs[i].Replies {
			if m.Packs[i].Replies[j].ID == id {
				return &m.Packs[i].Replies[j], true
			}
		}
	}
	return nil, false
}
func (m Model) CombatLine(id string) (*CombatLine, bool) {
	for i := range m.Combat.Lines {
		if m.Combat.Lines[i].ID == id {
			return &m.Combat.Lines[i], true
		}
	}
	return nil, false
}
func (m Model) voice(id string) (Voice, bool) {
	for _, v := range m.Combat.Voices {
		if v.ID == id {
			return v, true
		}
	}
	return Voice{}, false
}

var (
	lineIDPattern  = regexp.MustCompile(`^bark\.[a-z0-9_]+\.[a-z0-9_]+$`)
	replyIDPattern = regexp.MustCompile(`^reply\.[a-z0-9_]+\.[a-z0-9_]+$`)
)

// Validate is the structural gate of the catalog (the content rules are compiler/barks.Check at build time).
func (m Model) Validate() error {
	if m.TickHz < 1 || m.MaxActive < 1 || m.MaxActive > 6 || m.MovingTicks < 1 || len(m.Locales) == 0 {
		return fmt.Errorf("barks: invalid tick rate, active limit, moving window or locales")
	}
	t := m.Timing
	if t.SetMS < 1 || t.HoldMinMS < 1 || t.WordsPerMinute < 1 || t.FadeMS < 1 || t.OffscreenFadeMS < 1 || t.StampMinMS < 1 || t.GestureMS < 1 {
		return fmt.Errorf("barks: invalid timing")
	}
	faces := map[string]bool{}
	for _, f := range m.Faces {
		if faces[f.Caste] || f.CPS < 1 {
			return fmt.Errorf("barks: duplicate or invalid face %q", f.Caste)
		}
		faces[f.Caste] = true
	}
	for actor, c := range m.Castes {
		if !faces[c] {
			return fmt.Errorf("barks: actor %s has caste %q without a face", actor, c)
		}
	}
	for profile, c := range m.PopulationCastes {
		if !faces[c] {
			return fmt.Errorf("barks: population %s has caste %q without a face", profile, c)
		}
	}
	events := map[string]bool{}
	for _, e := range m.Events {
		if events[e.ID] || (e.Source != "go" && e.Source != "stub") || (e.Scope != "world" && e.Scope != "place" && e.Scope != "speaker") {
			return fmt.Errorf("barks: duplicate or invalid event %q", e.ID)
		}
		events[e.ID] = true
	}
	places := map[string]bool{}
	for _, p := range m.Places {
		if places[p.ID] || p.Level == "" || len(p.Spots)+len(p.Rects) == 0 {
			return fmt.Errorf("barks: duplicate or empty place %q", p.ID)
		}
		places[p.ID] = true
	}
	lines, replies := map[string]bool{}, map[string]bool{}
	for _, p := range m.Packs {
		if !places[p.Place] {
			return fmt.Errorf("barks: pack of unknown place %q", p.Place)
		}
		if p.HeroWithinCells < 1 || p.GapSeconds < 0 || p.MaxPerHour < 1 || p.MeanWaitSeconds < 1 {
			return fmt.Errorf("barks: pack %s has invalid hearing or clocks", p.Place)
		}
		periods := map[string]bool{}
		for _, pr := range p.Periods {
			periods[pr.ID] = true
		}
		for _, r := range p.Replies {
			if replies[r.ID] || !replyIDPattern.MatchString(r.ID) || r.DelaySeconds < 1 || (r.Kind != "line" && r.Kind != "gesture") {
				return fmt.Errorf("barks: duplicate or invalid reply %q", r.ID)
			}
			replies[r.ID] = true
		}
		for _, l := range p.Lines {
			if lines[l.ID] || !lineIDPattern.MatchString(l.ID) || l.Weight < 1 || l.CooldownSeconds < 1 || len(l.When) == 0 {
				return fmt.Errorf("barks: duplicate or invalid line %q", l.ID)
			}
			if !faces[l.Caste] {
				return fmt.Errorf("barks: line %s has caste %q without a face", l.ID, l.Caste)
			}
			for _, w := range l.When {
				if !periods[w] {
					return fmt.Errorf("barks: line %s names unknown period %q", l.ID, w)
				}
			}
			for _, r := range l.React {
				if !events[r.Event] {
					return fmt.Errorf("barks: line %s reacts to unknown event %q", l.ID, r.Event)
				}
			}
			lines[l.ID] = true
		}
		for _, l := range p.Lines {
			if l.Reply != "" && !replies[l.Reply] {
				return fmt.Errorf("barks: line %s replies with unknown %q", l.ID, l.Reply)
			}
		}
	}
	voices := map[string]bool{}
	for _, v := range m.Combat.Voices {
		if voices[v.ID] || (!v.Beast && !faces[v.Caste]) {
			return fmt.Errorf("barks: duplicate or invalid combat voice %q", v.ID)
		}
		voices[v.ID] = true
	}
	for _, l := range m.Combat.Lines {
		if lines[l.ID] || !lineIDPattern.MatchString(l.ID) || !voices[l.Voice] || l.Weight < 1 || l.CooldownSeconds < 1 {
			return fmt.Errorf("barks: duplicate or invalid combat line %q", l.ID)
		}
		lines[l.ID] = true
	}
	if len(m.Combat.Lines) > 0 && (m.Combat.HeroWithinCells < 1 || m.Combat.RollMax < 1) {
		return fmt.Errorf("barks: combat hearing and roll must be positive")
	}
	if len(m.Exchanges) > 0 && (m.Talk.MaxPerHour < 1 || m.Talk.MeanWaitSeconds < 1 || m.Talk.GapSeconds < 0 || m.Talk.PauseMS < 0 || m.Talk.GiveUpMS < 1) {
		return fmt.Errorf("barks: invalid talk rules")
	}
	exchanges := map[string]bool{}
	for _, e := range m.Exchanges {
		if exchanges[e.ID] || e.WithinCells < 1 || e.HeroWithinCells < 1 || e.Weight < 1 || e.CooldownSeconds < 1 || len(e.Periods) == 0 || len(e.Turns) < 2 || len(e.Turns) > 3 {
			return fmt.Errorf("barks: duplicate or invalid exchange %q", e.ID)
		}
		exchanges[e.ID] = true
		if e.A.Who == "" || e.B.Who == "" || (e.A.Who == e.B.Who && e.A.Who != Anyone) || !faces[e.A.Caste] || !faces[e.B.Caste] {
			return fmt.Errorf("barks: exchange %s needs two different people with faces", e.ID)
		}
		for i, t := range e.Turns {
			if lines[t.ID] || !lineIDPattern.MatchString(t.ID) || t.By < 0 || t.By > 1 || (i == 0 && t.By != 0) || (i > 0 && t.By == e.Turns[i-1].By) {
				return fmt.Errorf("barks: exchange %s: turn %q is a duplicate, or the sides do not alternate", e.ID, t.ID)
			}
			lines[t.ID] = true
		}
	}
	return nil
}

// Sorted returns a copy with packs, places and lines in a fixed order (selection never follows map or file order).
func (m Model) Sorted() Model {
	out := m
	out.Places = append([]Place(nil), m.Places...)
	sort.Slice(out.Places, func(i, j int) bool { return out.Places[i].ID < out.Places[j].ID })
	out.Packs = append([]Pack(nil), m.Packs...)
	sort.Slice(out.Packs, func(i, j int) bool { return out.Packs[i].Place < out.Packs[j].Place })
	out.Exchanges = append([]Exchange(nil), m.Exchanges...)
	sort.Slice(out.Exchanges, func(i, j int) bool { return out.Exchanges[i].ID < out.Exchanges[j].ID })
	if len(out.Exchanges) == 0 {
		out.Exchanges = nil
	}
	return out
}
