package gamepoc

import (
	"encoding/json"
	"testing"

	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// demo-hub v12-vending: a terminal binds walkable panel/approach cells of a Life location, a blocked spine, the system's
// own stores at the right anchors and locks, physical goods priced in the account currency and the system account.
func vendingFixture() Compiled {
	c := Compiled{Walkable: map[string]bool{".": true}}
	c.Life = json.RawMessage(`{"locations":{"hall":{"terrain":["#######","#.....#","#.###.#","#.....#","#######"]}}}`)
	c.Inventory = inv.Model{Owners: map[string]bool{"fact.system": true}, Stores: map[string]inv.Store{}, Definitions: map[string]inv.Definition{},
		Currencies: map[string]inv.Currency{"CRD": {Code: "CRD", Carrier: "account", Scale: 100}}, Accounts: map[string]inv.Account{"system": {ID: "system", Owner: "fact.system", Currency: "CRD"}}}
	c.Inventory.Definitions["item.water"] = inv.Definition{ID: "item.water", Carrier: "physical", PriceUnit: "CRD", PriceNumerator: 30, PriceDenominator: 1}
	c.Inventory.Definitions["item.bottle"] = inv.Definition{ID: "item.bottle", Carrier: "physical", PriceUnit: "CRD", PriceNumerator: 5, PriceDenominator: 1}
	v := &VendingCatalog{Instance: "block", Level: "hall", Owner: "fact.system", Account: "system", BuybackDailyLimitMinor: 500, ServingTicks: 20, RefusalTicks: 30}
	v.Refill.FromSecond, v.Refill.SecondsPerUnit = 3*3600, 600
	v.Light = VendingLight{DayPPM: 1000000, NightPPM: 150000, SensorPPM: 1000000, NightFrom: 82800, NightTo: 18000, Fixture: VendingFixture{Lumens: 1890, BeamDeg: 120}}
	v.Signals.Status = map[string]VendingSignal{}
	for _, s := range []string{"working", "serving", "refused", "service", "fault", "lockdown"} {
		v.Signals.Status[s] = VendingSignal{Color: "#33D46A"}
	}
	v.Signals.Dock = map[string]VendingSignal{"off": {Color: "#000000"}, "charging": {Color: "#2BE3FF"}, "unlocking": {Color: "#F2F6FF"}, "locked": {Color: "#FF3030"}}
	u := VendingUnit{ID: "tm1", N: 1, Kind: "water", Face: "south", Panel: inv.Cell{X: 3, Y: 3}, Spine: inv.Cell{X: 3, Y: 2}, Approach: []inv.Cell{{X: 2, Y: 3}, {X: 3, Y: 3}, {X: 4, Y: 3}}, Lane: []inv.Cell{{X: 3, Y: 3}},
		DisplayStore: "vm/display", CassetteStore: "vm/cassette", IntakeStore: "vm/intake", Sells: []VendingOffer{{Item: "item.water", Par: 10, Reserve: 100}}, Buys: []VendingBuy{{Item: "item.bottle", PriceMinor: 5}}, Dock: "none"}
	v.Units = []VendingUnit{u}
	c.Inventory.Stores["vm/display"] = inv.Store{ID: "vm/display", Owner: "fact.system", Anchor: inv.Anchor{Kind: "world", Level: "hall", At: u.Panel}}
	c.Inventory.Stores["vm/cassette"] = inv.Store{ID: "vm/cassette", Owner: "fact.system", Locked: true, Anchor: inv.Anchor{Kind: "world", Level: "hall", At: u.Spine}}
	c.Inventory.Stores["vm/intake"] = inv.Store{ID: "vm/intake", Owner: "fact.system", Locked: true, Anchor: inv.Anchor{Kind: "world", Level: "hall", At: u.Spine}}
	c.Vending = v
	return c
}

func TestVendingCatalogRejectsTerminalsThatAreNotPhysical(t *testing.T) {
	if err := validateVending(vendingFixture()); err != nil {
		t.Fatal(err)
	}
	if err := validateVending(Compiled{}); err != nil {
		t.Fatal("a world without terminals must stay valid", err)
	}
	for _, kind := range []string{"panel_in_wall", "spine_walkable", "display_locked", "foreign_store", "cassette_at_panel", "unknown_item", "slt_price", "no_account", "foreign_account", "unknown_buyback", "buyback_over_limit", "unknown_service", "par_zero_open", "duplicate_unit", "shared_approach", "refill_past_midnight", "bad_colour"} {
		t.Run(kind, func(t *testing.T) {
			c := vendingFixture()
			v := c.Vending
			u := &v.Units[0]
			switch kind {
			case "panel_in_wall":
				u.Panel = inv.Cell{X: 0, Y: 0}
			case "spine_walkable":
				u.Spine = inv.Cell{X: 1, Y: 1}
			case "display_locked":
				s := c.Inventory.Stores["vm/display"]
				s.Locked = true
				c.Inventory.Stores["vm/display"] = s
			case "foreign_store":
				s := c.Inventory.Stores["vm/intake"]
				s.Owner = "char.thief"
				c.Inventory.Stores["vm/intake"] = s
			case "cassette_at_panel":
				s := c.Inventory.Stores["vm/cassette"]
				s.Anchor.At = u.Panel
				c.Inventory.Stores["vm/cassette"] = s
			case "unknown_item":
				u.Sells[0].Item = "item.invented"
			case "slt_price":
				d := c.Inventory.Definitions["item.water"]
				d.PriceUnit = "SLT"
				c.Inventory.Definitions["item.water"] = d
			case "no_account":
				v.Account = "missing"
			case "foreign_account":
				a := c.Inventory.Accounts["system"]
				a.Owner = "char.hero"
				c.Inventory.Accounts["system"] = a
			case "unknown_buyback":
				u.Buys[0].Item = "item.relic"
			case "buyback_over_limit":
				u.Buys[0].PriceMinor = 501
			case "unknown_service":
				u.Services = []string{"teleport"}
			case "par_zero_open":
				u.Sells[0].Par = 0
			case "duplicate_unit":
				v.Units = append(v.Units, *u)
			case "shared_approach":
				w := *u
				w.ID = "tm2"
				v.Units = append(v.Units, w)
			case "refill_past_midnight":
				v.Refill.FromSecond = 86000
			case "bad_colour":
				v.Signals.Status["refused"] = VendingSignal{Color: "red"}
			}
			if validateVending(c) == nil {
				t.Fatal("accepted a terminal that is not physical or not the system's")
			}
		})
	}
}
