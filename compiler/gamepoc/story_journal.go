package gamepoc

import "fmt"

type storyJournalRules struct {
	Sources         map[string]string `json:"sources"`
	RumourRetention float64           `json:"rumour_retention"`
	BarkCells       int               `json:"bark_cells"`
	Barks           []storyBarkDef    `json:"barks"`
}

type storyBarkDef struct {
	Fact     string `json:"fact"`
	Speaker  string `json:"speaker"`
	Text     string `json:"text"`
	Belief   string `json:"belief"`
	Existing bool   `json:"existing,omitempty"`
}

func (k *memoryChecker) checkStoryJournal() error {
	const where = "world.memory.journal"
	for _, id := range sortedKeys(k.m.Facts) {
		f := k.m.Facts[id]
		if f.RecordAccess != "" && (!f.Recorded || f.RecordAccess != "hero_chip") {
			return fmt.Errorf("%s: %s record access requires a recorded fact", where, id)
		}
		if f.Refutes != "" {
			target := k.m.Facts[f.Refutes]
			if target == nil || f.Refutes == id || !target.Rumour || target.Journal == "" || f.Journal == "" {
				return fmt.Errorf("%s: %s must refute a different journalled rumour with a journal line", where, id)
			}
			k.read(true, f.Refutes, "журнал "+f.Journal)
		}
	}
	rules := k.m.Journal
	if rules == nil {
		return nil
	}
	if rules.RumourRetention <= 0 || rules.RumourRetention >= 1 || rules.BarkCells < 1 || rules.BarkCells > 30 {
		return fmt.Errorf("%s: invalid rumour retention or bark reach", where)
	}
	for _, source := range []string{"saw", "told", "record", "rumour"} {
		if err := k.text(where, rules.Sources[source]); err != nil {
			return err
		}
	}
	seen := map[[3]string]bool{}
	for _, bark := range rules.Barks {
		def := k.m.Facts[bark.Fact]
		if def == nil || (def.Visibility != "public" && def.Visibility != "witnessed") ||
			(bark.Belief != "witness" && !(bark.Belief == "heard" && def.Rumour)) {
			return fmt.Errorf("%s: invalid fact or belief for %s", where, bark.Text)
		}
		if err := k.person(where, bark.Speaker, false); err != nil {
			return err
		}
		if err := k.text(where, bark.Text); err != nil {
			return err
		}
		if k.c.Barks == nil || k.c.Barks.Talk.MaxPerHour < 1 || k.c.Barks.Talk.MaxPerHour > 10 || k.c.Barks.Talk.MeanWaitSeconds <= 0 {
			return fmt.Errorf("%s: fact barks need a valid shared talk clock", where)
		}
		if bark.Existing {
			line, _, ok := k.c.Barks.Line(bark.Text)
			if !ok || line.Speaker != bark.Speaker || line.AtOnce() {
				return fmt.Errorf("%s: %s must be an existing line for %s", where, bark.Text, bark.Speaker)
			}
		}
		key := [3]string{bark.Speaker, bark.Fact, bark.Belief}
		if seen[key] {
			return fmt.Errorf("%s: duplicate bark for %s, %s, %s", where, key[0], key[1], key[2])
		}
		seen[key] = true
		k.read(true, bark.Fact, "барк "+bark.Text)
	}
	return nil
}
