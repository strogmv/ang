package gamepoc

import (
	"fmt"

	craftingRuntime "github.com/strogmv/ang/compiler/crafting/runtime"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// Crafting stage 2 (doc 93 §4, doc 84 п. 21): what only the compiled world knows of the sources of knowledge and the
// rebinding — the carriers as finite instances at a stall, a store or a cache of doc 84; the caches as unlocked containers
// at their entry cell; the equipment terminals of the vending block and the system account that takes the rebinding fee
// and the fines; the residents' worn things; the stock of every part class at the start (law.price_market's balance).

const craftingCacheStorePrefix = "crafting/cache/"

func applyCraftingSources(out *Compiled, m2 *craftingRuntime.Model, m *inv.Model, shops map[string]inv.Shopfront) error {
	add := func(src inv.Source) { m.Sources = append(m.Sources, src) }
	for i := range m2.Sources.Caches {
		c := &m2.Sources.Caches[i]
		c.Store = craftingCacheStorePrefix + c.ID
		if c.Level == "" || (c.At == craftingRuntime.Cell{}) {
			return fmt.Errorf("world.crafting: cache %s has no entry cell in the hub plan", c.ID)
		}
		if _, exists := m.Stores[c.Store]; exists {
			return fmt.Errorf("world.crafting: duplicate store %s", c.Store)
		}
		m.Stores[c.Store] = inv.Store{ID: c.Store, Name: c.Name, Kind: "container", Anchor: inv.Anchor{Kind: "world", Level: c.Level, At: inv.Cell{X: c.At.X, Y: c.At.Y}}, Capacity: inv.Capacity{Mass: 60000, Volume: 120000, Stacks: 16}}
		for k, t := range c.Things {
			if _, ok := m.Definitions[t.Item]; !ok {
				return fmt.Errorf("world.crafting: cache %s holds unknown %s", c.ID, t.Item)
			}
			add(inv.Source{ID: fmt.Sprintf("crafting.cache.%s.%02d", c.ID, k), Definition: t.Item, Quantity: t.N, Condition: t.ConditionPPM, Location: inv.Location{Kind: "store", Store: c.Store}, Grade: inv.Grade{Bound: t.Bound}})
		}
	}
	for k, c := range m2.Sources.Carriers {
		if _, ok := m.Definitions[c.Item]; !ok {
			return fmt.Errorf("world.crafting: carrier %s is not an item", c.Item)
		}
		src := inv.Source{ID: fmt.Sprintf("crafting.carrier.%02d", k), Definition: c.Item, Quantity: c.Place.N, Condition: 1000000}
		switch c.Place.Kind {
		case "shop":
			sh, ok := shops[c.Place.Shop]
			if !ok {
				return fmt.Errorf("world.crafting: carrier %s sold at unknown shop %s", c.Item, c.Place.Shop)
			}
			src.Owner, src.ForSale, src.Location = sh.Owner, true, inv.Location{Kind: "store", Store: sh.DisplayStore}
		case "store":
			st, ok := m.Stores[c.Place.Store]
			if !ok || st.Owner != c.Place.Owner {
				return fmt.Errorf("world.crafting: carrier %s kept in %s, whose owner is not %s", c.Item, c.Place.Store, c.Place.Owner)
			}
			src.Owner, src.Location = c.Place.Owner, inv.Location{Kind: "store", Store: c.Place.Store}
		case "cache":
			src.Quantity, src.Location = 1, inv.Location{Kind: "store", Store: craftingCacheStorePrefix + c.Place.Cache}
		}
		add(src)
	}
	for _, t := range m2.Sources.Teaching {
		if !m.Actors[t.Master] {
			return fmt.Errorf("world.crafting: teacher %s is not an actor", t.Master)
		}
	}
	for _, o := range m2.Sources.Oral {
		if !m.Actors[o.From] {
			return fmt.Errorf("world.crafting: %s tells a schematic but is not an actor", o.From)
		}
	}
	for k, t := range m2.Market.Residents {
		if !m.Actors[t.Actor] {
			return fmt.Errorf("world.crafting: resident %s is not an actor", t.Actor)
		}
		add(inv.Source{ID: fmt.Sprintf("crafting.resident.%s.%02d", t.Actor, k), Definition: t.Item, Quantity: 1, Condition: t.ConditionPPM, Owner: t.Actor, Location: inv.Location{Kind: "store", Store: inv.ActorStore(t.Actor)}})
	}
	// the law: the terminals that rebind and the system account of the fee and the fines; the confiscation store
	if len(m2.Law.Acts) > 0 {
		if out.Vending == nil {
			return fmt.Errorf("world.crafting: rebinding needs the vending block")
		}
		for _, u := range out.Vending.Units {
			if u.Kind == m2.Law.Rebind.TerminalKind {
				m2.Law.Rebind.Terminals = append(m2.Law.Rebind.Terminals, craftingRuntime.Terminal{Unit: u.ID, Level: out.Vending.Level, At: craftingRuntime.Cell{X: u.Panel.X, Y: u.Panel.Y}})
			}
		}
		if len(m2.Law.Rebind.Terminals) == 0 {
			return fmt.Errorf("world.crafting: no %s terminal rebinds", m2.Law.Rebind.TerminalKind)
		}
		m2.Law.Rebind.Account, m2.Law.Ledger.Account = out.Vending.Account, out.Vending.Account
		if _, ok := m.Stores[m2.Law.Rebind.ConfiscateStore]; !ok {
			return fmt.Errorf("world.crafting: confiscation store %s missing", m2.Law.Rebind.ConfiscateStore)
		}
		for _, id := range m2.Law.Obliged {
			if !m.Actors[id] {
				return fmt.Errorf("world.crafting: obliged witness %s is not an actor", id)
			}
		}
	}
	// law.price_market's balance: the units of every part class on the counters and in the masters' boxes at the start
	m2.Economy.Par = map[string]int64{}
	for _, s := range m.Sources {
		if s.Location.Kind != "store" {
			continue
		}
		st := m.Stores[s.Location.Store]
		if !(s.ForSale || len(st.ID) > len(craftingStockStorePrefix) && st.ID[:len(craftingStockStorePrefix)] == craftingStockStorePrefix) {
			continue
		}
		if class, _, ok := m2.PartOf(s.Definition); ok {
			m2.Economy.Par[class.ID] += s.Quantity
		}
	}
	return nil
}
