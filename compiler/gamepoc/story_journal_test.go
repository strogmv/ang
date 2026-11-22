package gamepoc

import (
	"strings"
	"testing"
)

func TestStoryJournalAuthoringChecks(t *testing.T) {
	c := memoryWorld(t)
	m := c.Memory
	m.Facts["fact.trader.met"].Rumour = true
	const correction = "fact.trader.correction"
	m.Facts[correction] = &MemoryFact{Visibility: "recorded", Recorded: true, MemoryDays: 14,
		Journal: "t.journal", RecordAccess: "hero_chip", Refutes: "fact.trader.met",
		ExternalWriters: []string{"fixture"}}
	m.Journal = &storyJournalRules{RumourRetention: .75, BarkCells: 10,
		Sources: map[string]string{"saw": "t.journal", "told": "t.journal", "record": "t.journal", "rumour": "t.journal"}}
	conv := c.Conversations["dlg.trader"]
	conv.Entries[0].When = &When{Knows: &KnownFactWhen{Who: "hero", Fact: "fact.trader.met", AtLeast: 1}}
	opt := &conv.Nodes["open"].Options[0]
	opt.Do = append(opt.Do, Do{Learn: correction})
	if err := checkMemory(c); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, want    string
		change, reset func()
	}{
		{"source text", "t.missing", func() { m.Journal.Sources["rumour"] = "t.missing" }, func() { m.Journal.Sources["rumour"] = "t.journal" }},
		{"refutation", "different journalled rumour", func() { m.Facts[correction].Refutes = correction }, func() { m.Facts[correction].Refutes = "fact.trader.met" }},
		{"record", "requires a recorded fact", func() { m.Facts[correction].Recorded = false }, func() { m.Facts[correction].Recorded = true }},
		{"learn", "declared fact", func() { opt.Do[2].Learn = "fact.trader.unknown" }, func() { opt.Do[2].Learn = correction }},
		{"belief", "belief threshold", func() { conv.Entries[0].When.Knows.AtLeast = 2 }, func() { conv.Entries[0].When.Knows.AtLeast = 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.change()
			defer test.reset()
			if err := checkMemory(c); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want %s, got %v", test.want, err)
			}
		})
	}
}
