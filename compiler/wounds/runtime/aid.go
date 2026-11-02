package wounds

import "fmt"

// AidAct is one act of field aid on one wound. The host checks reach and hands, spends the item when Spent and lets
// Seconds of world time pass for the helper.
type AidAct struct {
	Kind    string `json:"kind"`
	Wound   string `json:"wound"`
	Item    string `json:"item,omitempty"`
	By      string `json:"by,omitempty"`
	Skill   int64  `json:"skill,omitempty"` // the helper's medicine practice 0..100
	Seed    int64  `json:"seed"`
	Release bool   `json:"release,omitempty"` // pressure: let go
	// Force: the act succeeds (scenario rows assume the help worked; the success chance is in the aid table)
	Force bool `json:"force,omitempty"`
}

// AidResult: applied or the refusal key (wounds.aid.*), whether the item is spent, the work time, what changed.
type AidResult struct {
	Applied  bool    `json:"applied"`
	Reason   string  `json:"reason,omitempty"`
	Spent    bool    `json:"spent,omitempty"`
	Seconds  int64   `json:"seconds"`
	Treated  bool    `json:"treated,omitempty"`
	Splinted bool    `json:"splinted,omitempty"`
	Events   []Event `json:"events,omitempty"`
}

// externalFlowing: a vessel of the wound that bleeds outside now (or inside a limb a tourniquet would close).
func (m Model) externalFlowing(b *Body, w *Wound, limbInside bool) bool {
	for j := range w.Bleeds {
		bl := &w.Bleeds[j]
		if bl.Stopped || w.Treated || bl.Internal && !limbInside {
			continue
		}
		if flow, _ := m.flow(b, w, bl, b.Clock); flow > 0 {
			return true
		}
	}
	return false
}

func (m Model) successPPM(base, skill int64) int64 {
	p := base*m.Difficulty.AidPermille/1000 + min(max(skill, 0), 100)*m.Aid.Skill.PerPointPPM
	return min(p, max(m.Aid.Skill.CapPPM, base))
}

// ApplyAid applies pressure, a tourniquet, packing, a dressing, a chest seal or a universal medkit.
func (m Model) ApplyAid(b *Body, a AidAct) (AidResult, error) {
	kind, ok := m.Aid.Kinds[a.Kind]
	if !ok {
		return AidResult{}, fmt.Errorf("wounds: unknown aid %s", a.Kind)
	}
	res := AidResult{Seconds: kind.Seconds}
	if b.Dead {
		res.Reason = "wounds.aid.dead"
		return res, nil
	}
	w := b.wound(a.Wound)
	if w == nil {
		return AidResult{}, fmt.Errorf("wounds: no wound %s", a.Wound)
	}
	z, _ := m.Zone(w.Zone)
	if a.Release {
		if a.Kind != "pressure" {
			return AidResult{}, fmt.Errorf("wounds: only pressure is released")
		}
		w.Pressure = ""
		res.Applied = true
		m.settle(b, &res.Events)
		return res, nil
	}
	if !contains(kind.Zones, w.Zone) {
		res.Reason = "wounds.aid.wrong_zone"
		return res, nil
	}
	success := int64(Million)
	var kit Medkit
	switch a.Kind {
	case "pressure":
	case "medkit":
		if kit, ok = m.Medkit(a.Item); !ok {
			res.Reason = "wounds.aid.wrong_item"
			return res, nil
		}
		success = kit.SuccessPPM
	default:
		it, ok := m.AidItem(a.Item, a.Kind)
		if !ok {
			res.Reason = "wounds.aid.wrong_item"
			return res, nil
		}
		success = it.SuccessPPM
	}
	if !m.aidUseful(b, w, z, a.Kind, kit) {
		res.Reason = "wounds.aid.nothing"
		return res, nil
	}
	res.Spent = a.Kind != "pressure"
	by := a.By
	if by == "" {
		by = b.ID
	}
	if a.Kind != "pressure" && !a.Force && !chance(m.successPPM(success, a.Skill), a.Seed, b.ID, w.ID, "aid", a.Kind, a.Item, fmt.Sprint(len(w.Aiders), len(w.Controls))) {
		res.Reason = "wounds.aid.failed"
		res.Events = append(res.Events, Event{Type: "aid_failed", Body: b.ID, At: b.Clock, Wound: w.ID, Detail: a.Kind})
		return res, nil
	}
	switch a.Kind {
	case "pressure":
		w.Pressure = by
	case "tourniquet":
		m.tourniquet(b, w)
	case "medkit":
		res.Treated, res.Splinted = m.medkit(b, w, z, kit)
	default:
		w.Controls = addUnique(w.Controls, a.Kind)
		w.Effects = without(w.Effects, kind.Ease...)
	}
	if a.Kind != "pressure" && w.Pressure == by {
		w.Pressure = "" // packing and dressings are pressed in; the hands are free again
	}
	w.Aiders = addUnique(w.Aiders, by)
	res.Applied = true
	res.Reason = "wounds.aid.applied"
	if res.Treated {
		res.Reason = "wounds.aid.treated"
	} else if res.Splinted {
		res.Reason = "wounds.aid.splinted"
	}
	res.Events = append(res.Events, Event{Type: "aid", Body: b.ID, At: b.Clock, Wound: w.ID, Culprit: by, Detail: a.Kind})
	m.settle(b, &res.Events)
	return res, nil
}

