package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"

	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// dressFixture: the heap fixture with a dressed counter (a deck, a bench with a lamp, a stack that follows the stock)
// and a dressed bay of another shop.
func dressFixture() Compiled {
	c := heapFixture()
	g := c.Craft
	for id, kind := range map[string]string{
		"market.stall.stand.front_deck": "prop", "market.stall.repair.bench": "prop",
		"market.stall.pancake.stack.full": "prop", "market.stall.pancake.stack.half": "prop", "market.stall.pancake.stack.few": "prop",
		"market.pavilion.clothes.fixture": "fixture", "market.pavilion.clothes.goods.full": "goods", "market.pavilion.clothes.goods.half": "goods", "market.pavilion.clothes.goods.few": "goods",
	} {
		g.Models[id] = CraftModel{Kind: kind, Scene: "res://kit/market-goods/" + id + ".tscn", SizeMM: [3]int{100, 100, 100}, Triangles: 10}
	}
	c.Inventory.Sources[0].ForSale, c.Inventory.Sources[0].Owner = true, "char.t"
	c.Inventory.Shopfronts = append(c.Inventory.Shopfronts, inv.Shopfront{ID: "bay", Owner: "char.b", DisplayStore: "bay/display", ReserveStore: "bay/reserve"})
	c.Inventory.Sources = append(c.Inventory.Sources,
		inv.Source{ID: "c", Definition: "item.fruit", Quantity: 10, Owner: "char.b", ForSale: true, Location: inv.Location{Kind: "store", Store: "bay/display"}},
		inv.Source{ID: "d", Definition: "item.fruit", Quantity: 8, Owner: "char.b", ForSale: true, Location: inv.Location{Kind: "store", Store: "bay/reserve"}})
	g.Dress = []CraftDress{
		{Place: "C1-s0-1", Shop: "shop", Frame: "counter", Props: []CraftProp{
			{ID: "deck", Model: "market.stall.stand.front_deck", XMM: 760, DMM: 165, ZMM: 950, Show: "always"},
			{ID: "bench", Model: "market.stall.repair.bench", XMM: 500, DMM: 250, ZMM: 1090, Show: "always", Lit: true},
			{ID: "stack", Model: "market.stall.pancake.stack.full", XMM: 1000, DMM: 150, ZMM: 1090, Show: "open",
				Stock: &CraftPropStock{Items: []string{"item.fruit"}, Full: 4, Half: "market.stall.pancake.stack.half", Few: "market.stall.pancake.stack.few"}},
		}, Replaces: []string{"old.row"}},
		{Place: "G1", Shop: "bay", Frame: "bay", AtMM: []int{13250, 4750, 3100}, Props: []CraftProp{
			{ID: "fixture", Model: "market.pavilion.clothes.fixture", Show: "open"},
			{ID: "goods", Model: "market.pavilion.clothes.goods.full", Show: "open",
				Stock: &CraftPropStock{Full: 18, Half: "market.pavilion.clothes.goods.half", Few: "market.pavilion.clothes.goods.few"}},
		}},
	}
	return c
}

