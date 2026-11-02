package crafting

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"sort"
)

// Refusal is a rule's "no": a localization key and the id it is about (part class, tool kind, station kind...).
type Refusal struct {
	Key    string `json:"key"`
	Detail string `json:"detail,omitempty"`
}

func (r Refusal) Error() string { return r.Key + " " + r.Detail }

// Stack is one inventory instance as the engine sees it. Quality 0 / Origin "" mean "as the class default".
type Stack struct {
	ID         string
	Definition string
	Quantity   int64
	Condition  int64
	Quality    int
	Origin     string
	Bound      string
}

// Worker is the knowledge of whoever does the job: discipline points (milli) and explicitly known recipes.
type Worker struct {
	ID     string
	Points map[string]int64
	Known  map[string]bool
	Flaws  map[string]string // stage 2: errors of copied schematics in memory (sources.go)
}

// Bench is where the job is done: the station the worker stands at, if any, and whether its owner lets it be used now.
type Bench struct {
	Station string
	Present bool
}
type Request struct {
	Recipe string
	Target string
	JobID  string
	Seed   int64
}

// Use is a quantity of parts taken from one stack (its grade kept for the quality and for the return on failure).
type Use struct {
	Stack      string `json:"stack"`
	Definition string `json:"definition"`
	Quantity   int64  `json:"quantity"`
	Quality    int    `json:"quality"`
	Origin     string `json:"origin"`
}
type Made struct {
	Definition string `json:"definition"`
	Quantity   int64  `json:"quantity"`
	Quality    int    `json:"quality"`
	Origin     string `json:"origin"`
	Condition  int64  `json:"condition"`
}
type Wear struct {
	Stack string `json:"stack"`
	PPM   int64  `json:"ppm"`
}

// Plan is a fully decided job: what is used, how long it takes and how it ends (rolled once from the seed and the job id).
type Plan struct {
	Recipe       string `json:"recipe"`
	Kind         string `json:"kind"`
	Worker       string `json:"worker"`
	Target       string `json:"target,omitempty"`
	Uses         []Use  `json:"uses,omitempty"`
	Wear         []Wear `json:"wear,omitempty"`
	Station      string `json:"station,omitempty"`
	Seconds      int64  `json:"seconds"`
	Unattended   bool   `json:"unattended,omitempty"`
	Quality      int    `json:"quality"`
	RiskPermille int64  `json:"risk_permille"`
	Outcome      string `json:"outcome"`
	Spoil        []Use  `json:"spoil,omitempty"`
	Condition    int64  `json:"condition"`
	Makes        []Made `json:"makes,omitempty"`
	Learn        string `json:"learn,omitempty"`
	GainMilli    int64  `json:"gain_milli"`
	RentMinor    int64  `json:"rent_minor,omitempty"`
	Flaw         string `json:"flaw,omitempty"` // stage 2: the copy error this build carries
}

func roll(seed int64, job, stream string, index int) uint64 {
	b, _ := json.Marshal([]any{seed, job, stream, index})
	h := sha256.Sum256(b)
	return binary.LittleEndian.Uint64(h[:8])
}

// TierOf: the tier the worker's points reach in a discipline (1..5).
func (m Model) TierOf(w Worker, discipline string) int {
	d, ok := m.Discipline(discipline)
	if !ok {
		return 0
	}
	points := w.Points[discipline]
	tier := 1
	for _, t := range d.Tiers {
		if points >= t.Points*1000 {
			tier = t.Tier
		}
	}
	return tier
}
func (m Model) threshold(discipline string, tier int) int64 {
	d, _ := m.Discipline(discipline)
	if tier < 1 || tier > len(d.Tiers) {
		return 0
	}
	return d.Tiers[tier-1].Points
}

// Knows: the recipe is in the worker's memory, or it is general knowledge of a tier the worker has reached.
func (m Model) Knows(w Worker, r Recipe) bool {
	if w.Known[r.ID] {
		return true
	}
	for _, l := range r.Learn {
		if l.Kind == "tier" && m.TierOf(w, r.Discipline) >= r.Tier {
			return true
		}
	}
	return false
}

