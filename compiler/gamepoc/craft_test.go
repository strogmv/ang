package gamepoc

import (
	"strings"
	"testing"

	inv "github.com/strogmv/ang/compiler/inventory/runtime"
	"github.com/strogmv/ang/compiler/locationsdk"
)

// craftFixture: one island module (passport hub.island.loop, 6.0 x 3.5 x 1.25 m), one stall of two units with a row, a
// powered device wired to the bus, a light of that device, a sign slot of the island; the trader runs the shopfront.
func craftFixture() Compiled {
	var c Compiled
	p := CraftPassport{ID: "hub.island.loop", Levels: []CraftLevel{
		{ID: "front", ZMM: 950, ZMaxMM: 950, DMM: [2]int{30, 290}, HeightMM: 400},
		{ID: "riser", ZMM: 1090, ZMaxMM: 1090, DMM: [2]int{300, 490}, HeightMM: 1100},
		{ID: "rail", ZMM: 1950, ZMaxMM: 1950, DMM: [2]int{30, 300}, HeightMM: 500, Hang: true},
	}, Bus: []CraftBus{{ID: "counter", DMM: [2]int{500, 540}, ZMM: [2]int{820, 900}}}}
	p.SizeMM.W, p.SizeMM.D, p.SizeMM.H = 6000, 3500, 1250
	p.Cells.W, p.Cells.D = 12, 7
	stall := CraftStall{Place: "C1-s0-1", Island: "C1", Face: "south", Units: []string{"s0", "s1"}, XMM: [2]int{0, 3000}, Trader: "char.t", Shop: "shop",
		Kit: "kit.k", BrainPlace: "shop.shop", Signs: []string{"C1-s0-1.sign"},
		Rows:    []CraftRow{{ID: "pots", Mount: "riser", Item: "pot", N: 10, PitchMM: 100, X0MM: 100, DMM: 395, ZMM: 1090, WMM: 90, HMM: 300}},
		Devices: []CraftDevice{{ID: "griddle", Item: "griddle", Mount: "front", AtMM: [3]int{500, 150, 950}, WMM: 400, HMM: 70, Watts: 1500, Cable: &CraftCable{RouteMM: [][3]int{{600, 250, 980}, {600, 520, 980}, {600, 520, 860}}}}},
		Lights:  []CraftLight{{ID: "glow", Kind: "glow", Watts: 6, AtMM: [3]int{500, 150, 1150}, Of: "griddle"}},
		Seller:  []CraftAction{{ID: "flip", Clip: "Interact", Src: "UAL1", Seconds: 6, Weight: 1, Prop: "spatula"}},
		Effects: []CraftEffect{{ID: "steam", Kind: "steam", Particles: 10, During: []string{"flip"}}}}
	c.Craft = &CraftCatalog{Level: "level.c7_market", Passport: p, Kits: []CraftKit{{ID: "kit.k", Closure: CraftClosure{Mode: "pack_away", PhaseMinutes: 10}}},
		Stalls: []CraftStall{stall}, HandProps: []string{"spatula"}}
	c.LocationSDK = &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{{ID: "level.c7_market",
		Instances: []locationsdk.CompiledInstance{{ID: "island.C1", ModuleID: "kit.geo.hub.island.loop", Width: 12, Depth: 7, Height: 3}},
		TextSlots: []locationsdk.TextSlot{{ID: "C1-s0-1.sign", Instance: "island.C1"}}}}}
	c.Life = []byte(`{"brain":{"places":[{"id":"shop.shop"}]}}`)
	c.Inventory.Shopfronts = []inv.Shopfront{{ID: "shop", Owner: "char.t"}}
	return c
}

