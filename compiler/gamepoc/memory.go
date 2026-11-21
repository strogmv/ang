package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
)

// Story memory (doc 103 step 1, owner 20.11; design/dialogue/memory-v1): conversations are not linked to each other but
// through the world's memory — facts (what happened, who knows it), the hero's topics (what he learned in talk) and
// interjections (a person who knows a fresh fact says it first in any conversation of his). One condition and effect
// language (When, Do) serves conversations, voices and interjections; one server executor runs every conversation
// (templates/conversation.go.tmpl). Here: reading, the checks that keep the links whole (checkMemory) and the links page.

// Memory is world.memory.
type Memory struct {
	Facts             map[string]*MemoryFact  `json:"facts"`
	Topics            map[string]*MemoryTopic `json:"topics"`
	Interjections     []Interjection          `json:"interjections"`
	InterjectionRules struct {
		FreshDays   int     `json:"fresh_days"`
		HeardBelief float64 `json:"heard_belief"`
	} `json:"interjection_rules"`
	Talks []MemoryTalk `json:"talks"`
}

// MemoryFact is one significant event of the hero's story, defined ahead like an item (doc 103 §1).
type MemoryFact struct {
	Visibility      string              `json:"visibility"`
	Recorded        bool                `json:"recorded"`
	MemoryDays      int                 `json:"memory_days"`
	Args            map[string][]string `json:"args"`
	KnownBy         []string            `json:"known_by"`
	SightCells      int                 `json:"sight_cells"`
	HearingCells    int                 `json:"hearing_cells"`
	Rumour          bool                `json:"rumour"`
	Journal         string              `json:"journal,omitempty"`
	Mention         string              `json:"mention,omitempty"`
	ExternalWriters []string            `json:"external_writers"`
	ExternalReaders []string            `json:"external_readers"`
	OnTrade         *struct {
		Seller string            `json:"seller"`
		Args   map[string]string `json:"args"`
	} `json:"on_trade,omitempty"`
}

// MemoryTopic is a piece of knowledge the hero heard in talk; it opens answers elsewhere (doc 103 §2).
type MemoryTopic struct {
	Journal         string   `json:"journal"`
	FromBarks       []string `json:"from_barks"`
	HearingCells    int      `json:"hearing_cells"`
	ExternalReaders []string `json:"external_readers"`
}

// Interjection: a short line a person says first in any conversation of his when he knows a fresh fact (doc 103 §4.3).
type Interjection struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Fact    string `json:"fact"`
	Speaker string `json:"speaker"`
	Belief  string `json:"belief"`
	When    *When  `json:"when,omitempty"`
	Do      []Do   `json:"do"`
	Voice   *struct {
		Skill   string `json:"skill"`
		AtLeast int    `json:"at_least"`
		Text    string `json:"text"`
		When    *When  `json:"when,omitempty"`
	} `json:"voice,omitempty"`
	Waits string `json:"waits,omitempty"`
}

// MemoryTalk: who opens a shared conversation, and whether only when he has something to say (news).
type MemoryTalk struct {
	Conversation string   `json:"conversation"`
	People       []string `json:"people"`
	Open         string   `json:"open"`
}

// When is a condition: every field set holds; Any — one of; Not — negation.
type When struct {
	Fact     *FactWhen `json:"fact,omitempty"`
	NotFact  string    `json:"not_fact,omitempty"`
	Topic    string    `json:"topic,omitempty"`
	NotTopic string    `json:"not_topic,omitempty"`
	Knows    *struct {
		Who  string `json:"who"`
		Fact string `json:"fact"`
	} `json:"knows,omitempty"`
	Trust *struct {
		Who     string   `json:"who"`
		AtLeast *float64 `json:"at_least,omitempty"`
		Below   *float64 `json:"below,omitempty"`
	} `json:"trust,omitempty"`
	Skill *struct {
		ID      string `json:"id"`
		AtLeast int    `json:"at_least"`
	} `json:"skill,omitempty"`
	Day *struct {
		From int `json:"from,omitempty"`
		To   int `json:"to,omitempty"`
	} `json:"day,omitempty"`
	Asked       string    `json:"asked,omitempty"`
	NotAsked    string    `json:"not_asked,omitempty"`
	Voiced      string    `json:"voiced,omitempty"`
	Entry       string    `json:"entry,omitempty"`
	Interjected *bool     `json:"interjected,omitempty"`
	Speaker     []string  `json:"speaker,omitempty"`
	Caste       []string  `json:"caste,omitempty"`
	Item        *WhenItem `json:"item,omitempty"`
	World       string    `json:"world,omitempty"`
	SeenBy      string    `json:"seen_by,omitempty"`
	NotSeenBy   string    `json:"not_seen_by,omitempty"`
	Any         []When    `json:"any,omitempty"`
	Not         *When     `json:"not,omitempty"`
}

// FactWhen: the fact was written (with these arguments, recently, or first before today). CUE allows the bare id.
type FactWhen struct {
	ID          string            `json:"id"`
	Args        map[string]string `json:"args,omitempty"`
	WithinHours int               `json:"within_hours,omitempty"`
	BeforeToday bool              `json:"before_today,omitempty"`
}

func (f *FactWhen) UnmarshalJSON(b []byte) error {
	var id string
	if err := json.Unmarshal(b, &id); err == nil {
		*f = FactWhen{ID: id}
		return nil
	}
	type plain FactWhen
	var p plain
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return err
	}
	*f = FactWhen(p)
	return nil
}

type WhenItem struct {
	Owner            string   `json:"owner"`
	Item             string   `json:"item"`
	ConditionBelow   *float64 `json:"condition_below,omitempty"`
	ConditionAtLeast *float64 `json:"condition_at_least,omitempty"`
}