func (m Model) tourniquet(b *Body, w *Wound) {
	if !contains(w.Controls, "tourniquet") {
		w.Controls = append(w.Controls, "tourniquet")
		w.TourniquetAt = b.Clock
	}
}

func (m Model) aidUseful(b *Body, w *Wound, z Zone, kind string, kit Medkit) bool {
	if kind != "medkit" && contains(w.Controls, kind) {
		return false
	}
	switch kind {
	case "tourniquet":
		return m.externalFlowing(b, w, true)
	case "seal":
		for _, e := range m.Aid.Kinds["seal"].Ease {
			if contains(w.Effects, e) {
				return true
			}
		}
		return m.externalFlowing(b, w, false)
	case "medkit":
		if m.externalFlowing(b, w, z.Tourniquet && kit.Tourniquet) || kit.InfuseML > 0 && b.LostUL > 0 || kit.PainOff && b.PainOffUntil <= b.Clock && contains(w.Effects, "pain") {
			return true
		}
		return !w.Treated && m.kitTreats(w, kit) || kit.Splint && !w.Splinted && m.hasKind(w, "bone")
	}
	return m.externalFlowing(b, w, false)
}

func (m Model) hasKind(w *Wound, kind string) bool {
	for _, sid := range w.Structures {
		if st, _, ok := m.Structure(sid); ok && st.Kind == kind {
			return true
		}
	}
	return false
}

// kitTreats: every met structure is of a kind the kit closes, the wound is not deeper than it reaches, nothing inside.
func (m Model) kitTreats(w *Wound, kit Medkit) bool {
	if depthRank[w.Depth] > depthRank[kit.MaxDepth] || w.DoomAt > 0 {
		return false
	}
	for _, sid := range w.Structures {
		st, _, ok := m.Structure(sid)
		if !ok || st.Internal || !contains(kit.Treats, st.Kind) {
			return false
		}
	}
	return true
}

// medkit: stop the outside bleeding with what the kit holds, splint, take the pain, close what it can close.
func (m Model) medkit(b *Body, w *Wound, z Zone, kit Medkit) (treated, splinted bool) {
	big := false
	for _, bl := range w.Bleeds {
		if !bl.Stopped && (bl.Kind == "artery" || bl.Kind == "critical" || bl.Kind == "deep_vein") && (!bl.Internal || z.Tourniquet) {
			big = true
		}
	}
	switch {
	case big && z.Tourniquet && kit.Tourniquet:
		m.tourniquet(b, w)
	case big && z.Junctional && kit.Packing:
		w.Controls = addUnique(w.Controls, "packing")
	case m.externalFlowing(b, w, false):
		w.Controls = addUnique(w.Controls, "dressing")
	}
	if kit.PainOff {
		b.PainOffUntil = max(b.PainOffUntil, b.Clock+kit.PainOffS)
	}
	if kit.InfuseML > 0 {
		b.LostUL = max(0, b.LostUL-kit.InfuseML*1000)
	}
	if kit.Splint && m.hasKind(w, "bone") {
		w.Splinted, splinted = true, true
	}
	if !w.Treated && m.kitTreats(w, kit) {
		w.Treated, treated = true, true
		for j := range w.Bleeds {
			w.Bleeds[j].Stopped = true
		}
		// the vessel is closed: the tourniquet comes off before it costs the limb
		w.Controls = without(w.Controls, "tourniquet", "packing")
		w.TourniquetAt = 0
	}
	return treated, splinted
}
