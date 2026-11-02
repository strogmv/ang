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