func TestCraftKitsAgainstThePassport(t *testing.T) {
	if err := validateCraft(craftFixture()); err != nil {
		t.Fatalf("valid craft place rejected: %v", err)
	}
	cases := []struct {
		name, want string
		edit       func(c *Compiled)
	}{
		{"level not in the passport", "not a level of the passport", func(c *Compiled) { c.Craft.Stalls[0].Rows[0].Mount = "shelf" }},
		{"nothing on the floor", "below the counter top", func(c *Compiled) { c.Craft.Passport.Levels[0].ZMM = 0 }},
		{"row outside the stall's units", "outside the stall's units", func(c *Compiled) { c.Craft.Stalls[0].Rows[0].N = 40 }},
		{"row too tall for the level", "outside level riser", func(c *Compiled) { c.Craft.Stalls[0].Rows[0].HMM = 1500 }},
		{"row off its level height", "level riser is at", func(c *Compiled) { c.Craft.Stalls[0].Rows[0].ZMM = 1200 }},
		{"module changed", "the passport hub.island.loop says", func(c *Compiled) { c.LocationSDK.Levels[0].Instances[0].Width = 13 }},
		{"consumer without a cable", "no cable to the island bus", func(c *Compiled) { c.Craft.Stalls[0].Devices[0].Cable = nil }},
		{"cable not ending on the bus", "does not end on the island bus", func(c *Compiled) { c.Craft.Stalls[0].Devices[0].Cable.RouteMM[2] = [3]int{600, 300, 980} }},
		{"cable on the floor", "below the counter", func(c *Compiled) { c.Craft.Stalls[0].Devices[0].Cable.RouteMM[1] = [3]int{600, 520, 10} }},
		{"light without a feed", "not a fixture or device", func(c *Compiled) { c.Craft.Stalls[0].Lights[0].Of = "nothing" }},
		{"inscription outside the slots", "not a world-text slot", func(c *Compiled) { c.Craft.Stalls[0].Signs = []string{"C1-s0-1.banner"} }},
		{"trader does not run the shop", "is not run by", func(c *Compiled) { c.Craft.Stalls[0].Trader = "char.other" }},
		{"buyers' place missing", "brain place", func(c *Compiled) { c.Craft.Stalls[0].BrainPlace = "shop.none" }},
		{"unknown hand prop", "unknown prop", func(c *Compiled) { c.Craft.Stalls[0].Seller[0].Prop = "hammer" }},
		{"effect during an unknown action", "not a seller action", func(c *Compiled) { c.Craft.Stalls[0].Effects[0].During = []string{"dance"} }},
	}
	for _, tc := range cases {
		c := craftFixture()
		tc.edit(&c)
		err := validateCraft(c)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: want an error with %q, got %v", tc.name, tc.want, err)
		}
	}
}