func TestCraftDressAgainstTheKitTheCounterAndTheStock(t *testing.T) {
	if err := validateCraft(dressFixture()); err != nil {
		t.Fatalf("valid dressed places rejected: %v", err)
	}
	cases := []struct {
		name, want string
		edit       func(c *Compiled)
	}{
		{"place dressed twice", "repeated place", func(c *Compiled) { c.Craft.Dress[1].Place = "C1-s0-1" }},
		{"unknown shop", "is not a shopfront", func(c *Compiled) { c.Craft.Dress[1].Shop = "none" }},
		{"counter of another shop", "no counter of shop", func(c *Compiled) { c.Craft.Dress[0].Shop = "bay" }},
		{"bay without its origin", "plan origin", func(c *Compiled) { c.Craft.Dress[1].AtMM = nil }},
		{"bay replacing a node of no module", "not \"<instance>:<node>\"", func(c *Compiled) { c.Craft.Dress[1].Replaces = []string{"goods.mesh"} }},
		{"unknown frame", "frame", func(c *Compiled) { c.Craft.Dress[1].Frame = "roof" }},
		{"shutter of no module", "not \"<instance>:<node>\"", func(c *Compiled) { c.Craft.Dress[1].Shut = []string{"shutter"} }},
		{"shutter on a counter", "only a bay", func(c *Compiled) { c.Craft.Dress[0].Shut = []string{"island.C1:lid"} }},
		{"prop id repeats", "repeats", func(c *Compiled) { c.Craft.Dress[0].Props[1].ID = "deck" }},
		{"model not in the kit", "not a fixture, goods or prop model", func(c *Compiled) { c.Craft.Dress[0].Props[0].Model = "market.stall.none" }},
		{"a heap is not a prop", "not a fixture, goods or prop model", func(c *Compiled) { c.Craft.Dress[0].Props[0].Model = "market.goods.heap.apple.pyramid.full" }},
		{"unknown show rule", "show", func(c *Compiled) { c.Craft.Dress[0].Props[0].Show = "sometimes" }},
		{"outside the counter's units", "outside the counter's own units", func(c *Compiled) { c.Craft.Dress[0].Props[0].XMM = 3200 }},
		{"on the other half of the island", "outside the counter's own units", func(c *Compiled) { c.Craft.Dress[0].Props[0].DMM = 1900 }},
		{"stock at a shut place", "only at the open place", func(c *Compiled) { c.Craft.Dress[0].Props[2].Show = "always" }},
		{"level model not in the kit", "not a fixture, goods or prop model", func(c *Compiled) { c.Craft.Dress[0].Props[2].Stock.Few = "market.stall.none" }},
		{"goods the shop does not start with", "does not start with", func(c *Compiled) { c.Craft.Dress[0].Props[2].Stock.Items = []string{"item.pear"} }},
		{"full is not the starting stock", "the shop starts with 4", func(c *Compiled) { c.Craft.Dress[0].Props[2].Stock.Full = 24 }},
		{"full of the whole shop is not its stock", "the shop starts with 18", func(c *Compiled) { c.Craft.Dress[1].Props[1].Stock.Full = 10 }},
	}
	for _, tc := range cases {
		c := dressFixture()
		tc.edit(&c)
		err := validateCraft(c)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: want an error with %q, got %v", tc.name, tc.want, err)
		}
	}
}