// Do is one effect, with an optional condition.
type Do struct {
	When  *When             `json:"when,omitempty"`
	Fact  string            `json:"fact,omitempty"`
	Args  map[string]string `json:"args,omitempty"`
	Topic string            `json:"topic,omitempty"`
	Trust *struct {
		Who   string  `json:"who"`
		Delta float64 `json:"delta"`
	} `json:"trust,omitempty"`
	GiveItem *struct {
		Item string `json:"item"`
		From string `json:"from"`
	} `json:"give_item,omitempty"`
	ItemCondition *struct {
		Owner string  `json:"owner"`
		Item  string  `json:"item"`
		Set   float64 `json:"set"`
	} `json:"item_condition,omitempty"`
}

// Conversation is world.conversations[id] (#Conversation).
type Conversation struct {
	ID       string   `json:"id"`
	Event    bool     `json:"event"`
	Speakers []string `json:"speakers"`
	Reach    int      `json:"reach"`
	// the talk's clock (owner 21.11): while the column is open the world waits; these are real seconds of the open talk
	AnswerSeconds int                   `json:"answer_seconds,omitempty"`
	LingerSeconds int                   `json:"linger_seconds"`
	Entries       []ConvEntry           `json:"entries"`
	Nodes         map[string]*ConvNode  `json:"nodes"`
	Events        map[string]*ConvEvent `json:"events"`
	Waits         string                `json:"waits,omitempty"`
}
type ConvEntry struct {
	ID    string     `json:"id"`
	When  *When      `json:"when,omitempty"`
	Node  string     `json:"node"`
	Lines []ConvLine `json:"lines"`
}
type ConvNode struct {
	Lines   []ConvLine   `json:"lines"`
	Voices  []ConvVoice  `json:"voices"`
	Options []ConvOption `json:"options"`
}
type ConvVariant struct {
	When When   `json:"when"`
	Text string `json:"text"`
}
type ConvLine struct {
	Speaker  string        `json:"speaker"`
	Text     string        `json:"text"`
	When     *When         `json:"when,omitempty"`
	Variants []ConvVariant `json:"variants"`
}
type ConvVoice struct {
	ID       string        `json:"id"`
	Skill    string        `json:"skill"`
	AtLeast  int           `json:"at_least"`
	Text     string        `json:"text"`
	When     *When         `json:"when,omitempty"`
	Variants []ConvVariant `json:"variants"`
	Do       []Do          `json:"do"`
}
type ConvOption struct {
	ID       string         `json:"id"`
	Text     string         `json:"text"`
	Variants []ConvVariant  `json:"variants"`
	When     *When          `json:"when,omitempty"`
	Once     bool           `json:"once"`
	Replaces string         `json:"replaces,omitempty"`
	Reply    []ConvLine     `json:"reply"`
	Do       []Do           `json:"do"`
	Next     string         `json:"next,omitempty"`
	Close    string         `json:"close,omitempty"`
	Trade    bool           `json:"trade"`
	Check    *DialogueCheck `json:"check,omitempty"`
	Fail     *ConvFail      `json:"fail,omitempty"`
}
type ConvFail struct {
	Reply []ConvLine `json:"reply"`
	Do    []Do       `json:"do"`
	Next  string     `json:"next,omitempty"`
	Close string     `json:"close,omitempty"`
}
type ConvEvent struct {
	Lines []ConvLine `json:"lines"`
	Do    []Do       `json:"do"`
	Close string     `json:"close"`
}

func decodeStrict(v cue.Value, where string, out any) error {
	raw, err := v.MarshalJSON()
	if err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	return nil
}

func readMemory(world cue.Value) (*Memory, map[string]*Conversation, error) {
	var m *Memory
	if v := world.LookupPath(cue.ParsePath("memory")); v.Exists() {
		m = &Memory{}
		if err := decodeStrict(v, "world.memory", m); err != nil {
			return nil, nil, err
		}
	}
	var convs map[string]*Conversation
	if v := world.LookupPath(cue.ParsePath("conversations")); v.Exists() {
		if err := decodeStrict(v, "world.conversations", &convs); err != nil {
			return nil, nil, err
		}
	}
	if len(convs) > 0 && m == nil {
		return nil, nil, fmt.Errorf("world.conversations: conversations need world.memory")
	}
	return m, convs, nil
}

var memoryWord = regexp.MustCompile(`^[a-z_]+$`)

// memoryLinks: who writes and who reads every fact and topic, and what every conversation opens and closes; the checks
// fill it and the links page shows it.
type memoryLinks struct {
	factWriters, factReaders, topicWriters, topicReaders map[string][]string
	opened                                               map[string]string
	closes                                               map[string][]string
	people                                               map[string][]string
}

func (l *memoryLinks) add(m map[string][]string, id, where string) {
	for _, w := range m[id] {
		if w == where {
			return
		}
	}
	m[id] = append(m[id], where)
}

// memoryChecker holds what the checks look ids up in.
type memoryChecker struct {
	c        *Compiled
	m        *Memory
	persons  map[string]bool
	castes   map[string]bool
	links    *memoryLinks
	messages map[string]bool
}

func (k *memoryChecker) text(where, key string) error {
	if key == "" || !k.messages[key] {
		return fmt.Errorf("%s: missing localization %q", where, key)
	}
	return nil
}

func (k *memoryChecker) person(where, id string, self bool) error {
	if id == "self" && self {
		return nil
	}
	if !k.persons[id] {
		return fmt.Errorf("%s: unknown person %q", where, id)
	}
	return nil
}

// factArgs: every argument the fact declares, with one of its values; nothing it does not declare.
func (k *memoryChecker) factArgs(where, fact string, args map[string]string, all bool) error {
	f := k.m.Facts[fact]
	for name, value := range args {
		values, ok := f.Args[name]
		if !ok {
			return fmt.Errorf("%s: fact %s has no argument %s", where, fact, name)
		}
		found := false
		for _, v := range values {
			found = found || v == value
		}
		if !found {
			return fmt.Errorf("%s: fact %s argument %s has no value %q", where, fact, name, value)
		}
	}
	if all {
		for name := range f.Args {
			if _, ok := args[name]; !ok {
				return fmt.Errorf("%s: writing fact %s needs its argument %s", where, fact, name)
			}
		}
	}
	return nil
}

