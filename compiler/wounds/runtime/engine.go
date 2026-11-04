package wounds

import (
	"errors"
	"fmt"
	"sort"
)

// Hit is one blow or projectile that reached the body (the combat layer decided it hit). Zone/Depth/Structures are
// forced only by scenarios and tests; normally the engine rolls them from the weapon, the aim, the angle, the armour
// and the attacker's skill.
type Hit struct {
	Limb       string   `json:"limb,omitempty"`
	ID         string   `json:"id,omitempty"`
	Author     string   `json:"author,omitempty"`
	Weapon     string   `json:"weapon,omitempty"`
	Mechanism  string   `json:"mechanism,omitempty"` // a future firearm without a combat weapon
	Mode       string   `json:"mode,omitempty"`
	Technique  string   `json:"technique,omitempty"`
	Part       string   `json:"part,omitempty"` // combat aim: head / torso / arms / legs
	Zone       string   `json:"zone,omitempty"`
	Depth      string   `json:"depth,omitempty"`
	Angle      string   `json:"angle,omitempty"` // front / flank / rear
	Armor      []string `json:"armor,omitempty"`
	Skill      int64    `json:"skill,omitempty"`   // attacker's practice 0..100
	HoldS      int64    `json:"hold_s,omitempty"`  // a chokehold: how long it was held
	Elapsed    bool     `json:"elapsed,omitempty"` // a chokehold played out second by second: the body's clock has already run through HoldS
	Structures []string `json:"structures,omitempty"`
	Seed       int64    `json:"seed"`
}

// HitResult: the wound (nil when the armour stopped it), whether it is serious (resolve.cue) and the events.
type HitResult struct {
	Wound   *Wound  `json:"wound,omitempty"`
	Blocked bool    `json:"blocked,omitempty"`
	Serious bool    `json:"serious,omitempty"`
	Events  []Event `json:"events,omitempty"`
}

var errDead = errors.New("wounds: the body is dead")

func (m Model) chip(b *Body) Chip {
	if c, ok := m.Chips[b.Chip]; ok {
		return c
	}
	return m.Chips["none"]
}

