package gamepoc

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// RegionCatalog is private authoring data. Unity only receives the server's
// knowledge projection, never this catalog or its undiscovered rooms.
type RegionCatalog struct {
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	Decks []RegionDeck `json:"decks"`
	Nodes []RegionNode `json:"nodes"`
	Links []RegionLink `json:"links"`
}
type RegionDeck struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	ElevationM float64 `json:"elevation_m"`
}
type RegionNode struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	DeckID           string  `json:"deck_id"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	Width            float64 `json:"width"`
	Height           float64 `json:"height"`
	LevelID          string  `json:"level_id"`
	InitialKnowledge string  `json:"initial_knowledge"`
	Description      string  `json:"description"`
}
type RegionLink struct {
	ID           string   `json:"id"`
	FromNode     string   `json:"from_node"`
	ToNode       string   `json:"to_node"`
	Mode         string   `json:"mode"`
	InitialKnown bool     `json:"initial_known"`
	LifeEdges    []string `json:"life_edges,omitempty"`
}

func (r *RegionCatalog) validate(life json.RawMessage) error {
	if r.ID == "" && len(r.Nodes) == 0 && len(r.Links) == 0 && len(r.Decks) == 0 {
		return nil
	}
	if r.ID == "" || r.Name == "" || len(r.Nodes) == 0 || len(r.Nodes) > 4096 || len(r.Links) > 8192 || len(r.Decks) == 0 || len(r.Decks) > 64 {
		return fmt.Errorf("invalid region map catalog")
	}
	var l struct {
		Locations map[string]json.RawMessage `json:"locations"`
		Edges     []struct {
			ID   string `json:"id"`
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(life, &l); err != nil {
		return fmt.Errorf("region map needs Life locations: %w", err)
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= 1e6 }
	decks := map[string]bool{}
	nodes := map[string]RegionNode{}
	levels := map[string]bool{}
	links := map[string]bool{}
	lifeEdges := map[string]struct{ from, to string }{}
	for _, edge := range l.Edges {
		lifeEdges[edge.ID] = struct{ from, to string }{edge.From, edge.To}
	}
	boundLifeEdges := map[string]bool{}
	for _, d := range r.Decks {
		if d.ID == "" || d.Name == "" || decks[d.ID] || !finite(d.ElevationM) {
			return fmt.Errorf("invalid region deck %s", d.ID)
		}
		decks[d.ID] = true
	}
	for _, n := range r.Nodes {
		if n.ID == "" || n.Name == "" || nodes[n.ID].ID != "" || !decks[n.DeckID] || !finite(n.X) || !finite(n.Y) || !finite(n.Width) || !finite(n.Height) || n.Width <= 0 || n.Height <= 0 {
			return fmt.Errorf("invalid region node %s", n.ID)
		}
		if n.InitialKnowledge != "unknown" && n.InitialKnowledge != "known" && n.InitialKnowledge != "visited" {
			return fmt.Errorf("invalid region knowledge %s", n.ID)
		}
		if n.LevelID != "" {
			if _, ok := l.Locations[n.LevelID]; !ok || levels[n.LevelID] {
				return fmt.Errorf("invalid/duplicate region level %s", n.LevelID)
			}
			levels[n.LevelID] = true
		}
		nodes[n.ID] = n
	}
	for _, e := range r.Links {
		a, b := nodes[e.FromNode], nodes[e.ToNode]
		if e.ID == "" || links[e.ID] || a.ID == "" || b.ID == "" || a.ID == b.ID {
			return fmt.Errorf("invalid region link %s", e.ID)
		}
		switch e.Mode {
		case "corridor", "stairs", "lift", "monorail":
		default:
			return fmt.Errorf("invalid region mode %s", e.ID)
		}
		if e.InitialKnown && (a.InitialKnowledge == "unknown" || b.InitialKnowledge == "unknown") {
			return fmt.Errorf("known region link exposes unknown endpoint %s", e.ID)
		}
		for _, id := range e.LifeEdges {
			edge, exists := lifeEdges[id]
			if !exists || boundLifeEdges[id] || a.LevelID == "" || b.LevelID == "" || !((edge.from == a.LevelID && edge.to == b.LevelID) || (edge.from == b.LevelID && edge.to == a.LevelID)) {
				return fmt.Errorf("invalid/duplicate region Life edge %s", id)
			}
			boundLifeEdges[id] = true
		}
		links[e.ID] = true
	}
	sort.Slice(r.Decks, func(i, j int) bool { return r.Decks[i].ID < r.Decks[j].ID })
	sort.Slice(r.Nodes, func(i, j int) bool { return r.Nodes[i].ID < r.Nodes[j].ID })
	sort.Slice(r.Links, func(i, j int) bool { return r.Links[i].ID < r.Links[j].ID })
	return nil
}
