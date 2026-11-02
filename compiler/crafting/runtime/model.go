// Package crafting is the rule engine of repair, disassembly, maintenance and crafting by schematics (doc 93). It is
// deterministic and owns no custody: a job is planned from the worker's knowledge, the parts and tools at hand and the
// bench, and the host turns the plan into one atomic inventory transform. The hero and the NPC masters obey the same law.
package crafting

import (
	"fmt"
	"regexp"
	"strings"
)

const Million = 1000000

type Cell struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type SourceRef struct {
	Path   string `json:"path"`
	Anchor string `json:"anchor,omitempty"`
	Note   string `json:"note,omitempty"`
}

// PartItem is an inventory definition counted as a part class, with the grade of an ungraded instance.
type PartItem struct {
	Item    string `json:"item"`
	Origin  string `json:"origin"`
	Quality int    `json:"quality"`
}

// PartClass groups the definitions a schematic can use; Items[0] is what salvage of the class produces.
type PartClass struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Items []PartItem `json:"items"`
}
type Tier struct {
	Tier   int    `json:"tier"`
	Points int64  `json:"points"`
	Name   string `json:"name"`
}
type Discipline struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Skills []string `json:"skills"`
	Tiers  []Tier   `json:"tiers"`
}
type ToolDef struct {
	Item    string `json:"item"`
	Kind    string `json:"kind"`
	Quality int    `json:"quality"`
	WearPPM int64  `json:"wear_ppm"`
}
type ToolUse struct {
	Kind    string `json:"kind"`
	Quality int    `json:"quality"`
}

// Station is a workplace: a kind, the tools it lends and, when it belongs to a master, the rent per hour.
type Station struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Kind             string    `json:"kind"`
	Owner            string    `json:"owner,omitempty"`
	Shop             string    `json:"shop,omitempty"`
	Level            string    `json:"level"`
	At               Cell      `json:"at"`
	Reach            int       `json:"reach"`
	Tools            []ToolUse `json:"tools"`
	RentMinorPerHour int64     `json:"rent_minor_per_hour"`
	QualityBonus     int       `json:"quality_bonus"`
	Account          string    `json:"account,omitempty"`
}
type Input struct {
	Class      string `json:"class,omitempty"`
	Item       string `json:"item,omitempty"`
	N          int64  `json:"n"`
	MinQuality int    `json:"min_quality"`
}
type Yield struct {
	Class   string `json:"class"`
	Min     int64  `json:"min"`
	Max     int64  `json:"max"`
	Quality int    `json:"quality"`
	Origin  string `json:"origin"`
}
type Output struct {
	Item   string `json:"item"`
	N      int64  `json:"n"`
	Origin string `json:"origin"`
}

// Learn is one way to obtain a recipe (doc 93 §4): "tier" — general knowledge opened by the tier, "start" — known at the
// start, "teardown" — learnt by taking a sample apart; apprentice / copy / manual / oral / find are data of stage 2.
type Learn struct {
	Kind string `json:"kind"`
	From string `json:"from,omitempty"`
	Note string `json:"note,omitempty"`
}
type Recipe struct {
	ID            string      `json:"id"`
	Kind          string      `json:"kind"`
	Name          string      `json:"name"`
	Hint          string      `json:"hint"`
	Discipline    string      `json:"discipline"`
	Tier          int         `json:"tier"`
	Target        string      `json:"target,omitempty"`
	Inputs        []Input     `json:"inputs"`
	Tools         []string    `json:"tools"`
	Station       string      `json:"station,omitempty"`
	Minutes       int64       `json:"minutes"`
	Unattended    bool        `json:"unattended"`
	Base          int         `json:"base"`
	RestorePPM    int64       `json:"restore_ppm"`
	RiskPermille  int64       `json:"risk_permille"`
	Legal         string      `json:"legal"`
	Basis         string      `json:"basis"`
	Output        *Output     `json:"output,omitempty"`
	Yields        []Yield     `json:"yields,omitempty"`
	Teaches       string      `json:"teaches,omitempty"`
	TeachPermille int64       `json:"teach_permille,omitempty"`
	Learn         []Learn     `json:"learn"`
	Heirloom      bool        `json:"heirloom,omitempty"`
	Sources       []SourceRef `json:"sources"`
}

