package gamepoc

import (
	"fmt"
	"slices"
	"strings"
)

type storyQuestStep struct {
	ID     string `json:"id"`
	Step   string `json:"step"`
	Second int64  `json:"second,omitempty"`
}

type storyHours struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

type storyCell struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type storyLocation struct {
	Level string     `json:"level"`
	At    *storyCell `json:"at,omitempty"`
	Reach int        `json:"reach"`
}

type storyPresence struct {
	storyLocation
	People []string `json:"people"`
}

type storyPlacementCondition struct {
	Placement string  `json:"placement"`
	Set       float64 `json:"set"`
}

type storyTransfer struct {
	Ground    *storyLocation `json:"ground,omitempty"`
	Owner     string         `json:"owner,omitempty"`
	Also      []string       `json:"also,omitempty"`
	Placement string         `json:"placement"`
	To        string         `json:"to"`
	PayCRD    int64          `json:"pay_crd_minor,omitempty"`
}

type storyRule[W, D any] struct {
	ID    string     `json:"id"`
	Level string     `json:"level"`
	At    *storyCell `json:"at,omitempty"`
	When  *W         `json:"when"`
	Do    []D        `json:"do"`
}

type storyAutoTalk[W any] struct {
	Conversation string `json:"conversation"`
	Speaker      string `json:"speaker"`
	When         *W     `json:"when"`
}

type storyArrival[W, D any] struct {
	When *W  `json:"when"`
	Do   []D `json:"do"`
}

type storyDeckPhase struct {
	Deck   string   `json:"deck"`
	Phases []string `json:"phases"`
}

type storyGathering[W any] struct {
	When      *W                   `json:"when"`
	Level     string               `json:"level"`
	Positions map[string]storyCell `json:"positions"`
}

type storyPlacementMigration struct {
	Placement string         `json:"placement"`
	Owner     string         `json:"owner"`
	Quest     storyQuestStep `json:"quest"`
	Facts     []string       `json:"facts"`
	Witness   string         `json:"witness"`
}

func (k *memoryChecker) storyQuest(where string, q *storyQuestStep) error {
	if q != nil && (!slices.Contains(k.m.Quests[q.ID], q.Step) || q.Second != 0) {
		return fmt.Errorf("%s: unknown quest step %s/%s", where, q.ID, q.Step)
	}
	return nil
}

func (k *memoryChecker) storyPlacement(where, id string) error {
	for _, source := range k.c.Inventory.Sources {
		if source.ID == id && source.Definition != "" {
			return nil
		}
	}
	return fmt.Errorf("%s: unknown fixed placement %s", where, id)
}