// heapFixture: the craft fixture with a deck level, a front deck and one heap slot of the produce kit; the stall's display
// stocks the slot's item and a locked consignment of the trader holds more of it.
func heapFixture() Compiled {
	c := craftFixture()
	g := c.Craft
	g.Passport.Levels = append(g.Passport.Levels, CraftLevel{ID: "deck", ZMM: 1090, ZMaxMM: 1090, DMM: [2]int{30, 490}, HeightMM: 620})
	g.Models = map[string]CraftModel{
		"market.goods.container.tray":          {Kind: "container", Scene: "res://kit/market-goods/market.goods.container.tray.tscn", SizeMM: [3]int{620, 460, 60}, Triangles: 100},
		"market.goods.container.tray.red":      {Kind: "container", Scene: "res://kit/market-goods/market.goods.container.tray.red.tscn", SizeMM: [3]int{620, 460, 60}, Triangles: 100},
		"market.goods.container.crate":         {Kind: "container", Scene: "res://kit/market-goods/market.goods.container.crate.tscn", SizeMM: [3]int{312, 412, 260}, Triangles: 100},
		"market.goods.heap.apple.pyramid.full": {Kind: "heap", Scene: "res://kit/market-goods/a.full.tscn", SizeMM: [3]int{560, 420, 530}, Triangles: 5000, Produce: "apple", Fill: "full", Container: "market.goods.container.tray"},
		"market.goods.heap.apple.pyramid.half": {Kind: "heap", Scene: "res://kit/market-goods/a.half.tscn", SizeMM: [3]int{540, 400, 400}, Triangles: 3000, Produce: "apple", Fill: "half", Container: "market.goods.container.tray"},
		"market.goods.heap.apple.pyramid.few":  {Kind: "heap", Scene: "res://kit/market-goods/a.few.tscn", SizeMM: [3]int{300, 300, 120}, Triangles: 600, Produce: "apple", Fill: "few", Container: "market.goods.container.tray"},
		"market.goods.heap.plum.crate.half":    {Kind: "heap", Scene: "res://kit/market-goods/p.half.tscn", SizeMM: [3]int{300, 300, 200}, Triangles: 600, Produce: "plum", Fill: "half", Container: "market.goods.container.tray"},
	}
	s := &g.Stalls[0]
	s.Rows, s.Devices, s.Lights, s.Effects = nil, nil, nil, nil
	s.Fixtures = []CraftFixture{{ID: "deck.w", Kind: "front_deck", Mount: "front", XMM: [2]int{40, 1480}, DMM: [2]int{30, 300}, ZMM: [2]int{950, 1090}}}
	h := CraftHeap{ID: "fruit", Item: "item.fruit", Mount: "deck", Tare: "market.goods.container.tray", XMM: 400, DMM: 260, ZMM: 1090, Full: 4}
	h.Heaps.Full, h.Heaps.Half, h.Heaps.Few = "market.goods.heap.apple.pyramid.full", "market.goods.heap.apple.pyramid.half", "market.goods.heap.apple.pyramid.few"
	s.Heaps = []CraftHeap{h}
	s.Supply = &CraftSupply{Store: "shop/consignment", Days: 30, WindowMinutes: 60}
	g.Counters = []CraftCounter{{Place: "C1-s0-1", Island: "island.C1", Shop: "shop", Side: "south", Units: []int{0, 1}, PackAway: true}}
	c.Inventory.Shopfronts = []inv.Shopfront{{ID: "shop", Owner: "char.t", DisplayStore: "shop/display", ReserveStore: "shop/reserve", PackAway: true}}
	c.Inventory.Definitions = map[string]inv.Definition{"item.fruit": {}}
	c.Inventory.Stores = map[string]inv.Store{"shop/consignment": {ID: "shop/consignment", Owner: "char.t", Locked: true}}
	c.Inventory.Sources = []inv.Source{
		{ID: "a", Definition: "item.fruit", Quantity: 4, Location: inv.Location{Kind: "store", Store: "shop/display"}},
		{ID: "b", Definition: "item.fruit", Quantity: 120, Location: inv.Location{Kind: "store", Store: "shop/consignment"}},
	}
	return c
}

