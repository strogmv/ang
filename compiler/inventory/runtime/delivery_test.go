package inventory

import (
	"encoding/json"
	"reflect"
	"testing"
)

func commerceFixture(t *testing.T) (Model, State) {
	t.Helper()
	m := Model{Reservation: ReservationPolicy{MaxTicks: 600, MaxPerActor: 16, MaxTotal: 128}, Version: 1, MaxDepth: 8, MaxItems: 100, Actors: map[string]bool{"merchant": true, "hero": true}, Owners: map[string]bool{}, Levels: map[string]Cell{"hub": {10, 10}, "market": {10, 10}}, Definitions: map[string]Definition{"goods": {ID: "goods", Carrier: "physical", Mass: 100, Volume: 200, StackLimit: 100}}, Stores: map[string]Store{}, Accounts: map[string]Account{}, Entitlements: map[string]Entitlement{}}
	cap := Capacity{Mass: 10000, Volume: 10000, Stacks: 16}
	for _, id := range []string{"merchant", "hero"} {
		m.Stores[ActorStore(id)] = Store{ID: ActorStore(id), Kind: "actor", Owner: id, Anchor: Anchor{Kind: "actor", Actor: id}, Capacity: cap}
	}
	for _, id := range []string{"source", "display", "reserve"} {
		a := Anchor{Kind: "world", Level: "market", At: Cell{5, 5}}
		if id == "source" {
			a.Level = "hub"
			a.At = Cell{2, 2}
		}
		m.Stores[id] = Store{ID: id, Kind: "container", Owner: "merchant", Anchor: a, Capacity: cap, Locked: id != "display"}
	}
	m.Deliveries = []Delivery{{ID: "route", Carrier: "merchant", SourceStore: "source", DestinationStore: "reserve", Pickup: DeliveryPoint{"hub", Cell{2, 2}}, Dropoff: DeliveryPoint{"market", Cell{5, 5}}, Definition: "goods", Quantity: 3, FirstSecond: 100, PeriodSeconds: 86400}}
	m.Shopfronts = []Shopfront{{ID: "shop", Owner: "merchant", Service: DeliveryPoint{"market", Cell{5, 5}}, DisplayStore: "display", ReserveStore: "reserve", PackAway: true, OpenSecond: 200, CloseSecond: 1000}}
	m.Sources = []Source{{ID: "finite", Definition: "goods", Quantity: 5, Condition: 1000000, Owner: "merchant", ForSale: true, Location: Location{Kind: "store", Store: "source"}}}
	s, err := New(m, 1)
	if err != nil {
		t.Fatal(err)
	}
	return m, s
}
func countStore(s State, store string) int64 {
	n := int64(0)
	for _, i := range s.Items {
		if i.Location.Store == store {
			n += i.Quantity
		}
	}
	return n
}
func TestFiniteDeliveryRequiresPhysicalCarrierPersistsAndNeverMints(t *testing.T) {
	m, s := commerceFixture(t)
	pose := map[string]DeliveryPose{"merchant": {Level: "hub", At: Cell{2, 3}, Present: true}}
	step := func(at int64) {
		t.Helper()
		if err := s.AdvanceCommerce(m, at, pose); err != nil {
			t.Fatal(err)
		}
	}
	step(100)
	if countStore(s, "source") != 5 {
		t.Fatal("remote pickup")
	}
	pose["merchant"] = DeliveryPose{Level: "hub", At: Cell{2, 2}, Present: true}
	step(100)
	if countStore(s, "source") != 2 || countStore(s, ActorStore("merchant")) != 3 {
		t.Fatal("pickup missing")
	}
	raw, _ := json.Marshal(s)
	var loaded State
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	if err := loaded.Validate(m); err != nil {
		t.Fatal(err)
	}
	if err := loaded.AdvanceCommerce(m, 100, pose); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, loaded) {
		t.Fatal("replayed pickup changed state")
	}
	clone := s.Clone()
	clone.Deliveries["route"].Cargo[0].Quantity++
	if err := clone.Validate(m); err == nil {
		t.Fatal("accepted corrupt saved cargo")
	}
	if err := s.Validate(m); err != nil {
		t.Fatal("clone mutated source", err)
	}
	pose["merchant"] = DeliveryPose{Level: "market", At: Cell{5, 5}, Present: false}
	step(200)
	if countStore(s, "display") != 0 {
		t.Fatal("inside/travelling merchant unloaded")
	}
	pose["merchant"] = DeliveryPose{Level: "market", At: Cell{5, 5}, Present: true}
	step(200)
	if countStore(s, "display") != 3 || countStore(s, ActorStore("merchant")) != 0 {
		t.Fatal("no real display stock")
	}
	if open, stock := s.ShopStatus(m.Shopfronts[0]); !open || !stock {
		t.Fatal("shop unavailable")
	}
	step(1000)
	if countStore(s, "display") != 0 || countStore(s, "reserve") != 3 || s.ShopAccess(m, "hero", "display", 1000) {
		t.Fatal("closed packed shop accessible")
	}
	if !s.Stores["reserve"].Locked || !s.Stores["source"].Locked {
		t.Fatal("trusted transfer left stock unlocked")
	}
	for day := int64(1); day <= 4; day++ {
		pose["merchant"] = DeliveryPose{Level: "hub", At: Cell{2, 2}, Present: true}
		step(day*86400 + 100)
		pose["merchant"] = DeliveryPose{Level: "market", At: Cell{5, 5}, Present: true}
		step(day*86400 + 200)
	}
	total := int64(0)
	for _, i := range s.Items {
		total += i.Quantity
	}
	if total != 5 || countStore(s, "source") != 0 || countStore(s, "display") != 5 {
		t.Fatalf("minted or lost stock %+v", s.Items)
	}
}
func TestDeliveryCapacityFailureIsAtomicAndProtectedDisplayRemains(t *testing.T) {
	m, s := commerceFixture(t)
	pose := map[string]DeliveryPose{"merchant": {Level: "hub", At: Cell{2, 2}, Present: true}}
	cap := m.Stores[ActorStore("merchant")]
	cap.Capacity.Mass = 200
	m.Stores[cap.ID] = cap
	s.Stores[cap.ID] = cap
	if err := s.AdvanceCommerce(m, 100, pose); err != nil {
		t.Fatal(err)
	}
	if countStore(s, "source") != 5 || len(s.Deliveries["route"].Cargo) > 0 || s.Cursors["merchant"].Sequence != 0 {
		t.Fatal("failed pickup partly applied")
	}
	cap.Capacity.Mass = 10000
	m.Stores[cap.ID] = cap
	s.Stores[cap.ID] = cap
	if err := s.AdvanceCommerce(m, 101, pose); err != nil {
		t.Fatal(err)
	}
	m.Shopfronts[0].PackAway = false
	pose["merchant"] = DeliveryPose{Level: "market", At: Cell{5, 5}, Present: true}
	if err := s.AdvanceCommerce(m, 200, pose); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceCommerce(m, 1000, pose); err != nil {
		t.Fatal(err)
	}
	if countStore(s, "display") != 3 || s.ShopAccess(m, "hero", "display", 1000) {
		t.Fatal("protected stock lost or accessible")
	}
	before := s.Clone()
	if err := s.AdvanceCommerce(m, 999, pose); err == nil || !reflect.DeepEqual(before, s) {
		t.Fatal("reverse time changed delivery")
	}
}

