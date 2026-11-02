package barks

import "sort"

// Cue is a combat event the fighting layer reports (owner 02.11): who, what he declares and what he really does. The
// line is chosen by the declared intent, so a lie works on the hero; only a cunning speaker (an encounter leader) may
// lie, everybody else says what he does. Facts are free conditions of the moment ("last_standing", "pack").
type Cue struct {
	Speaker   string   `json:"speaker"`
	Archetype string   `json:"archetype,omitempty"`
	Voice     string   `json:"voice,omitempty"`
	Role      string   `json:"role,omitempty"`
	Declared  string   `json:"declared"`
	Truth     string   `json:"truth,omitempty"`
	Facts     []string `json:"facts,omitempty"`
	Level     string   `json:"level"`
	At        Cell     `json:"at"`
}

// Sense is what the hero brings to a lie: where he is and his skills (practice values 0..100).
type Sense struct {
	Actor  Actor
	Skills map[string]int
}

// Outcome of a cue: the shout (or the beast's sound and sign), and the hero's thought when he saw through a lie.
type Outcome struct {
	Said     *Speech
	Thought  *Speech
	Deceived bool // the declared intent differs from the truth and the lie was allowed
	Reason   string
}

// VoiceOf: the explicit voice of the cue, else the speaker's, else his archetype's; "" = silent (no voice in data).
func (m Model) VoiceOf(c Cue) (Voice, bool) {
	id := c.Voice
	if id == "" {
		id = m.Combat.Speakers[c.Speaker]
	}
	if id == "" {
		id = m.Combat.Archetypes[c.Archetype]
	}
	return m.voice(id)
}
func (m Model) cunning(role string) bool {
	for _, r := range m.Combat.CunningRoles {
		if r == role && role != "" {
			return true
		}
	}
	return false
}
func hasAll(have, need []string) bool {
	for _, n := range need {
		ok := false
		for _, h := range have {
			ok = ok || h == n
		}
		if !ok {
			return false
		}
	}
	return true
}

// BarkEvent turns a combat cue into a shout. Without cues it is never called and the fight stays silent.
func (m Model) BarkEvent(s *State, now, tick, seed int64, c Cue, hero Sense) Outcome {
	if s.Voices == nil {
		s.Voices = map[string]int64{}
	}
	out := Outcome{}
	v, ok := m.VoiceOf(c)
	if !ok {
		out.Reason = "no_voice"
		return out
	}
	declared, truth := c.Declared, c.Truth
	if truth == "" {
		truth = declared
	}
	if declared != truth && !m.cunning(c.Role) {
		declared = truth // only the cunning lie: the rest shout what they do
	}
	out.Deceived = declared != truth
	speaker := Actor{ID: c.Speaker, Level: c.Level, At: c.At}
	if !m.hears(hero.Actor, speaker, m.Combat.HeroWithinCells) {
		out.Reason = "unheard"
		return out
	}
	if s.speaking(c.Speaker, tick) {
		out.Reason = "busy"
		return out
	}
	var lines []*CombatLine
	total := 0
	for i := range m.Combat.Lines {
		l := &m.Combat.Lines[i]
		if l.Voice != v.ID || l.Intent != declared || !hasAll(c.Facts, l.Requires) {
			continue
		}
		// a shout cools down per fighter: every man of a gang may cry "reloading"
		if last, said := s.Voices[c.Speaker+"|"+l.ID]; said && now < last+l.CooldownSeconds {
			continue
		}
		lines = append(lines, l)
		total += l.Weight
	}
	if total == 0 {
		out.Reason = "no_line"
		return out
	}
	s.Serial++
	pick := int(Roll(seed, "barks.combat", c.Speaker+"|"+declared, int64(s.Serial)) % uint64(total))
	var l *CombatLine
	for _, x := range lines {
		if pick < x.Weight {
			l = x
			break
		}
		pick -= x.Weight
	}
	caste := v.Caste
	if known, ok := m.Castes[c.Speaker]; ok && !v.Beast {
		caste = known // the voice is the side's; the hand is the speaker's own caste
	}
	sp := Speech{Serial: s.Serial, Kind: "combat", Line: l.ID, Key: l.ID, Speaker: c.Speaker, Caste: caste, To: "hall", Sfx: l.Sfx, Second: now, Level: c.Level, At: c.At}
	if v.Beast {
		// a beast has no words: its sound and a short mark over it
		sp.Kind, sp.Key, sp.Caste, sp.Sign = "sign", "", "", l.Sign
		sp.Start, sp.HoldMS, sp.FadeMS = tick, m.Timing.GestureMS, m.Timing.FadeMS
		sp.Until = tick + m.ticks(sp.HoldMS+sp.FadeMS)
	} else {
		face, _ := m.Face(caste)
		m.clock(&sp, face, l.Measure, nil, tick)
	}
	s.Voices[c.Speaker+"|"+l.ID] = now
	s.Active = append(s.Active, sp)
	out.Said = &sp
	if !out.Deceived {
		return out
	}
	// the hero may see through it: the first tell whose skill + roll reaches the threshold becomes his thought
	for _, d := range m.Combat.Deceptions {
		if d.Declared != declared || d.Truth != truth {
			continue
		}
		tells := append([]Tell(nil), d.Tells...)
		sort.SliceStable(tells, func(i, j int) bool { return tells[i].Threshold < tells[j].Threshold })
		for _, t := range tells {
			roll := int(Roll(seed, "barks.tell", c.Speaker+"|"+t.Key, int64(sp.Serial)) % uint64(m.Combat.RollMax))
			if hero.Skills[t.Skill]+roll < t.Threshold {
				continue
			}
			s.Serial++
			th := Speech{Serial: s.Serial, Kind: "thought", Line: l.ID, Key: t.Key, Speaker: hero.Actor.ID, Caste: "none", Second: now}
			face, _ := m.Face("none")
			m.clock(&th, face, t.Measure, nil, tick)
			s.Active = append(s.Active, th)
			out.Thought = &th
			return out
		}
	}
	return out
}
