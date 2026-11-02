package crafting

import (
	"fmt"
	"sort"
)

// Stage 2: crafting executes the EXISTING laws, it has none of its own. The world file binds them (crafting_law.cue):
// law.attribution_basic — evidence weights, decay, report delay, obliged castes, the crowd curve, the known / suspected
// thresholds; the justice ladder — the crime of an act, its rung, reaction and sanctions; ledger.dome — the fine;
// law.price_market — scarcity curve × legality risk (market.go). The host feeds acts and witnesses; this file decides.

type Point struct {
	X int64 `json:"x"`
	Y int64 `json:"y"`
}

// Act is a way an offence is committed or detected and what the ladder makes of it.
type Act struct {
	ID             string   `json:"id"`
	Crime          string   `json:"crime"`
	Name           string   `json:"name"`
	Rung           string   `json:"rung"`
	Rank           int      `json:"rank"`
	Reaction       string   `json:"reaction"`
	Sanctions      []string `json:"sanctions"`
	Evidence       string   `json:"evidence"`
	DetectPermille int64    `json:"detect_permille"`
	WeightPermille int64    `json:"weight_permille"`
}
type Ledger struct {
	BaseMinor      int64  `json:"base_minor"`
	GrowthPermille int64  `json:"growth_permille"`
	CapPermille    int64  `json:"cap_permille"`
	Account        string `json:"account,omitempty"`
}
type Trust struct {
	StartPermille     int64 `json:"start_permille"`
	RebindMinPermille int64 `json:"rebind_min_permille"`
	GainLegalJob      int64 `json:"gain_legal_job"`
	GainCleanDay      int64 `json:"gain_clean_day"`
	LossJournalEntry  int64 `json:"loss_journal_entry"`
}
type Raid struct {
	EveryDays int64  `json:"every_days"`
	Hour      int64  `json:"hour"`
	Level     string `json:"level"`
}

// Terminal is an equipment unit of the vending block that rebinds (filled from the compiled vending block).
type Terminal struct {
	Unit  string `json:"unit"`
	Level string `json:"level"`
	At    Cell   `json:"at"`
}
type Rebind struct {
	Guards          []string   `json:"guards,omitempty"`
	TerminalKind    string     `json:"terminal_kind"`
	PriceMinor      int64      `json:"price_minor"`
	Reach           int        `json:"reach"`
	MisfirePermille int64      `json:"misfire_permille"`
	InjurePermille  int64      `json:"injure_permille"`
	Raid            Raid       `json:"raid"`
	ConfiscateStore string     `json:"confiscate_store"`
	Contraband      []string   `json:"contraband"` // definitions forbidden in the dome the raid takes away
	Terminals       []Terminal `json:"terminals,omitempty"`
	Account         string     `json:"account,omitempty"`
}
type Law struct {
	KnownPermille      int64    `json:"known_permille"`
	SuspectedPermille  int64    `json:"suspected_permille"`
	DecayPPMPerDay     int64    `json:"decay_ppm_per_day"`
	ReportDelaySeconds int64    `json:"report_delay_seconds"`
	Obliged            []string `json:"obliged"`
	Crowd              []Point  `json:"crowd"`
	CrowdPeak          int64    `json:"crowd_peak"`
	WitnessCells       int      `json:"witness_cells"`
	Acts               []Act    `json:"acts"`
	Ledger             Ledger   `json:"ledger"`
	Trust              Trust    `json:"trust"`
	Rebind             Rebind   `json:"rebind"`
}

// Evidence is one piece of a case: its kind and weight (law), when it reached the system and from whom.
type Evidence struct {
	Kind    string `json:"kind"`
	Weight  int64  `json:"weight_permille"`
	Second  int64  `json:"second"`
	Witness string `json:"witness,omitempty"`
}

// Case is what the system knows of one offence of the hero (the act, the item, the evidence and the reports on the way).
type Case struct {
	ID       string     `json:"id"`
	Act      string     `json:"act"`
	Suspect  string     `json:"suspect"`
	Item     string     `json:"item,omitempty"`
	Opened   int64      `json:"opened"`
	Evidence []Evidence `json:"evidence,omitempty"`
	Pending  []Evidence `json:"pending,omitempty"` // reports on their way (Second = when they arrive)
	Closed   bool       `json:"closed,omitempty"`  // known: sanctions applied
}

func (m Model) Act(id string) (Act, bool) {
	for _, a := range m.Law.Acts {
		if a.ID == id {
			return a, true
		}
	}
	return Act{}, false
}

// Curve evaluates a piecewise-linear curve of permille points (clamped at the ends).
func Curve(points []Point, x int64) int64 {
	if len(points) == 0 {
		return 1000
	}
	if x <= points[0].X {
		return points[0].Y
	}
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		if x <= b.X {
			if b.X == a.X {
				return b.Y
			}
			return a.Y + (b.Y-a.Y)*(x-a.X)/(b.X-a.X)
		}
	}
	return points[len(points)-1].Y
}

