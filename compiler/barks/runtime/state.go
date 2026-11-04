package barks

import (
	"fmt"
	"sort"
)

// Speech is a line on screen: the key the host localizes, who says it, and its clock in ticks. Kind: line (a pack's
// line), reply (an answer line), gesture (a silent answer), combat (a shout), sign (a beast: sound and mark, no words),
// thought (the hero sees through a lie: drawn in the HUD, never over the enemy), talk (a turn of an exchange: To is the other person, Gesture the talk gesture of the view).
type Speech struct {
	Serial   uint64 `json:"serial"`
	Kind     string `json:"kind"`
	Line     string `json:"line"`
	Key      string `json:"key,omitempty"`
	Speaker  string `json:"speaker"`
	Caste    string `json:"caste,omitempty"`
	To       string `json:"to,omitempty"`
	Place    string `json:"place,omitempty"`
	Gesture  string `json:"gesture,omitempty"`
	Sign     string `json:"sign,omitempty"`
	Sfx      string `json:"sfx,omitempty"`
	Level    string `json:"level,omitempty"` // a combatant outside Life: where he shouted from
	At       Cell   `json:"at"`
	Start    int64  `json:"start"` // tick
	RevealMS int64  `json:"reveal_ms"`
	FixMS    int64  `json:"fix_ms,omitempty"`
	HoldMS   int64  `json:"hold_ms"`
	FadeMS   int64  `json:"fade_ms"`
	Until    int64  `json:"until"` // tick: gone
	Second   int64  `json:"second"`
}

// PackClock: the last line of a place, the hour it counted in, and until when the exchange (line + answer) holds the floor.
type PackClock struct {
	Last  int64 `json:"last"`
	Hour  int64 `json:"hour"`
	Count int   `json:"count"`
	Busy  int64 `json:"busy"`
}

// Pending is an answer due: after DelaySeconds of game time and not before the line it answers starts to fade.
type Pending struct {
	Reply   string `json:"reply"`
	Place   string `json:"place"`
	Line    string `json:"line"`
	Second  int64  `json:"second"`
	Tick    int64  `json:"tick"`
	Speaker string `json:"speaker"`
}

// TalkRun is an exchange in progress: who the two people are, the turn due next and from which tick it may start.
type TalkRun struct {
	Exchange string `json:"exchange"`
	A        string `json:"a"`
	B        string `json:"b"`
	Turn     int    `json:"turn"`
	Due      int64  `json:"due"`
	GiveUp   int64  `json:"give_up"`
}

// TalkClock: the last exchange of the hero's surroundings and the hour it counted in.
type TalkClock struct {
	Last  int64 `json:"last,omitempty"`
	Hour  int64 `json:"hour,omitempty"`
	Count int   `json:"count,omitempty"`
}

// HeroTrace remembers where the hero last stepped (a passer-by notices someone pushing past).
type HeroTrace struct {
	Level string `json:"level,omitempty"`
	At    Cell   `json:"at"`
	Moved int64  `json:"moved"`
}

// State is everything the engine remembers; it is saved with the game, so a loaded game does not repeat a line early.
type State struct {
	Serial  uint64               `json:"serial"`
	Now     int64                `json:"now"`
	Lines   map[string]int64     `json:"lines,omitempty"`  // line id -> game second last spoken
	Packs   map[string]PackClock `json:"packs,omitempty"`  // place -> clock
	Facts   map[string]int64     `json:"facts,omitempty"`  // event|subject|level -> onset (game second)
	Voices  map[string]int64     `json:"voices,omitempty"` // speaker|shout -> last combat shout (game second)
	Pending []Pending            `json:"pending,omitempty"`
	Active  []Speech             `json:"active,omitempty"`
	Hero    HeroTrace            `json:"hero"`
	Talks   []TalkRun            `json:"talks,omitempty"`
	Talk    TalkClock            `json:"talk,omitempty"`
}