// Open: the worker knows of the recipe — it is in memory, or it is general knowledge that a tier will open.
func (m Model) Open(w Worker, r Recipe) bool {
	if w.Known[r.ID] {
		return true
	}
	for _, l := range r.Learn {
		if l.Kind == "tier" {
			return true
		}
	}
	return false
}
func (m Model) MasterWorker(s Master) Worker {
	w := Worker{ID: s.Actor, Points: map[string]int64{}, Known: map[string]bool{}}
	for d, p := range s.Points {
		w.Points[d] = p * 1000
	}
	for _, id := range s.Known {
		w.Known[id] = true
	}
	return w
}

// grade: the effective quality and provenance of a stack (its own grade, else the class default).
func (m Model) grade(s Stack) (int, string) {
	q, o := s.Quality, s.Origin
	if _, item, ok := m.PartOf(s.Definition); ok {
		if q == 0 {
			q = item.Quality
		}
		if o == "" {
			o = item.Origin
		}
	}
	if q == 0 {
		q = 3
	}
	if o == "" {
		o = "handmade"
	}
	return q, o
}
func (m Model) matches(in Input, s Stack) bool {
	if in.Item != "" {
		return s.Definition == in.Item
	}
	p, _, ok := m.PartOf(s.Definition)
	return ok && p.ID == in.Class
}
func clamp(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Plan decides a job or refuses it. stacks are the parts and tools the worker can use (carried, or the master's stock);
// target is the item worked on (repair, maintenance, disassembly).
func (m Model) Plan(w Worker, q Request, stacks []Stack, target *Stack, bench Bench) (Plan, error) {
	r, ok := m.Recipe(q.Recipe)
	if !ok {
		return Plan{}, Refusal{Key: "crafting.error.unknown"}
	}
	if !m.Open(w, r) {
		return Plan{}, Refusal{Key: "crafting.error.unknown_schematic", Detail: r.ID}
	}
	if m.TierOf(w, r.Discipline) < r.Tier {
		return Plan{}, Refusal{Key: "crafting.error.tier", Detail: r.Discipline}
	}
	p := Plan{Recipe: r.ID, Kind: r.Kind, Worker: w.ID, Seconds: r.Minutes * 60, Unattended: r.Unattended}
	if r.Kind != "craft" {
		if target == nil || target.Definition != r.Target || target.Quantity < 1 {
			return Plan{}, Refusal{Key: "crafting.error.no_target", Detail: r.Target}
		}
		if (r.Kind == "repair" || r.Kind == "maintain") && target.Condition >= Million {
			return Plan{}, Refusal{Key: "crafting.error.intact", Detail: r.Target}
		}
		if r.Kind == "bypass" && (target.Bound == "" || target.Bound == Chip(w.ID)) {
			return Plan{}, Refusal{Key: "crafting.error.not_bound", Detail: r.Target}
		}
		p.Target = target.ID
	}
	// the bench: a required station kind, and the tools it lends
	var station Station
	if bench.Station != "" {
		station, _ = m.Station(bench.Station)
	}
	if r.Station != "" && (station.Kind != r.Station || !bench.Present) {
		if station.Kind == r.Station {
			return Plan{}, Refusal{Key: "crafting.error.station_closed", Detail: r.Station}
		}
		return Plan{}, Refusal{Key: "crafting.error.no_station", Detail: r.Station}
	}
	usesStation := r.Station != "" && station.Kind == r.Station
	excluded := map[string]bool{p.Target: true}
	toolQ := int64(3)
	if len(r.Tools) > 0 {
		toolQ = 5
	}
	for _, kind := range r.Tools {
		best, from, tool := 0, "", ToolDef{}
		if bench.Present {
			for _, t := range station.Tools {
				if t.Kind == kind && t.Quality > best {
					best, from = t.Quality, ""
				}
			}
		}
		ids := make([]Stack, 0, len(stacks))
		ids = append(ids, stacks...)
		sort.Slice(ids, func(i, j int) bool { return ids[i].ID < ids[j].ID })
		for _, s := range ids {
			// a tool's own grade (the grandfather's prewar screwdrivers) beats the quality of its definition
			if t, ok := m.Tool(s.Definition); ok && t.Kind == kind && s.Condition > 0 {
				q := t.Quality
				if s.Quality > 0 {
					q = s.Quality
				}
				if q > best {
					best, from, tool = q, s.ID, t
				}
			}
		}
		if best == 0 {
			return Plan{}, Refusal{Key: "crafting.error.no_tool", Detail: kind}
		}
		if from == "" {
			usesStation = true
		} else {
			excluded[from] = true
			if tool.WearPPM > 0 {
				p.Wear = append(p.Wear, Wear{Stack: from, PPM: tool.WearPPM})
			}
		}
		toolQ = min(toolQ, int64(best))
	}
	if usesStation {
		if !bench.Present {
			return Plan{}, Refusal{Key: "crafting.error.station_closed", Detail: station.Kind}
		}
		p.Station = station.ID
		if station.Owner != "" && station.Owner != w.ID {
			p.RentMinor = station.RentMinorPerHour * ((r.Minutes + 59) / 60)
		}
	}
	// parts: the best quality first (then id), never the target or a tool in use
	var sumQ, units int64
	taken := map[string]int64{}
	for _, in := range r.Inputs {
		cands := []Stack{}
		for _, s := range stacks {
			if excluded[s.ID] || !m.matches(in, s) {
				continue
			}
			if q, _ := m.grade(s); q >= in.MinQuality && s.Quantity-taken[s.ID] > 0 {
				cands = append(cands, s)
			}
		}
		sort.Slice(cands, func(i, j int) bool {
			qi, _ := m.grade(cands[i])
			qj, _ := m.grade(cands[j])
			if qi != qj {
				return qi > qj
			}
			return cands[i].ID < cands[j].ID
		})
		need := in.N
		for _, s := range cands {
			if need == 0 {
				break
			}
			n := min(need, s.Quantity-taken[s.ID])
			q, o := m.grade(s)
			taken[s.ID] += n
			need -= n
			p.Uses = append(p.Uses, Use{Stack: s.ID, Definition: s.Definition, Quantity: n, Quality: q, Origin: o})
			sumQ += int64(q) * n
			units += n
		}
		if need > 0 {
			detail := in.Class
			if detail == "" {
				detail = in.Item
			}
			return Plan{}, Refusal{Key: "crafting.error.no_parts", Detail: detail}
		}
	}
	// quality and risk: the parts, the margin of skill over the tier, the tools and the station
	t := m.Tuning
	partQ := int64(300)
	if units > 0 {
		partQ = sumQ * 100 / units
	}
	margin := w.Points[r.Discipline]/1000 - m.threshold(r.Discipline, r.Tier)
	q100 := int64(r.Base)*100 + (partQ-300)*t.PartWeight/100 + margin*t.SkillWeight + (toolQ-3)*t.ToolWeight
	if p.Station != "" {
		q100 += int64(station.QualityBonus) * 100
	}
	p.Quality = int(clamp((q100+50)/100, 1, 5))
	p.RiskPermille = clamp(r.RiskPermille-margin*t.RiskPerPoint-(toolQ-3)*t.RiskPerTool, t.MinRisk, t.MaxRisk)
	// stage 2: a schematic learnt from a flawed copy builds worse or spoils more until a clean source replaces it
	if f, ok := m.Flaw(w.Flaws[r.ID]); ok {
		p.Flaw = f.Kind
		p.Quality = int(clamp(int64(p.Quality+f.QualityDelta), 1, 5))
		p.RiskPermille = clamp(p.RiskPermille+f.RiskPermille, t.MinRisk, t.MaxRisk)
	}
	dice := int64(roll(q.Seed, q.JobID, "outcome", 0) % 1000)
	switch {
	case dice < p.RiskPermille*t.CriticalPermille/1000:
		p.Outcome = "critical"
	case dice < p.RiskPermille:
		p.Outcome = "failure"
	default:
		p.Outcome = "success"
	}
	// what a failure spoils: one unit of the parts (all of them on a critical failure of a schematic)
	if p.Outcome != "success" && units > 0 {
		if p.Outcome == "critical" && r.Kind == "craft" {
			p.Spoil = append(p.Spoil, p.Uses...)
		} else {
			k := int64(roll(q.Seed, q.JobID, "spoil", 0) % uint64(units))
			for _, u := range p.Uses {
				if k < u.Quantity {
					u.Quantity = 1
					p.Spoil = append(p.Spoil, u)
					break
				}
				k -= u.Quantity
			}
		}
	}
	gain := t.GainMilli[r.Kind] * int64(r.Tier)
	switch r.Kind {
	case "craft":
		if p.Outcome == "success" {
			p.Makes = []Made{{Definition: r.Output.Item, Quantity: r.Output.N, Quality: p.Quality, Origin: r.Output.Origin, Condition: Million}}
		}
	case "repair":
		p.Condition = target.Condition
		if p.Outcome == "success" {
			p.Condition = max(target.Condition, clamp(r.RestorePPM+int64(p.Quality-3)*t.RestorePerQuality, 1, Million))
		}
		if p.Outcome == "critical" {
			p.Condition = max(0, target.Condition-t.CriticalDamagePPM)
		}
	case "bypass":
		p.Condition = target.Condition
		if p.Outcome == "critical" {
			p.Condition = max(0, target.Condition-t.CriticalDamagePPM)
		}
	case "maintain":
		p.Condition = target.Condition
		if p.Outcome == "success" {
			p.Condition = clamp(target.Condition+r.RestorePPM+int64(p.Quality-3)*t.RestorePerQuality/4, 0, Million)
		}
	case "disassemble":
		survive := clamp(t.SurvivePermille+margin*t.SurvivePerPoint+(toolQ-3)*t.RiskPerTool, 50, 1000)
		if p.Outcome == "critical" {
			survive /= 2
		}
		worn := 0
		if target.Condition < t.WornBelowPPM {
			worn = 1
		}
		for i, y := range r.Yields {
			n := y.Min
			if y.Max > y.Min {
				n += int64(roll(q.Seed, q.JobID, "yield", i) % uint64(y.Max-y.Min+1))
			}
			kept := int64(0)
			for u := int64(0); u < n; u++ {
				if int64(roll(q.Seed, q.JobID, "survive", i*100+int(u))%1000) < survive {
					kept++
				}
			}
			if kept > 0 {
				class, _ := m.Part(y.Class)
				p.Makes = append(p.Makes, Made{Definition: class.Items[0].Item, Quantity: kept, Quality: int(clamp(int64(y.Quality-worn), 1, 5)), Origin: y.Origin, Condition: Million})
			}
		}
		if p.Outcome == "success" && r.Teaches != "" && !w.Known[r.Teaches] && int64(roll(q.Seed, q.JobID, "teach", 0)%1000) < r.TeachPermille {
			p.Learn = r.Teaches
		}
	}
	if p.Outcome != "success" {
		gain = gain * t.FailureGainPermille / 1000
	}
	p.GainMilli = gain
	return p, nil
}

// OrderPrice: what a master asks for a job — the parts at their price and the labour by the hour (rounded up).
func (m Model) OrderPrice(s Master, p Plan) int64 {
	var n int64
	for _, u := range p.Uses {
		n += u.Quantity * m.Prices[u.Definition]
	}
	return n + (s.LaborMinorPerHour*p.Seconds+3599)/3600
}

// Award applies a finished job to the worker: points (up to the cap) and a learnt schematic.
func (m Model) Award(a *ActorState, discipline string, p Plan) {
	if a.Points == nil {
		a.Points = map[string]int64{}
	}
	if a.Known == nil {
		a.Known = map[string]bool{}
	}
	a.Points[discipline] = min(a.Points[discipline]+p.GainMilli, m.Tuning.CapPoints*1000)
	if p.Learn != "" {
		a.Known[p.Learn] = true
	}
	a.Jobs++
}
