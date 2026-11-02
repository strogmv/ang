package wounds

import "sort"

// Forecast: seconds from now until the body falls (cannot stand), loses consciousness and dies without any help;
// -1 — not within the horizon.
type Forecast struct {
	FallS int64 `json:"fall_s"`
	DownS int64 `json:"down_s"`
	DeadS int64 `json:"dead_s"`
}

type marks struct{ fall, down, dead int64 }

// observe advances the body to `until` seconds after t0 in quiet leaps and single steps, noting the first fall,
// unconsciousness and death (relative to t0). After the first look the body changes only through events.
func (m Model) observe(b *Body, t0, until int64, mk *marks) {
	note := func() {
		st := m.Status(*b)
		at := b.Clock - t0
		if mk.fall < 0 && !st.Standing && b.Care == nil {
			mk.fall = at
		}
		if mk.down < 0 && !st.Conscious {
			mk.down = at
		}
		if mk.dead < 0 && b.Dead {
			mk.dead = at
		}
	}
	note()
	var ev []Event
	for b.Clock-t0 < until && !b.Dead {
		left := until - (b.Clock - t0)
		if n := m.quiet(b, left); n > 1 {
			m.leap(b, n, &ev)
		} else {
			m.step(b, &ev)
		}
		if len(ev) > 0 || b.StunUntil == b.Clock || b.KnockoutFrom == b.Clock || b.KnockoutUntil == b.Clock {
			note()
		}
		ev = ev[:0]
	}
}

// Forecast simulates a copy of the body without help (the live body is not touched).
func (m Model) Forecast(b Body, horizon int64) Forecast {
	c := b.Clone()
	mk := marks{-1, -1, -1}
	m.observe(&c, c.Clock, horizon, &mk)
	return Forecast{FallS: mk.fall, DownS: mk.down, DeadS: mk.dead}
}

// WalkResult: whether the wounded reached the place on his own feet and when (seconds of walking), or when he fell.
type WalkResult struct {
	Reached bool  `json:"reached"`
	Seconds int64 `json:"seconds"`
	FellAtS int64 `json:"fell_at_s"`
}

// WalkTo moves the body `mm` at its wounded pace, ticking the world every second (bounded by limit seconds).
func (m Model) WalkTo(b *Body, mm, limit int64) (WalkResult, []Event) {
	var ev []Event
	done := int64(0)
	for s := int64(0); s < limit; s++ {
		st := m.Status(*b)
		if !st.CanWalk {
			return WalkResult{FellAtS: s}, ev
		}
		done += st.PaceMMS
		var e []Event
		m.step(b, &e)
		ev = append(ev, e...)
		if done >= mm {
			return WalkResult{Reached: true, Seconds: s + 1, FellAtS: -1}, ev
		}
	}
	return WalkResult{FellAtS: -1}, ev
}

// WoundLine is one line of the hero's own wound figure: «бедро, бедренная артерия — ~3 мин».
type WoundLine struct {
	Wound      string   `json:"wound"`
	Zone       string   `json:"zone"`                 // key
	Structures []string `json:"structures,omitempty"` // keys of what he can know; organs show as wounds.wound.inside
	Mechanism  string   `json:"mechanism"`            // key
	Bleeding   bool     `json:"bleeding,omitempty"`
	Controls   []string `json:"controls,omitempty"` // aid keys
	Effects    []string `json:"effects,omitempty"`  // effect keys
	Danger     bool     `json:"danger,omitempty"`   // the wound that drives the forecast
}

// SelfView is what the hero sees of his own body: the figure, the time, the state. No health bar.
type SelfView struct {
	State       string      `json:"state"` // key
	Lines       []WoundLine `json:"lines,omitempty"`
	Conditions  []string    `json:"conditions,omitempty"` // keys of conditions already felt
	Forecast    Forecast    `json:"forecast"`
	ForecastKey string      `json:"forecast_key"`
	Minutes     int64       `json:"minutes,omitempty"`
	Hours       int64       `json:"hours,omitempty"`
}

