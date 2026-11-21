package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"

	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// memoryWorld: a small world with two people, a fact that one answer writes and an entry reads, a topic a question
// gives and another answer needs, a check whose failure has content, and an interjection in the talk.
func memoryWorld(t *testing.T) *Compiled {
	t.Helper()
	c := &Compiled{Life: json.RawMessage(`{"actors":[{"id":"char.hero"},{"id":"char.trader"},{"id":"char.guard"}]}`)}
	keys := []string{"t.open", "t.first", "t.ask", "t.answer", "t.fix", "t.ok", "t.fail", "t.bye", "t.reply", "t.interject", "t.topic", "t.journal", "t.voice"}
	for _, k := range keys {
		c.Localization.Messages = append(c.Localization.Messages, LocalizationMessage{Key: k, Context: "c", Texts: map[string]string{"ru": k, "en": k}})
	}
	c.DialogueRules = &DialogueRules{Skills: map[string]DialogueSkillTag{"maintenance": {Name: "t.voice", Colour: "#ffffff"}}, Chance: map[string][]int{"easy": make([]int, 101)}}
	c.Inventory = inv.Model{Definitions: map[string]inv.Definition{"item.lamp": {}}}
	m := &Memory{Facts: map[string]*MemoryFact{}, Topics: map[string]*MemoryTopic{}}
	m.InterjectionRules.FreshDays, m.InterjectionRules.HeardBelief = 14, 0.5
	m.Facts["fact.trader.met"] = &MemoryFact{Visibility: "witnessed", MemoryDays: 14, KnownBy: []string{"self"}, SightCells: 6, Journal: "t.journal"}
	m.Topics["topic.trader_work"] = &MemoryTopic{Journal: "t.topic", HearingCells: 10}
	m.Interjections = []Interjection{{ID: "interject.met.trader", Text: "t.interject", Fact: "fact.trader.met", Speaker: "char.trader", Belief: "witness"}}
	m.Talks = []MemoryTalk{{Conversation: "dlg.trader", People: []string{"char.trader"}, Open: "news"}}
	c.Memory = m
	c.Conversations = map[string]*Conversation{"dlg.trader": {
		ID: "dlg.trader", Reach: 4, LingerSeconds: 6,
		Entries: []ConvEntry{
			{ID: "e.met", When: &When{Fact: &FactWhen{ID: "fact.trader.met"}}, Node: "open", Lines: []ConvLine{{Speaker: "self", Text: "t.open"}}},
			{ID: "e.first", Node: "open", Lines: []ConvLine{{Speaker: "self", Text: "t.first"}}},
		},
		Nodes: map[string]*ConvNode{"open": {
			Voices: []ConvVoice{{ID: "lamp", Skill: "maintenance", AtLeast: 20, Text: "t.voice"}},
			Options: []ConvOption{
				{ID: "ask", Text: "t.ask", Reply: []ConvLine{{Speaker: "self", Text: "t.answer"}}, Do: []Do{{Topic: "topic.trader_work"}, {Fact: "fact.trader.met"}}},
				{ID: "fix", Text: "t.fix", When: &When{Topic: "topic.trader_work", Voiced: "lamp"}, Check: &DialogueCheck{Skill: "maintenance", Difficulty: "easy", Kind: "once"},
					Reply: []ConvLine{{Speaker: "self", Text: "t.ok"}}, Do: []Do{{ItemCondition: &struct {
						Owner string  `json:"owner"`
						Item  string  `json:"item"`
						Set   float64 `json:"set"`
					}{Owner: "self", Item: "item.lamp", Set: 1}}},
					Fail: &ConvFail{Reply: []ConvLine{{Speaker: "self", Text: "t.fail"}}, Do: []Do{{GiveItem: &struct {
						Item string `json:"item"`
						From string `json:"from"`
					}{Item: "item.lamp", From: "self"}}}}},
				{ID: "bye", Text: "t.bye", Reply: []ConvLine{{Speaker: "self", Text: "t.reply"}}, Close: "bye"},
			},
		}},
	}}
	return c
}

func TestMemoryChecksAcceptAWholeWorldAndDrawItsLinks(t *testing.T) {
	c := memoryWorld(t)
	if err := checkMemory(c); err != nil {
		t.Fatal(err)
	}
	page := string(c.linksPage)
	for _, want := range []string{"fact.trader.met", "topic.trader_work", "dlg.trader", "char.trader", "разговор dlg.trader: ответ ask", "вставка interject.met.trader"} {
		if !strings.Contains(page, want) {
			t.Fatalf("the links page lacks %q", want)
		}
	}
}