func TestClosedCounterCannotBeBypassedByPortableNestedStorage(t *testing.T) {
	m, s := commerceFixture(t)
	m.Definitions["box"] = Definition{ID: "box", Carrier: "physical", Mass: 10, Volume: 10, StackLimit: 1, ContainerVolume: 1000}
	box := Item{ID: "test.box", Definition: "box", Quantity: 1, Condition: 1000000, Owner: "merchant", Origin: "test", Location: Location{Kind: "store", Store: "display"}}
	s.Items[box.ID] = box
	s.Stores[ItemStore(box.ID)] = Store{ID: ItemStore(box.ID), Kind: "container", Anchor: Anchor{Kind: "item", Item: box.ID}, Capacity: Capacity{Mass: moneyLimit, Volume: 1000, Stacks: m.MaxItems}}
	if err := s.Validate(m); err != nil {
		t.Fatal(err)
	}
	s.Shopfronts["shop"] = ShopProgress{Open: false}
	if s.ShopAccess(m, "hero", ItemStore(box.ID), 500) || s.ShopAccess(m, "hero", "display", 500) {
		t.Fatal("closed display bypassed by nested address")
	}
	if !s.ShopAccess(m, "merchant", ItemStore(box.ID), 500) {
		t.Fatal("owner denied own stock")
	}
	s.Shopfronts["shop"] = ShopProgress{Open: true}
	if !s.ShopAccess(m, "hero", ItemStore(box.ID), 500) {
		t.Fatal("open nested stock denied")
	}
	if s.ShopAccess(m, "hero", ItemStore(box.ID), 1000) {
		t.Fatal("stale open state bypassed closing hours")
	}
}
func TestMissingOrOrphanedSavedConsignmentIsRejected(t *testing.T) {
	m, s := commerceFixture(t)
	if err := s.AdvanceCommerce(m, 100, map[string]DeliveryPose{"merchant": {Level: "hub", At: Cell{2, 2}, Present: true}}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"delivery", "shop", "version", "item"} {
		t.Run(kind, func(t *testing.T) {
			broken := s.Clone()
			switch kind {
			case "delivery":
				delete(broken.Deliveries, "route")
			case "shop":
				delete(broken.Shopfronts, "shop")
			case "version":
				broken.CommerceVersion = 0
			case "item":
				delete(broken.Items, broken.Deliveries["route"].Cargo[0].Item)
			}
			if broken.Validate(m) == nil {
				t.Fatal("accepted damaged commerce save")
			}
			before := digestState(broken)
			if broken.AdvanceCommerce(m, 200, map[string]DeliveryPose{"merchant": {Level: "market", At: Cell{5, 5}, Present: true}}) == nil || digestState(broken) != before {
				t.Fatal("silently migrated damaged in-flight cargo")
			}
		})
	}
}