func (k *memoryChecker) storyWhen(where string, w *When) error {
	if w.TrainBerth != "" && (w.TrainBerth != "stopped" || k.c.TrainMotion == nil || k.c.TrainMotion.BerthLevelID == "") {
		return fmt.Errorf("%s: train predicate needs a physical berth", where)
	}
	if p := w.DeckPhase; p != nil {
		valid := k.c.LiftingDecks != nil && slices.ContainsFunc(k.c.LiftingDecks.Decks, func(d LiftingDeckDefinition) bool { return d.ID == p.Deck })
		if !valid || len(p.Phases) == 0 {
			return fmt.Errorf("%s: deck phases need a lifting deck", where)
		}
	}
	if err := k.storyQuest(where, w.Quest); err != nil {
		return err
	}
	if w.Placement != "" {
		if err := k.storyPlacement(where, w.Placement); err != nil {
			return err
		}
	}
	for _, loc := range []*storyLocation{w.HeroAt, func() *storyLocation {
		if w.Present != nil {
			return &w.Present.storyLocation
		}
		return nil
	}()} {
		if loc != nil {
			if _, ok := k.c.Inventory.Levels[loc.Level]; !ok {
				return fmt.Errorf("%s: unknown level %s", where, loc.Level)
			}
		}
	}
	if w.Present != nil {
		for _, person := range w.Present.People {
			if err := k.person(where, person, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (k *memoryChecker) storyDo(where string, d Do, index int) error {
	if len(d.DeckWith) > 0 {
		if d.DeckRide == "" {
			return fmt.Errorf("%s: deck companions require a deck ride", where)
		}
		seen := map[string]bool{"char.hero": true}
		for _, person := range d.DeckWith {
			if seen[person] {
				return fmt.Errorf("%s: duplicate deck rider %s", where, person)
			}
			seen[person] = true
			if err := k.person(where, person, false); err != nil {
				return err
			}
		}
	}
	if d.ResumeRoutine != "" {
		return k.person(where, d.ResumeRoutine, false)
	}
	if d.DeckRide != "" {
		if index != 0 || k.c.LiftingDecks == nil || !slices.ContainsFunc(k.c.LiftingDecks.Decks, func(deck LiftingDeckDefinition) bool {
			return deck.TravelEdge == d.DeckRide || deck.ReturnEdge == d.DeckRide
		}) {
			return fmt.Errorf("%s: deck ride must be the first effect and use an actual deck edge", where)
		}
	}
	if d.RatingLoss < 0 {
		return fmt.Errorf("%s: invalid rating loss", where)
	}
	if err := k.storyQuest(where, d.Quest); err != nil {
		return err
	}
	if q := d.Transfer; q != nil {
		if index != 0 {
			return fmt.Errorf("%s: an inventory transfer must precede other effects", where)
		}
		if err := k.storyPlacement(where, q.Placement); err != nil {
			return err
		}
		seen := map[string]bool{q.Placement: true}
		for _, id := range q.Also {
			if seen[id] || q.PayCRD > 0 {
				return fmt.Errorf("%s: a paid transfer needs one placement; unpaid transfers use distinct placements", where)
			}
			if err := k.storyPlacement(where, id); err != nil {
				return err
			}
			seen[id] = true
		}

		if q.Owner != "" {
			if q.To == "hero" {
				return fmt.Errorf("%s: hero transfers assign the hero as owner", where)
			}
			if err := k.person(where, q.Owner, false); err != nil {
				return err
			}
		}
		if q.Ground != nil {
			size, ok := k.c.Inventory.Levels[q.Ground.Level]
			at := q.Ground.At
			if !ok || q.To != "ground" || at == nil || at.X < 0 || at.Y < 0 || at.X >= size.X || at.Y >= size.Y {
				return fmt.Errorf("%s: invalid ground destination", where)
			}
		} else if q.To != "hero" {
			if _, ok := k.c.Inventory.Stores[q.To]; !ok {
				return fmt.Errorf("%s: unknown destination store %s", where, q.To)
			}
		}
	}
	if pc := d.PlacementCondition; pc != nil {
		if index != 0 || pc.Set < 0 || pc.Set > 1 {
			return fmt.Errorf("%s: placement condition must precede other effects and be in [0,1]", where)
		}
		return k.storyPlacement(where, pc.Placement)
	}
	if d.RemovePlacement != "" {
		if index != 0 {
			return fmt.Errorf("%s: placement removal must precede other effects", where)
		}
		return k.storyPlacement(where, d.RemovePlacement)
	}
	if d.RevealNode != "" {
		for _, node := range k.c.Region.Nodes {
			if node.ID == d.RevealNode {
				return nil
			}
		}
		return fmt.Errorf("%s: unknown region node %s", where, d.RevealNode)
	}
	return nil
}

func (k *memoryChecker) checkStoryRules(levels map[string]struct{ Terrain []string }) error {
	for _, id := range sortedKeys(k.m.Quests) {
		if !(strings.HasPrefix(id, "quest.") || strings.HasPrefix(id, "sq.") || strings.HasPrefix(id, "mq.")) {
			return fmt.Errorf("world.memory.quests: invalid quest %s", id)
		}
		seen := map[string]bool{}
		for _, step := range k.m.Quests[id] {
			if seen[step] {
				return fmt.Errorf("%s: repeated step %s", id, step)
			}
			seen[step] = true
		}
	}
	seen := map[string]bool{}
	for _, rule := range k.m.Rules {
		where := "world.memory.rules." + rule.ID
		if _, ok := levels[rule.Level]; !ok || seen[rule.ID] {
			return fmt.Errorf("%s: need a unique rule id and known level", where)
		}
		seen[rule.ID] = true
		if err := k.when(where, rule.When, nil, where); err != nil {
			return err
		}
		if err := k.do(where, rule.Do, nil, where, rule.When); err != nil {
			return err
		}
	}
	for _, meeting := range k.m.Gatherings {
		where := "world.memory.gatherings"
		if err := k.when(where, meeting.When, nil, where); err != nil {
			return err
		}
		level := levels[meeting.Level].Terrain
		taken := map[storyCell]bool{}
		for _, id := range sortedKeys(meeting.Positions) {
			if err := k.person(where, id, false); err != nil {
				return err
			}
			at := meeting.Positions[id]
			if at.Y < 0 || at.Y >= len(level) || at.X < 0 || at.X >= len(level[at.Y]) ||
				level[at.Y][at.X] == '#' || level[at.Y][at.X] == ' ' || taken[at] {
				return fmt.Errorf("%s: %s needs a distinct floor cell in %s", where, id, meeting.Level)
			}
			taken[at] = true
		}
	}
	for _, id := range sortedKeys(k.m.PlacementVersions) {
		if !slices.Contains(k.m.BackfillPlacements, id) || k.m.PlacementVersions[id] < 1 || k.m.PlacementVersions[id] > 2 {
			return fmt.Errorf("world.memory.placement_versions: invalid migration for %s", id)
		}
	}
	for _, id := range k.m.BackfillPlacements {
		if err := k.storyPlacement("world.memory.backfill_placements", id); err != nil {
			return err
		}
	}
	for _, m := range k.m.MigratePlacements {
		where := "world.memory.migrate_placements." + m.Placement
		if err := k.storyPlacement(where, m.Placement); err != nil {
			return err
		}
		if err := k.storyQuest(where, &m.Quest); err != nil {
			return err
		}
		if err := k.person(where, m.Owner, false); err != nil {
			return err
		}
		if err := k.person(where, m.Witness, false); err != nil {
			return err
		}
		for _, id := range m.Facts {
			if err := k.factArgs(where, id, nil, true); err != nil {
				return err
			}
			k.links.add(k.links.factWriters, id, where)
		}
	}
	return nil
}
