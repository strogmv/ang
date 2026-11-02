package crafting

import (
	"fmt"
	"strings"
)

// Scenario is a host test written as data in CUE (crafting/scenarios.cue): where the hero stands, which stall is open,
// what the hero carries, the command and the expected refusal key or "started".
type Scenario struct {
	ID     string           `json:"id"`
	Stand  string           `json:"stand,omitempty"`
	Open   string           `json:"open,omitempty"`
	Points map[string]int64 `json:"points,omitempty"`
	Give   []ScenarioGive   `json:"give"`
	Action string           `json:"action"`
	Recipe string           `json:"recipe"`
	Target string           `json:"target,omitempty"`
	Expect string           `json:"expect"`
	// stage 2: memory, the relation of the person stood at / near to the hero, a person brought near, the hero at the
	// equipment terminal, the system trust and the CRD balance
	Known            []string `json:"known,omitempty"`
	RelationPermille *int64   `json:"relation_permille,omitempty"`
	Near             string   `json:"near,omitempty"`
	Terminal         bool     `json:"terminal,omitempty"`
	Trust            *int64   `json:"trust,omitempty"`
	Money            *int64   `json:"money,omitempty"`
}
type ScenarioGive struct {
	Item string `json:"item"`
	N    int64  `json:"n"`
}

var scenarioActions = map[string]bool{"start": true, "order": true, "study": true, "apprentice": true, "ask": true, "steal": true, "rebind": true}

func (m Model) validateScenarios() error {
	seen := map[string]bool{}
	for _, sc := range m.Scenarios {
		if !stableID.MatchString(sc.ID) || seen[sc.ID] {
			return fmt.Errorf("crafting scenario %q: invalid or duplicate id", sc.ID)
		}
		seen[sc.ID] = true
		_, known := m.Recipe(sc.Recipe)
		_, stand := m.Master(sc.Stand)
		_, open := m.Master(sc.Open)
		if sc.Recipe == "" && sc.Action != "start" && sc.Action != "order" {
			known = true
		}
		for _, id := range sc.Known {
			if _, ok := m.Recipe(id); !ok {
				known = false
			}
		}
		expect := sc.Expect == "started" || sc.Expect == "learned" || sc.Expect == "done" || strings.HasPrefix(sc.Expect, "crafting.error.")
		if !known || (sc.Stand != "" && !stand) || (sc.Open != "" && !open) || !scenarioActions[sc.Action] || !expect {
			return fmt.Errorf("crafting scenario %s: unknown recipe / master, action or expectation", sc.ID)
		}
	}
	return nil
}