// convScope is a conversation the condition lives in (nil for an interjection): its answers, voices and entries.
type convScope struct {
	id                       string
	options, voices, entries map[string]bool
	self                     bool
}

func (k *memoryChecker) when(where string, w *When, s *convScope, reader string) error {
	if w == nil {
		return nil
	}
	fact := func(id string) error {
		if k.m.Facts[id] == nil {
			return fmt.Errorf("%s: unknown fact %s", where, id)
		}
		k.links.add(k.links.factReaders, id, reader)
		return nil
	}
	topic := func(id string) error {
		if k.m.Topics[id] == nil {
			return fmt.Errorf("%s: unknown topic %s", where, id)
		}
		k.links.add(k.links.topicReaders, id, reader)
		return nil
	}
	if w.Fact != nil {
		if err := fact(w.Fact.ID); err != nil {
			return err
		}
		if err := k.factArgs(where, w.Fact.ID, w.Fact.Args, false); err != nil {
			return err
		}
	}
	if w.NotFact != "" {
		if err := fact(w.NotFact); err != nil {
			return err
		}
	}
	for _, id := range []string{w.Topic, w.NotTopic} {
		if id != "" {
			if err := topic(id); err != nil {
				return err
			}
		}
	}
	if w.Knows != nil {
		if err := k.person(where, w.Knows.Who, s != nil && s.self); err != nil {
			return err
		}
		if err := fact(w.Knows.Fact); err != nil {
			return err
		}
	}
	if w.Trust != nil {
		if err := k.person(where, w.Trust.Who, s != nil && s.self); err != nil {
			return err
		}
		if w.Trust.AtLeast == nil && w.Trust.Below == nil {
			return fmt.Errorf("%s: a trust condition needs at_least or below", where)
		}
	}
	if w.Skill != nil {
		if k.c.DialogueRules == nil {
			return fmt.Errorf("%s: a skill condition needs world.dialogue_rules", where)
		}
		if _, ok := k.c.DialogueRules.Skills[w.Skill.ID]; !ok {
			return fmt.Errorf("%s: unknown skill %s", where, w.Skill.ID)
		}
	}
	if w.Day != nil && (w.Day.From == 0 && w.Day.To == 0 || w.Day.To != 0 && w.Day.To < w.Day.From) {
		return fmt.Errorf("%s: an empty day range", where)
	}
	inScope := func(kind, id string, set func(*convScope) map[string]bool) error {
		if id == "" {
			return nil
		}
		if s == nil || !set(s)[id] {
			return fmt.Errorf("%s: %s %q is not of this conversation", where, kind, id)
		}
		return nil
	}
	for _, check := range []struct {
		kind, id string
		set      func(*convScope) map[string]bool
	}{
		{"answer", w.Asked, func(s *convScope) map[string]bool { return s.options }},
		{"answer", w.NotAsked, func(s *convScope) map[string]bool { return s.options }},
		{"voice", w.Voiced, func(s *convScope) map[string]bool { return s.voices }},
		{"entry", w.Entry, func(s *convScope) map[string]bool { return s.entries }},
	} {
		if err := inScope(check.kind, check.id, check.set); err != nil {
			return err
		}
	}
	if (len(w.Speaker) > 0 || len(w.Caste) > 0 || w.Interjected != nil) && (s == nil || !s.self) {
		return fmt.Errorf("%s: speaker, caste and interjected need a conversation opened with a person", where)
	}
	for _, id := range w.Speaker {
		if err := k.person(where, id, false); err != nil {
			return err
		}
	}
	for _, caste := range w.Caste {
		if !k.castes[caste] {
			return fmt.Errorf("%s: unknown caste %s", where, caste)
		}
	}
	if w.Item != nil {
		if w.Item.Owner != "hero" {
			if err := k.person(where, w.Item.Owner, s != nil && s.self); err != nil {
				return err
			}
		}
		if _, ok := k.c.Inventory.Definitions[w.Item.Item]; !ok {
			return fmt.Errorf("%s: unknown item %s", where, w.Item.Item)
		}
	}
	switch w.World {
	case "", "passage_open", "self_trades":
	default:
		return fmt.Errorf("%s: unknown world query %s", where, w.World)
	}
	for _, id := range []string{w.SeenBy, w.NotSeenBy} {
		if id != "" {
			if err := k.person(where, id, false); err != nil {
				return err
			}
		}
	}
	for i := range w.Any {
		if err := k.when(fmt.Sprintf("%s.any[%d]", where, i), &w.Any[i], s, reader); err != nil {
			return err
		}
	}
	if len(w.Any) == 1 {
		return fmt.Errorf("%s: any of one condition", where)
	}
	return k.when(where+".not", w.Not, s, reader)
}