// Master is an NPC who takes orders at the own counter and works them by the same recipes, parts and law as the hero.
type Master struct {
	Actor             string           `json:"actor"`
	Name              string           `json:"name"`
	Shop              string           `json:"shop"`
	Station           string           `json:"station"`
	Level             string           `json:"level"`
	Approach          Cell             `json:"approach"`
	Reach             int              `json:"reach"`
	Points            map[string]int64 `json:"points"`
	Known             []string         `json:"known"`
	Offers            []string         `json:"offers"`
	LaborMinorPerHour int64            `json:"labor_minor_per_hour"`
	WorkAction        string           `json:"work_action,omitempty"`
	Stores            []string         `json:"stores"`
	OrderStore        string           `json:"order_store"`
	Account           string           `json:"account"`
}
type Hero struct {
	Actor     string           `json:"actor"`
	Points    map[string]int64 `json:"points"`
	Known     []string         `json:"known"`
	WorkStore string           `json:"work_store"`
	Account   string           `json:"account"`
}
type Knowledge struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Executable bool   `json:"executable"`
}
type Tuning struct {
	GainMilli           map[string]int64 `json:"gain_milli"`
	FailureGainPermille int64            `json:"failure_gain_permille"`
	CapPoints           int64            `json:"cap_points"`
	PartWeight          int64            `json:"part_weight"`
	SkillWeight         int64            `json:"skill_weight"`
	ToolWeight          int64            `json:"tool_weight"`
	RiskPerPoint        int64            `json:"risk_per_point"`
	RiskPerTool         int64            `json:"risk_per_tool"`
	MinRisk             int64            `json:"min_risk"`
	MaxRisk             int64            `json:"max_risk"`
	CriticalPermille    int64            `json:"critical_permille"`
	RestorePerQuality   int64            `json:"restore_per_quality"`
	CriticalDamagePPM   int64            `json:"critical_damage_ppm"`
	SurvivePermille     int64            `json:"survive_permille"`
	SurvivePerPoint     int64            `json:"survive_per_point"`
	WornBelowPPM        int64            `json:"worn_below_ppm"`
}

type Model struct {
	Parts       []PartClass      `json:"parts"`
	Disciplines []Discipline     `json:"disciplines"`
	Tools       []ToolDef        `json:"tools"`
	Stations    []Station        `json:"stations"`
	Recipes     []Recipe         `json:"recipes"`
	Masters     []Master         `json:"masters"`
	Hero        Hero             `json:"hero"`
	Knowledge   []Knowledge      `json:"knowledge"`
	Tuning      Tuning           `json:"tuning"`
	Prices      map[string]int64 `json:"prices"`
	Level       string           `json:"level"`
	Scenarios   []Scenario       `json:"scenarios,omitempty"`
	// stage 2 (sources.go, law.go, market.go)
	Sources Sources `json:"sources"`
	Law     Law     `json:"law"`
	Economy Economy `json:"economy"`
	Market  Market  `json:"market"`
}

