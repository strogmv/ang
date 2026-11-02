package crafting

import "fmt"

// Stage 2 (doc 93 §4): the sources of knowledge run. A schematic comes from a carrier studied by hand (a grey-exchange
// copy that may carry an error, a system manual, a unique find), from hours at a working master who is well disposed,
// from a person who tells it, or from a sample taken apart (stage 1). An error of a copy stays in memory until a clean
// source replaces it; it is applied to every build and becomes visible after the first one.

// Carrier is a schematic as a physical item and where its instances are at the start.
type Carrier struct {
	Item         string       `json:"item"`
	Kind         string       `json:"kind"`
	Teaches      string       `json:"teaches"`
	StudyMinutes int64        `json:"study_minutes"`
	FlawPermille int64        `json:"flaw_permille"`
	Access       []string     `json:"access_castes"`
	Place        CarrierPlace `json:"place"`
}
type CarrierPlace struct {
	Kind  string `json:"kind"`
	Shop  string `json:"shop"`
	Cache string `json:"cache"`
	Store string `json:"store"`
	Owner string `json:"owner"`
	N     int64  `json:"n"`
}
type Flaw struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	QualityDelta int    `json:"quality_delta"`
	RiskPermille int64  `json:"risk_permille"`
}
type Teaching struct {
	Master              string `json:"master"`
	Recipe              string `json:"recipe"`
	MinRelationPermille int64  `json:"min_relation_permille"`
	Hours               int64  `json:"hours"`
	FeeMinor            int64  `json:"fee_minor"`
	GainPermille        int64  `json:"gain_permille"`
}
type Oral struct {
	From                string `json:"from"`
	Recipe              string `json:"recipe"`
	MinRelationPermille int64  `json:"min_relation_permille"`
	Reach               int    `json:"reach"`
}
type CacheThing struct {
	Item         string `json:"item"`
	N            int64  `json:"n"`
	ConditionPPM int64  `json:"condition_ppm"`
	Bound        string `json:"bound"`
}

// Cache is a hidden place of doc 84 turned into an unlocked container at its entry (found by whoever stands there).
type Cache struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Level  string       `json:"level"`
	At     Cell         `json:"at"`
	Store  string       `json:"store,omitempty"`
	Things []CacheThing `json:"things"`
}
type Sources struct {
	Carriers []Carrier  `json:"carriers"`
	Flaws    []Flaw     `json:"flaws"`
	Teaching []Teaching `json:"teaching"`
	Oral     []Oral     `json:"oral"`
	Caches   []Cache    `json:"caches"`
}

func (m Model) Carrier(definition string) (Carrier, bool) {
	for _, c := range m.Sources.Carriers {
		if c.Item == definition {
			return c, true
		}
	}
	return Carrier{}, false
}
func (m Model) Teaching(master, recipe string) (Teaching, bool) {
	for _, t := range m.Sources.Teaching {
		if t.Master == master && t.Recipe == recipe {
			return t, true
		}
	}
	return Teaching{}, false
}
func (m Model) Flaw(kind string) (Flaw, bool) {
	for _, f := range m.Sources.Flaws {
		if f.Kind == kind {
			return f, true
		}
	}
	return Flaw{}, false
}

// CopyFlaw: the error a physical carrier instance carries ("" clean) — a function of the seed and the instance id, so
// the same sheet always teaches the same mistake and two sheets of one copy may differ.
func (m Model) CopyFlaw(seed int64, instance string, c Carrier) string {
	if c.FlawPermille <= 0 || len(m.Sources.Flaws) == 0 {
		return ""
	}
	if int64(roll(seed, instance, "flaw", 0)%1000) >= c.FlawPermille {
		return ""
	}
	return m.Sources.Flaws[roll(seed, instance, "flaw.kind", 0)%uint64(len(m.Sources.Flaws))].Kind
}

// Learn puts a schematic into memory from a source. A clean source replaces an error; a flawed copy never spoils a clean
// memory. It reports whether memory changed.
func (a *ActorState) Learn(recipe, flaw string) bool {
	if a.Known == nil {
		a.Known = map[string]bool{}
	}
	if a.Flaws == nil {
		a.Flaws = map[string]string{}
	}
	_, flawed := a.Flaws[recipe]
	switch {
	case !a.Known[recipe]:
		a.Known[recipe] = true
		if flaw != "" {
			a.Flaws[recipe] = flaw
		}
		return true
	case flawed && flaw == "":
		delete(a.Flaws, recipe)
		delete(a.Revealed, recipe)
		return true
	}
	return false
}

