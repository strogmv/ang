package gamepoc

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
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
	c.Region.Nodes[1].Label = "PRIVATE_SHORT_REGION_LABEL"
	c.Region.Links[0].LifeEdges = []string{"PRIVATE_PHYSICAL_EDGE"}
	raw, err := clientResource(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"SECRET_REGION_ROOM", "PRIVATE_SHORT_REGION_LABEL", "secret.link", "test.region", "initial_knowledge", "PRIVATE_PHYSICAL_EDGE", "life_edges"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("private region field entered client resource: %s", secret)
		}
	}
}

func TestRegionOptionalShortLabelsPreserveFullNames(t *testing.T) {
	life := json.RawMessage(`{"locations":{"home.level":{}}}`)
	for _, label := range []string{"", "Home", "Общий контейнер"} {
		r := regionFixture()
		r.Nodes[0].Label = label
		if err := r.validate(life); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var decoded RegionCatalog
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		var found *RegionNode
		for i := range decoded.Nodes {
			if decoded.Nodes[i].ID == "home" {
				found = &decoded.Nodes[i]
			}
		}
		if found == nil || found.Name != "Home" || found.Label != label {
			t.Fatal("short label overwrote identity/full name or failed private roundtrip")
		}
		if label == "" && bytes.Contains(data, []byte(`"label"`)) {
			t.Fatal("omitted legacy label changed catalog JSON")
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

func TestRegionNodeKindsAreOptionalFiniteAndPreserved(t *testing.T) {
	life := json.RawMessage(`{"locations":{"home.level":{}}}`)
	kinds := []string{"", "housing", "food", "hygiene", "laundry", "commons", "market", "station", "transport_hub", "clinic", "control", "junction", "refuge", "repair_workshop", "controlled_reception", "freight", "delivery_port", "maintenance_port", "robot_service", "utility_power", "utility_water", "utility_air", "utility_extract", "utility_sewer", "utility_waste", "stairs"}
	for _, kind := range kinds {
		t.Run("kind_"+kind, func(t *testing.T) {
			r := regionFixture()
			r.Nodes[0].Kind = kind
			if err := r.validate(life); err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			var decoded RegionCatalog
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Nodes[1].ID == "home" {
				decoded.Nodes[0], decoded.Nodes[1] = decoded.Nodes[1], decoded.Nodes[0]
			}
			if decoded.Nodes[0].ID != "home" || decoded.Nodes[0].Kind != kind {
				t.Fatal("authored node kind lost in private catalog")
			}
			if kind == "" && bytes.Contains(raw, []byte(`"kind"`)) {
				t.Fatal("optional empty kind changed legacy catalog JSON")
			}
		})
	}
	for _, invalid := range []string{"Housing", "medical_alias", "SECRET_FUNCTION", "housing/clinic", " housing"} {
		r := regionFixture()
		r.Nodes[0].Kind = invalid
		err := r.validate(life)
		if err == nil || !strings.Contains(err.Error(), "region node kind home") {
			t.Fatalf("invalid kind %q not rejected actionably: %v", invalid, err)
		}
	}
	r := regionFixture()
	r.Nodes[0].Kind = "housing"
	r.Nodes[1].Kind = "utility_waste"
	raw, err := clientResource(Compiled{Region: r})
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"housing", "utility_waste"} {
		if bytes.Contains(raw, []byte(private)) {
			t.Fatalf("private full-catalog kind leaked into client resource: %s", private)
		}
	}
}
