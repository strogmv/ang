package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// memoryWorld: a small world with two people, a fact that one answer writes and an entry reads, a topic a question
// gives and another answer needs, a check whose failure has content, and an interjection in the talk.
func memoryWorld(t *testing.T) *Compiled {
	t.Helper()
	c := &Compiled{Life: json.RawMessage(`{"actors":[{"id":"char.hero"},{"id":"char.trader"},{"id":"char.guard"}]}`)}
	keys := []string{"t.open", "t.first", "t.ask", "t.answer", "t.fix", "t.ok", "t.fail", "t.bye", "t.reply", "t.interject", "t.topic", "t.journal", "t.voice", "actor.char.trader.name"}
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
			c.Memory.Facts["fact.trader.met"].Journal = ""
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
		"facts.fact.trader.unread: nothing reads it": func(c *Compiled) {
			node(c).Options[1].When = &When{Voiced: "lamp"}
			c.Memory.Facts["fact.trader.unread"] = &MemoryFact{Visibility: "private", MemoryDays: 14, ExternalWriters: []string{"test event"}}
		},
		"exactly one thing": func(c *Compiled) {
			node(c).Options[0].Do[0].Fact = "fact.trader.met"
		},
		"is not of this conversation":     func(c *Compiled) { node(c).Options[1].When.Voiced = "other" },
		"missing localization":            func(c *Compiled) { node(c).Options[0].Text = "t.unknown" },
		"does not take part":              func(c *Compiled) { node(c).Options[0].Reply[0].Speaker = "char.guard" },
		"a private fact has no witnesses": func(c *Compiled) { c.Memory.Facts["fact.trader.met"].Visibility = "private" },
		// talk with anyone, trade inside the talk (owner 21.11)
		"the name of char.trader": func(c *Compiled) { c.Localization.Messages = c.Localization.Messages[:len(c.Localization.Messages)-1] },
		"opens always":            func(c *Compiled) { c.Memory.Talks[0].Others = true },
		"world.memory.trade.rows": func(c *Compiled) { node(c).Options[0].Trade = true },
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

func TestMemoryChecksRequireNamesForSharedTalkTargets(t *testing.T) {
	c := memoryWorld(t)
	talk := &c.Memory.Talks[0]
	talk.Others, talk.Open = true, "always"
	if err := checkMemory(c); err == nil || !strings.Contains(err.Error(), "actor.char.guard.name") {
		t.Fatalf("unnamed shared talk target: %v", err)
	}
	name := LocalizationMessage{Key: "actor.char.guard.name", Context: "Guard's role", Texts: map[string]string{"ru": "Дежурный"}}
	c.Localization.Messages = append(c.Localization.Messages, name)
	if err := checkMemory(c); err == nil || !strings.Contains(err.Error(), "actor.char.guard.name") {
		t.Fatalf("shared talk target with no English name: %v", err)
	}
	name.Texts["en"] = "Guard"
	if err := checkMemory(c); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryChecksRequireFollowUpToReachAnExit(t *testing.T) {
	c := memoryWorld(t)
	d := c.Conversations["dlg.trader"]
	d.Nodes["open"].Options[2].FollowUp = "later"
	d.Nodes["later"] = &ConvNode{AfterDecision: true, Options: []ConvOption{{ID: "leave", Text: "t.bye", Close: "bye"}}}
	if err := checkMemory(c); err != nil {
		t.Fatal(err)
	}
	d.Nodes["later"].Options[0].Close = ""
	if err := checkMemory(c); err == nil || !strings.Contains(err.Error(), "no way out of this node") {
		t.Fatalf("follow-up without an exit: %v", err)
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

func TestMemoryVisibleReaders(t *testing.T) {
	for _, kind := range []string{"effect", "waits", "heard", "external", "journal"} {
		t.Run(kind, func(t *testing.T) {
			c := memoryWorld(t)
			f := c.Memory.Facts["fact.trader.met"]
			f.Journal = ""
			c.Conversations["dlg.trader"].Entries = c.Conversations["dlg.trader"].Entries[1:]
			c.Memory.Interjections = nil
			switch kind {
			case "effect":
				c.Conversations["dlg.trader"].Nodes["open"].Options[0].Do = append(c.Conversations["dlg.trader"].Nodes["open"].Options[0].Do, Do{When: &When{Fact: &FactWhen{ID: "fact.trader.met"}}, Topic: "topic.trader_work"})
			case "waits":
				c.Memory.Interjections = []Interjection{{ID: "interject.met.guard", Text: "t.interject", Fact: "fact.trader.met", Speaker: "char.guard", Belief: "witness", Waits: "later"}}
			case "heard":
				c.Memory.Interjections = []Interjection{{ID: "interject.met.trader", Text: "t.interject", Fact: "fact.trader.met", Speaker: "char.trader", Belief: "heard"}}
			case "external":
				f.ExternalReaders = []string{"later system"}
			case "journal":
				f.Journal = "t.journal"
			}
			err := checkMemory(c)
			if kind == "journal" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "nothing reads it") {
				t.Fatalf("%s hidden-only reader passed: %v", kind, err)
			}
		})
	}
}
func TestMemoryWritersNeedReachableFacts(t *testing.T) {
	for _, kind := range []string{"effect", "answer", "entry", "cycle", "alternative"} {
		t.Run(kind, func(t *testing.T) {
			c := memoryWorld(t)
			c.Memory.Facts["fact.trader.never"] = &MemoryFact{Visibility: "private", MemoryDays: 14, Journal: "t.journal"}
			o := &c.Conversations["dlg.trader"].Nodes["open"].Options[0]
			gate := &When{Fact: &FactWhen{ID: "fact.trader.never"}}
			if kind == "answer" {
				o.When = gate
			} else {
				o.Do[1].When = gate
			}
			if kind == "entry" {
				o.Do = o.Do[:1]
				c.Conversations["dlg.trader"].Entries[0].When = gate
				c.Conversations["dlg.trader"].Entries[0].Node = "later"
				c.Conversations["dlg.trader"].Nodes["later"] = &ConvNode{Options: []ConvOption{{ID: "later_bye", Text: "t.bye", Close: "bye", Do: []Do{{Fact: "fact.trader.met"}}}}}
			}
			if kind == "cycle" {
				o.Do = append(o.Do, Do{Fact: "fact.trader.never", When: &When{Fact: &FactWhen{ID: "fact.trader.met"}}})
			}
			if kind == "alternative" {
				o.Do[1].When = &When{Any: []When{*gate, {NotFact: "fact.trader.met"}}}
				o.Do = append(o.Do, Do{Fact: "fact.trader.never"})
			}
			err := checkMemory(c)
			if kind == "alternative" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "writer cannot run") || !strings.Contains(err.Error(), "fact.trader.never") {
				t.Fatalf("unreachable writer accepted: %v", err)
			}
		})
	}
}

func TestMemoryDeclaredScriptReaderIsVisible(t *testing.T) {
	c := memoryWorld(t)
	c.memoryScripts = map[string]string{"01": "01-prokhor-week.md"}
	f := c.Memory.Facts["fact.trader.met"]
	f.Journal = ""
	f.ExternalReaders = []string{"01"}
	c.Memory.Interjections = nil
	c.Conversations["dlg.trader"].Entries = c.Conversations["dlg.trader"].Entries[1:]
	if err := checkMemory(c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(c.linksPage), "читается только в сценарии") || !strings.Contains(string(c.linksPage), "сценарий 01") {
		t.Fatal("links page does not mark the script-only fact")
	}
	// Losing the declared reader makes this fact dead again; a prose annotation cannot replace it.
	for _, readers := range [][]string{nil, {"later system"}} {
		f.ExternalReaders = readers
		if err := checkMemory(c); err == nil || !strings.Contains(err.Error(), "nothing reads it") {
			t.Fatalf("undeclared reader accepted: %v", err)
		}
	}
	f.ExternalReaders = []string{"99"}
	if err := checkMemory(c); err == nil || !strings.Contains(err.Error(), "unknown external reader script 99") {
		t.Fatalf("missing script accepted: %v", err)
	}
	// The same declaration surface is accepted for a topic.
	f.ExternalReaders = []string{"01"}
	c.Memory.Topics["topic.trader_work"].ExternalReaders = []string{"01"}
	if err := checkMemory(c); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryScriptRegistry(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "design", "dialogue", "scripts")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"01-prokhor.md", "26-hero.md", "README.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("script"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := readMemoryScripts(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids["01"] != "01-prokhor.md" {
		t.Fatalf("wrong registry: %v", ids)
	}
	if err := os.WriteFile(filepath.Join(dir, "01-duplicate.md"), []byte("script"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readMemoryScripts(root); err == nil {
		t.Fatal("duplicate script id accepted")
	}
}
