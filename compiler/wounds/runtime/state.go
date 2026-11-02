package wounds

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

// Status is what the body can do now: the combat layer reads AP, standing, walking, holding a weapon.
type Status struct {
	State        string   `json:"state"` // ok | weak | confused | down | dead
	Conscious    bool     `json:"conscious"`
	Standing     bool     `json:"standing"`
	CanWalk      bool     `json:"can_walk"`
	HoldWeapon   bool     `json:"hold_weapon"`
	APPenalty    int      `json:"ap_penalty"`
	PaceMMS      int64    `json:"pace_mm_s"`
	LostPermille int64    `json:"lost_permille"`
	Effects      []string `json:"effects,omitempty"`
	ExternalULS  int64    `json:"external_ul_s"`
	InternalULS  int64    `json:"internal_ul_s"`
	InCare       bool     `json:"in_care,omitempty"`
}

// Status reads the body at its current second.
func (m Model) Status(b Body) Status {
	s := Status{State: m.stateName(&b), LostPermille: b.LostPermille(), InCare: b.Care != nil}
	s.Conscious = s.State != "down" && s.State != "dead"
	fx := map[string]bool{}
	splint := false
	for i := range b.Wounds {
		w := &b.Wounds[i]
		for _, e := range w.Effects {
			if e == "no_walk" && w.Splinted && !w.LimbLost {
				splint = true
				continue
			}
			fx[e] = true
		}
		if contains(w.Controls, "tourniquet") && !w.LimbLost {
			if z, ok := m.Zone(w.Zone); ok {
				for _, e := range m.Aid.Kinds["tourniquet"].LimbEffects[z.Part] {
					fx[e] = true
				}
			}
		}
		for j := range w.Bleeds {
			f, _ := m.flow(&b, w, &w.Bleeds[j], b.Clock)
			if w.Bleeds[j].Internal {
				s.InternalULS += f
			} else {
				s.ExternalULS += f
			}
		}
	}
	for _, c := range b.Conditions {
		if b.Clock >= c.OnsetAt {
			for _, e := range c.Effects {
				fx[e] = true
			}
		}
	}
	if b.PainOffUntil > b.Clock {
		delete(fx, "pain")
	}
	pace := m.Walk.NormalMMS
	falls, hold := false, true
	for _, e := range sortedKeys(fx) {
		d := m.Effects[e]
		s.Effects = append(s.Effects, e)
		s.APPenalty += d.AP
		pace = pace * d.PacePermille / 1000
		falls = falls || d.Falls
		hold = hold && d.HoldWeapon
	}
	if splint {
		s.Effects = append(s.Effects, "splinted")
		pace = pace * m.Walk.SplintedPacePermille / 1000
		s.APPenalty += m.Effects["no_walk"].AP
	}
	switch s.State {
	case "weak":
		s.APPenalty += m.States.Weak.AP
		pace = pace * m.States.Weak.PacePermille / 1000
	case "confused":
		s.APPenalty += m.States.Confused.AP
		pace = pace * m.States.Confused.PacePermille / 1000
	}
	stunned := b.StunUntil > b.Clock
	s.Standing = s.Conscious && !falls && !stunned && b.Care == nil
	s.CanWalk = s.Standing && pace > 0
	s.HoldWeapon = s.Conscious && hold && !stunned
	if s.CanWalk {
		s.PaceMMS = pace
	}
	return s
}

// Serious: a met structure of the serious severity, or blood loss reaching «weak» (resolve.cue).
func (m Model) Serious(b Body) bool {
	if b.LostPermille() >= m.States.WeakPermille || b.Dead {
		return true
	}
	for _, w := range b.Wounds {
		for _, sid := range w.Structures {
			if st, _, ok := m.Structure(sid); ok && st.Severity >= m.Resolve.SeriousSeverity {
				return true
			}
		}
	}
	return false
}

// Psyche is what the stop-of-fight roll reads: the stress response of the character's vectors (drives.cue), the fear
// after the chip's suppression (‰) and the combat courage (0..100).
type Psyche struct {
	Response     string `json:"response"`
	FearPermille int64  `json:"fear_permille"`
	Courage      int64  `json:"courage"`
}

// Reaction to a serious wound: fight_on | flee | fall | surrender.
type Reaction struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

// StopFight rolls how a wounded person ends the fight. A body that cannot stand falls; one that cannot walk does not
// flee; a lost grip halves the will to go on.
func (m Model) StopFight(b Body, p Psyche, seed int64) Reaction {
	st := m.Status(b)
	switch {
	case !st.Conscious:
		return Reaction{Kind: "fall", Reason: "unconscious"}
	case !st.Standing:
		if p.Response == "fight" && st.HoldWeapon {
			return Reaction{Kind: "fight_on", Reason: "on the ground, still armed"}
		}
		return Reaction{Kind: "surrender", Reason: "cannot stand"}
	case !m.Serious(b):
		return Reaction{Kind: "fight_on", Reason: "not serious"}
	}
	base, ok := m.Resolve.Reactions[p.Response]
	if !ok {
		base = m.Resolve.Reactions["fight"]
	}
	w := map[string]int64{}
	for k, v := range base {
		w[k] = v
	}
	cb := m.Resolve.CourageBase
	w["fight_on"] = w["fight_on"] * (1000 - min(max(p.FearPermille, 0), 1000)) / 1000 * (cb + min(max(p.Courage, 0), 100)) / (cb + 100)
	if !st.HoldWeapon {
		w["surrender"] += w["fight_on"] / 2
		w["fight_on"] /= 2
	}
	if !st.CanWalk {
		w["surrender"] += w["flee"]
		w["flee"] = 0
	}
	k := pick(w, seed, b.ID, "resolve", fmt.Sprint(b.Clock))
	if k == "" {
		k = "fall"
	}
	return Reaction{Kind: k, Reason: p.Response}
}