func (k *memoryChecker) do(where string, list []Do, s *convScope, writer string) error {
	for i, d := range list {
		at := fmt.Sprintf("%s.do[%d]", where, i)
		n := 0
		for _, set := range []bool{d.Fact != "", d.Topic != "", d.Trust != nil, d.GiveItem != nil, d.ItemCondition != nil} {
			if set {
				n++
			}
		}
		if n != 1 {
			return fmt.Errorf("%s: an effect does exactly one thing", at)
		}
		if len(d.Args) > 0 && d.Fact == "" {
			return fmt.Errorf("%s: arguments belong to a fact", at)
		}
		if err := k.when(at+".when", d.When, s, writer); err != nil {
			return err
		}
		self := s != nil && s.self
		switch {
		case d.Fact != "":
			if k.m.Facts[d.Fact] == nil {
				return fmt.Errorf("%s: unknown fact %s", at, d.Fact)
			}
			if err := k.factArgs(at, d.Fact, d.Args, true); err != nil {
				return err
			}
			k.links.add(k.links.factWriters, d.Fact, writer)
		case d.Topic != "":
			if k.m.Topics[d.Topic] == nil {
				return fmt.Errorf("%s: unknown topic %s", at, d.Topic)
			}
			k.links.add(k.links.topicWriters, d.Topic, writer)
		case d.Trust != nil:
			if err := k.person(at, d.Trust.Who, self); err != nil {
				return err
			}
		case d.GiveItem != nil:
			if err := k.person(at, d.GiveItem.From, self); err != nil {
				return err
			}
			if _, ok := k.c.Inventory.Definitions[d.GiveItem.Item]; !ok {
				return fmt.Errorf("%s: unknown item %s", at, d.GiveItem.Item)
			}
		case d.ItemCondition != nil:
			if err := k.person(at, d.ItemCondition.Owner, self); err != nil {
				return err
			}
			if _, ok := k.c.Inventory.Definitions[d.ItemCondition.Item]; !ok {
				return fmt.Errorf("%s: unknown item %s", at, d.ItemCondition.Item)
			}
		}
	}
	return nil
}

func (k *memoryChecker) lines(where string, lines []ConvLine, speakers map[string]bool, s *convScope, reader string) error {
	for i, l := range lines {
		at := fmt.Sprintf("%s.lines[%d]", where, i)
		if l.Speaker == "self" {
			if s == nil || !s.self {
				return fmt.Errorf("%s: self speaks only in a conversation opened with a person", at)
			}
		} else if !speakers[l.Speaker] {
			return fmt.Errorf("%s: %s does not take part in this conversation", at, l.Speaker)
		}
		if err := k.variants(at, l.Text, l.Variants, s, reader); err != nil {
			return err
		}
		if err := k.when(at+".when", l.When, s, reader); err != nil {
			return err
		}
	}
	return nil
}

func (k *memoryChecker) variants(where, text string, variants []ConvVariant, s *convScope, reader string) error {
	if err := k.text(where, text); err != nil {
		return err
	}
	for i, v := range variants {
		at := fmt.Sprintf("%s.variants[%d]", where, i)
		if err := k.text(at, v.Text); err != nil {
			return err
		}
		if err := k.when(at+".when", &v.When, s, reader); err != nil {
			return err
		}
	}
	return nil
}

