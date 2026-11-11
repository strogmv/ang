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
	v := &VendingCatalog{Instance: "block", Level: "hall", Owner: "fact.system", Account: "system", BuybackDailyLimitMinor: 500, ServingTicks: 44, RefusalTicks: 30}
	v.Refill.FromSecond, v.Refill.SecondsPerUnit, v.Refill.Order = 3*3600, 600, []string{"tm1"}
	// torgomat v2: the dispense adds up to the band, the buy-back fits it, the scenes are in the kit, the robot drives round
	d := &v.Dispense
	d.RingTicks, d.LiftTicks, d.WindowTicks, d.OpenTicks, d.TakeTicks, d.CloseTicks = 6, 18, 4, 5, 7, 4
	k := &v.Intake
	k.LightTicks, k.OpenTicks, k.TakeTicks, k.CloseTicks = 5, 5, 6, 5
	v.Kit = []string{"res://k/water.glb", "res://k/water.full.glb", "res://k/water.half.glb", "res://k/water.few.glb", "res://k/package.glb", "res://k/robot.glb"}
	v.Models = map[string]VendingModel{"item.water": {Item: "res://k/water.glb", Full: "res://k/water.full.glb", Half: "res://k/water.half.glb", Few: "res://k/water.few.glb"}}
	v.NeutralModel = "res://k/package.glb"
	v.Robot = VendingRobot{Resource: "res://k/robot.glb", Home: inv.Cell{X: 1, Y: 1}, Out: []inv.Cell{{X: 1, Y: 1}, {X: 5, Y: 1}, {X: 5, Y: 2}},
		Back: []inv.Cell{{X: 5, Y: 2}, {X: 5, Y: 1}, {X: 1, Y: 1}}, DockCell: inv.Cell{X: 5, Y: 2}, Dock: VendingDock{X: 2.8, Y: 1.25, YawDegrees: 180},
		CellSeconds: 3, CoupleSeconds: 40, FillSeconds: 480, GiveWaySeconds: 120}
	v.Look = map[string]any{"glass": map[string]any{"alpha": 0.05}}
	v.Light = VendingLight{DayPPM: 1000000, NightPPM: 150000, SensorPPM: 1000000, NightFrom: 82800, NightTo: 18000, Fixture: VendingFixture{Lumens: 1890, BeamDeg: 120}}
	v.Signals.Status = map[string]VendingSignal{}
	for _, s := range []string{"working", "serving", "refused", "service", "fault", "lockdown"} {
		v.Signals.Status[s] = VendingSignal{Color: "#33D46A"}
	}
	v.Signals.Dock = map[string]VendingSignal{"off": {Color: "#000000"}, "charging": {Color: "#2BE3FF"}, "unlocking": {Color: "#F2F6FF"}, "locked": {Color: "#FF3030"}}
	u := VendingUnit{ID: "tm1", N: 1, Kind: "water", Face: "south", Panel: inv.Cell{X: 3, Y: 3}, Spine: inv.Cell{X: 3, Y: 2}, Approach: []inv.Cell{{X: 2, Y: 3}, {X: 3, Y: 3}, {X: 4, Y: 3}}, Lane: []inv.Cell{{X: 3, Y: 3}},
		DisplayStore: "vm/display", CassetteStore: "vm/cassette", IntakeStore: "vm/intake", Sells: []VendingOffer{{Item: "item.water", Par: 10, Reserve: 100}}, Buys: []VendingBuy{{Item: "item.bottle", PriceMinor: 5}}, Dock: "none",
		Shelves: []VendingShelf{{ID: "s0", Item: "item.water", Units: 6, Kind: "shelf"}, {ID: "s1", Item: "item.water", Units: 4, Kind: "shelf"}}, Lift: &VendingLift{RestZ: 0.786, WindowZ: 0.366}}
	v.Units = []VendingUnit{u}
	c.Inventory.Stores["vm/display"] = inv.Store{ID: "vm/display", Owner: "fact.system", Anchor: inv.Anchor{Kind: "world", Level: "hall", At: u.Panel}}
	c.Inventory.Stores["vm/cassette"] = inv.Store{ID: "vm/cassette", Owner: "fact.system", Locked: true, Anchor: inv.Anchor{Kind: "world", Level: "hall", At: u.Spine}}
	c.Inventory.Stores["vm/intake"] = inv.Store{ID: "vm/intake", Owner: "fact.system", Locked: true, Anchor: inv.Anchor{Kind: "world", Level: "hall", At: u.Spine}}
	c.Vending = v
	return c
}

