package gamepoc

import (
	"strings"
	"testing"
)

func loadingFixture() Compiled {
	text := func(key string) LocalizationMessage {
		return LocalizationMessage{Key: key, Context: "test", Texts: map[string]string{"ru": "слово", "en": "word"}}
	}
	return Compiled{
		Localization: LocalizationCatalog{Locales: []string{"ru", "en"}, Messages: []LocalizationMessage{
			text("ui.place.a"), text("ui.place.b"), text("ui.from"), text("fact.a.journal"), text("topic.b.journal")}},
		Life:   []byte(`{"locations": {"level.a": {}, "level.b": {}}}`),
		Region: RegionCatalog{Nodes: []RegionNode{{ID: "hd.a", LevelID: "level.a"}, {ID: "hd.b", LevelID: "level.b"}}},
		Memory: &Memory{Facts: map[string]*MemoryFact{"fact.a.seen": {Journal: "fact.a.journal"}, "fact.a.silent": {}},
			Topics: map[string]*MemoryTopic{"topic.b": {Journal: "topic.b.journal"}}},
		LoadingScreen: &JourneyScreen{
			ShortLimitMS: 1500,
			Start:        JourneyStart{Scenario: "story", Level: "level.a"},
			FallbackKind: "walk",
			Stages:       map[string][]string{"entry": {"world", "scene", "ready"}, "travel": {"trip", "arrival", "ready"}},
			Kinds: map[string]JourneyKind{
				"new_game": {Path: "entry", CoverFrom: "request", ExpectedMS: 9000, FromKey: "ui.from", Form: "full"},
				"load":     {Path: "entry", CoverFrom: "request", ExpectedMS: 7000, Form: "full"},
				"walk":     {Path: "travel", CoverFrom: "trip", ExpectedMS: 900, Form: "short"},
				"rail":     {Path: "travel", CoverFrom: "trip", ExpectedMS: 4000, Form: "full"},
				"train":    {Path: "travel", CoverFrom: "arrival", ExpectedMS: 900, Form: "short"},
				"vehicle":  {Path: "travel", CoverFrom: "arrival", ExpectedMS: 900, Form: "short"},
			},
			Places: map[string]JourneyPlace{
				"level.a": {NameKey: "ui.place.a", RouteNode: "hd.a", Knowledge: []string{"fact.a.seen"}},
				"level.b": {NameKey: "ui.place.b", RouteNode: "hd.b", Knowledge: []string{"topic.b"}},
			},
			SourceShown: []string{"rumour"},
		},
	}
}

// The loading screen's data is checked: a kind's form follows its expected duration, the stages are the client's in
// order, every Life location is a place, its words exist, its pictures are where it says, its route node is its own, and
// what its line may name is a journal line of the story memory.
func TestLoadingScreenDataIsChecked(t *testing.T) {
	if err := validateLoadingScreen(t.TempDir(), loadingFixture()); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, want string
		edit       func(*Compiled)
	}{
		{"form against the duration", "kinds.walk.form", func(c *Compiled) {
			k := c.LoadingScreen.Kinds["walk"]
			k.ExpectedMS = 3000
			c.LoadingScreen.Kinds["walk"] = k
		}},
		{"a stage the client does not detect", "stages.travel", func(c *Compiled) {
			c.LoadingScreen.Stages["travel"] = []string{"trip", "teleport", "ready"}
		}},
		{"stages out of order", "stages.entry", func(c *Compiled) {
			c.LoadingScreen.Stages["entry"] = []string{"scene", "world", "ready"}
		}},
		{"a kind the server names is missing", "kinds.train", func(c *Compiled) { delete(c.LoadingScreen.Kinds, "train") }},
		{"an entry covered from its trip", "kinds.load.cover_from", func(c *Compiled) {
			k := c.LoadingScreen.Kinds["load"]
			k.CoverFrom = "trip"
			c.LoadingScreen.Kinds["load"] = k
		}},
		{"a location without a place", "places: Life location level.b", func(c *Compiled) { delete(c.LoadingScreen.Places, "level.b") }},
		{"a name with no text", "places.level.a.name_key", func(c *Compiled) {
			p := c.LoadingScreen.Places["level.a"]
			p.NameKey = "ui.place.none"
			c.LoadingScreen.Places["level.a"] = p
		}},
		{"a picture that is not there", "places.level.a.render", func(c *Compiled) {
			p := c.LoadingScreen.Places["level.a"]
			p.Render = "res://assets/loading/a.jpg"
			c.LoadingScreen.Places["level.a"] = p
			c.GodotTheme = &GodotTheme{Target: "client"}
		}},
		{"another place's route node", "places.level.a.route_node", func(c *Compiled) {
			p := c.LoadingScreen.Places["level.a"]
			p.RouteNode = "hd.b"
			c.LoadingScreen.Places["level.a"] = p
		}},
		{"knowledge with no journal line", "fact.a.silent has no journal line", func(c *Compiled) {
			p := c.LoadingScreen.Places["level.a"]
			p.Knowledge = []string{"fact.a.silent"}
			c.LoadingScreen.Places["level.a"] = p
		}},
		{"knowledge the story does not have", "fact.none is no fact or topic", func(c *Compiled) {
			p := c.LoadingScreen.Places["level.a"]
			p.Knowledge = []string{"fact.none"}
			c.LoadingScreen.Places["level.a"] = p
		}},
		{"an unknown journal source", "source_shown", func(c *Compiled) { c.LoadingScreen.SourceShown = []string{"dream"} }},
	}
	for _, test := range cases {
		c := loadingFixture()
		test.edit(&c)
		err := validateLoadingScreen(t.TempDir(), c)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%s: got %v, want an error naming %q", test.name, err, test.want)
		}
	}
	// the client's projection carries no knowledge; the server's carries only it
	view := clientLoadingScreen(loadingFixture().LoadingScreen)
	if place := view["places"].(map[string]any)["level.a"].(map[string]any); place["knowledge"] != nil || place["name_key"] != "ui.place.a" {
		t.Fatalf("client projection %+v", place)
	}
}