func TestMemoryChecksRejectBrokenLinks(t *testing.T) {
	node := func(c *Compiled) *ConvNode { return c.Conversations["dlg.trader"].Nodes["open"] }
	cases := map[string]func(*Compiled){
		// a dead branch: nothing writes the fact the entry reads
		"nothing writes it": func(c *Compiled) { node(c).Options[0].Do = node(c).Options[0].Do[:1] },
		// a fact nobody remembers is a flag
		"nothing reads it": func(c *Compiled) {
			c.Conversations["dlg.trader"].Entries = c.Conversations["dlg.trader"].Entries[1:]
			c.Memory.Interjections = nil
		},
		"unknown fact": func(c *Compiled) {
			c.Conversations["dlg.trader"].Entries[0].When = &When{Fact: &FactWhen{ID: "fact.trader.gone"}}
		},
		// failure is content: lines and a consequence
		"the failure of a check needs":    func(c *Compiled) { node(c).Options[1].Fail.Do = nil },
		"a skill check needs its failure": func(c *Compiled) { node(c).Options[1].Fail = nil },
		"no «usual» entry": func(c *Compiled) {
			c.Conversations["dlg.trader"].Entries[1].When = &When{NotFact: "fact.trader.met"}
		},
		"no way out of this node": func(c *Compiled) {
			o := &node(c).Options[2]
			o.Close = ""
		},
		"no entry or answer leads": func(c *Compiled) {
			c.Conversations["dlg.trader"].Nodes["lost"] = &ConvNode{Options: []ConvOption{{ID: "leave", Text: "t.bye", Close: "bye"}}}
		},
		"talks nowhere":   func(c *Compiled) { c.Memory.Interjections[0].Speaker = "char.guard" },
		"nobody opens it": func(c *Compiled) { c.Memory.Talks = nil; c.Memory.Interjections[0].Waits = "later" },
		"topics.topic.trader_work: nothing reads it": func(c *Compiled) { node(c).Options[1].When = &When{Voiced: "lamp"} },
		"exactly one thing": func(c *Compiled) {
			node(c).Options[0].Do[0].Fact = "fact.trader.met"
		},
		"is not of this conversation":     func(c *Compiled) { node(c).Options[1].When.Voiced = "other" },
		"missing localization":            func(c *Compiled) { node(c).Options[0].Text = "t.unknown" },
		"does not take part":              func(c *Compiled) { node(c).Options[0].Reply[0].Speaker = "char.guard" },
		"a private fact has no witnesses": func(c *Compiled) { c.Memory.Facts["fact.trader.met"].Visibility = "private" },
	}
	for want, mutate := range cases {
		c := memoryWorld(t)
		mutate(c)
		err := checkMemory(c)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: got %v", want, err)
		}
	}
}

func TestMemoryChecksKeepNineAnswersOnScreen(t *testing.T) {
	ten := func() []ConvOption {
		out := []ConvOption{}
		for i := 0; i < 10; i++ {
			out = append(out, ConvOption{ID: "a" + string(rune('a'+i))})
		}
		return out
	}
	if most, _ := maxAnswersShown(ten()); most != 10 {
		t.Fatalf("ten free answers: %d", most)
	}
	// a fact's answer in place of the general one counts once
	replaced := ten()
	replaced[9].Replaces = replaced[8].ID
	if most, _ := maxAnswersShown(replaced); most != 9 {
		t.Fatalf("with a replacement: %d", most)
	}
	// answers of different entries, or of a fact and its absence, are never together
	split := ten()
	split[0].When = &When{Entry: "e.first"}
	split[1].When = &When{Entry: "e.usual"}
	if most, _ := maxAnswersShown(split); most != 9 {
		t.Fatalf("entry-apart answers: %d", most)
	}
	split = ten()
	split[0].When = &When{Fact: &FactWhen{ID: "fact.a.b"}}
	split[1].When = &When{NotFact: "fact.a.b"}
	if most, _ := maxAnswersShown(split); most != 9 {
		t.Fatalf("fact-apart answers: %d", most)
	}
	// the conversation check fails the build on ten
	c := memoryWorld(t)
	n := c.Conversations["dlg.trader"].Nodes["open"]
	for i := 0; i < 7; i++ {
		n.Options = append(n.Options, ConvOption{ID: "extra_" + string(rune('a'+i)), Text: "t.ask"})
	}
	if err := checkMemory(c); err == nil || !strings.Contains(err.Error(), "up to 10 answers") {
		t.Fatalf("ten answers passed: %v", err)
	}
	c = memoryWorld(t)
	c.Conversations["dlg.trader"].Nodes["open"].Options[0].Replaces = "nothing"
	if err := checkMemory(c); err == nil || !strings.Contains(err.Error(), "replaces") {
		t.Fatalf("a replacement of nothing passed: %v", err)
	}
}