// ApplyHit turns a blow into a wound at the body's current second.
func (m Model) ApplyHit(b *Body, h Hit) (HitResult, error) {
	var res HitResult
	if b.Dead {
		return res, errDead
	}
	b.Seq++
	id := h.ID
	if id == "" {
		id = fmt.Sprintf("w%d", b.Seq)
	}
	if b.wound(id) != nil {
		return res, fmt.Errorf("wounds: wound %s exists", id)
	}
	var mech, zone, angle string
	var dist map[string]int64
	var tech Technique
	deepen := int64(0)
	if h.Technique != "" {
		t, ok := m.Technique(h.Technique)
		if !ok {
			return res, fmt.Errorf("wounds: unknown technique %s", h.Technique)
		}
		tech, mech, zone, angle, dist = t, t.Mechanism, t.Zone, t.Angle, t.Depth
	} else {
		if h.Weapon != "" {
			w, ok := m.Weapon(h.Weapon)
			if !ok {
				return res, fmt.Errorf("wounds: weapon %s has no wound profile", h.Weapon)
			}
			mode, ok := w.Modes[h.Mode]
			if !ok {
				mode = w.Modes["*"]
			}
			mech, dist, deepen = mode.Mechanism, mode.Depth, w.SkillDeepenPPM
		}
		if h.Mechanism != "" {
			mech = h.Mechanism
		}
		if mech == "" {
			return res, errors.New("wounds: a hit needs a weapon, a technique or a mechanism")
		}
		if dist == nil {
			dist = map[string]int64{"deep": 1}
		}
		zone, angle = h.Zone, h.Angle
	}
	if _, ok := m.Mechanisms[mech]; !ok {
		return res, fmt.Errorf("wounds: unknown mechanism %s", mech)
	}
	if h.Zone != "" {
		zone = h.Zone
	}
	if zone == "" {
		weights := map[string]int64{}
		aim, ok := m.Aim[h.Part]
		if !ok {
			return res, fmt.Errorf("wounds: unknown aim %q", h.Part)
		}
		for z, w := range aim {
			weights[z] += w
		}
		// the angle opens zones of the same aim (from the flank the armpit, from behind the neck)
		for z, w := range m.Angles[angle] {
			if aim[z] > 0 {
				weights[z] += w
			}
		}
		zone = pick(weights, h.Seed, id, "zone")
	}
	if h.Limb == "" && h.Zone == "" {
		ids := []string{}
		for id, l := range m.Limbs {
			if l.Zone == zone {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		if len(ids) > 0 {
			h.Limb = ids[int(roll(h.Seed, id, "limb")%uint64(len(ids)))]
		}
	}
	z, ok := m.Zone(zone)
	if !ok {
		return res, fmt.Errorf("wounds: unknown zone %s", zone)
	}
	depth := h.Depth
	if depth == "" {
		depth = pick(dist, h.Seed, id, "depth")
		if r := depthRank[depth]; r < 3 && chance(min(h.Skill, 100)*deepen, h.Seed, id, "skill") {
			depth = depthNames[r+1]
		}
	}
	if _, ok := depthRank[depth]; !ok {
		return res, fmt.Errorf("wounds: unknown depth %s", depth)
	}
	// Separate same-zone blunt channels accumulate tissue damage, not an HP pool.
	if mech == "blunt" && m.RepeatBluntHits > 0 && m.RepeatBluntWindowS > 0 {
		n := 0
		for _, prior := range b.Wounds {
			if prior.Mechanism == mech && prior.Zone == zone && prior.Limb == h.Limb && b.Clock-prior.At <= m.RepeatBluntWindowS {
				n++
			}
		}
		depth = depthNames[min(3, depthRank[depth]+n/m.RepeatBluntHits)]
	}
	if h.Limb != "" {
		limb, ok := m.Limbs[h.Limb]
		if !ok || limb.Zone != zone {
			return res, fmt.Errorf("wounds: invalid limb %s for %s", h.Limb, zone)
		}
	}
	for _, item := range h.Armor {
		for _, a := range m.Armor {
			if a.Item != item || !contains(a.Zones, zone) {
				continue
			}
			if chance(a.Stop[mech], h.Seed, id, "stop", item) {
				res.Blocked = true
				res.Events = append(res.Events, Event{Type: "blocked", Body: b.ID, At: b.Clock, Detail: item})
				return res, nil
			}
			if chance(a.Shift[mech], h.Seed, id, "shift", item) {
				if depthRank[depth] == 0 {
					res.Blocked = true
					res.Events = append(res.Events, Event{Type: "blocked", Body: b.ID, At: b.Clock, Detail: item})
					return res, nil
				}
				depth = depthNames[depthRank[depth]-1]
			}
		}
	}
	w := Wound{Limb: h.Limb, ID: id, Zone: zone, Depth: depth, Mechanism: mech, Weapon: h.Weapon, Author: h.Author, At: b.Clock, HealAt: b.Clock + 86400}
	if h.Technique != "" && h.Weapon == "" {
		w.Weapon = ""
	}
	md := m.Mechanisms[mech]
	if mech == "choke" && tech.Knockout == nil {
		return HitResult{}, errors.New("wounds: a choke is the chokehold technique")
	}
	if mech == "choke" {
		m.choke(b, &w, tech, h)
	} else {
		met := h.Structures
		if met == nil && len(tech.Structures) > 0 {
			met = append([]string(nil), tech.Structures...)
		}
		if met == nil {
			for _, st := range z.Structures {
				if depthRank[st.MinDepth] <= depthRank[depth] && chance(st.Exposure[mech], h.Seed, id, "meets", st.ID) {
					met = append(met, st.ID)
				}
			}
		}
		maxHeal := int64(1)
		for _, sid := range met {
			st, sz, ok := m.Structure(sid)
			if !ok || sz.ID != zone {
				return HitResult{}, fmt.Errorf("wounds: structure %s is not in %s", sid, zone)
			}
			w.Structures = append(w.Structures, sid)
			w.Effects = addUnique(w.Effects, st.Effects...)
			maxHeal = max(maxHeal, st.HealDays)
			if st.Severity >= m.Resolve.SeriousSeverity {
				res.Serious = true
			}
			rate := st.BleedMLMin * md.BleedPermille * m.Difficulty.BleedPermille / 60000
			if rate > 0 {
				bl := Bleed{Structure: sid, Kind: st.Kind, Internal: st.Internal, RateUL: rate, ClotHalfS: st.ClotHalfS, Since: b.Clock}
				if st.Knockout != nil {
					bl.KnockoutAt = b.Clock + max(1, inSpan(*st.Knockout, h.Seed, id, "perfusion", sid))
				}
				w.Bleeds = append(w.Bleeds, bl)
			}
			if st.Doom != nil {
				at := b.Clock + inSpan(*st.Doom, h.Seed, id, "doom", sid)*m.Difficulty.DoomPermille/1000
				if w.DoomAt == 0 || at < w.DoomAt {
					w.DoomAt = at
				}
			}
			if st.LethalPPM > 0 && chance(st.LethalPPM, h.Seed, id, "lethal", sid) {
				w.DoomAt = b.Clock + 1 // within the second (0 means «no doom»)
			}
			if st.KnockoutFor != nil {
				m.knockout(b, b.Clock, b.Clock+inSpan(*st.KnockoutFor, h.Seed, id, "concussion", sid))
			}
			if st.Knockout != nil && rate == 0 {
				until := b.Clock + 7*86400
				if w.DoomAt > 0 {
					until = w.DoomAt
				}
				m.knockout(b, b.Clock+inSpan(*st.Knockout, h.Seed, id, "brain", sid), until)
			}
		}
		w.HealAt = b.Clock + maxHeal*86400
		// A held cut: while the attacker holds, the body goes limp on the technique's clock; let go before that, only the vessel's own clock is left.
		if tech.Knockout != nil && h.HoldS > 0 {
			w.HeldKOAt = b.Clock + max(1, inSpan(*tech.Knockout, h.Seed, id, "held_cut"))
		}
		if md.Incapacitate != nil {
			b.StunUntil = max(b.StunUntil, b.Clock+inSpan(*md.Incapacitate, h.Seed, id, "shock"))
		}
	}
	b.Wounds = append(b.Wounds, w)
	if q := m.RepeatBluntStun; mech == "blunt" && q.Hits > 0 && depthRank[w.Depth] >= depthRank[q.MinDepth] {
		n := 0
		for _, prior := range b.Wounds {
			if prior.Mechanism == "blunt" && m.sameBluntRegion(prior, w) && prior.Limb == w.Limb && b.Clock-prior.At <= q.WindowS && depthRank[prior.Depth] >= depthRank[q.MinDepth] {
				n++
			}
		}
		if n >= q.Hits {
			b.StunUntil = max(b.StunUntil, b.Clock+inSpan(q.Duration, h.Seed, id, "repeated blunt trauma"))
			res.Serious = true
		}
	}

	crime := m.Justice.Hit
	if res.Serious {
		crime = m.Justice.Serious
	}
	res.Events = append(res.Events, Event{Type: "wound", Body: b.ID, At: b.Clock, Wound: id, Culprit: h.Author, Crime: crime, Detail: mech})
	if md.Condition != "" {
		ev, err := m.ApplyCondition(b, md.Condition, h.Author, h.Seed)
		if err != nil {
			return res, err
		}
		res.Events = append(res.Events, ev)
	}
	m.settle(b, &res.Events)
	res.Wound = b.wound(id)
	return res, nil
}

// choke: limp after the knockout span; released then — wakes later; held past the safe span — the brain starves.
func (m Model) choke(b *Body, w *Wound, t Technique, h Hit) {
	ko := inSpan(*t.Knockout, h.Seed, w.ID, "choke_ko")
	hold := max(h.HoldS, 0)
	start := b.Clock
	if h.Elapsed {
		start -= hold
	}
	b.StunUntil = max(b.StunUntil, start+hold)
	if hold >= ko {
		m.knockout(b, start+ko, start+hold+inSpan(*t.Wake, h.Seed, w.ID, "choke_wake"))
		survive := int64(Million)
		for i := int64(0); i < hold-ko-t.SafeAfterKOS && survive > 0; i++ {
			survive = survive * (Million - t.DeathPPMPerS) / Million
		}
		if chance(Million-survive, h.Seed, w.ID, "hypoxia") || chance(t.ArrestPPM, h.Seed, w.ID, "arrest") {
			w.DoomAt = start + hold
		}
	} else if chance(t.ArrestPPM, h.Seed, w.ID, "arrest") {
		w.DoomAt = start + hold
	}
}

// ChokeSeconds: after how many seconds of a hold this body goes limp. The very draw choke() makes for this seed on the body's next wound, so a host playing the hold second by second knows the second it ends on.
func (m Model) ChokeSeconds(b Body, technique string, seed int64) int64 {
	t, ok := m.Technique(technique)
	if !ok || t.Knockout == nil {
		return 0
	}
	return inSpan(*t.Knockout, seed, fmt.Sprintf("w%d", b.Seq+1), "choke_ko")
}

// HoldSeconds: seconds from now until a held body goes limp (0 — nothing is held on it).
func (m Model) HoldSeconds(b Body) int64 {
	for i := range b.Wounds {
		if at := b.Wounds[i].HeldKOAt; at > b.Clock {
			return at - b.Clock
		}
	}
	return 0
}

// ReleaseHold: the hands are off before the knockout. The wound stays, the vessels keep their own perfusion clocks.
func (m Model) ReleaseHold(b *Body) {
	for i := range b.Wounds {
		if b.Wounds[i].HeldKOAt > b.Clock {
			b.Wounds[i].HeldKOAt = 0
		}
	}
}

func (m Model) knockout(b *Body, from, until int64) {
	if until <= from {
		return
	}
	if b.KnockoutUntil <= b.Clock || from < b.KnockoutFrom {
		b.KnockoutFrom = from
	}
	b.KnockoutUntil = max(b.KnockoutUntil, until)
}

// ApplyCondition puts a poison, a disease, a burn or an infection on the body.
func (m Model) ApplyCondition(b *Body, id, author string, seed int64) (Event, error) {
	if b.Dead {
		return Event{}, errDead
	}
	def, ok := m.Condition(id)
	if !ok {
		return Event{}, fmt.Errorf("wounds: unknown condition %s", id)
	}
	b.Seq++
	key := fmt.Sprintf("c%d", b.Seq)
	c := Condition{ID: id, Kind: def.Kind, Author: author, At: b.Clock, OnsetAt: b.Clock + inSpan(def.Onset, seed, key, "onset"), Effects: append([]string(nil), def.Effects...)}
	if def.Doom != nil {
		c.DoomAt = b.Clock + max(1, inSpan(*def.Doom, seed, key, "doom")*m.Difficulty.DoomPermille/1000)
	} else {
		c.HealAt = c.OnsetAt + def.HealDays*86400
	}
	b.Conditions = append(b.Conditions, c)
	return Event{Type: "condition", Body: b.ID, At: b.Clock, Culprit: author, Detail: id}, nil
}

// controlRemain: the share (‰) of a bleeding left after the hands, dressings and tourniquets on its wound.
func (m Model) controlRemain(w *Wound, bl *Bleed) int64 {
	remain := int64(1000)
	for _, c := range w.Controls {
		k := m.Aid.Kinds[c]
		if bl.Internal && !(k.LimbInside && c == "tourniquet") {
			continue
		}
		r := k.Reduce[bl.Kind]
		if bl.Internal {
			r = 1000
		}
		remain = remain * (1000 - min(r, 1000)) / 1000
	}
	if w.Pressure != "" && !bl.Internal {
		remain = remain * (1000 - min(m.Aid.Kinds["pressure"].Reduce[bl.Kind], 1000)) / 1000
	}
	return remain
}

// clotBase: the vessel's own rate at body second t (halving by the clot, linear inside a half-life).
func (m Model) clotBase(b *Body, bl *Bleed, t int64) int64 {
	base := bl.RateUL
	if bl.ClotHalfS > 0 {
		half := max(bl.ClotHalfS*m.chip(b).ClotPermille/1000, 1)
		e := max(t-bl.Since, 0)
		k := e / half
		if k >= 40 {
			return 0
		}
		base >>= k
		base -= base * (e % half) / (2 * half)
	}
	return base
}

// flow: microlitres this second; mark: the clot has stopped it for good.
func (m Model) flow(b *Body, w *Wound, bl *Bleed, t int64) (int64, bool) {
	if bl.Stopped || w.Treated || b.Care != nil {
		return 0, false
	}
	base := m.clotBase(b, bl, t)
	if bl.ClotHalfS > 0 && base*60 < m.Blood.StoppedBelowMLMin*1000 {
		return 0, true
	}
	p := max(1000-m.Pressure.SlopePermille*b.LostPermille()/1000, m.Pressure.FloorPermille)
	return base * p / 1000 * m.controlRemain(w, bl) / 1000, false
}

// Tick advances the body by dt seconds of world time: bleeding, refill, timers, collapse and death.
func (m Model) Tick(b *Body, dt int64) []Event {
	var ev []Event
	for dt > 0 {
		if b.Dead {
			b.Clock += dt
			break
		}
		if n := m.quiet(b, dt); n > 1 {
			m.leap(b, n, &ev)
			dt -= n
			continue
		}
		m.step(b, &ev)
		dt--
	}
	return ev
}

func (m Model) regenULH(b *Body) int64 {
	return m.Blood.RegenMLH * 1000 * m.chip(b).RegenPermille / 1000
}

func (m Model) step(b *Body, ev *[]Event) {
	t := b.Clock
	loss := int64(0)
	for i := range b.Wounds {
		w := &b.Wounds[i]
		for j := range w.Bleeds {
			bl := &w.Bleeds[j]
			f, stop := m.flow(b, w, bl, t)
			if stop {
				bl.Stopped = true
				*ev = append(*ev, Event{Type: "stopped", Body: b.ID, At: t + 1, Wound: w.ID, Detail: bl.Structure})
				continue
			}
			w.LostUL += f
			loss += f
		}
	}
	b.RegenAcc += m.regenULH(b)
	dec := b.RegenAcc / 3600
	b.RegenAcc %= 3600
	b.LostUL = max(0, b.LostUL+loss-dec)
	b.Clock++
	m.timers(b, ev)
	m.settle(b, ev)
}

// quiet: how many seconds from now nothing but the refill changes (no flowing vessel, no timer due).
func (m Model) quiet(b *Body, dt int64) int64 {
	if b.Care == nil {
		for i := range b.Wounds {
			w := &b.Wounds[i]
			for j := range w.Bleeds {
				bl := &w.Bleeds[j]
				if !bl.Stopped && !w.Treated && m.controlRemain(w, bl) > 0 {
					return 1
				}
			}
		}
	}
	next := b.Clock + dt
	at := func(t int64) {
		if t > b.Clock && t < next {
			next = t
		}
	}
	at(b.KnockoutFrom)
	at(b.KnockoutUntil)
	at(b.StunUntil)
	at(b.PainOffUntil)
	if b.Care != nil {
		at(b.Care.End)
	}
	safe := m.Aid.Kinds["tourniquet"].LimbLostS
	for _, w := range b.Wounds {
		at(w.DoomAt)
		at(w.HealAt)
		if w.TourniquetAt > 0 && safe > 0 && !w.LimbLost {
			at(w.TourniquetAt + safe)
		}
	}
	for _, c := range b.Conditions {
		at(c.OnsetAt)
		at(c.DoomAt)
		at(c.HealAt)
	}
	// the refill crosses the next lower threshold (a state change or waking)
	if r := m.regenULH(b); r > 0 && b.LostUL > 0 {
		for _, p := range []int64{m.States.DownPermille + m.chip(b).DownBonusPermille, m.States.WakeBelowPermille, m.States.ConfusedPermille, m.States.WeakPermille, 0} {
			limit := (p*b.VolumeUL + 999) / 1000
			if b.LostUL < limit {
				continue
			}
			need := b.LostUL - limit + 1
			if p == 0 {
				need = b.LostUL
			}
			s := (need*3600 - b.RegenAcc + r - 1) / r
			at(b.Clock + max(s, 1))
			break
		}
	}
	return next - b.Clock
}

// leap: n quiet seconds at once, exactly as n steps would do them.
func (m Model) leap(b *Body, n int64, ev *[]Event) {
	total := b.RegenAcc + n*m.regenULH(b)
	b.LostUL = max(0, b.LostUL-total/3600)
	b.RegenAcc = total % 3600
	b.Clock += n
	m.timers(b, ev)
	m.settle(b, ev)
}

func (m Model) timers(b *Body, ev *[]Event) {
	tq := m.Aid.Kinds["tourniquet"]
	for i := range b.Wounds {
		w := &b.Wounds[i]
		if w.TourniquetAt > 0 && tq.LimbLostS > 0 && !w.LimbLost && b.Clock >= w.TourniquetAt+tq.LimbLostS {
			w.LimbLost = true
			if z, ok := m.Zone(w.Zone); ok {
				w.Effects = addUnique(w.Effects, tq.LimbLostFx[z.Part]...)
			}
			*ev = append(*ev, Event{Type: "limb_lost", Body: b.ID, At: b.Clock, Wound: w.ID})
		}
	}
	if b.Care != nil && b.Clock >= b.Care.End {
		m.finishCare(b, ev)
	}
	if b.Dead {
		return
	}
	kept := b.Wounds[:0]
	for _, w := range b.Wounds {
		if w.HealAt > 0 && b.Clock >= w.HealAt && w.DoomAt == 0 && !m.flowing(b, &w) && !w.LimbLost {
			*ev = append(*ev, Event{Type: "healed", Body: b.ID, At: b.Clock, Wound: w.ID})
			continue
		}
		kept = append(kept, w)
	}
	b.Wounds = kept
	keptC := b.Conditions[:0]
	for _, c := range b.Conditions {
		if c.HealAt > 0 && b.Clock >= c.HealAt && c.DoomAt == 0 {
			*ev = append(*ev, Event{Type: "healed", Body: b.ID, At: b.Clock, Detail: c.ID})
			continue
		}
		keptC = append(keptC, c)
	}
	b.Conditions = keptC
}

func (m Model) flowing(b *Body, w *Wound) bool {
	for j := range w.Bleeds {
		bl := &w.Bleeds[j]
		if !bl.Stopped && !w.Treated && m.controlRemain(w, bl) > 0 {
			return true
		}
	}
	return false
}

// settle: death, collapse, waking, the reported state and the chip alarm at the current second.
func (m Model) settle(b *Body, ev *[]Event) {
	if b.Dead {
		return
	}
	chip := m.chip(b)
	lp := b.LostPermille()
	cause, culprit := "", ""
	if b.Care == nil {
		if lp >= m.States.DeadPermille {
			cause, culprit = "blood", m.bloodCulprit(b)
		}
		for _, w := range b.Wounds {
			if cause == "" && w.DoomAt > 0 && b.Clock >= w.DoomAt {
				cause, culprit = w.ID, w.Author
			}
		}
		for _, c := range b.Conditions {
			if cause == "" && c.DoomAt > 0 && b.Clock >= c.DoomAt {
				cause, culprit = c.ID, c.Author
			}
		}
	}
	if cause != "" {
		if chip.Protected {
			if !b.Story {
				b.Story = true
				*ev = append(*ev, Event{Type: "story_node", Body: b.ID, At: b.Clock, Culprit: culprit, Detail: cause})
			}
		} else {
			b.Dead, b.DeadAt, b.Cause, b.Culprit = true, b.Clock, cause, culprit
			e := Event{Type: "dead", Body: b.ID, At: b.Clock, Culprit: culprit, Detail: cause}
			if culprit != "" && culprit != b.ID {
				e.Crime = m.Justice.Death
			}
			*ev = append(*ev, e)
			return
		}
	}
	if !b.Down && lp >= m.States.DownPermille+chip.DownBonusPermille {
		b.Down = true
	} else if b.Down && lp < m.States.WakeBelowPermille {
		b.Down = false
	}
	st := m.stateName(b)
	if st != b.Reported {
		t := st
		if b.Reported == "down" {
			*ev = append(*ev, Event{Type: "wake", Body: b.ID, At: b.Clock})
		}
		b.Reported = st
		*ev = append(*ev, Event{Type: t, Body: b.ID, At: b.Clock})
	}
	if chip.Monitor && !b.Alarmed && stateRank[st] >= stateRank[chip.AlarmAt] && stateRank[chip.AlarmAt] > 0 {
		b.Alarmed = true
		*ev = append(*ev, Event{Type: "chip_alarm", Body: b.ID, At: b.Clock, Detail: st})
	}
}

var stateRank = map[string]int{"ok": 0, "weak": 1, "confused": 2, "down": 3, "dead": 4}

// bloodCulprit: the author of the wound that let out the most blood (the earliest on a tie).
func (m Model) bloodCulprit(b *Body) string {
	best, who := int64(-1), ""
	for _, w := range b.Wounds {
		if w.LostUL > best {
			best, who = w.LostUL, w.Author
		}
	}
	return who
}

// unconscious: blood loss, a knockout, or a severed carotid / a heart still running out.
func (m Model) unconscious(b *Body) bool {
	if b.Dead || b.Down || b.Story {
		return true
	}
	if b.KnockoutFrom <= b.Clock && b.Clock < b.KnockoutUntil {
		return true
	}
	if b.Care != nil {
		return false
	}
	for i := range b.Wounds {
		w := &b.Wounds[i]
		for j := range w.Bleeds {
			bl := &w.Bleeds[j]
			at := bl.KnockoutAt
			if w.HeldKOAt > 0 && at > 0 && w.HeldKOAt < at {
				at = w.HeldKOAt
			}
			if at > 0 && b.Clock >= at && !bl.Stopped && !w.Treated && m.controlRemain(w, bl) >= 300 {
				return true
			}
		}
	}
	return false
}

func (m Model) stateName(b *Body) string {
	switch lp := b.LostPermille(); {
	case b.Dead:
		return "dead"
	case m.unconscious(b):
		return "down"
	case lp >= m.States.ConfusedPermille:
		return "confused"
	case lp >= m.States.WeakPermille:
		return "weak"
	}
	return "ok"
}

func (m Model) sameBluntRegion(a, b Wound) bool {
	za, oka := m.Zone(a.Zone)
	zb, okb := m.Zone(b.Zone)
	return oka && okb && za.Part == zb.Part
}