// Ledger counts who killed whom (owner 02.11: a playthrough without killing; the endings, barks and justice read it).
type Ledger struct {
	Kills map[string][]string `json:"kills,omitempty"`
}

// Record notes the deaths in events: the culprit is who dealt the wound that killed — even hours later.
func (l *Ledger) Record(events []Event) {
	for _, e := range events {
		if e.Type != "dead" || e.Culprit == "" || e.Culprit == e.Body {
			continue
		}
		if l.Kills == nil {
			l.Kills = map[string][]string{}
		}
		if !contains(l.Kills[e.Culprit], e.Body) {
			l.Kills[e.Culprit] = append(l.Kills[e.Culprit], e.Body)
		}
	}
}

// Count is how many people this actor has killed.
func (l Ledger) Count(actor string) int { return len(l.Kills[actor]) }

// Save is the persistent part: the bodies and the kill ledger.
type Save struct {
	Version int    `json:"version"`
	Bodies  []Body `json:"bodies"`
	Ledger  Ledger `json:"ledger"`
}

// Marshal writes a validated save (bodies sorted by id).
func (m Model) Marshal(s Save) ([]byte, error) {
	s.Version = 1
	s.Bodies = append([]Body(nil), s.Bodies...)
	sort.Slice(s.Bodies, func(i, j int) bool { return s.Bodies[i].ID < s.Bodies[j].ID })
	for _, b := range s.Bodies {
		if err := m.ValidateBody(b); err != nil {
			return nil, err
		}
	}
	return json.Marshal(s)
}

// Load reads a save strictly and validates every body against the catalog.
func (m Model) Load(data []byte) (Save, error) {
	var s Save
	d := json.NewDecoder(bytesReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return s, fmt.Errorf("wounds save: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return s, fmt.Errorf("wounds save: trailing data")
	}
	if s.Version != 1 {
		return s, fmt.Errorf("wounds save: version %d", s.Version)
	}
	seen := map[string]bool{}
	for _, b := range s.Bodies {
		if seen[b.ID] {
			return s, fmt.Errorf("wounds save: duplicate body %s", b.ID)
		}
		seen[b.ID] = true
		if err := m.ValidateBody(b); err != nil {
			return s, err
		}
	}
	return s, nil
}

// ValidateBody: ids against the catalog, ranges, timers not before the wound.
func (m Model) ValidateBody(b Body) error {
	if b.ID == "" || b.VolumeUL <= 0 || b.LostUL < 0 || b.Clock < 0 || b.RegenAcc < 0 || b.RegenAcc >= 3600 {
		return fmt.Errorf("wounds: body %q invalid", b.ID)
	}
	if _, ok := m.Chips[b.Chip]; !ok {
		return fmt.Errorf("wounds: body %s chip %q", b.ID, b.Chip)
	}
	ids := map[string]bool{}
	for _, w := range b.Wounds {
		if ids[w.ID] || w.At > b.Clock {
			return fmt.Errorf("wounds: body %s wound %s", b.ID, w.ID)
		}
		ids[w.ID] = true
		if _, ok := m.Zone(w.Zone); !ok {
			return fmt.Errorf("wounds: body %s wound %s zone", b.ID, w.ID)
		}
		if _, ok := depthRank[w.Depth]; !ok {
			return fmt.Errorf("wounds: body %s wound %s depth", b.ID, w.ID)
		}
		if _, ok := m.Mechanisms[w.Mechanism]; !ok {
			return fmt.Errorf("wounds: body %s wound %s mechanism", b.ID, w.ID)
		}
		for _, sid := range w.Structures {
			if _, z, ok := m.Structure(sid); !ok || z.ID != w.Zone {
				return fmt.Errorf("wounds: body %s wound %s structure %s", b.ID, w.ID, sid)
			}
		}
		for _, bl := range w.Bleeds {
			if !contains(w.Structures, bl.Structure) || bl.RateUL < 0 || bl.Since < w.At {
				return fmt.Errorf("wounds: body %s wound %s bleed", b.ID, w.ID)
			}
		}
		for _, c := range w.Controls {
			if _, ok := m.Aid.Kinds[c]; !ok {
				return fmt.Errorf("wounds: body %s wound %s control %s", b.ID, w.ID, c)
			}
		}
		for _, e := range w.Effects {
			if _, ok := m.Effects[e]; !ok {
				return fmt.Errorf("wounds: body %s wound %s effect %s", b.ID, w.ID, e)
			}
		}
	}
	for _, c := range b.Conditions {
		if _, ok := m.Condition(c.ID); !ok || c.At > b.Clock {
			return fmt.Errorf("wounds: body %s condition %s", b.ID, c.ID)
		}
	}
	if b.Care != nil {
		if _, ok := m.Provider(b.Care.Provider); !ok || b.Care.End < b.Care.Start || b.Care.Mode != "full" && b.Care.Mode != "stabilize" {
			return fmt.Errorf("wounds: body %s care", b.ID)
		}
	}
	return nil
}