func TestFullDisplayKeepsServiceOpenAndRefillsAfterPurchase(t *testing.T) {
	m, s := commerceFixture(t)
	display := m.Stores["display"]
	display.Capacity.Mass = 300
	m.Stores["display"] = display
	s.Stores["display"] = display
	original := s.Items[InstanceID("finite", 0, 0)]
	original.ID = "already.displayed"
	original.Quantity = 3
	original.Location = Location{Kind: "store", Store: "display"}
	s.Items[original.ID] = original
	pose := map[string]DeliveryPose{"merchant": {Level: "hub", At: Cell{2, 2}, Present: true}}
	if err := s.AdvanceCommerce(m, 100, pose); err != nil {
		t.Fatal(err)
	}
	pose["merchant"] = DeliveryPose{Level: "market", At: Cell{5, 5}, Present: true}
	if err := s.AdvanceCommerce(m, 200, pose); err != nil {
		t.Fatal(err)
	}
	if !s.Shopfronts["shop"].Open || !s.ShopAccess(m, "hero", "display", 200) || countStore(s, "display") != 3 || countStore(s, "reserve") != 3 {
		t.Fatal("full display locked a staffed shop or discarded reserve")
	}
	item := s.Items[original.ID]
	if _, err := s.Apply(m, Transaction{Actor: "hero", Sequence: 1, Moves: []Move{{Item: item.ID, Quantity: 3, ExpectedRevision: item.Revision, To: Location{Kind: "store", Store: ActorStore("hero")}, ChangeOwner: true, Owner: "hero"}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceCommerce(m, 201, pose); err != nil {
		t.Fatal(err)
	}
	if countStore(s, "display") != 3 || countStore(s, "reserve") != 0 || countStore(s, ActorStore("hero")) != 3 || !s.Shopfronts["shop"].Open {
		t.Fatal("selling display failed to unblock atomic replenishment")
	}
}

// A peddler's mobile shopfront (demo-hub v23): the stores are carried by the owner; the shop opens at any stop in its
// hours, shuts while he walks, and a store at a stop is rejected (no carried stock, no world anchor).
func TestMobileShopfrontOpensAtAnyStopAndOnlyThere(t *testing.T) {
	m := Model{Reservation: ReservationPolicy{MaxTicks: 600, MaxPerActor: 16, MaxTotal: 128}, Version: 1, MaxDepth: 8, MaxItems: 100, Actors: map[string]bool{"peddler": true, "hero": true}, Owners: map[string]bool{}, Levels: map[string]Cell{"market": {10, 10}}, Definitions: map[string]Definition{"goods": {ID: "goods", Carrier: "physical", Mass: 100, Volume: 200, StackLimit: 100}}, Stores: map[string]Store{}, Accounts: map[string]Account{}, Entitlements: map[string]Entitlement{}}
	cap := Capacity{Mass: 10000, Volume: 10000, Stacks: 16}
	for _, id := range []string{"peddler", "hero"} {
		m.Stores[ActorStore(id)] = Store{ID: ActorStore(id), Kind: "actor", Owner: id, Anchor: Anchor{Kind: "actor", Actor: id}, Capacity: cap}
	}
	carried := Anchor{Kind: "actor", Actor: "peddler"}
	m.Stores["tray"] = Store{ID: "tray", Kind: "container", Owner: "peddler", Anchor: carried, Capacity: cap}
	m.Stores["pack"] = Store{ID: "pack", Kind: "container", Owner: "peddler", Anchor: carried, Capacity: cap, Locked: true}
	stops := []DeliveryPoint{{"market", Cell{2, 2}}, {"market", Cell{7, 3}}}
	m.Shopfronts = []Shopfront{{ID: "peddler.x", Owner: "peddler", Service: stops[0], Stops: stops, DisplayStore: "tray", ReserveStore: "pack", OpenSecond: 100, CloseSecond: 900}}
	m.Sources = []Source{{ID: "goods", Definition: "goods", Quantity: 4, Condition: 1000000, Owner: "peddler", ForSale: true, Location: Location{Kind: "store", Store: "tray"}}}
	s, err := New(m, 1)
	if err != nil {
		t.Fatal(err)
	}
	sh := m.Shopfronts[0]
	if !sh.Mobile() || !sh.ServesCell("market", Cell{7, 3}) || sh.ServesCell("market", Cell{5, 5}) {
		t.Fatal("mobile shop serves the wrong cells")
	}
	at := func(sec int64, c Cell, present bool) bool {
		t.Helper()
		if err := s.AdvanceCommerce(m, sec, map[string]DeliveryPose{"peddler": {Level: "market", At: c, Present: present}}); err != nil {
			t.Fatal(err)
		}
		return s.Shopfronts["peddler.x"].Open
	}
	if !at(200, Cell{7, 3}, true) || !s.ShopAccess(m, "hero", "tray", 200) {
		t.Fatal("not open at the second stop")
	}
	if at(210, Cell{5, 5}, true) || s.ShopAccess(m, "hero", "tray", 210) {
		t.Fatal("open between the stops")
	}
	if at(220, Cell{2, 2}, false) {
		t.Fatal("open while walking through a stop")
	}
	if !at(230, Cell{2, 2}, true) || at(950, Cell{2, 2}, true) {
		t.Fatal("hours ignored at the first stop")
	}
	if countStore(s, "tray") != 4 || s.ShopAccess(m, "hero", "pack", 230) {
		t.Fatal("mobile shop lost goods or opened its pack to a stranger")
	}
	bad := m
	bad.Shopfronts = []Shopfront{sh}
	bad.Shopfronts[0].Stops = []DeliveryPoint{stops[1], stops[0]}
	if _, err := New(bad, 1); err == nil {
		t.Fatal("service point must be the first stop")
	}
	bad.Shopfronts[0].Stops = stops
	bad.Stores = map[string]Store{}
	for k, v := range m.Stores {
		bad.Stores[k] = v
	}
	world := bad.Stores["tray"]
	world.Anchor = Anchor{Kind: "world", Level: "market", At: Cell{2, 2}}
	bad.Stores["tray"] = world
	if _, err := New(bad, 1); err == nil {
		t.Fatal("a mobile shop with a world-anchored tray was accepted")
	}
}
