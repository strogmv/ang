package gamepoc

import (
	"os"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func TestStoryActDraftAndAuthorRefusals(t *testing.T) {
	fixture, err := os.ReadFile("testdata/story-acts.cue")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ label, from, to, contains string }{
		{"valid", "", "", ""},
		{"unreachable", "place: \"home\"}]", "place: \"away\"}]", "куда герой не попадёт"},
		{"missing fate", "{mode: \"expire\", at: \"a2\", step: \"expired\", fact: \"fact.expired\"}", "{}", "укажите судьбу"},
		{"unwritten return", "when: fact: \"fact.expired\"", "when: fact: \"fact.unwritten\"", "никто не записывает"},
		{"undecided return", "mode: \"changed\", after: \"return\", entry: \"e.back\", when: fact: \"fact.expired\"", "mode: \"unresolved\", after: \"return\"", "до сцены можно вернуться"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			source := string(fixture)
			if tc.from != "" {
				if !strings.Contains(source, tc.from) {
					t.Fatal("fixture replacement did not match")
				}
				source = strings.Replace(source, tc.from, tc.to, 1)
			}
			world := cuecontext.New().CompileString(source).LookupPath(cue.ParsePath("world"))
			c := Compiled{
				Life: []byte(`{"locations":{"away":{},"home":{}}}`),
				Memory: &Memory{
					Facts:   map[string]*MemoryFact{"fact.left": {}, "fact.expired": {Journal: "test.expiry"}, "fact.unwritten": {}},
					Quests:  map[string][]string{"test.side": {"active", "done", "expired"}, "test.main": {"done"}},
					Journal: &storyJournalRules{},
				},
				Conversations: map[string]*Conversation{"dlg.fixture": {Entries: []ConvEntry{{ID: "e.back"}}}},
				memoryLinks:   &memoryLinks{factWriters: map[string][]string{"fact.left": {"left"}, "fact.expired": {"border"}}},
			}
			if err := readStoryActs(world, &c); err != nil {
				t.Fatal(err)
			}
			pending, err := checkStoryActs(&c)
			if tc.contains == "" {
				if err != nil || len(pending) > 0 {
					t.Fatalf("valid fixture: %v %v", pending, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("wanted %q; %v", tc.contains, err)
			}
		})
	}
}
