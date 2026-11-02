package wounds

import (
	"fmt"
	"sort"
)

// Horizon of the timeline table: a week of world time.
const Horizon = 7 * 86400

// ScenarioSeed: the seed of the reproducible rows (the risk runs derive their own seeds from it).
const ScenarioSeed = 42

// ScenarioBody applies the row's wound (forced structures) to a fresh body — for tests and the host's demo.
func (m Model) ScenarioBody(s Scenario, seed int64) (Body, error) {
	b, _, err := m.scenarioBody(s, seed, true)
	return b, err
}

// WalkRow: from a route's start to the capsule on his own feet.
type WalkRow struct {
	Route   string `json:"route"`
	MM      int64  `json:"mm"`
	Reached bool   `json:"reached"`
	AtS     int64  `json:"at_s"` // seconds after the wound when he arrives (or falls); 0 and not reached — cannot get up
}

// Risk is the honest Monte Carlo of the same blow with depth and structures rolled.
type Risk struct {
	Runs        int   `json:"runs"`
	FallPPM     int64 `json:"fall_ppm"`
	DownPPM     int64 `json:"down_ppm"`
	SeriousPPM  int64 `json:"serious_ppm"`
	DeadSoonPPM int64 `json:"dead_soon_ppm"` // within ten minutes, no help possible in time
	DeadPPM     int64 `json:"dead_ppm"`      // within the horizon without help (a capsule would save the rest)
}

// Timeline is one row of design/wounds/timelines.md.
type Timeline struct {
	Scenario   string    `json:"scenario"`
	Wound      []string  `json:"wound"` // structures met
	FallS      int64     `json:"fall_s"`
	DownS      int64     `json:"down_s"`
	DeadS      int64     `json:"dead_s"`
	Aid        []string  `json:"aid"` // what was done and when, or the refusal
	AidedFallS int64     `json:"aided_fall_s"`
	AidedDownS int64     `json:"aided_down_s"`
	AidedDeadS int64     `json:"aided_dead_s"`
	Walk       []WalkRow `json:"walk"`
	AidedWalk  []WalkRow `json:"aided_walk"`
	Capsule    Quote     `json:"capsule"` // the capsule's price on arrival (the hero's 10 CRD)
	Risk       *Risk     `json:"risk,omitempty"`
}

func (m Model) scenarioHit(s Scenario, seed int64, forced bool) Hit {
	h := Hit{ID: "w1", Author: "attacker", Seed: seed}
	if s.Hit != nil {
		h.Weapon, h.Mechanism, h.Mode, h.Zone = s.Hit.Weapon, s.Hit.Mechanism, s.Hit.Mode, s.Hit.Zone
		if forced {
			h.Depth, h.Structures = s.Hit.Depth, append([]string(nil), s.Hit.Structures...)
		} else if s.Hit.Weapon == "" {
			h.Depth = s.Hit.Depth
		}
	}
	if s.Technique != nil {
		h.Technique, h.HoldS = s.Technique.ID, s.Technique.HoldS
		if s.Technique.Weapon != "" {
			h.Weapon = s.Technique.Weapon
		}
		if t, ok := m.Technique(s.Technique.ID); ok {
			h.Angle, h.Zone = t.Angle, t.Zone
		}
	}
	return h
}

func (m Model) scenarioBody(s Scenario, seed int64, forced bool) (Body, bool, error) {
	b := m.NewBody("victim", s.Chip)
	serious := false
	if s.Hit != nil || s.Technique != nil {
		r, err := m.ApplyHit(&b, m.scenarioHit(s, seed, forced))
		if err != nil {
			return b, false, err
		}
		serious = r.Serious
	}
	if s.Condition != "" {
		if _, err := m.ApplyCondition(&b, s.Condition, "attacker", seed); err != nil {
			return b, false, err
		}
	}
	return b, serious, nil
}