// Belief of the system that the suspect did it (permille): 1 − Π(1 − weight·(1 − decay)^days).
func (m Model) Belief(c Case, now int64) int64 {
	miss := int64(Million)
	for _, e := range c.Evidence {
		w := e.Weight * 1000
		for d := int64(0); d < (now-e.Second)/86400 && d < 400; d++ {
			w = w * (Million - m.Law.DecayPPMPerDay) / Million
		}
		miss = miss * (Million - clamp(w, 0, Million)) / Million
	}
	return (Million - miss) / 1000
}

// Status: "open" (below suspicion), "suspected", "known".
func (m Model) Status(c Case, now int64) string {
	b := m.Belief(c, now)
	switch {
	case b >= m.Law.KnownPermille:
		return "known"
	case b >= m.Law.SuspectedPermille:
		return "suspected"
	}
	return "open"
}

// Reports decides which witnesses report an act and when: the obliged (the castes of the law, the guards) always and
// at once; everybody else by the law's crowd curve at the density of the crowd, after the law's report delay.
func (m Model) Reports(seed int64, caseID string, act Act, witnesses []string, now int64) []Evidence {
	sorted := append([]string(nil), witnesses...)
	sort.Strings(sorted)
	obliged := map[string]bool{}
	for _, id := range m.Law.Obliged {
		obliged[id] = true
	}
	x := int64(len(sorted)) * 1000 / max(1, m.Law.CrowdPeak)
	chance := Curve(m.Law.Crowd, x)
	out := []Evidence{}
	for i, w := range sorted {
		if obliged[w] {
			out = append(out, Evidence{Kind: act.Evidence, Weight: act.WeightPermille, Second: now, Witness: w})
			continue
		}
		if int64(roll(seed, caseID, "report:"+w, i)%1000) < chance*act.DetectPermille/1000 {
			out = append(out, Evidence{Kind: act.Evidence, Weight: act.WeightPermille, Second: now + m.Law.ReportDelaySeconds, Witness: w})
		}
	}
	return out
}

// Fine of the n-th offence (ledger.dome: base × growth^(n−1), capped at base × cap).
func (m Model) Fine(offences int64) int64 {
	l := m.Law.Ledger
	f := l.BaseMinor
	for i := int64(1); i < offences; i++ {
		f = f * l.GrowthPermille / 1000
	}
	return min(f, l.BaseMinor*l.CapPermille/1000)
}

// Chip: the chip id of an actor as a binding reads it.
func Chip(actor string) string { return "chip:" + actor }

func (m Model) validateLaw() error {
	l := m.Law
	if len(l.Acts) == 0 {
		return nil
	}
	if l.KnownPermille <= l.SuspectedPermille || l.KnownPermille > 1000 || l.DecayPPMPerDay < 0 || l.DecayPPMPerDay >= Million || l.ReportDelaySeconds < 0 || l.CrowdPeak < 1 || l.WitnessCells < 1 || len(l.Crowd) < 2 {
		return fmt.Errorf("crafting law: thresholds, decay, delay, crowd invalid")
	}
	seen := map[string]bool{}
	for _, a := range l.Acts {
		if seen[a.ID] || a.Crime == "" || a.Rung == "" || a.Rank < 1 || a.WeightPermille < 1 || a.WeightPermille > 1000 || a.DetectPermille < 0 || a.DetectPermille > 1000 || len(a.Sanctions) == 0 {
			return fmt.Errorf("crafting law: act %s has no crime, rung or evidence weight", a.ID)
		}
		seen[a.ID] = true
	}
	for _, id := range []string{"hack", "carry", "scan", "raid", "theft"} {
		if !seen[id] {
			return fmt.Errorf("crafting law: act %s missing", id)
		}
	}
	if l.Ledger.BaseMinor < 1 || l.Ledger.GrowthPermille < 1000 || l.Ledger.CapPermille < 1000 {
		return fmt.Errorf("crafting law: ledger invalid")
	}
	t := l.Trust
	if t.StartPermille < 0 || t.StartPermille > 1000 || t.RebindMinPermille <= t.StartPermille || t.RebindMinPermille > 1000 {
		return fmt.Errorf("crafting law: the rebinding threshold must be above the starting trust")
	}
	r := l.Rebind
	if r.PriceMinor < 1 || r.Reach < 1 || r.MisfirePermille < 0 || r.InjurePermille < 0 || r.MisfirePermille+r.InjurePermille > 1000 || r.Raid.EveryDays < 1 || r.Raid.Hour < 0 || r.Raid.Hour > 23 || r.Raid.Level == "" || r.ConfiscateStore == "" {
		return fmt.Errorf("crafting law: rebinding data invalid")
	}
	return nil
}