var stableID = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)*$`)
var kinds = map[string]bool{"craft": true, "repair": true, "maintain": true, "disassemble": true, "bypass": true}
var origins = map[string]bool{"heirloom": true, "system": true, "handmade": true}
var learnKinds = map[string]bool{"tier": true, "start": true, "teardown": true, "apprentice": true, "copy": true, "manual": true, "oral": true, "find": true}

func (m Model) Discipline(id string) (Discipline, bool) {
	for _, d := range m.Disciplines {
		if d.ID == id {
			return d, true
		}
	}
	return Discipline{}, false
}
func (m Model) Recipe(id string) (Recipe, bool) {
	for _, r := range m.Recipes {
		if r.ID == id {
			return r, true
		}
	}
	return Recipe{}, false
}
func (m Model) Station(id string) (Station, bool) {
	for _, s := range m.Stations {
		if s.ID == id {
			return s, true
		}
	}
	return Station{}, false
}
func (m Model) Master(actor string) (Master, bool) {
	for _, s := range m.Masters {
		if s.Actor == actor {
			return s, true
		}
	}
	return Master{}, false
}
func (m Model) Part(id string) (PartClass, bool) {
	for _, p := range m.Parts {
		if p.ID == id {
			return p, true
		}
	}
	return PartClass{}, false
}

// PartOf: the class and the default grade of a definition, if it is a part.
func (m Model) PartOf(definition string) (PartClass, PartItem, bool) {
	for _, p := range m.Parts {
		for _, i := range p.Items {
			if i.Item == definition {
				return p, i, true
			}
		}
	}
	return PartClass{}, PartItem{}, false
}
func (m Model) Tool(definition string) (ToolDef, bool) {
	for _, t := range m.Tools {
		if t.Item == definition {
			return t, true
		}
	}
	return ToolDef{}, false
}

// Validate checks the structure of the catalog. Content rules that need the compiled world (reachability, prices,
// localization, canon sources, no copying) are ANG build checks.
func (m Model) Validate() error {
	seen := map[string]bool{}
	unique := func(kind, id string) error {
		if !stableID.MatchString(id) || seen[kind+"/"+id] {
			return fmt.Errorf("crafting %s %q: invalid or duplicate id", kind, id)
		}
		seen[kind+"/"+id] = true
		return nil
	}
	if m.Level == "" || m.Hero.Actor == "" || m.Hero.WorkStore == "" || m.Hero.Account == "" {
		return fmt.Errorf("crafting: level, hero actor, work store and account are required")
	}
	defs := map[string]string{}
	for _, p := range m.Parts {
		if err := unique("part", p.ID); err != nil {
			return err
		}
		if p.Name == "" || len(p.Items) == 0 {
			return fmt.Errorf("crafting part %s: name and items required", p.ID)
		}
		for _, i := range p.Items {
			if owner, dup := defs[i.Item]; dup || !strings.HasPrefix(i.Item, "item.") || !origins[i.Origin] || i.Quality < 1 || i.Quality > 5 {
				return fmt.Errorf("crafting part %s: item %s invalid or already a part of %s", p.ID, i.Item, owner)
			}
			defs[i.Item] = p.ID
		}
	}
	for _, d := range m.Disciplines {
		if err := unique("discipline", d.ID); err != nil {
			return err
		}
		if d.Name == "" || len(d.Tiers) != 5 {
			return fmt.Errorf("crafting discipline %s: name and five tiers required", d.ID)
		}
		for i, t := range d.Tiers {
			if t.Tier != i+1 || t.Name == "" || t.Points < 0 || t.Points > m.Tuning.CapPoints || (i == 0 && t.Points != 0) || (i > 0 && t.Points <= d.Tiers[i-1].Points) {
				return fmt.Errorf("crafting discipline %s: tier %d thresholds must rise from 0", d.ID, i+1)
			}
		}
	}
	toolKinds := map[string]bool{}
	for _, t := range m.Tools {
		if err := unique("tool", t.Item); err != nil {
			return err
		}
		if _, isPart := defs[t.Item]; isPart || t.Kind == "" || t.Quality < 1 || t.Quality > 5 || t.WearPPM < 0 || t.WearPPM > Million {
			return fmt.Errorf("crafting tool %s: invalid", t.Item)
		}
		toolKinds[t.Kind] = true
	}
	stationKinds := map[string]bool{}
	for _, s := range m.Stations {
		if err := unique("station", s.ID); err != nil {
			return err
		}
		if s.Name == "" || s.Kind == "" || s.Level == "" || s.Reach < 0 || s.Reach > 6 || s.RentMinorPerHour < 0 || s.QualityBonus < 0 || s.QualityBonus > 1 || (s.Owner != "" && s.RentMinorPerHour > 0 && s.Account == "") {
			return fmt.Errorf("crafting station %s: invalid", s.ID)
		}
		for _, t := range s.Tools {
			if t.Kind == "" || t.Quality < 1 || t.Quality > 5 {
				return fmt.Errorf("crafting station %s: invalid tool", s.ID)
			}
			toolKinds[t.Kind] = true
		}
		stationKinds[s.Kind] = true
	}
	for _, r := range m.Recipes {
		if err := unique("recipe", r.ID); err != nil {
			return err
		}
		d, ok := m.Discipline(r.Discipline)
		if !ok || !kinds[r.Kind] || !strings.HasPrefix(r.ID, r.Kind+".") || r.Name == "" || r.Hint == "" || r.Tier < 1 || r.Tier > len(d.Tiers) || r.Minutes < 1 || r.Minutes > 30*1440 || r.RiskPermille < 0 || r.RiskPermille > 950 || r.Base < 1 || r.Base > 5 {
			return fmt.Errorf("crafting recipe %s: invalid kind, discipline, tier, time, risk or base", r.ID)
		}
		if r.Legal != "legal" && r.Legal != "grey" && r.Legal != "forbidden" || (r.Basis != "canon" && r.Basis != "proposal") || len(r.Sources) == 0 || len(r.Learn) == 0 {
			return fmt.Errorf("crafting recipe %s: legality, basis, sources and learn required", r.ID)
		}
		for _, l := range r.Learn {
			if !learnKinds[l.Kind] {
				return fmt.Errorf("crafting recipe %s: unknown learn kind %s", r.ID, l.Kind)
			}
		}
		if r.Station != "" && !stationKinds[r.Station] {
			return fmt.Errorf("crafting recipe %s: no station of kind %s", r.ID, r.Station)
		}
		for _, k := range r.Tools {
			if !toolKinds[k] {
				return fmt.Errorf("crafting recipe %s: no tool of kind %s", r.ID, k)
			}
		}
		for _, in := range r.Inputs {
			if (in.Class == "") == (in.Item == "") || in.N < 1 || in.N > 100 || in.MinQuality < 1 || in.MinQuality > 5 {
				return fmt.Errorf("crafting recipe %s: input needs exactly one of class / item, n 1..100, min quality 1..5", r.ID)
			}
			if _, ok := m.Part(in.Class); in.Class != "" && !ok {
				return fmt.Errorf("crafting recipe %s: unknown part class %s", r.ID, in.Class)
			}
		}
		switch r.Kind {
		case "craft":
			if r.Output == nil || r.Output.N < 1 || !origins[r.Output.Origin] || r.Target != "" || len(r.Inputs) == 0 || len(r.Yields) > 0 {
				return fmt.Errorf("crafting recipe %s: a schematic has inputs and one output", r.ID)
			}
		case "repair", "maintain":
			if r.Target == "" || r.Output != nil || len(r.Yields) > 0 || r.RestorePPM < 1 || r.RestorePPM > Million {
				return fmt.Errorf("crafting recipe %s: repair / maintenance needs a target and a restored condition", r.ID)
			}
		case "bypass":
			if r.Target == "" || r.Output != nil || len(r.Yields) > 0 || r.Legal != "forbidden" {
				return fmt.Errorf("crafting recipe %s: a chip bypass needs a bound target and is forbidden", r.ID)
			}
		case "disassemble":
			if r.Target == "" || r.Output != nil || len(r.Yields) == 0 || len(r.Inputs) > 0 {
				return fmt.Errorf("crafting recipe %s: disassembly needs a target and yields", r.ID)
			}
			for _, y := range r.Yields {
				if _, ok := m.Part(y.Class); !ok || y.Min < 0 || y.Max < y.Min || y.Max > 50 || y.Quality < 1 || y.Quality > 5 || !origins[y.Origin] {
					return fmt.Errorf("crafting recipe %s: invalid yield %s", r.ID, y.Class)
				}
			}
			if r.Teaches != "" {
				if t, ok := m.Recipe(r.Teaches); !ok || t.Kind != "craft" || r.TeachPermille < 1 || r.TeachPermille > 1000 {
					return fmt.Errorf("crafting recipe %s: teaches an unknown schematic", r.ID)
				}
			}
		}
	}
	for _, s := range m.Masters {
		if err := unique("master", strings.ReplaceAll(s.Actor, "char.", "char_")); err != nil {
			return err
		}
		st, ok := m.Station(s.Station)
		if !ok || st.Owner != s.Actor || s.Shop == "" || s.Level == "" || s.Reach < 1 || s.Reach > 6 || s.LaborMinorPerHour < 0 || s.OrderStore == "" || s.Account == "" || len(s.Stores) == 0 || len(s.Offers) == 0 {
			return fmt.Errorf("crafting master %s: own station, shop, reach, stores, account and offers required", s.Actor)
		}
		for _, id := range s.Offers {
			r, ok := m.Recipe(id)
			if !ok {
				return fmt.Errorf("crafting master %s: unknown offer %s", s.Actor, id)
			}
			w := m.MasterWorker(s)
			if !m.Knows(w, r) || m.TierOf(w, r.Discipline) < r.Tier {
				return fmt.Errorf("crafting master %s: offers %s without the knowledge or the tier", s.Actor, id)
			}
		}
	}
	for d, p := range m.Hero.Points {
		if _, ok := m.Discipline(d); !ok || p < 0 || p > m.Tuning.CapPoints {
			return fmt.Errorf("crafting hero: invalid points of %s", d)
		}
	}
	for _, id := range m.Hero.Known {
		if _, ok := m.Recipe(id); !ok {
			return fmt.Errorf("crafting hero: unknown recipe %s", id)
		}
	}
	if err := m.validateScenarios(); err != nil {
		return err
	}
	t := m.Tuning
	if t.CapPoints < 10 || t.CapPoints > 100 || t.MinRisk < 0 || t.MaxRisk < t.MinRisk || t.MaxRisk > 950 || t.CriticalPermille < 0 || t.CriticalPermille > 1000 || t.FailureGainPermille < 0 || t.FailureGainPermille > 1000 || t.SurvivePermille < 1 || t.SurvivePermille > 1000 {
		return fmt.Errorf("crafting tuning: invalid bounds")
	}
	for _, k := range []string{"craft", "repair", "maintain", "disassemble"} {
		if t.GainMilli[k] < 0 || t.GainMilli[k] > 50000 {
			return fmt.Errorf("crafting tuning: gain of %s out of bounds", k)
		}
	}
	if err := m.validateSources(); err != nil {
		return err
	}
	if err := m.validateLaw(); err != nil {
		return err
	}
	return m.validateMarket()
}