func TestVendingCatalogRejectsTerminalsThatAreNotPhysical(t *testing.T) {
	ok := vendingFixture()
	if err := validateVending(ok); err != nil {
		t.Fatal(err)
	}
	// the cabinet's legs: home -> the dock cell (5 cells: east, south) and back; the coupled pose ends / begins them
	r := ok.Vending.Robot
	if r.OutLeg == nil || r.BackLeg == nil || len(r.OutLeg.Cells) != 6 || r.OutLeg.Seconds != 15 || r.BackLeg.Seconds != 15 ||
		r.OutLeg.Points[5] != [2]float64{2.8, 1.25} || r.BackLeg.Points[0] != [2]float64{2.8, 1.25} || r.BackLeg.Cells[5] != r.Home {
		t.Fatalf("robot legs out %+v back %+v", r.OutLeg, r.BackLeg)
	}
	if err := validateVending(Compiled{}); err != nil {
		t.Fatal("a world without terminals must stay valid", err)
	}
	for _, kind := range []string{"panel_in_wall", "spine_walkable", "display_locked", "foreign_store", "cassette_at_panel", "unknown_item", "slt_price", "no_account", "foreign_account", "unknown_buyback", "buyback_over_limit", "unknown_service", "par_zero_open", "duplicate_unit", "shared_approach", "refill_past_midnight", "bad_colour",
		"shelves_short_of_par", "shelf_of_a_foreign_good", "shelf_bay_on_a_showcase", "scene_not_in_kit", "dispense_not_the_band", "buyback_longer_than_band",
		"order_misses_a_unit", "order_twice", "robot_through_a_wall", "robot_diagonal", "robot_not_home", "robot_too_slow", "robot_misses_the_dock", "robot_docks_far", "robot_out_of_the_night", "robot_too_wide_for_its_lane", "showcase_without_lift", "no_look"} {
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
			case "shelves_short_of_par":
				u.Shelves[1].Units = 3
			case "shelf_of_a_foreign_good":
				u.Shelves[1].Item = "item.bottle"
				v.Models["item.bottle"] = v.Models["item.water"]
			case "shelf_bay_on_a_showcase":
				u.Shelves[0].Kind = "bay"
			case "scene_not_in_kit":
				m := v.Models["item.water"]
				m.Half = "res://k/elsewhere.glb"
				v.Models["item.water"] = m
			case "dispense_not_the_band":
				v.Dispense.TakeTicks++
			case "buyback_longer_than_band":
				v.Intake.TakeTicks = 40
			case "order_misses_a_unit":
				v.Refill.Order = []string{"tm9"}
			case "order_twice":
				v.Refill.Order = []string{"tm1", "tm1"}
			case "robot_through_a_wall":
				v.Robot.Out = []inv.Cell{{X: 1, Y: 1}, {X: 1, Y: 3}, {X: 3, Y: 3}, {X: 3, Y: 2}, {X: 5, Y: 2}}
			case "robot_diagonal":
				v.Robot.Out = []inv.Cell{{X: 1, Y: 1}, {X: 5, Y: 2}}
			case "robot_not_home":
				v.Robot.Back = v.Robot.Back[:len(v.Robot.Back)-1]
			case "robot_too_slow":
				v.Robot.FillSeconds = 601
			case "robot_misses_the_dock":
				v.Robot.Out = []inv.Cell{{X: 1, Y: 1}, {X: 5, Y: 1}}
			case "robot_docks_far":
				v.Robot.Dock.X = 4.0
			case "robot_out_of_the_night":
				v.Refill.FromSecond = 10
			case "robot_too_wide_for_its_lane":
				v.Robot.ClearanceCells = 1
			case "showcase_without_lift":
				u.Lift = nil
			case "no_look":
				v.Look = nil
			}
			if validateVending(c) == nil {
				t.Fatal("accepted a terminal that is not physical or not the system's")
			}
		})
	}
}
