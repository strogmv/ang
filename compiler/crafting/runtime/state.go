package crafting

import (
	"fmt"
	"sort"
	"strings"
)

// ActorState is what a worker has learnt: discipline points (milli) and explicitly known recipes.
type ActorState struct {
	Points map[string]int64 `json:"points"`
	Known  map[string]bool  `json:"known"`
	Jobs   int64            `json:"jobs"`
	// stage 2: errors of copied schematics in memory and those already seen in a build
	Flaws    map[string]string `json:"flaws,omitempty"`
	Revealed map[string]bool   `json:"revealed,omitempty"`
}

// Job is a planned job in the world: the parts and the target wait in Store while it runs; an order (Client set) ends in
// Results waiting at the master's counter until the client collects them.
type Job struct {
	ID        string   `json:"id"`
	Worker    string   `json:"worker"`
	Client    string   `json:"client,omitempty"`
	Plan      Plan     `json:"plan"`
	Store     string   `json:"store"`
	Start     int64    `json:"start"`
	Progress  int64    `json:"progress"`
	Due       int64    `json:"due"`
	Done      bool     `json:"done"`
	Results   []string `json:"results,omitempty"`
	PaidMinor int64    `json:"paid_minor,omitempty"`
	Refunded  bool     `json:"refunded,omitempty"`
	Swap      Swap     `json:"swap,omitzero"` // stage 2: a master's substitution in this order (Class "" none)
}

// Entry is a line of the repair journal of an heirloom ("носил ещё дед"): who, what, how it ended.
type Entry struct {
	Second    int64  `json:"second"`
	Worker    string `json:"worker"`
	Recipe    string `json:"recipe"`
	Outcome   string `json:"outcome"`
	Quality   int    `json:"quality"`
	Condition int64  `json:"condition"`
	Swap      string `json:"swap,omitempty"` // stage 2: the class a master swapped for a handmade part
}
type Notice struct {
	Key     string `json:"key"`
	Recipe  string `json:"recipe,omitempty"`
	Outcome string `json:"outcome,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Second  int64  `json:"second"`
}
type State struct {
	Actors  map[string]ActorState `json:"actors"`
	Jobs    []Job                 `json:"jobs,omitempty"`
	Serial  uint64                `json:"serial"`
	Journal map[string][]Entry    `json:"journal,omitempty"`
	Notice  Notice                `json:"notice"`
	// stage 2: the hero before the law (cases, trust, hacked things, the system journal) and the market (swaps, clocks)
	Law    LawState    `json:"law"`
	Market MarketState `json:"market"`
}

// NewState: the hero and every master with their starting points and memory.
func (m Model) NewState() State {
	s := State{Actors: map[string]ActorState{}, Law: LawState{Trust: m.Law.Trust.StartPermille}}
	hero := ActorState{Points: map[string]int64{}, Known: map[string]bool{}}
	for d, p := range m.Hero.Points {
		hero.Points[d] = p * 1000
	}
	for _, id := range m.Hero.Known {
		hero.Known[id] = true
	}
	s.Actors[m.Hero.Actor] = hero
	for _, ms := range m.Masters {
		w := m.MasterWorker(ms)
		s.Actors[ms.Actor] = ActorState{Points: w.Points, Known: w.Known}
	}
	return s
}
func (a ActorState) Worker(id string) Worker {
	return Worker{ID: id, Points: a.Points, Known: a.Known, Flaws: a.Flaws}
}
func (s State) Clone() State {
	out := State{Serial: s.Serial, Notice: s.Notice, Actors: map[string]ActorState{}}
	for id, a := range s.Actors {
		c := ActorState{Jobs: a.Jobs, Points: map[string]int64{}, Known: map[string]bool{}}
		for k, v := range a.Points {
			c.Points[k] = v
		}
		for k, v := range a.Known {
			c.Known[k] = v
		}
		if a.Flaws != nil {
			c.Flaws = map[string]string{}
			for k, v := range a.Flaws {
				c.Flaws[k] = v
			}
		}
		if a.Revealed != nil {
			c.Revealed = map[string]bool{}
			for k, v := range a.Revealed {
				c.Revealed[k] = v
			}
		}
		out.Actors[id] = c
	}
	for _, j := range s.Jobs {
		j.Results = append([]string(nil), j.Results...)
		j.Plan.Uses = append([]Use(nil), j.Plan.Uses...)
		j.Plan.Spoil = append([]Use(nil), j.Plan.Spoil...)
		j.Plan.Makes = append([]Made(nil), j.Plan.Makes...)
		j.Plan.Wear = append([]Wear(nil), j.Plan.Wear...)
		out.Jobs = append(out.Jobs, j)
	}
	if s.Journal != nil {
		out.Journal = map[string][]Entry{}
		for k, v := range s.Journal {
			out.Journal[k] = append([]Entry(nil), v...)
		}
	}
	out.Law, out.Market = s.Law.clone(), s.Market.clone()
	return out
}

// JobOf: the index of the unfinished own job of a worker (-1 none). A worker does one job at a time.
func (s State) JobOf(worker string) int {
	for i, j := range s.Jobs {
		if j.Worker == worker && !j.Done {
			return i
		}
	}
	return -1
}

// Validate checks a saved state against the catalog (custody is checked by the host against the inventory).
func (m Model) ValidateState(s State) error {
	if s.Actors == nil {
		return fmt.Errorf("crafting state: no actors")
	}
	workers := map[string]bool{m.Hero.Actor: true}
	for _, ms := range m.Masters {
		workers[ms.Actor] = true
	}
	ids := make([]string, 0, len(s.Actors))
	for id := range s.Actors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !workers[id] {
			return fmt.Errorf("crafting state: unknown worker %s", id)
		}
		for d, p := range s.Actors[id].Points {
			if _, ok := m.Discipline(d); !ok || p < 0 || p > m.Tuning.CapPoints*1000 {
				return fmt.Errorf("crafting state: invalid points %s/%s", id, d)
			}
		}
		for r := range s.Actors[id].Known {
			if _, ok := m.Recipe(r); !ok {
				return fmt.Errorf("crafting state: unknown recipe %s", r)
			}
		}
	}
	seen := map[string]bool{}
	open := map[string]int{}
	for _, j := range s.Jobs {
		r, ok := m.Recipe(j.Plan.Recipe)
		learning := j.Plan.Kind == "study" || j.Plan.Kind == "apprentice"
		if !ok || seen[j.ID] || j.ID == "" || !workers[j.Worker] || j.Plan.Worker != j.Worker || (j.Plan.Kind != r.Kind && !learning) || j.Store == "" || j.Progress < 0 || j.Progress > j.Plan.Seconds || (j.Plan.Seconds != r.Minutes*60 && !learning) || (learning && (j.Client != "" || j.Plan.Seconds < 60)) {
			return fmt.Errorf("crafting state: invalid job %s", j.ID)
		}
		// an order may be the hero's or a resident's (stage 2: NPCs order repairs by the same law)
		if j.Client == "" && j.Worker != m.Hero.Actor || j.Client != "" && !strings.HasPrefix(j.Client, "char.") {
			return fmt.Errorf("crafting state: job %s has an invalid client", j.ID)
		}
		if j.Done && j.Client == "" {
			return fmt.Errorf("crafting state: finished own job %s kept", j.ID)
		}
		if !j.Done {
			open[j.Worker]++
			if j.Client == "" && open[j.Worker] > 1 {
				return fmt.Errorf("crafting state: the hero runs two jobs")
			}
		}
		seen[j.ID] = true
	}
	return m.validateLawState(s)
}