// visitorFixture: a level of one floor row with a stall, a peddler-less world and the smallest valid visitors catalog.
func visitorFixture(t *testing.T) Compiled {
	t.Helper()
	life, err := json.Marshal(map[string]any{
		"locations": map[string]any{"level.hall": map[string]any{"terrain": []string{"........", "........"}, "cell_mm": 500}},
		"actors": []any{map[string]any{"id": "char.t", "steps": []any{
			map[string]any{"level": "level.home", "at": map[string]int{"X": 1, "Y": 1}, "activity": "sleep"},
			map[string]any{"level": "level.hall", "at": map[string]int{"X": 2, "Y": 0}, "activity": "service"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := Compiled{Life: life, Walkable: map[string]bool{".": true}}
	m := &c.Inventory
	m.Version, m.MaxDepth, m.MaxItems = 1, 4, 100000
	m.Actions = inv.ActionPolicy{ReachCells: 2, FocusTicks: 1, ConsentTicks: 1}
	m.Reservation = inv.ReservationPolicy{MaxTicks: 10, MaxPerActor: 4, MaxTotal: 64}
	m.Definitions = map[string]inv.Definition{"item.thing": {ID: "item.thing", Name: "thing", Carrier: "physical", Mass: 100, Volume: 100, StackLimit: 100, PriceUnit: "CRD", PriceNumerator: 50, PriceDenominator: 1}}
	m.Currencies = map[string]inv.Currency{"CRD": {Code: "CRD", Name: "crd", Carrier: "account", Scale: 100}}
	m.Actors = map[string]bool{"char.t": true}
	m.Owners = map[string]bool{}
	m.Levels = map[string]inv.Cell{"level.hall": {X: 8, Y: 2}, "level.home": {X: 4, Y: 4}}
	m.Markets, m.LevelSettlement, m.IssuePoints, m.Entitlements = map[string]inv.Market{}, map[string]string{}, map[string]inv.IssuePoint{}, map[string]inv.Entitlement{}
	room := inv.Capacity{Mass: 100000, Volume: 100000, Stacks: 64}
	at := inv.Anchor{Kind: "world", Level: "level.hall", At: inv.Cell{X: 2, Y: 0}}
	m.Stores = map[string]inv.Store{
		"actor/char.t": {ID: "actor/char.t", Name: "bag", Kind: "actor", Anchor: inv.Anchor{Kind: "actor", Actor: "char.t"}, Owner: "char.t", Capacity: room},
		"shop/display": {ID: "shop/display", Name: "display", Kind: "container", Anchor: at, Owner: "char.t", Capacity: room},
		"shop/reserve": {ID: "shop/reserve", Name: "reserve", Kind: "container", Anchor: at, Owner: "char.t", Locked: true, Capacity: room},
	}
	m.Accounts = map[string]inv.Account{"account.t": {ID: "account.t", Owner: "char.t", Currency: "CRD"}}
	m.Shopfronts = []inv.Shopfront{{ID: "shop", Owner: "char.t", Service: inv.DeliveryPoint{Level: "level.hall", At: inv.Cell{X: 2, Y: 0}}, DisplayStore: "shop/display", ReserveStore: "shop/reserve", OpenSecond: 7 * 3600, CloseSecond: 21 * 3600}}
	m.Sources = []inv.Source{{ID: "stock.a", Definition: "item.thing", Quantity: 6, Condition: 1000000, Owner: "char.t", ForSale: true, Location: inv.Location{Kind: "store", Store: "shop/display"}}}
	v := &VisitorCatalog{Level: "level.hall", OpenSecond: 6 * 3600, CloseSecond: 8 * 3600, PerDay: 100, Hours: []int64{2, 1}, MaxBodies: 3, MoveTicks: 3,
		DwellTicks: VisitorRange{Min: 10, Max: 20}, LeaveSecond: 8*3600 + 600, MaxStops: 2, WalkStops: 1, PeddlerCategory: "junk",
		Entries: []VisitorEntry{{ID: "a", At: inv.Cell{X: 0, Y: 1}}, {ID: "b", At: inv.Cell{X: 7, Y: 1}}},
		Kinds: []VisitorKind{{ID: "worker", Look: "copper", Share: 1, Purse: VisitorRange{Min: 100, Max: 300}, Entries: []VisitorShare{{ID: "a", Share: 1}},
			Wants: []VisitorWant{{Category: "food", ChancePermille: 500, Units: VisitorRange{Min: 1, Max: 2}}}}},
		Targets: []VisitorTarget{{ID: "shop.shop", Shop: "shop", Category: "food", Store: "shop/display", At: inv.Cell{X: 2, Y: 1}}},
	}
	v.Outside.Owner, v.Outside.Account, v.Outside.Balance = "visitors.outside", "account.visitors.outside", 1000000
	v.Supply.Days, v.Supply.WindowMinutes = 30, 60
	c.Visitors = v
	return c
}

func TestVisitorsAddSlotsTheOutsideAccountAndTheConsignments(t *testing.T) {
	c := visitorFixture(t)
	if err := applyVisitors(&c); err != nil {
		t.Fatalf("valid visitors rejected: %v", err)
	}
	v, m := c.Visitors, c.Inventory
	if len(v.Slots) != v.MaxBodies+1 {
		t.Fatalf("slots %v", v.Slots)
	}
	for _, id := range v.Slots {
		if !visitorSlot(id) || !m.Actors[id] || m.Stores[inv.ActorStore(id)].Kind != "actor" || m.Accounts["account."+id+".crd"].Balance != 0 {
			t.Fatalf("slot %s has no bag or purse account", id)
		}
	}
	if a := m.Accounts[v.Outside.Account]; a.Balance != 1000000 || a.Owner != v.Outside.Owner || !m.Owners[v.Outside.Owner] {
		t.Fatalf("outside account %+v", a)
	}
	if len(v.Supply.Rules) != 1 || v.Supply.Rules[0].Shop != "shop" || len(v.Supply.Rules[0].Lots) != 1 || v.Supply.Rules[0].Lots[0] != (VisitorLot{Item: "item.thing", Full: 6}) {
		t.Fatalf("supply rules %+v", v.Supply.Rules)
	}
	store := m.Stores[v.Supply.Rules[0].Store]
	if !store.Locked || store.Owner != "char.t" || store.Anchor.Level != "level.hall" {
		t.Fatalf("consignment store %+v", store)
	}
	var behind int64
	for _, src := range m.Sources {
		if src.Location.Store == store.ID {
			behind += src.Quantity
		}
	}
	if behind != 6*30 {
		t.Fatalf("consignment holds %d, want thirty days of six", behind)
	}
	cases := []struct {
		name, want string
		edit       func(c *Compiled)
	}{
		{"unknown level", "unknown level", func(c *Compiled) { c.Visitors.Level = "level.none" }},
		{"hours not whole", "hours", func(c *Compiled) { c.Visitors.OpenSecond = 6*3600 + 60 }},
		{"a share an hour", "hours", func(c *Compiled) { c.Visitors.Hours = []int64{1} }},
		{"no visitors", "per_day", func(c *Compiled) { c.Visitors.PerDay = 0 }},
		{"crowd without a limit", "max_bodies", func(c *Compiled) { c.Visitors.MaxBodies = 500 }},
		{"one way in", "two entries", func(c *Compiled) { c.Visitors.Entries = c.Visitors.Entries[:1] }},
		{"entry in a wall", "not on the floor", func(c *Compiled) { c.Visitors.Entries[0].At = inv.Cell{X: 40, Y: 1} }},
		{"showcase behind a lock", "not an open store", func(c *Compiled) { c.Visitors.Targets[0].Store = "shop/reserve" }},
		{"target of another shop", "does not keep showcase", func(c *Compiled) { c.Visitors.Targets[0].Shop = "none" }},
		{"kind without a purse", "purse", func(c *Compiled) { c.Visitors.Kinds[0].Purse = VisitorRange{} }},
		{"kind through an unknown entry", "unknown entry", func(c *Compiled) { c.Visitors.Kinds[0].Entries[0].ID = "z" }},
		{"want without a selling place", "no selling place", func(c *Compiled) { c.Visitors.Kinds[0].Wants[0].Category = "veg" }},
		{"peddlers' goods without peddlers", "no selling place", func(c *Compiled) { c.Visitors.Kinds[0].Wants[0].Category = "junk" }},
		{"outside account too small", "outside", func(c *Compiled) { c.Visitors.Outside.Balance = 10 }},
		{"supply for no days", "supply", func(c *Compiled) { c.Visitors.Supply.Days = 0 }},
		{"supply skips an unknown shop", "skips unknown shop", func(c *Compiled) { c.Visitors.Supply.Skip = []string{"none"} }},
		// torgomat v2: a place's own standing cells are floor next to it; its facing points somewhere; the wait is bounded
		{"standing cell in a wall", "standing cell", func(c *Compiled) { c.Visitors.Targets[0].Spots = []inv.Cell{{X: 40, Y: 1}} }},
		{"standing cell far away", "standing cell", func(c *Compiled) { c.Visitors.Targets[0].Spots = []inv.Cell{{X: 6, Y: 1}} }},
		{"facing nowhere", "facing", func(c *Compiled) { c.Visitors.Targets[0].Face = &VisitorFace{} }},
		{"endless wait after paying", "wait after paying", func(c *Compiled) { c.Visitors.Targets[0].TakeTicks = 601 }},
		// walkers v1, addendum 1: a walker walks to at least one and at most max_stops places; a place's own look is bounded
		{"walkers who walk nowhere", "walk_stops", func(c *Compiled) { c.Visitors.WalkStops = 0 }},
		{"more walks than a list", "walk_stops", func(c *Compiled) { c.Visitors.WalkStops = 3 }},
		{"an endless look", "own look", func(c *Compiled) { c.Visitors.Targets[0].DwellTicks = &VisitorRange{Min: 10, Max: 9000} }},
	}
	// a place with acts: the look of 10..20 ticks taps the panel from tick 2 to 3 ticks before its end; the 30 ticks after
	// paying the hand takes the thing at the window from tick 20 to 26
	acts := func(c *Compiled) {
		c.Visitors.Targets[0].TakeTicks = 30
		c.Visitors.Targets[0].Acts = []VisitorAct{
			{Phase: "stand", Act: "use_panel", ReachM: 0.7, Touches: []VisitorTouch{{At: VisitorPoint{X: 1.25, Y: 0.4, Z: 1.3}, Hand: "right", Part: "fingertips", From: visitorTick(2), ToEnd: visitorTick(3)}}},
			{Phase: "take", Act: "take_from_window", ReachM: 0.7, Touches: []VisitorTouch{{At: VisitorPoint{X: 1.1, Y: 0.4, Z: 0.35}, Hand: "right", Part: "palm", From: visitorTick(20), To: visitorTick(26)}}},
		}
		// the panel cell and one cell beside it along the counter; the far one is out of the hand's reach of the panel
		c.Visitors.Targets[0].Spots = []inv.Cell{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}}
	}
	withActs := visitorFixture(t)
	acts(&withActs)
	if err := applyVisitors(&withActs); err != nil {
		t.Fatalf("valid acts rejected: %v", err)
	}
	if from, to := VisitorTouchSpan(withActs.Visitors.Targets[0].Acts[0].Touches[0], 17); from != 2 || to != 14 {
		t.Fatalf("a look of 17 ticks touches %d..%d, want 2..14", from, to)
	}
	// the cells the acts are done on: the panel cell (middle 1.25, 0.75: the points 0.35 and 0.38 m away); the one beside
	// (1.75) reaches the panel (0.61 m) but not the window (0.74 m), the far cell (2.25) neither: the whole purchase is done
	// from the panel cell, for both acts
	for _, a := range withActs.Visitors.Targets[0].Acts {
		if len(a.Cells) != 1 || a.Cells[0] != (inv.Cell{X: 2, Y: 1}) {
			t.Fatalf("%s is done on %v, want the panel cell only", a.Act, a.Cells)
		}
	}
	actCases := []struct {
		name, want string
		edit       func(c *Compiled)
	}{
		// market people part C: what the hands do with a machine is bounded by the phases and the reach of the place
		{"an act of no phase", "is not stand or take", func(c *Compiled) { c.Visitors.Targets[0].Acts[0].Phase = "walk" }},
		{"two acts of a phase", "a second act", func(c *Compiled) { c.Visitors.Targets[0].Acts[1].Phase = "stand" }},
		{"an act with a strange name", "lower-case words", func(c *Compiled) { c.Visitors.Targets[0].Acts[0].Act = "Use Panel" }},
		{"a take without a wait", "without a wait after paying", func(c *Compiled) { c.Visitors.Targets[0].TakeTicks = 0 }},
		{"a third hand", "hand", func(c *Compiled) { c.Visitors.Targets[0].Acts[0].Touches[0].Hand = "both" }},
		{"from both ends", "give from or from_end", func(c *Compiled) { c.Visitors.Targets[0].Acts[0].Touches[0].FromEnd = visitorTick(3) }},
		{"a touch after the look", "outside a phase", func(c *Compiled) { c.Visitors.Targets[0].Acts[0].Touches[0].From = visitorTick(15) }},
		{"a touch after the dispense", "outside a phase", func(c *Compiled) { c.Visitors.Targets[0].Acts[1].Touches[0].To = visitorTick(31) }},
		{"a touch out of reach", "out of reach", func(c *Compiled) { c.Visitors.Targets[0].Acts[1].Touches[0].At.X = 3.5 }},
		{"an arm of two metres", "not a hand's reach", func(c *Compiled) { c.Visitors.Targets[0].Acts[0].ReachM = 2 }},
		{"a touch above the head", "out of reach", func(c *Compiled) { c.Visitors.Targets[0].Acts[1].Touches[0].At.Z = 2.5 }},
	}
	for _, tc := range actCases {
		c := visitorFixture(t)
		acts(&c)
		tc.edit(&c)
		err := applyVisitors(&c)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: want an error with %q, got %v", tc.name, tc.want, err)
		}
	}
	for _, tc := range cases {
		c := visitorFixture(t)
		tc.edit(&c)
		err := applyVisitors(&c)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: want an error with %q, got %v", tc.name, tc.want, err)
		}
	}
	// a skipped shop keeps its own supply chain: no consignment, no rule
	skipped := visitorFixture(t)
	skipped.Visitors.Supply.Skip = []string{"shop"}
	if err := applyVisitors(&skipped); err != nil || len(skipped.Visitors.Supply.Rules) != 0 {
		t.Fatalf("skipped shop: rules %+v, err %v", skipped.Visitors.Supply.Rules, err)
	}
}

func visitorTick(n int64) *int64 { return &n }
