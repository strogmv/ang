// Package wounds compiles the wound model (doc 97, world.wounds): it checks the catalog against the compiled world, emits
// the standalone deterministic engine internal/wounds with its catalog and renders the timeline table
// design/wounds/timelines.md from the CUE scenarios. It is not wired into combat: the fighting layer calls the engine.
package wounds

import (
	"errors"
	"fmt"
	"sort"

	runtime "github.com/strogmv/ang/compiler/wounds/runtime"
)

// World is what the content checks need from the compiled world.
type World struct {
	Items     map[string]string // item id -> carrier
	Weapons   map[string]bool   // combat_runtime.weapons and the bare-hand weapon ids of combat profiles
	Armor     map[string]bool   // combat_runtime.armor
	Crimes    map[string]string // justice crime id -> outcome
	Persons   map[string]bool
	Responses map[string]bool // stress responses of the drive vectors (drives.cue)
	Locales   []string
	Texts     map[string]map[string]string
}

// EngineKeys are the fixed texts the engine and its views name (localization.cue of the package).
var EngineKeys = []string{
	"wounds.state.ok", "wounds.state.weak", "wounds.state.confused", "wounds.state.down", "wounds.state.dead", "wounds.state.fallen",
	"wounds.sign.none", "wounds.sign.blood", "wounds.sign.blood_flow", "wounds.sign.blood_spurt", "wounds.sign.limp", "wounds.sign.clutch",
	"wounds.sign.gasp", "wounds.sign.still", "wounds.sign.bruise", "wounds.sign.pale",
	"wounds.effect.pain", "wounds.effect.drop_weapon", "wounds.effect.limp", "wounds.effect.no_walk", "wounds.effect.breathless", "wounds.effect.concussion",
	"wounds.forecast.down", "wounds.forecast.dead", "wounds.forecast.hours", "wounds.forecast.stable", "wounds.wound.line", "wounds.wound.inside",
	"wounds.reaction.fight_on", "wounds.reaction.flee", "wounds.reaction.fall", "wounds.reaction.surrender",
	"wounds.aid.applied", "wounds.aid.failed", "wounds.aid.wrong_zone", "wounds.aid.wrong_item", "wounds.aid.nothing", "wounds.aid.dead",
	"wounds.aid.treated", "wounds.aid.splinted", "wounds.aid.limb_lost",
	"wounds.care.full", "wounds.care.stabilize", "wounds.care.no_chip", "wounds.care.trust", "wounds.care.dead", "wounds.care.refuses",
	"wounds.care.money", "wounds.care.nothing", "wounds.care.done", "wounds.care.complication",
	"wounds.event.chip_alarm", "wounds.event.story_node", "wounds.ledger.kills", "wounds.ledger.none",
}

// Keys lists every localization key the catalog names.
func Keys(m runtime.Model) []string {
	keys := append([]string(nil), EngineKeys...)
	for _, z := range m.Zones {
		keys = append(keys, z.Name)
		for _, s := range z.Structures {
			keys = append(keys, s.Name)
		}
	}
	for _, id := range sortedKeys(m.Mechanisms) {
		keys = append(keys, m.Mechanisms[id].Name)
	}
	for _, w := range m.Weapons {
		keys = append(keys, w.Name)
	}
	for _, t := range m.Techniques {
		keys = append(keys, t.Name)
	}
	for _, id := range sortedKeys(m.Aid.Kinds) {
		keys = append(keys, m.Aid.Kinds[id].Name)
	}
	for _, p := range m.Providers {
		keys = append(keys, p.Name)
	}
	for _, c := range m.Conditions {
		keys = append(keys, c.Name)
	}
	for _, r := range m.Routes {
		keys = append(keys, r.Name)
	}
	for _, s := range m.Scenarios {
		keys = append(keys, s.Name)
	}
	return keys
}

// Check runs the structural gate and every content rule; a violation fails generate.
func Check(m runtime.Model, w World) error {
	if err := m.Validate(); err != nil {
		return err
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf("wounds: "+format, a...)) }
	if len(w.Locales) == 0 {
		fail("no locales")
	}
	for _, k := range Keys(m) {
		for _, loc := range w.Locales {
			if w.Texts[k][loc] == "" {
				fail("text %s has no %s", k, loc)
			}
		}
	}
	for _, it := range m.Aid.Items {
		if w.Items[it.Item] != "physical" {
			fail("aid item %s (%s) is not a physical item of world.items", it.Item, it.Kind)
		}
	}
	for _, k := range m.Aid.Medkits {
		if w.Items[k.Item] != "physical" {
			fail("medkit %s is not a physical item of world.items", k.Item)
		}
	}
	for _, wp := range m.Weapons {
		if !w.Weapons[wp.Weapon] {
			fail("weapon profile %s names no combat weapon (combat_runtime)", wp.Weapon)
		}
	}
	for _, a := range m.Armor {
		if !w.Armor[a.Item] {
			fail("armour %s is not combat_runtime armour", a.Item)
		}
	}
	for crime, outcome := range map[string]string{m.Justice.Death: "death", m.Justice.Serious: "wound", m.Justice.Hit: "hit"} {
		if w.Crimes[crime] != outcome {
			fail("justice crime %s must exist with outcome %s", crime, outcome)
		}
	}
	if !w.Persons[m.Justice.Hero] {
		fail("the hero %s is no person", m.Justice.Hero)
	}
	for r := range w.Responses {
		if _, ok := m.Resolve.Reactions[r]; !ok {
			fail("stress response %s has no reactions", r)
		}
	}
	for r := range m.Resolve.Reactions {
		if len(w.Responses) > 0 && !w.Responses[r] {
			fail("reactions for unknown stress response %s", r)
		}
	}
	for _, s := range m.Scenarios {
		if s.Hit != nil && s.Hit.Weapon != "" {
			if _, ok := m.Weapon(s.Hit.Weapon); !ok {
				fail("scenario %s weapon %s has no wound profile", s.ID, s.Hit.Weapon)
			}
		}
		if s.Technique != nil {
			if _, ok := m.Technique(s.Technique.ID); !ok {
				fail("scenario %s technique %s", s.ID, s.Technique.ID)
			}
		}
		for _, a := range s.Aid {
			if a.Kind == "pressure" {
				continue
			}
			if _, ok := m.AidItem(a.Item, a.Kind); !ok {
				if _, kit := m.Medkit(a.Item); !(a.Kind == "medkit" && kit) {
					fail("scenario %s aid %s with %q", s.ID, a.Kind, a.Item)
				}
			}
		}
		// the expectations of the row hold (the same run as the table and the generated test)
		quick := s
		quick.RiskRuns = 0
		tl, err := m.RunScenario(quick, runtime.ScenarioSeed)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := s.Check(tl); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