// KnowsClean: the worker can teach it (in memory without an error, or general knowledge of a reached tier).
func (m Model) KnowsClean(w Worker, r Recipe) bool {
	return m.Knows(w, r) && w.Flaws[r.ID] == ""
}

func (m Model) validateSources() error {
	seen := map[string]bool{}
	for _, c := range m.Sources.Carriers {
		r, ok := m.Recipe(c.Teaches)
		if seen[c.Item] || !ok || c.StudyMinutes < 1 || c.FlawPermille < 0 || c.FlawPermille > 1000 || (c.Kind != "copy" && c.Kind != "manual" && c.Kind != "find") || (c.Kind != "copy" && c.FlawPermille != 0) {
			return fmt.Errorf("crafting carrier %s: invalid", c.Item)
		}
		listed := false
		for _, l := range r.Learn {
			listed = listed || l.Kind == c.Kind && l.From == c.Item
		}
		if !listed {
			return fmt.Errorf("crafting carrier %s: recipe %s does not list it as a %s source", c.Item, r.ID, c.Kind)
		}
		switch c.Place.Kind {
		case "shop":
			if c.Place.Shop == "" || c.Place.N < 1 {
				return fmt.Errorf("crafting carrier %s: a shop and a count required", c.Item)
			}
		case "cache":
			found := false
			for _, k := range m.Sources.Caches {
				found = found || k.ID == c.Place.Cache
			}
			if !found {
				return fmt.Errorf("crafting carrier %s: unknown cache %s", c.Item, c.Place.Cache)
			}
		case "store":
			if c.Place.Store == "" || c.Place.Owner == "" || c.Place.N < 1 {
				return fmt.Errorf("crafting carrier %s: a store with an owner required", c.Item)
			}
		default:
			return fmt.Errorf("crafting carrier %s: unknown place %s", c.Item, c.Place.Kind)
		}
		seen[c.Item] = true
	}
	for _, f := range m.Sources.Flaws {
		if f.Name == "" || f.QualityDelta > 0 || f.QualityDelta < -2 || f.RiskPermille < 0 || f.RiskPermille > 500 || (f.QualityDelta == 0) == (f.RiskPermille == 0) {
			return fmt.Errorf("crafting flaw %s: one effect required", f.Kind)
		}
	}
	for _, t := range m.Sources.Teaching {
		ms, ok := m.Master(t.Master)
		r, known := m.Recipe(t.Recipe)
		if !ok || !known || t.Hours < 1 || t.FeeMinor < 0 || t.MinRelationPermille < -1000 || t.MinRelationPermille > 1000 || !m.KnowsClean(m.MasterWorker(ms), r) {
			return fmt.Errorf("crafting teaching %s/%s: master must know the schematic", t.Master, t.Recipe)
		}
	}
	for _, o := range m.Sources.Oral {
		if _, ok := m.Recipe(o.Recipe); !ok || o.From == "" || o.Reach < 1 || o.MinRelationPermille < -1000 || o.MinRelationPermille > 1000 {
			return fmt.Errorf("crafting oral %s/%s: invalid", o.From, o.Recipe)
		}
	}
	// every non-general learn row of a recipe resolves to an executable source
	for _, r := range m.Recipes {
		for _, l := range r.Learn {
			ok := true
			switch l.Kind {
			case "apprentice":
				_, ok = m.Teaching(l.From, r.ID)
			case "copy", "manual", "find":
				c, found := m.Carrier(l.From)
				ok = found && c.Teaches == r.ID && c.Kind == l.Kind
			case "oral":
				ok = false
				for _, o := range m.Sources.Oral {
					ok = ok || o.From == l.From && o.Recipe == r.ID
				}
			case "teardown":
				ok = false
				for _, d := range m.Recipes {
					ok = ok || d.Kind == "disassemble" && d.Teaches == r.ID && d.Target == l.From
				}
			}
			if !ok {
				return fmt.Errorf("crafting recipe %s: learn source %s %s does not run", r.ID, l.Kind, l.From)
			}
		}
	}
	return nil
}
