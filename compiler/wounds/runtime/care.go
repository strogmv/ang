package wounds

import (
	"fmt"
	"sort"
)

// Payer is what the provider reads: the chip binding of the patient ("own" — his own chip; "foreign" — someone else's
// binding; "none"), the CRD on his account (minor units) and the system trust (crafting.system_trust, ‰).
type Payer struct {
	Chip          string `json:"chip"`
	BalanceMinor  int64  `json:"balance_minor"`
	TrustPermille int64  `json:"trust_permille"`
}

// Quote: full treatment, stabilisation only (the account is short) or a refusal (wounds.care.* key). The host debits
// PriceMinor when the treatment starts; the engine owns no money.
type Quote struct {
	Provider       string   `json:"provider"`
	Mode           string   `json:"mode"` // full | stabilize | refused
	Reason         string   `json:"reason,omitempty"`
	PriceMinor     int64    `json:"price_minor"`
	FullPriceMinor int64    `json:"full_price_minor"`
	Seconds        int64    `json:"seconds"`
	FullSeconds    int64    `json:"full_seconds"`
	Journal        []string `json:"journal,omitempty"` // mechanism:wound — written to the system journal (a crime hook)
}

func (m Model) cures(p Provider, kind string) bool {
	need := "wounds"
	switch kind {
	case "bone":
		need = "fractures"
	case "poison", "disease", "burn", "infection":
		need = kind
	}
	return contains(p.Cures, need)
}

// Quote prices a treatment of the whole body at a provider.
func (m Model) Quote(b *Body, provider string, pay Payer) (Quote, error) {
	p, ok := m.Provider(provider)
	if !ok {
		return Quote{}, fmt.Errorf("wounds: unknown provider %s", provider)
	}
	q := Quote{Provider: provider, Mode: "refused"}
	switch {
	case b.Dead:
		q.Reason = "wounds.care.dead"
		return q, nil
	case p.Chip == "own" && pay.Chip != "own":
		q.Reason = "wounds.care.no_chip"
		return q, nil
	case pay.TrustPermille < p.TrustMinPermille:
		q.Reason = "wounds.care.trust"
		return q, nil
	}
	severity, minutes := int64(0), int64(0)
	for _, w := range b.Wounds {
		for _, sid := range w.Structures {
			st, _, _ := m.Structure(sid)
			if contains(p.RefusesKinds, st.Kind) && !w.Treated {
				q.Reason = "wounds.care.refuses"
				return q, nil
			}
			if m.cures(p, st.Kind) {
				severity += int64(st.Severity)
				minutes += st.CapsuleMin
			}
		}
		if len(w.Structures) == 0 && (w.DoomAt > 0 || w.LimbLost) {
			severity++
			minutes += 15
		}
	}
	for _, c := range b.Conditions {
		if def, ok := m.Condition(c.ID); ok && m.cures(p, c.Kind) {
			severity += int64(def.Severity)
			minutes += def.CapsuleMin
		}
	}
	if contains(p.Cures, "blood") && b.LostUL > 0 {
		minutes += p.BloodMinPer100ML * ((b.LostUL + 99999) / 100000)
	}
	if severity == 0 && (b.LostUL == 0 || !contains(p.Cures, "blood")) {
		q.Reason = "wounds.care.nothing"
		return q, nil
	}
	q.FullSeconds = max(minutes*60*p.TimePermille/1000, 60)
	hours := (q.FullSeconds + 3599) / 3600
	q.FullPriceMinor = (p.Price.BaseMinor + p.Price.PerSeverityMinor*severity + p.Price.PerHourMinor*hours) * m.Difficulty.PricePermille / 1000
	stab := p.Price.StabilizeMinor * m.Difficulty.PricePermille / 1000
	for _, w := range b.Wounds {
		if contains(p.Journal, w.Mechanism) {
			q.Journal = append(q.Journal, w.Mechanism+":"+w.ID)
		}
	}
	sort.Strings(q.Journal)
	switch {
	case pay.BalanceMinor >= q.FullPriceMinor:
		q.Mode, q.PriceMinor, q.Seconds = "full", q.FullPriceMinor, q.FullSeconds
		q.Reason = "wounds.care.full"
	case pay.BalanceMinor >= stab:
		q.Mode, q.PriceMinor, q.Seconds = "stabilize", stab, p.Stabilize.Min*60
		q.Reason = "wounds.care.stabilize"
	default:
		q.Reason = "wounds.care.money"
	}
	return q, nil
}

// StartCare begins a quoted treatment: the vessels are clamped at once, the shock is lifted, the body lies in the
// capsule (or on the healer's table) until Care.End, ticking with the world.
func (m Model) StartCare(b *Body, q Quote, seed int64) ([]Event, error) {
	p, ok := m.Provider(q.Provider)
	if !ok || q.Mode != "full" && q.Mode != "stabilize" {
		return nil, fmt.Errorf("wounds: not a startable quote")
	}
	if b.Dead {
		return nil, errDead
	}
	if b.Care != nil {
		return nil, fmt.Errorf("wounds: already in care")
	}
	var ev []Event
	b.Care = &Care{Provider: p.ID, Mode: q.Mode, Start: b.Clock, End: b.Clock + q.Seconds, PriceMinor: q.PriceMinor, Seed: seed}
	for i := range b.Wounds {
		w := &b.Wounds[i]
		for j := range w.Bleeds {
			w.Bleeds[j].Stopped = true
		}
		w.Pressure = ""
	}
	if limit := p.Stabilize.RestoreToPermille * b.VolumeUL / 1000; b.LostUL > limit {
		b.LostUL = limit
	}
	ev = append(ev, Event{Type: "care_start", Body: b.ID, At: b.Clock, Detail: p.ID + ":" + q.Mode})
	for _, j := range q.Journal {
		ev = append(ev, Event{Type: "journal", Body: b.ID, At: b.Clock, Detail: j})
	}
	m.settle(b, &ev)
	return ev, nil
}

func (m Model) finishCare(b *Body, ev *[]Event) {
	c := b.Care
	b.Care = nil
	p, _ := m.Provider(c.Provider)
	if c.Mode == "full" {
		kept := b.Wounds[:0]
		for _, w := range b.Wounds {
			cured := true
			for _, sid := range w.Structures {
				st, _, _ := m.Structure(sid)
				cured = cured && m.cures(p, st.Kind) && !contains(p.RefusesKinds, st.Kind)
			}
			if cured {
				continue
			}
			kept = append(kept, w)
		}
		b.Wounds = kept
		keptC := b.Conditions[:0]
		for _, x := range b.Conditions {
			if !m.cures(p, x.Kind) {
				keptC = append(keptC, x)
			}
		}
		b.Conditions = keptC
		if contains(p.Cures, "blood") {
			b.LostUL, b.RegenAcc = 0, 0
		}
		if b.KnockoutUntil > b.Clock {
			b.KnockoutUntil = b.Clock
		}
		b.StunUntil = min(b.StunUntil, b.Clock)
	}
	*ev = append(*ev, Event{Type: "care_done", Body: b.ID, At: b.Clock, Detail: c.Provider + ":" + c.Mode})
	if c.Mode == "full" && p.ComplicationPPM > 0 && p.Complication != "" && chance(p.ComplicationPPM, c.Seed, b.ID, "complication", fmt.Sprint(c.Start)) {
		if e, err := m.ApplyCondition(b, p.Complication, "", c.Seed); err == nil {
			e.Type = "complication"
			*ev = append(*ev, e)
		}
	}
}