// RunScenario plays a scenario row: without help, with the help of the row, the walks to the capsule and the risk.
func (m Model) RunScenario(s Scenario, seed int64) (Timeline, error) {
	tl := Timeline{Scenario: s.ID}
	b, _, err := m.scenarioBody(s, seed, true)
	if err != nil {
		return tl, fmt.Errorf("scenario %s: %w", s.ID, err)
	}
	if len(b.Wounds) > 0 {
		tl.Wound = append(tl.Wound, b.Wounds[0].Structures...)
	}
	t0 := b.Clock
	f := m.Forecast(b, Horizon)
	tl.FallS, tl.DownS, tl.DeadS = f.FallS, f.DownS, f.DeadS
	for _, r := range m.Routes {
		c := b.Clone()
		w, _ := m.WalkTo(&c, r.MM, 3600)
		tl.Walk = append(tl.Walk, WalkRow{Route: r.ID, MM: r.MM, Reached: w.Reached, AtS: max(w.Seconds, w.FellAtS)})
	}
	// with help: each act takes its seconds of work; it acts from then on
	a := b.Clone()
	mk := marks{-1, -1, -1}
	aid := append([]ScenarioAid(nil), s.Aid...)
	sort.SliceStable(aid, func(i, j int) bool { return aid[i].AtS < aid[j].AtS })
	for _, act := range aid {
		at := act.AtS + m.Aid.Kinds[act.Kind].Seconds
		if act.Kind == "pressure" {
			at = act.AtS + m.Aid.Kinds["pressure"].Seconds
		}
		m.observe(&a, t0, at, &mk)
		if len(a.Wounds) == 0 {
			break
		}
		r, err := m.ApplyAid(&a, AidAct{Kind: act.Kind, Wound: a.Wounds[0].ID, Item: act.Item, By: "helper", Seed: seed, Force: true})
		if err != nil {
			return tl, fmt.Errorf("scenario %s aid: %w", s.ID, err)
		}
		tl.Aid = append(tl.Aid, fmt.Sprintf("%s@%d:%s", act.Kind, at, r.Reason))
	}
	after := a.Clone()
	rest := Horizon - (a.Clock - t0)
	m.observe(&a, t0, Horizon, &mk)
	tl.AidedFallS, tl.AidedDownS, tl.AidedDeadS = mk.fall, mk.down, mk.dead
	for _, r := range m.Routes {
		c := after.Clone()
		start := c.Clock - t0
		w, _ := m.WalkTo(&c, r.MM, min(rest, 3600))
		row := WalkRow{Route: r.ID, MM: r.MM, Reached: w.Reached && mk.dead < 0 || w.Reached && mk.dead > start+w.Seconds, AtS: start + max(w.Seconds, w.FellAtS)}
		if !w.Reached && w.FellAtS == 0 {
			row.AtS = 0 // could not get up at all: he has to be carried
		}
		tl.AidedWalk = append(tl.AidedWalk, row)
	}
	// what the capsule asks on arrival from a hero with 10 CRD
	q, err := m.Quote(&after, "capsule", Payer{Chip: "own", BalanceMinor: 1000})
	if err != nil {
		return tl, err
	}
	tl.Capsule = q
	if s.RiskRuns > 0 {
		r := Risk{Runs: s.RiskRuns}
		for i := 0; i < s.RiskRuns; i++ {
			rb, serious, err := m.scenarioBody(s, seed+int64(i)*7919+1, false)
			if err != nil {
				return tl, err
			}
			fc := m.Forecast(rb, Horizon)
			if fc.FallS >= 0 && fc.FallS <= 60 {
				r.FallPPM++
			}
			if fc.DownS >= 0 && fc.DownS <= 60 {
				r.DownPPM++
			}
			if serious {
				r.SeriousPPM++
			}
			if fc.DeadS >= 0 && fc.DeadS <= 600 {
				r.DeadSoonPPM++
			}
			if fc.DeadS >= 0 {
				r.DeadPPM++
			}
		}
		for _, p := range []*int64{&r.FallPPM, &r.DownPPM, &r.SeriousPPM, &r.DeadSoonPPM, &r.DeadPPM} {
			*p = *p * Million / int64(r.Runs)
		}
		tl.Risk = &r
	}
	return tl, nil
}

// Check compares a timeline with the scenario's expectations.
func (s Scenario) Check(tl Timeline) error {
	if !s.Expect.Down.Holds(tl.DownS) {
		return fmt.Errorf("scenario %s: unconscious at %d s, expected %+v", s.ID, tl.DownS, s.Expect.Down)
	}
	if !s.Expect.Dead.Holds(tl.DeadS) {
		return fmt.Errorf("scenario %s: dead at %d s, expected %+v", s.ID, tl.DeadS, s.Expect.Dead)
	}
	if !s.Expect.AidedDead.Holds(tl.AidedDeadS) {
		return fmt.Errorf("scenario %s: with help dead at %d s, expected %+v", s.ID, tl.AidedDeadS, s.Expect.AidedDead)
	}
	return nil
}