func TestCraftHeapSlotsAgainstTheKitAndTheStall(t *testing.T) {
	if err := validateCraft(heapFixture()); err != nil {
		t.Fatalf("valid heap stall rejected: %v", err)
	}
	second := func(c *Compiled, x int) {
		h := c.Craft.Stalls[0].Heaps[0]
		h.ID, h.XMM = "more", x
		c.Craft.Stalls[0].Heaps = append(c.Craft.Stalls[0].Heaps, h)
	}
	cases := []struct {
		name, want string
		edit       func(c *Compiled)
	}{
		{"tare not in the kit", "not a tare of the produce kit", func(c *Compiled) { c.Craft.Stalls[0].Heaps[0].Tare = "market.goods.container.bucket" }},
		{"heap not in the kit", "not a full heap of the produce kit", func(c *Compiled) { c.Craft.Stalls[0].Heaps[0].Heaps.Full = "market.goods.heap.pear" }},
		{"wrong level of a heap", "not a few heap of the produce kit", func(c *Compiled) { c.Craft.Stalls[0].Heaps[0].Heaps.Few = "market.goods.heap.apple.pyramid.half" }},
		{"heap of another tare", "the slot's tare is", func(c *Compiled) { c.Craft.Stalls[0].Heaps[0].Tare = "market.goods.container.crate" }},
		{"two kinds in one slot", "mixes apple and plum", func(c *Compiled) { c.Craft.Stalls[0].Heaps[0].Heaps.Half = "market.goods.heap.plum.crate.half" }},
		{"off the level height", "level deck is at", func(c *Compiled) { c.Craft.Stalls[0].Heaps[0].ZMM = 950 }},
		{"outside the stall's units", "outside the stall's units", func(c *Compiled) { c.Craft.Stalls[0].Heaps[0].XMM = 2900 }},
		{"too deep for the level", "mm deep at depth", func(c *Compiled) { c.Craft.Stalls[0].Heaps[0].DMM = 300 }},
		{"no deck under the slot", "without a front deck", func(c *Compiled) { c.Craft.Stalls[0].Heaps[0].XMM = 2000 }},
		{"slots overlap", "overlap", func(c *Compiled) { second(c, 700) }},
		{"item not on the display", "does not stock", func(c *Compiled) { c.Inventory.Sources = c.Inventory.Sources[1:] }},
		{"consignment not locked", "not a locked store", func(c *Compiled) {
			st := c.Inventory.Stores["shop/consignment"]
			st.Locked = false
			c.Inventory.Stores["shop/consignment"] = st
		}},
		{"consignment without the item", "holds no item.fruit", func(c *Compiled) { c.Inventory.Sources = c.Inventory.Sources[:1] }},
		{"supply without slots", "a supply without heap slots", func(c *Compiled) { c.Craft.Stalls[0].Heaps = nil }},
		{"counter units not adjacent", "not adjacent units", func(c *Compiled) { c.Craft.Counters[0].Units = []int{0, 2} }},
		{"counter unit taken twice", "also belongs to", func(c *Compiled) {
			c.Craft.Counters = append(c.Craft.Counters, CraftCounter{Place: "C1-s1", Island: "island.C1", Side: "south", Units: []int{1}})
		}},
		{"counter of an unknown shop", "is not a shopfront", func(c *Compiled) { c.Craft.Counters[0].Shop = "other" }},
	}
	for _, tc := range cases {
		c := heapFixture()
		tc.edit(&c)
		err := validateCraft(c)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: want an error with %q, got %v", tc.name, tc.want, err)
		}
	}
	// a colour variant of the heap's tare is the same tare; two decks carry a tray across their joint
	c := heapFixture()
	c.Craft.Stalls[0].Heaps[0].Tare = "market.goods.container.tray.red"
	c.Craft.Stalls[0].Fixtures = append(c.Craft.Stalls[0].Fixtures, CraftFixture{ID: "deck.e", Kind: "front_deck", Mount: "front", XMM: [2]int{1520, 2960}, DMM: [2]int{30, 300}, ZMM: [2]int{950, 1090}})
	second(&c, 1500)
	if err := validateCraft(c); err != nil {
		t.Fatalf("tray across the joint of two decks rejected: %v", err)
	}
	rect := clientCraftCounters(c.Craft)[0]
	if got := rect["rect_m"].([]float64); got[0] != 0 || got[1] != 1.75 || got[2] != 3 || got[3] != 3.5 || rect["goods_prefixes"].([]string)[1] != "C1-s1_" {
		t.Fatalf("counter rectangle and prefixes: %v", rect)
	}
}

// Market food court v1: a counter may stand on an instance of a counter module of the data (a half-island: one straight
// of the loop, counters of one side only, 1.5 m deep); a counter on any other module is still refused, and the client gets
// the half-island's own depth for the counter's rectangle.
func TestCraftCountersOnHalfIslandModules(t *testing.T) {
	c := craftFixture()
	c.LocationSDK.Levels[0].Instances = append(c.LocationSDK.Levels[0].Instances,
		locationsdk.CompiledInstance{ID: "island.F5", ModuleID: "kit.geo.hub.island.half", Width: 12, Depth: 3, Height: 3})
	c.Inventory.Shopfronts = append(c.Inventory.Shopfronts, inv.Shopfront{ID: "tenant31", Owner: "char.r"})
	c.Craft.Counters = []CraftCounter{{Place: "F5-n0-1", Island: "island.F5", Shop: "tenant31", Side: "north", Units: []int{0, 1}, DepthMM: 1500}}
	if err := validateCraft(c); err == nil || !strings.Contains(err.Error(), "is not an island module of the level") {
		t.Fatalf("a counter on an unlisted module must be refused, got %v", err)
	}
	c.Craft.CounterModules = []string{"kit.geo.hub.island.half"}
	if err := validateCraft(c); err != nil {
		t.Fatalf("a counter on a half-island module rejected: %v", err)
	}
	rows := clientCraftCounters(c.Craft)
	rect := rows[0]["rect_m"].([]float64)
	if rect[3]-rect[1] != 1.5 {
		t.Fatalf("the half-island counter's rectangle is %v deep, want its 1.5 m", rect[3]-rect[1])
	}
	c.Craft.Counters[0].DepthMM = 4000
	if err := validateCraft(c); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("a counter deeper than the passport must be refused, got %v", err)
	}
}
