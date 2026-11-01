package gamepoc

import (
	"encoding/json"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
	"testing"
)

func commerceBindingFixture() Compiled {
	c := Compiled{Walkable: map[string]bool{".": true}, Inventory: inv.Model{Stores: map[string]inv.Store{}}, PaintedInventoryAnchors: map[string]map[string]inv.Cell{"market": {"display": {X: 3, Y: 2}}}}
	c.Life = json.RawMessage(`{"locations":{"hub":{"terrain":["#####","#...#","#...#","#...#","#####"]},"market":{"terrain":["#####","#...#","#...#","#...#","#####"]}},"actors":[{"id":"vendor","steps":[{"level":"hub","at":{"x":2,"y":2}},{"level":"market","at":{"x":2,"y":2}}]}]}`)
	pickup, service := inv.DeliveryPoint{Level: "hub", At: inv.Cell{X: 2, Y: 2}}, inv.DeliveryPoint{Level: "market", At: inv.Cell{X: 2, Y: 2}}
	c.Inventory.Deliveries = []inv.Delivery{{ID: "route", Carrier: "vendor", SourceStore: "source", DestinationStore: "reserve", Pickup: pickup, Dropoff: service}}
	c.Inventory.Shopfronts = []inv.Shopfront{{ID: "shop", Owner: "vendor", Service: service, DisplayStore: "display", ReserveStore: "reserve"}}
	for id, p := range map[string]inv.DeliveryPoint{"source": pickup, "reserve": service, "display": service} {
		c.Inventory.Stores[id] = inv.Store{ID: id, Anchor: inv.Anchor{Kind: "world", Level: p.Level, At: p.At}}
	}
	return c
}
func TestCommerceBindingsRequireWalkableRoutineAndActualStorageApproach(t *testing.T) {
	c := commerceBindingFixture()
	if err := validateCommerce(c); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"wall", "missing_actor", "missing_pickup_step", "display_far", "reserve_wrong_level", "missing_store"} {
		t.Run(kind, func(t *testing.T) {
			c := commerceBindingFixture()
			switch kind {
			case "wall":
				c.Inventory.Deliveries[0].Pickup.At = inv.Cell{}
			case "missing_actor":
				c.Inventory.Deliveries[0].Carrier = "invented"
			case "missing_pickup_step":
				c.Inventory.Deliveries[0].Pickup.At.X = 1
			case "display_far":
				c.PaintedInventoryAnchors["market"]["display"] = inv.Cell{X: 4, Y: 4}
			case "reserve_wrong_level":
				s := c.Inventory.Stores["reserve"]
				s.Anchor.Level = "hub"
				c.Inventory.Stores["reserve"] = s
			case "missing_store":
				delete(c.Inventory.Stores, "source")
			}
			if validateCommerce(c) == nil {
				t.Fatal("accepted nonphysical commerce binding")
			}
		})
	}
	if err := validateCommerce(Compiled{}); err != nil {
		t.Fatal("optional commerce broke a legacy world", err)
	}
}