func (s State) Clone() State {
	out := s
	out.Lines = cloneInts(s.Lines)
	out.Facts = cloneInts(s.Facts)
	out.Voices = cloneInts(s.Voices)
	if s.Packs != nil {
		out.Packs = make(map[string]PackClock, len(s.Packs))
		for k, v := range s.Packs {
			out.Packs[k] = v
		}
	}
	out.Pending = append([]Pending(nil), s.Pending...)
	out.Active = append([]Speech(nil), s.Active...)
	out.Talks = append([]TalkRun(nil), s.Talks...)
	return out
}
func cloneInts(m map[string]int64) map[string]int64 {
	if m == nil {
		return nil
	}
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ValidateState: a loaded state may name only lines, places and replies of the catalog, and no clock may lie in the future.
func (m Model) ValidateState(s State, now, tick int64) error {
	if s.Now < 0 || s.Now > now {
		return fmt.Errorf("barks state: clock %d after the world %d", s.Now, now)
	}
	for id, at := range s.Lines {
		_, _, ambient := m.Line(id)
		_, combat := m.CombatLine(id)
		_, talk := m.Exchange(id)
		if !ambient && !combat && !talk {
			return fmt.Errorf("barks state: unknown line %s", id)
		}
		if at < 0 || at > now {
			return fmt.Errorf("barks state: line %s spoken in the future", id)
		}
	}
	for place, c := range s.Packs {
		if _, ok := m.Pack(place); !ok {
			return fmt.Errorf("barks state: unknown place %s", place)
		}
		if c.Last < 0 || c.Last > now || c.Count < 0 || c.Busy < 0 {
			return fmt.Errorf("barks state: invalid clock of %s", place)
		}
	}
	for k, at := range s.Facts {
		if at < 0 || at > now || k == "" {
			return fmt.Errorf("barks state: invalid fact %q", k)
		}
	}
	for k, at := range s.Voices {
		if at < 0 || at > now || k == "" {
			return fmt.Errorf("barks state: invalid voice clock %q", k)
		}
	}
	for _, p := range s.Pending {
		if _, ok := m.Reply(p.Reply); !ok || p.Second < 0 || p.Tick < 0 {
			return fmt.Errorf("barks state: unknown pending reply %s", p.Reply)
		}
	}
	for _, run := range s.Talks {
		e, ok := m.Exchange(run.Exchange)
		if !ok || run.Turn < 1 || run.Turn >= len(e.Turns) || run.A == "" || run.B == "" || run.Due < 0 {
			return fmt.Errorf("barks state: invalid exchange in progress %q", run.Exchange)
		}
	}
	if s.Talk.Last < 0 || s.Talk.Last > now || s.Talk.Count < 0 {
		return fmt.Errorf("barks state: invalid talk clock")
	}
	for _, a := range s.Active {
		if a.Serial == 0 || a.Serial > s.Serial || a.Start > tick || a.Until < a.Start {
			return fmt.Errorf("barks state: invalid active line %d", a.Serial)
		}
		switch a.Kind {
		case "line":
			if _, _, ok := m.Line(a.Line); !ok {
				return fmt.Errorf("barks state: unknown active line %s", a.Line)
			}
		case "reply", "gesture":
			if _, ok := m.Reply(a.Line); !ok {
				return fmt.Errorf("barks state: unknown active reply %s", a.Line)
			}
		case "talk":
			if _, _, ok := m.TurnOf(a.Line); !ok {
				return fmt.Errorf("barks state: unknown active turn %s", a.Line)
			}
		case "combat", "sign", "thought":
			if _, ok := m.CombatLine(a.Line); !ok && a.Kind != "thought" {
				return fmt.Errorf("barks state: unknown active shout %s", a.Line)
			}
		default:
			return fmt.Errorf("barks state: unknown kind %q", a.Kind)
		}
	}
	return nil
}

// Visible: the speeches alive at tick, oldest first.
func (s State) Visible(tick int64) []Speech {
	out := []Speech{}
	for _, a := range s.Active {
		if a.Start <= tick && tick < a.Until {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Serial < out[j].Serial })
	return out
}
