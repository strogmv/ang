package gamepoc

import (
	"reflect"
	"strings"
	"testing"
)

func TestHeroFileCUEKeepsActOneUsage(t *testing.T) {
	type counts = struct {
		Checks int `json:"checks"`
		Voices int `json:"voices"`
	}
	baseline := HeroFileUsage{
		ActOne: []string{"02", "03"}, FirstHour: []string{"03"}, MinAct: 1, MinFirstHour: 1,
		Counts: map[string]map[string]counts{
			"02": {"stealth": {Voices: 1}},
			"03": {"speech": {Checks: 1}, "stealth": {Voices: 1}},
		},
		Reads: map[string]map[string]int{
			"02": {"fact.file.receiving": 1},
			"03": {"fact.file.ventilation": 1, "topic.cassette_norm": 2},
		},
	}
	moved := HeroFileUsage{
		ActOne: baseline.ActOne, FirstHour: baseline.FirstHour, MinAct: 1, MinFirstHour: 1,
		Counts: map[string]map[string]counts{"02": {}},
		Reads:  map[string]map[string]int{"02": {"fact.file.receiving": 1}},
		Dialogue: map[string]heroFileUsageSource{
			"02": {Partial: true, Interjections: []string{"interject.escaped.akim", "interject.escaped.agafya"}},
			"03": {Conversations: []string{"dlg.test"}},
		},
	}
	var a = Interjection{ID: "interject.escaped.akim", Fact: "fact.prokhor.escaped"}
	a.Voice = &struct {
		Skill   string `json:"skill"`
		AtLeast int    `json:"at_least"`
		Text    string `json:"text"`
		When    *When  `json:"when,omitempty"`
	}{Skill: "stealth", Text: "voice.escaped.face"}
	b := a
	b.ID = "interject.escaped.agafya"
	c := &Compiled{
		Memory: &Memory{Interjections: []Interjection{a, b}},
		Conversations: map[string]*Conversation{"dlg.test": {
			Entries: []ConvEntry{{When: &When{Not: &When{Fact: &FactWhen{ID: "fact.file.ventilation"}}}}},
			Nodes: map[string]*ConvNode{"open": {
				Voices: []ConvVoice{{Skill: "stealth", When: &When{Topic: "topic.cassette_norm"}}},
				Options: []ConvOption{{
					Check: &DialogueCheck{Skill: "speech"},
					Do:    []Do{{Topic: "topic.only_written"}},
					Fail:  &ConvFail{Reply: []ConvLine{{Variants: []ConvVariant{{When: When{NotTopic: "topic.cassette_norm"}}}}}},
				}},
			}},
		}},
	}
	if err := moved.addDialogue(c); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(moved.Counts, baseline.Counts) || !reflect.DeepEqual(moved.Reads, baseline.Reads) {
		t.Fatalf("effective uses or reads changed: %+v %+v", moved.Counts, moved.Reads)
	}
	names := map[string]string{"speech": "Speech", "stealth": "Stealth"}
	before := HeroFile{Skills: []string{"speech", "stealth"}, Usage: baseline}
	after := HeroFile{Skills: before.Skills, Usage: moved}
	if err := before.checkUsage(names); err != nil {
		t.Fatal(err)
	}
	if err := after.checkUsage(names); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.uses, after.uses) {
		t.Fatalf("act and first-hour totals changed: %+v -> %+v", before.uses, after.uses)
	}
	if err := moved.addDialogue(c); err == nil || !strings.Contains(err.Error(), "Markdown contribution") {
		t.Fatalf("accepted counting the full script twice: %v", err)
	}
}

func TestHeroFileRejectsSharedCUEUsageSource(t *testing.T) {
	u := HeroFileUsage{
		ActOne: []string{"02", "03"},
		Counts: map[string]map[string]struct {
			Checks int `json:"checks"`
			Voices int `json:"voices"`
		}{},
		Reads: map[string]map[string]int{},
		Dialogue: map[string]heroFileUsageSource{
			"02": {Conversations: []string{"dlg.test"}},
			"03": {Conversations: []string{"dlg.test"}},
		},
	}
	c := &Compiled{Conversations: map[string]*Conversation{"dlg.test": {}}}
	if err := u.addDialogue(c); err == nil || !strings.Contains(err.Error(), "counted twice") {
		t.Fatalf("accepted two script owners: %v", err)
	}
}
