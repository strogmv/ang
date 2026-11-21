package inventory

import (
	"reflect"
	"testing"
)

// A saved inventory grows by a new owner's account, store and stock exactly as a new game would create them; nothing
// of the others comes back, and the state validates.
func TestGrowAddsANewOwnersThingsOnly(t *testing.T) {
	old := fixtureModel()
	saved := mustState(t, old)
	// the player took the hero's bag away: it is gone from the save and must not come back
	for id, item := range saved.Items {
		if item.Origin == "bag" {
			delete(saved.Items, id)
			delete(saved.Stores, ItemStore(id))
		}
	}
	m := fixtureModel()
	m.Actors["trader"], m.Owners["trader"] = true, true
	m.Accounts["c"] = Account{ID: "c", Owner: "trader", Currency: "CRD", Balance: 250}
	stall := "stall/trader"
	m.Stores[stall] = Store{ID: stall, Kind: "container", Anchor: Anchor{Kind: "world", Level: "map", At: Cell{X: 4, Y: 4}}, Owner: "trader", Capacity: Capacity{Mass: 1000, Volume: 1000, Stacks: 100}}
	m.Stores[ActorStore("trader")] = Store{ID: ActorStore("trader"), Kind: "actor", Anchor: Anchor{Kind: "actor", Actor: "trader"}, Owner: "trader", Capacity: Capacity{Mass: 1000, Volume: 1000, Stacks: 100}}
	m.Sources = append(m.Sources,
		Source{ID: "trader.stock", Definition: "ore", Quantity: 7, Condition: 1000000, Owner: "trader", Location: Location{Kind: "store", Store: stall}, ForSale: true},
		Source{ID: "trader.bag", Definition: "bag", Quantity: 1, Condition: 1000000, Location: Location{Kind: "store", Store: ActorStore("trader")}})
	m.Shopfronts = nil
	g, err := saved.Grow(m, 123, 0, map[string]bool{"trader": true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.Accounts, []string{"c"}) || len(g.Stores) != 3 || len(g.Items) != 2 {
		t.Fatalf("growth %+v", g)
	}
	if err := saved.Validate(m); err != nil {
		t.Fatalf("the grown save does not validate: %v", err)
	}
	fresh := mustState(t, m)
	for _, id := range g.Items {
		if !reflect.DeepEqual(saved.Items[id], fresh.Items[id]) {
			t.Fatalf("%s differs from what a new game creates", id)
		}
	}
	for _, item := range saved.Items {
		if item.Origin == "bag" {
			t.Fatal("a thing of an old owner came back")
		}
	}
	if saved.Accounts["c"].Balance != 250 {
		t.Fatal("the new account did not start at its initial balance")
	}
	again := saved.Clone()
	g2, err := again.Grow(m, 123, 0, map[string]bool{"trader": true})
	if err != nil || len(g2.Accounts)+len(g2.Stores)+len(g2.Items) != 0 || !reflect.DeepEqual(again.Items, saved.Items) {
		t.Fatalf("a second growth added something: %+v %v", g2, err)
	}
}

// A delivery new to a save starts with the next run due after now, not with a burst of the missed ones.
func TestGrowStartsANewDeliveryAtItsNextRun(t *testing.T) {
	s := State{}
	m := Model{Deliveries: []Delivery{{ID: "d", Carrier: "trader", FirstSecond: 100, PeriodSeconds: 50}, {ID: "other", Carrier: "npc", FirstSecond: 100, PeriodSeconds: 50}}, Shopfronts: []Shopfront{{ID: "shop", Owner: "trader"}, {ID: "elsewhere", Owner: "npc"}}}
	trader := map[string]bool{"trader": true}
	g, err := s.Grow(m, 1, 260, trader)
	if err != nil || s.Deliveries["d"].NextDue != 300 || len(g.Deliveries) != 1 || len(g.Shopfronts) != 1 {
		t.Fatalf("next due %d, growth %+v, %v", s.Deliveries["d"].NextDue, g, err)
	}
	if _, ok := s.Deliveries["other"]; ok {
		t.Fatal("a delivery of nobody new grew")
	}
	if _, ok := s.Shopfronts["elsewhere"]; ok {
		t.Fatal("a shop front of nobody new grew")
	}
	s = State{}
	if _, err := s.Grow(m, 1, 40, trader); err != nil || s.Deliveries["d"].NextDue != 100 {
		t.Fatalf("before the first run: next due %d", s.Deliveries["d"].NextDue)
	}
}

// Only what belongs to the new owners grows: a store of nobody new the save lacks stays missing (the save refuses as
// before); and a store the save already had, whose owner becomes a resident only now, does not get back the stock the
// player took from it.
func TestGrowOnlyTheNewOwnersRegistries(t *testing.T) {
	m := fixtureModel()
	m.Actors["trader"], m.Owners["trader"] = true, true
	shelf := "shelf/old"
	m.Stores[shelf] = Store{ID: shelf, Kind: "container", Anchor: Anchor{Kind: "world", Level: "map", At: Cell{X: 5, Y: 5}}, Owner: "trader", Capacity: Capacity{Mass: 1000, Volume: 1000, Stacks: 100}}
	m.Sources = append(m.Sources, Source{ID: "shelf.stock", Definition: "ore", Quantity: 5, Condition: 1000000, Owner: "trader", Location: Location{Kind: "store", Store: shelf}})
	saved := mustState(t, m)
	// the player took the whole shelf stock before the trader became a resident
	for id, item := range saved.Items {
		if item.Origin == "shelf.stock" {
			delete(saved.Items, id)
		}
	}
	// the content now also has a store of nobody new
	other := "stall/nobody"
	m.Stores[other] = Store{ID: other, Kind: "container", Anchor: Anchor{Kind: "world", Level: "map", At: Cell{X: 6, Y: 6}}, Owner: "npc", Capacity: Capacity{Mass: 1000, Volume: 1000, Stacks: 100}}
	g, err := saved.Grow(m, 123, 0, map[string]bool{"trader": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Items) != 0 || len(g.Stores) != 0 {
		t.Fatalf("growth %+v: the shelf stock came back or a store of nobody new grew", g)
	}
	for _, item := range saved.Items {
		if item.Origin == "shelf.stock" {
			t.Fatal("the stock the player took came back")
		}
	}
	if _, ok := saved.Stores[other]; ok {
		t.Fatal("a store of nobody new was added")
	}
	if err := saved.Validate(m); err == nil {
		t.Fatal("a save lacking a store of nobody new validated")
	}
}
