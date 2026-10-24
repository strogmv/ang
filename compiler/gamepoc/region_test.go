package gamepoc

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

func regionFixture() RegionCatalog {
	return RegionCatalog{ID: "test.region", Name: "Region", Decks: []RegionDeck{{ID: "deck", Name: "Deck"}}, Nodes: []RegionNode{
		{ID: "home", Name: "Home", DeckID: "deck", Width: 20, Height: 10, LevelID: "home.level", InitialKnowledge: "visited"},
		{ID: "hidden", Name: "SECRET_REGION_ROOM", DeckID: "deck", X: 40, Width: 20, Height: 10, InitialKnowledge: "unknown"},
	}, Links: []RegionLink{{ID: "secret.link", FromNode: "home", ToNode: "hidden", Mode: "corridor"}}}
}
func TestRegionCatalogRejectsBrokenKnowledgeAndGeometry(t *testing.T) {
	life := json.RawMessage(`{"locations":{"home.level":{}}}`)
	for _, tc := range []struct {
		name   string
		bad    bool
		change func(*RegionCatalog)
	}{
		{"valid", false, func(*RegionCatalog) {}},
		{"known_hidden_link", true, func(r *RegionCatalog) { r.Links[0].InitialKnown = true }},
		{"missing_deck", true, func(r *RegionCatalog) { r.Nodes[0].DeckID = "missing" }},
		{"missing_link_endpoint", true, func(r *RegionCatalog) { r.Links[0].ToNode = "missing" }},
		{"duplicate_node", true, func(r *RegionCatalog) { r.Nodes = append(r.Nodes, r.Nodes[0]) }},
		{"duplicate_level", true, func(r *RegionCatalog) { r.Nodes[1].LevelID = "home.level" }},
		{"missing_level", true, func(r *RegionCatalog) { r.Nodes[0].LevelID = "missing" }},
		{"nan_position", true, func(r *RegionCatalog) { r.Nodes[0].X = math.NaN() }},
		{"empty_footprint", true, func(r *RegionCatalog) { r.Nodes[0].Width = 0 }},
		{"unknown_mode", true, func(r *RegionCatalog) { r.Links[0].Mode = "teleport" }},
		{"invalid_knowledge", true, func(r *RegionCatalog) { r.Nodes[0].InitialKnowledge = "omniscient" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := regionFixture()
			tc.change(&r)
			if err := r.validate(life); (err != nil) != tc.bad {
				t.Fatalf("bad=%v: %v", tc.bad, err)
			}
		})
	}
}
func TestRegionPrivateCatalogNeverEntersClientResource(t *testing.T) {
	c := Compiled{Region: regionFixture()}
	c.Region.Links[0].LifeEdges = []string{"PRIVATE_PHYSICAL_EDGE"}
	raw, err := clientResource(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"SECRET_REGION_ROOM", "secret.link", "test.region", "initial_knowledge", "PRIVATE_PHYSICAL_EDGE", "life_edges"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("private region field entered client resource: %s", secret)
		}
	}
}

func TestRegionPhysicalConnectionsReferenceOnlyMatchingLifeEdges(t *testing.T) {
	life := json.RawMessage(`{"locations":{"home.level":{},"stair.level":{},"other.level":{}},"edges":[{"id":"down","from":"home.level","to":"stair.level"},{"id":"up","from":"stair.level","to":"home.level"}]}`)
	for _, tc := range []struct {
		name   string
		bad    bool
		change func(*RegionCatalog)
	}{
		{"paired_stairs", false, func(*RegionCatalog) {}},
		{"unknown_transition", true, func(r *RegionCatalog) { r.Links[0].LifeEdges = []string{"not.implemented"} }},
		{"wrong_landing", true, func(r *RegionCatalog) { r.Nodes[1].LevelID = "other.level" }},
		{"same_edge_twice", true, func(r *RegionCatalog) { r.Links[0].LifeEdges = []string{"down", "down"} }},
		{"ambiguous_parallel_passage", true, func(r *RegionCatalog) {
			other := r.Links[0]
			other.ID = "another.passage"
			other.LifeEdges = []string{"down"}
			r.Links = append(r.Links, other)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := regionFixture()
			r.Nodes[1].LevelID = "stair.level"
			r.Links[0].Mode = "stairs"
			r.Links[0].LifeEdges = []string{"down", "up"}
			tc.change(&r)
			if err := r.validate(life); (err != nil) != tc.bad {
				t.Fatalf("bad=%v: %v", tc.bad, err)
			}
		})
	}
}
