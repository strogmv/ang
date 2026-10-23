package gamepoc

import (
	"encoding/json"
	"testing"

	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

func paintedFixture() Compiled {
	terrain := []string{"####", "#..#", "#..#", "####"}
	return Compiled{
		Walkable:                map[string]bool{".": true},
		Life:                    json.RawMessage(`{"locations":{"market":{"terrain":["####","#..#","#..#","####"]}},"actors":[{"id":"hero"}]}`),
		Inventory:               inv.Model{Stores: map[string]inv.Store{"cassette": {ID: "cassette", Anchor: inv.Anchor{Kind: "world", Level: "market", At: inv.Cell{X: 1, Y: 1}}}}},
		PaintedInventoryAnchors: map[string]map[string]inv.Cell{"market": {"cassette": {X: 2, Y: 2}}},
		Art: Art{Painted: []PaintedScene{{
			LevelID: "market", Background: "MarketArt/architecture", Width: 512, Height: 512,
			Origin: PaintedPoint{200, 50}, AxisX: PaintedPoint{50, 25}, AxisY: PaintedPoint{-50, 25}, ActorHeight: 200,
			Navigation: terrain, ProofWaypoints: []Point{{1, 1}, {2, 1}}, ProofBlocked: Point{0, 0},
			Actors: []PaintedActor{{ActorID: "hero", Image: "MarketArt/people", SourceRect: &ArtSceneSourceRect{W: 80, H: 200}, FeetX: 40, FeetY: 195, Height: 200}},
			Props:  []PaintedProp{{ID: "one", StoreID: "cassette", Image: "MarketArt/props", At: Point{2, 2}, Width: 80, Height: 60, FootX: 40, FootY: 55}},
		}}},
	}
}

// Reject authoring mistakes before they become invisible walls, phantom loot or
// sprites disconnected from real entities in a generated game.
func TestPaintedCatalogRejectsGeometryAndCustodyDrift(t *testing.T) {
	if err := checkPaintedScenes(paintedFixture()); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Compiled){
		"invalid camera width": func(c *Compiled) { c.Art.Painted[0].ViewWidth = 1 },
		"singular projection":  func(c *Compiled) { c.Art.Painted[0].AxisY = c.Art.Painted[0].AxisX },
		"different Go terrain": func(c *Compiled) { c.Art.Painted[0].Navigation[1] = "####" },
		"phantom actor":        func(c *Compiled) { c.Art.Painted[0].Actors[0].ActorID = "invented" },
		"duplicate actor binding": func(c *Compiled) {
			c.Art.Painted[0].Actors = append(c.Art.Painted[0].Actors, c.Art.Painted[0].Actors[0])
		},
		"untracked prop":                func(c *Compiled) { c.Art.Painted[0].Props[0].StoreID = "missing" },
		"prop elsewhere than inventory": func(c *Compiled) { c.Art.Painted[0].Props[0].At = Point{1, 1} },
		"unknown layout store":          func(c *Compiled) { c.PaintedInventoryAnchors["market"]["missing"] = inv.Cell{X: 1, Y: 1} },
		"blocked waypoint":              func(c *Compiled) { c.Art.Painted[0].ProofWaypoints[0] = Point{0, 0} },
		"walkable negative fixture":     func(c *Compiled) { c.Art.Painted[0].ProofBlocked = Point{2, 1} },
		"texture budget": func(c *Compiled) {
			for i := range 33 {
				c.Art.Painted[0].Occluders = append(c.Art.Painted[0].Occluders, PaintedOccluder{ID: string(rune('a' + i)), DepthY: 400, Polygon: []PaintedPoint{{0, 0}, {500, 0}, {500, 500}}})
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := paintedFixture()
			mutate(&c)
			if err := checkPaintedScenes(c); err == nil {
				t.Fatal("invalid authored scene accepted")
			}
		})
	}
}

func TestPaintedClientProjectionOmitsPrivateInventoryLayout(t *testing.T) {
	c := paintedFixture()
	resource, err := clientResource(c)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err = json.Unmarshal(resource, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["painted_inventory_anchors"]; ok {
		t.Fatal("private custody layout leaked to presentation resource")
	}
	var art struct {
		Painted []PaintedScene `json:"painted"`
	}
	if err = json.Unmarshal(body["art"], &art); err != nil {
		t.Fatal(err)
	}
	if len(art.Painted) != 1 || art.Painted[0].Actors[0].ActorID != "hero" {
		t.Fatal("painted presentation lost during client projection")
	}
}