// ViewSelf builds the hero's own view (horizon for the forecast in seconds).
func (m Model) ViewSelf(b Body, horizon int64) SelfView {
	v := SelfView{State: "wounds.state." + m.stateName(&b)}
	if b.Dead {
		return v
	}
	if st := m.Status(b); st.Conscious && !st.Standing {
		v.State = "wounds.state.fallen"
	}
	v.Forecast = m.Forecast(b, horizon)
	switch {
	case v.Forecast.DownS >= 0 && v.Forecast.DownS < 3600 && (v.Forecast.DeadS < 0 || v.Forecast.DownS < v.Forecast.DeadS):
		v.ForecastKey, v.Minutes = "wounds.forecast.down", max(1, (v.Forecast.DownS+59)/60)
	case v.Forecast.DeadS >= 0 && v.Forecast.DeadS < 3600:
		v.ForecastKey, v.Minutes = "wounds.forecast.dead", max(1, (v.Forecast.DeadS+59)/60)
	case v.Forecast.DeadS >= 0:
		v.ForecastKey, v.Hours = "wounds.forecast.hours", max(1, (v.Forecast.DeadS+1799)/3600)
	default:
		v.ForecastKey = "wounds.forecast.stable"
	}
	danger, best := -1, int64(-1)
	for i := range b.Wounds {
		w := &b.Wounds[i]
		z, _ := m.Zone(w.Zone)
		line := WoundLine{Wound: w.ID, Zone: z.Name, Mechanism: m.Mechanisms[w.Mechanism].Name}
		inside := false
		for _, sid := range w.Structures {
			st, _, _ := m.Structure(sid)
			if st.Internal {
				inside = true
				continue
			}
			line.Structures = append(line.Structures, st.Name)
		}
		if inside {
			line.Structures = append(line.Structures, "wounds.wound.inside")
		}
		score := int64(0)
		for j := range w.Bleeds {
			f, _ := m.flow(&b, w, &w.Bleeds[j], b.Clock)
			if f > 0 && !w.Bleeds[j].Internal {
				line.Bleeding = true
			}
			score += f
		}
		if w.DoomAt > 0 {
			score += 1
		}
		for _, c := range w.Controls {
			line.Controls = append(line.Controls, m.Aid.Kinds[c].Name)
		}
		if w.Pressure != "" {
			line.Controls = append(line.Controls, m.Aid.Kinds["pressure"].Name)
		}
		for _, e := range w.Effects {
			line.Effects = append(line.Effects, "wounds.effect."+e)
		}
		if score > best {
			danger, best = i, score
		}
		v.Lines = append(v.Lines, line)
	}
	if danger >= 0 && best > 0 && v.ForecastKey != "wounds.forecast.stable" {
		v.Lines[danger].Danger = true
	}
	for _, c := range b.Conditions {
		if b.Clock >= c.OnsetAt {
			if def, ok := m.Condition(c.ID); ok {
				v.Conditions = append(v.Conditions, def.Name)
			}
		}
	}
	return v
}

// ViewOther: what anyone else sees from outside — blood, a limp, clutching, gasping, paleness, stillness. Never the
// numbers, never what is hit inside (sign keys, sorted).
func (m Model) ViewOther(b Body) []string {
	set := map[string]bool{}
	st := m.Status(b)
	if !st.Conscious {
		set["still"] = true
	}
	if b.LostPermille() >= m.States.WeakPermille {
		set["pale"] = true
	}
	for i := range b.Wounds {
		w := &b.Wounds[i]
		for j := range w.Bleeds {
			bl := &w.Bleeds[j]
			if f, _ := m.flow(&b, w, bl, b.Clock); f > 0 && !bl.Internal {
				switch bl.Kind {
				case "artery", "critical":
					set["blood_spurt"] = true
				case "vein", "deep_vein":
					set["blood_flow"] = true
				default:
					set["blood"] = true
				}
			}
		}
		for _, sid := range w.Structures {
			s, _, _ := m.Structure(sid)
			switch s.Sign {
			case "limp", "clutch", "gasp", "bruise":
				set[s.Sign] = true
			}
		}
	}
	for _, e := range st.Effects {
		if e == "limp" || e == "splinted" {
			set["limp"] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, "wounds.sign."+k)
	}
	sort.Strings(out)
	return out
}