func (k *memoryChecker) conversation(c *Conversation) error {
	where := "world.conversations." + c.ID
	if len(c.Entries) == 0 || len(c.Nodes) == 0 {
		return fmt.Errorf("%s: needs entries and nodes", where)
	}
	if c.Reach < 1 || c.LingerSeconds < 0 || c.LingerSeconds > 120 || c.AnswerSeconds < 0 || c.AnswerSeconds > 600 {
		return fmt.Errorf("%s: invalid reach, linger or answer time", where)
	}
	if c.AnswerSeconds > 0 && c.Events["timeout"] == nil {
		return fmt.Errorf("%s: a talk that waits for an answer needs its timeout event", where)
	}
	speakers := map[string]bool{}
	for _, id := range c.Speakers {
		if err := k.person(where, id, false); err != nil {
			return err
		}
		speakers[id] = true
	}
	s := &convScope{id: c.ID, options: map[string]bool{}, voices: map[string]bool{}, entries: map[string]bool{}, self: true}
	for _, e := range c.Entries {
		if s.entries[e.ID] {
			return fmt.Errorf("%s: repeated entry %s", where, e.ID)
		}
		s.entries[e.ID] = true
	}
	nodes := sortedKeys(c.Nodes)
	for _, id := range nodes {
		n := c.Nodes[id]
		if !memoryWord.MatchString(id) || len(n.Options) == 0 {
			return fmt.Errorf("%s.%s: a node needs answers", where, id)
		}
		for _, o := range n.Options {
			if s.options[o.ID] {
				return fmt.Errorf("%s: answer %s twice (answers are remembered by id per conversation)", where, o.ID)
			}
			s.options[o.ID] = true
		}
		for _, v := range n.Voices {
			if s.voices[v.ID] {
				return fmt.Errorf("%s: voice %s twice", where, v.ID)
			}
			s.voices[v.ID] = true
		}
	}
	reader := "разговор " + c.ID
	if c.Waits != "" {
		reader = "ждёт: " + reader
	}
	// entries: the first that holds; the last one without a condition is «usual», unless an event opens it in a window
	for i, e := range c.Entries {
		at := fmt.Sprintf("%s.entries.%s", where, e.ID)
		if c.Nodes[e.Node] == nil {
			return fmt.Errorf("%s: unknown node %s", at, e.Node)
		}
		if e.When == nil && i != len(c.Entries)-1 {
			return fmt.Errorf("%s: an entry without a condition must be the last (the ones after it are never taken)", at)
		}
		if err := k.when(at+".when", e.When, s, reader+" (вход "+e.ID+")"); err != nil {
			return err
		}
		if err := k.lines(at, e.Lines, speakers, s, reader); err != nil {
			return err
		}
	}
	if last := c.Entries[len(c.Entries)-1]; !c.Event && last.When != nil {
		return fmt.Errorf("%s: no «usual» entry (the last without a condition); a conversation opened only in a window is event: true", where)
	}
	writer := func(what string) string { return reader + ": " + what }
	closes := map[string]bool{}
	for _, id := range nodes {
		n := c.Nodes[id]
		at := where + "." + id
		if err := k.lines(at, n.Lines, speakers, s, reader); err != nil {
			return err
		}
		for _, v := range n.Voices {
			vat := at + ".voice." + v.ID
			if k.c.DialogueRules == nil {
				return fmt.Errorf("%s: a voice needs world.dialogue_rules", vat)
			}
			if _, ok := k.c.DialogueRules.Skills[v.Skill]; !ok || v.AtLeast < 0 || v.AtLeast > 100 {
				return fmt.Errorf("%s: unknown skill %s or threshold %d", vat, v.Skill, v.AtLeast)
			}
			if err := k.variants(vat, v.Text, v.Variants, s, reader); err != nil {
				return err
			}
			if err := k.when(vat+".when", v.When, s, reader+" (голос "+v.ID+")"); err != nil {
				return err
			}
			if err := k.do(vat, v.Do, s, writer("голос "+v.ID)); err != nil {
				return err
			}
		}
		for _, o := range n.Options {
			oat := at + "." + o.ID
			if !memoryWord.MatchString(o.ID) {
				return fmt.Errorf("%s: answer id", oat)
			}
			if err := k.variants(oat, o.Text, o.Variants, s, reader); err != nil {
				return err
			}
			if err := k.when(oat+".when", o.When, s, reader+" (ответ "+o.ID+")"); err != nil {
				return err
			}
			if err := k.lines(oat+".reply", o.Reply, speakers, s, reader); err != nil {
				return err
			}
			if err := k.do(oat, o.Do, s, writer("ответ "+o.ID)); err != nil {
				return err
			}
			if o.Next != "" && c.Nodes[o.Next] == nil {
				return fmt.Errorf("%s: unknown node %s", oat, o.Next)
			}
			if o.Trade && (o.Next != "" || o.Close != "" || o.Check != nil) {
				return fmt.Errorf("%s: a trade answer opens the trade window and nothing else", oat)
			}
			if o.Next != "" && o.Close != "" {
				return fmt.Errorf("%s: an answer goes on or closes, not both", oat)
			}
			if o.Close != "" {
				closes[o.Close] = true
			}
			if (o.Check == nil) != (o.Fail == nil) {
				return fmt.Errorf("%s: a skill check needs its failure, and only a check has one", oat)
			}
			if o.Check != nil {
				if err := k.c.DialogueRules.skillCheck(oat, *o.Check); err != nil {
					return err
				}
				// failure is content too (doc 103 §8а): its own lines and a consequence, never «the end with nothing»
				f := o.Fail
				if len(f.Reply) == 0 || len(f.Do) == 0 && f.Next == "" && f.Close == "" {
					return fmt.Errorf("%s: the failure of a check needs its lines and a consequence (an effect, a node or an outcome)", oat)
				}
				if f.Next != "" && c.Nodes[f.Next] == nil {
					return fmt.Errorf("%s.fail: unknown node %s", oat, f.Next)
				}
				if f.Next != "" && f.Close != "" {
					return fmt.Errorf("%s.fail: goes on or closes, not both", oat)
				}
				if (o.Close == "") != (f.Close == "") {
					return fmt.Errorf("%s: a closing answer closes on failure too", oat)
				}
				if f.Close != "" {
					closes[f.Close] = true
				}
				if err := k.lines(oat+".fail.reply", f.Reply, speakers, s, reader); err != nil {
					return err
				}
				if err := k.do(oat+".fail", f.Do, s, writer("провал "+o.ID)); err != nil {
					return err
				}
			}
		}
	}
	for _, id := range sortedKeys(c.Events) {
		e := c.Events[id]
		at := where + ".events." + id
		if !memoryWord.MatchString(e.Close) {
			return fmt.Errorf("%s: an event closes with a word", at)
		}
		closes[e.Close] = true
		if err := k.lines(at, e.Lines, speakers, s, reader); err != nil {
			return err
		}
		if err := k.do(at, e.Do, s, writer("событие "+id)); err != nil {
			return err
		}
	}
	// every node is reached from an entry, and from every node a closing answer is reachable (no dead end)
	reached := map[string]bool{}
	queue := []string{}
	for _, e := range c.Entries {
		if !reached[e.Node] {
			reached[e.Node] = true
			queue = append(queue, e.Node)
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, o := range c.Nodes[id].Options {
			for _, next := range []string{o.Next, failNext(o)} {
				if next != "" && !reached[next] {
					reached[next] = true
					queue = append(queue, next)
				}
			}
		}
	}
	exits := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, id := range nodes {
			if exits[id] {
				continue
			}
			for _, o := range c.Nodes[id].Options {
				if o.Close != "" || o.Trade || o.Next != "" && exits[o.Next] {
					exits[id], changed = true, true
					break
				}
			}
		}
	}
	for _, id := range nodes {
		if !reached[id] {
			return fmt.Errorf("%s.%s: no entry or answer leads to this node", where, id)
		}
		if !exits[id] {
			return fmt.Errorf("%s.%s: no way out of this node (no closing answer is reachable)", where, id)
		}
	}
	// at most nine answers on screen (the keys 1–9, owner 21.11): no two answers whose conditions can hold together
	// make more than nine, an answer that replaces another counting once
	for _, id := range nodes {
		n := c.Nodes[id]
		names := map[string]bool{}
		for _, o := range n.Options {
			names[o.ID] = true
		}
		for _, o := range n.Options {
			if o.Replaces != "" && (!names[o.Replaces] || o.Replaces == o.ID) {
				return fmt.Errorf("%s.%s.%s: replaces %q, no other answer of this node", where, id, o.ID, o.Replaces)
			}
		}
		if most, together := maxAnswersShown(n.Options); most > 9 {
			return fmt.Errorf("%s.%s: up to %d answers on screen at once (%s); the keys reach 9 — fold one, or let a fact's answer replace the general one", where, id, most, strings.Join(together, ", "))
		}
	}
	k.links.closes[c.ID] = sortedKeys(closes)
	return nil
}

// literals: what an answer's condition requires at its top level — a fact or topic there or not, the entry, the
// speaker or caste. Two answers whose requirements contradict are never on screen together.
func answerLiterals(w *When) map[string]string {
	out := map[string]string{}
	if w == nil {
		return out
	}
	if w.Fact != nil {
		out["fact:"+w.Fact.ID] = "yes"
	}
	if w.NotFact != "" {
		out["fact:"+w.NotFact] = "no"
	}
	if w.Topic != "" {
		out["topic:"+w.Topic] = "yes"
	}
	if w.NotTopic != "" {
		out["topic:"+w.NotTopic] = "no"
	}
	if w.Entry != "" {
		out["entry"] = w.Entry
	}
	if len(w.Speaker) == 1 {
		out["speaker"] = w.Speaker[0]
	}
	if len(w.Caste) == 1 {
		out["caste"] = w.Caste[0]
	}
	return out
}

// maxAnswersShown: the largest set of a node's answers that can be on screen together (brute force over the node's
// answers, a node holds few).
func maxAnswersShown(options []ConvOption) (int, []string) {
	n := len(options)
	if n > 20 {
		return n, nil
	}
	lits := make([]map[string]string, n)
	for i := range options {
		lits[i] = answerLiterals(options[i].When)
	}
	apart := func(a, b int) bool {
		if options[a].Replaces == options[b].ID || options[b].Replaces == options[a].ID {
			return true
		}
		for key, v := range lits[a] {
			if w, ok := lits[b][key]; ok && w != v {
				return true
			}
		}
		return false
	}
	best, bestSet := 0, 0
	for set := 1; set < 1<<n; set++ {
		size := 0
		ok := true
		for a := 0; a < n && ok; a++ {
			if set&(1<<a) == 0 {
				continue
			}
			size++
			for b := a + 1; b < n; b++ {
				if set&(1<<b) != 0 && apart(a, b) {
					ok = false
					break
				}
			}
		}
		if ok && size > best {
			best, bestSet = size, set
		}
	}
	names := []string{}
	for a := 0; a < n; a++ {
		if bestSet&(1<<a) != 0 {
			names = append(names, options[a].ID)
		}
	}
	return best, names
}

func failNext(o ConvOption) string {
	if o.Fail == nil {
		return ""
	}
	return o.Fail.Next
}

// checkMemory: every fact, topic, person, item, skill, answer, node, voice and entry a condition or an effect names exists;
// every fact and topic is written somewhere and read somewhere (an outside writer or reader is marked with its reason);
// every conversation has an owner that opens it (or waits for its place), a «usual» entry unless it is an event, a way
// out of every node and content in every failure; every text is localized. It runs after the barks are compiled (castes,
// bark lines) and fills the links page.
func checkMemory(c *Compiled) error {
	m := c.Memory
	if m == nil {
		return nil
	}
	var life struct {
		Actors []struct {
			ID string `json:"id"`
		} `json:"actors"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	k := &memoryChecker{c: c, m: m, persons: map[string]bool{}, castes: map[string]bool{}, messages: map[string]bool{},
		links: &memoryLinks{factWriters: map[string][]string{}, factReaders: map[string][]string{}, topicWriters: map[string][]string{},
			topicReaders: map[string][]string{}, opened: map[string]string{}, closes: map[string][]string{}, people: map[string][]string{}}}
	for _, a := range life.Actors {
		k.persons[a.ID] = true
	}
	if c.Barks != nil {
		for _, caste := range c.Barks.Castes {
			k.castes[caste] = true
		}
	}
	for _, msg := range c.Localization.Messages {
		k.messages[msg.Key] = true
	}
	if m.InterjectionRules.FreshDays < 1 || m.InterjectionRules.HeardBelief <= 0 || m.InterjectionRules.HeardBelief >= 1 {
		return fmt.Errorf("world.memory.interjection_rules: fresh days and the heard belief")
	}
	// facts: the visibility and who knows at once agree; texts localized; outside writers and readers count
	for _, id := range sortedKeys(m.Facts) {
		f := m.Facts[id]
		where := "world.memory.facts." + id
		witnesses := f.SightCells > 0 || f.HearingCells > 0
		switch f.Visibility {
		case "private":
			if witnesses || f.Recorded || f.Rumour {
				return fmt.Errorf("%s: a private fact has no witnesses, record or rumour", where)
			}
		case "witnessed":
			if !witnesses && len(f.KnownBy) == 0 {
				return fmt.Errorf("%s: a witnessed fact needs who sees or hears it", where)
			}
		case "recorded":
			if !f.Recorded {
				return fmt.Errorf("%s: a recorded fact is in the record", where)
			}
		case "public":
		default:
			return fmt.Errorf("%s: unknown visibility %s", where, f.Visibility)
		}
		if f.MemoryDays < 1 {
			return fmt.Errorf("%s: memory days", where)
		}
		for name, values := range f.Args {
			if !memoryWord.MatchString(name) || len(values) == 0 {
				return fmt.Errorf("%s: argument %s", where, name)
			}
		}
		for _, who := range f.KnownBy {
			if err := k.person(where, who, true); err != nil {
				return err
			}
		}
		for _, key := range []string{f.Journal, f.Mention} {
			if key != "" {
				if err := k.text(where, key); err != nil {
					return err
				}
			}
		}
		for _, w := range f.ExternalWriters {
			k.links.add(k.links.factWriters, id, "вне: "+w)
		}
		for _, r := range f.ExternalReaders {
			k.links.add(k.links.factReaders, id, "вне: "+r)
		}
		if f.OnTrade != nil {
			if err := k.person(where, f.OnTrade.Seller, false); err != nil {
				return err
			}
			if err := k.factArgs(where+".on_trade", id, f.OnTrade.Args, true); err != nil {
				return err
			}
			k.links.add(k.links.factWriters, id, "покупка у "+f.OnTrade.Seller)
		}
	}
	for _, id := range sortedKeys(m.Topics) {
		t := m.Topics[id]
		where := "world.memory.topics." + id
		if err := k.text(where, t.Journal); err != nil {
			return err
		}
		for _, line := range t.FromBarks {
			if c.Barks == nil {
				return fmt.Errorf("%s: heard barks need the barks catalog", where)
			}
			if _, _, ok := c.Barks.Line(line); !ok {
				return fmt.Errorf("%s: unknown bark line %s", where, line)
			}
			k.links.add(k.links.topicWriters, id, "услышанный барк "+line)
		}
		if t.HearingCells < 1 {
			return fmt.Errorf("%s: hearing cells", where)
		}
		for _, r := range t.ExternalReaders {
			k.links.add(k.links.topicReaders, id, "вне: "+r)
		}
	}
	// who opens which conversation
	talkOf := map[string]string{}
	for i, t := range m.Talks {
		where := fmt.Sprintf("world.memory.talks[%d]", i)
		conv := c.Conversations[t.Conversation]
		if conv == nil || conv.Event || conv.Waits != "" {
			return fmt.Errorf("%s: unknown, event or waiting conversation %s", where, t.Conversation)
		}
		if t.Open != "news" && t.Open != "always" {
			return fmt.Errorf("%s: open is news or always", where)
		}
		for _, p := range t.People {
			if err := k.person(where, p, false); err != nil {
				return err
			}
			if talkOf[p] != "" {
				return fmt.Errorf("%s: %s already talks in %s", where, p, talkOf[p])
			}
			talkOf[p] = t.Conversation
			k.links.add(k.links.people, p, t.Conversation)
		}
		k.links.opened[t.Conversation] = "«Поговорить» с человеком, когда ему есть что сказать (" + t.Open + ")"
	}
	if e := c.CalendarEncounter; e != nil {
		for _, slot := range []struct{ id, what string }{
			{e.Talk.Sack, "встреча: у мешка, дни 1–" + fmt.Sprint(e.FirstDays)},
			{e.Talk.Custody, "встреча: арест, день " + fmt.Sprint(e.FirstDays+e.MissingDays+1)},
			{e.Talk.Gate, "встреча: у ворот после побега"},
		} {
			if c.Conversations[slot.id] == nil {
				return fmt.Errorf("calendar encounter: unknown conversation %s", slot.id)
			}
			k.links.opened[slot.id] = slot.what
			k.links.add(k.links.people, e.Actor, slot.id)
		}
		for _, f := range []struct{ id, what string }{{e.Facts.Missing, "встреча: дни пропажи"}, {e.Facts.Fight, "встреча: бой с охранником во время ареста"}} {
			if m.Facts[f.id] == nil {
				return fmt.Errorf("calendar encounter: unknown fact %s", f.id)
			}
			k.links.add(k.links.factWriters, f.id, f.what)
		}
	}
	for _, id := range sortedKeys(c.Conversations) {
		conv := c.Conversations[id]
		if conv.ID != id {
			return fmt.Errorf("world.conversations.%s: id %s", id, conv.ID)
		}
		if conv.Waits != "" {
			if k.links.opened[id] != "" {
				return fmt.Errorf("world.conversations.%s: waits for its place, yet something opens it", id)
			}
			k.links.opened[id] = "ждёт места: " + conv.Waits
		}
		if k.links.opened[id] == "" {
			return fmt.Errorf("world.conversations.%s: nobody opens it (a talk, the encounter, or waits with its reason)", id)
		}
		if err := k.conversation(conv); err != nil {
			return err
		}
	}
	// interjections: one per person, fact and belief; spoken where the person talks, or waiting for that place
	seen := map[string]bool{}
	for i, in := range m.Interjections {
		where := fmt.Sprintf("world.memory.interjections[%d] %s", i, in.ID)
		if m.Facts[in.Fact] == nil {
			return fmt.Errorf("%s: unknown fact %s", where, in.Fact)
		}
		if err := k.person(where, in.Speaker, false); err != nil {
			return err
		}
		if err := k.text(where, in.Text); err != nil {
			return err
		}
		if in.Belief != "witness" && in.Belief != "heard" {
			return fmt.Errorf("%s: belief", where)
		}
		key := in.Speaker + "|" + in.Fact + "|" + in.Belief
		if seen[key] || seen["id|"+in.ID] {
			return fmt.Errorf("%s: a second interjection of %s about %s", where, in.Speaker, in.Fact)
		}
		seen[key], seen["id|"+in.ID] = true, true
		if talkOf[in.Speaker] == "" && in.Waits == "" {
			return fmt.Errorf("%s: %s talks nowhere (add him to a talk, or say where it waits)", where, in.Speaker)
		}
		if talkOf[in.Speaker] != "" && in.Waits != "" {
			return fmt.Errorf("%s: waits, yet %s already talks in %s", where, in.Speaker, talkOf[in.Speaker])
		}
		reader := "вставка " + in.ID
		switch {
		case in.Waits != "":
			reader = "ждёт: " + reader
		case in.Belief == "heard":
			reader = "слух, шаг 2: " + reader
		}
		k.links.add(k.links.factReaders, in.Fact, reader)
		if err := k.when(where+".when", in.When, nil, reader); err != nil {
			return err
		}
		if err := k.do(where, in.Do, nil, reader); err != nil {
			return err
		}
		if v := in.Voice; v != nil {
			if c.DialogueRules == nil {
				return fmt.Errorf("%s: a voice needs world.dialogue_rules", where)
			}
			if _, ok := c.DialogueRules.Skills[v.Skill]; !ok {
				return fmt.Errorf("%s: unknown skill %s", where, v.Skill)
			}
			if err := k.text(where+".voice", v.Text); err != nil {
				return err
			}
			if err := k.when(where+".voice.when", v.When, nil, reader+" (голос)"); err != nil {
				return err
			}
		}
	}
	// no dead branch: every fact and topic is written somewhere and read somewhere
	for _, id := range sortedKeys(m.Facts) {
		if len(k.links.factWriters[id]) == 0 {
			return fmt.Errorf("world.memory.facts.%s: nothing writes it (a dead branch for every condition on it)", id)
		}
		if len(k.links.factReaders[id]) == 0 {
			return fmt.Errorf("world.memory.facts.%s: nothing reads it (a fact nobody remembers is a flag)", id)
		}
	}
	for _, id := range sortedKeys(m.Topics) {
		if len(k.links.topicWriters[id]) == 0 {
			return fmt.Errorf("world.memory.topics.%s: nothing gives it to the hero", id)
		}
		if len(k.links.topicReaders[id]) == 0 {
			return fmt.Errorf("world.memory.topics.%s: nothing reads it", id)
		}
	}
	c.memoryLinks = k.links
	c.linksPage = memoryLinksPage(c, k.links)
	return nil
}

// memoryLinksPage: the map of links (doc 103 §6) — conversations ↔ facts ↔ topics ↔ people, who writes and who reads, and
// what hangs (data waiting for its place, readers outside the world yet).
func memoryLinksPage(c *Compiled, l *memoryLinks) []byte {
	m := c.Memory
	esc := html.EscapeString
	var b strings.Builder
	list := func(items []string) string {
		if len(items) == 0 {
			return `<span class="gap">—</span>`
		}
		out := make([]string, 0, len(items))
		for _, it := range items {
			class := ""
			if strings.HasPrefix(it, "вне: ") || strings.HasPrefix(it, "ждёт") || strings.HasPrefix(it, "слух") {
				class = ` class="out"`
			}
			out = append(out, "<li"+class+">"+esc(it)+"</li>")
		}
		return "<ul>" + strings.Join(out, "") + "</ul>"
	}
	b.WriteString(`<!doctype html>
<html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Карта связей разговоров</title>
<style>
:root{--bg:#f6f3ee;--ink:#22201c;--muted:#6f6a60;--line:#d9d2c5;--out:#8a5a1c;--head:#ece6db}
@media (prefers-color-scheme: dark){:root{--bg:#1b1a18;--ink:#e9e4da;--muted:#a49d90;--line:#3a362f;--out:#e0a85c;--head:#26241f}}
body{margin:0;background:var(--bg);color:var(--ink);font:15px/1.45 system-ui,sans-serif}
main{max-width:1180px;margin:0 auto;padding:24px 16px 64px}
h1{font-size:24px;margin:0 0 4px}h2{font-size:18px;margin:32px 0 8px}
p{color:var(--muted);margin:0 0 12px}
.wrap{overflow-x:auto}
table{border-collapse:collapse;width:100%;min-width:640px}
th,td{border-bottom:1px solid var(--line);padding:6px 8px;text-align:left;vertical-align:top}
th{background:var(--head);font-weight:600}
code{font:13px ui-monospace,monospace}
ul{margin:0;padding-left:16px}.out{color:var(--out)}.gap{color:var(--muted)}
</style></head><body><main>
<h1>Карта связей разговоров</h1>
<p>Сгенерировано ANG из CUE (док. 103 §6). Факты и темы — кто пишет и кто читает; разговоры — кто открывает и чем
кончаются; люди — где говорят. Оранжевым — то, что ждёт своего места, читается вне мира или ждёт слухов (шаг 2).</p>
`)
	b.WriteString(`<h2>Факты</h2><div class="wrap"><table><tr><th>факт</th><th>видимость</th><th>пишет</th><th>читает</th></tr>`)
	for _, id := range sortedKeys(m.Facts) {
		f := m.Facts[id]
		vis := f.Visibility
		if f.Recorded && vis != "recorded" {
			vis += " + recorded"
		}
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s, %d дн.</td><td>%s</td><td>%s</td></tr>", esc(id), esc(vis), f.MemoryDays, list(l.factWriters[id]), list(l.factReaders[id]))
	}
	b.WriteString(`</table></div><h2>Темы героя</h2><div class="wrap"><table><tr><th>тема</th><th>даёт</th><th>читает</th></tr>`)
	for _, id := range sortedKeys(m.Topics) {
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td></tr>", esc(id), list(l.topicWriters[id]), list(l.topicReaders[id]))
	}
	b.WriteString(`</table></div><h2>Разговоры</h2><div class="wrap"><table><tr><th>разговор</th><th>открывает</th><th>входы</th><th>итоги</th></tr>`)
	for _, id := range sortedKeys(c.Conversations) {
		conv := c.Conversations[id]
		entries := []string{}
		for _, e := range conv.Entries {
			entries = append(entries, e.ID)
		}
		opened := l.opened[id]
		fmt.Fprintf(&b, "<tr><td><code>%s</code>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", esc(id), map[bool]string{true: " · событие", false: ""}[conv.Event], list([]string{opened}), esc(strings.Join(entries, ", ")), esc(strings.Join(l.closes[id], ", ")))
	}
	b.WriteString(`</table></div><h2>Люди</h2><div class="wrap"><table><tr><th>человек</th><th>разговоры</th><th>вставки</th></tr>`)
	byPerson := map[string][]string{}
	for _, in := range m.Interjections {
		label := in.Fact
		if in.Belief == "heard" {
			label += " (слух, шаг 2)"
		}
		if in.Waits != "" {
			label = "ждёт: " + label
		}
		byPerson[in.Speaker] = append(byPerson[in.Speaker], label)
	}
	people := map[string]bool{}
	for p := range l.people {
		people[p] = true
	}
	for p := range byPerson {
		people[p] = true
	}
	for _, p := range sortedKeys(people) {
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td></tr>", esc(p), list(l.people[p]), list(byPerson[p]))
	}
	b.WriteString("</table></div></main></body></html>\n")
	return []byte(b.String())
}
