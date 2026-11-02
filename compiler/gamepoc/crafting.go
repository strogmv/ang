package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/crafting"
	craftingRuntime "github.com/strogmv/ang/compiler/crafting/runtime"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// Crafting stage 1 (doc 93, world.crafting): the CUE catalog plus what only the compiled world knows — the stores of the
// jobs (the hero's work in hands, the orders at each master's counter), the stalls' part stock and the hero's kit as
// finite initial inventory, the accounts of the stations and masters, and the CRD prices of every named definition.
// The content checks of compiler/crafting (reachable inputs, closed price chain, canon or proposal, RU/EN keys, no
// copying) run here against that world; a violation fails the build.

type craftingStock struct {
	Shop    string `json:"shop"`
	Item    string `json:"item"`
	N       int64  `json:"n"`
	Where   string `json:"where"`
	Quality int    `json:"quality"`
	Origin  string `json:"origin"`
}
type craftingKit struct {
	Item      string `json:"item"`
	N         int64  `json:"n"`
	Condition int64  `json:"condition_ppm"`
	Quality   int    `json:"quality"`
	Origin    string `json:"origin"`
}
type craftingInput struct {
	craftingRuntime.Model
	Stock []craftingStock `json:"stock"`
	Kit   []craftingKit   `json:"kit"`
}

const craftingWorkStorePrefix, craftingOrderStorePrefix, craftingStockStorePrefix = "crafting/work/", "crafting/orders/", "crafting/stock/"

func readCrafting(world cue.Value) (*craftingInput, error) {
	v := world.LookupPath(cue.ParsePath("crafting"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.crafting: %w", err)
	}
	var in craftingInput
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&in); err != nil {
		return nil, fmt.Errorf("world.crafting: %w", err)
	}
	var rates []struct {
		From        string `json:"from"`
		To          string `json:"to"`
		Numerator   int64  `json:"numerator"`
		Denominator int64  `json:"denominator"`
	}
	if r := world.LookupPath(cue.ParsePath("economy.reference_rates")); r.Exists() {
		if err := r.Decode(&rates); err != nil {
			return nil, fmt.Errorf("economy.reference_rates: %w", err)
		}
	}
	in.Prices = map[string]int64{}
	for _, r := range rates {
		// the canon reference course CRD -> SLT (3 salt for 1 CRD) values salt-priced things in CRD for the price checks
		if r.From == "CRD" && r.To == "SLT" {
			in.Prices["rate:SLT"] = r.Numerator * 1000 / r.Denominator
		}
	}
	return &in, nil
}

// applyCrafting completes the catalog from the compiled world and adds its stores and finite stock to the inventory.
func applyCrafting(out *Compiled, in *craftingInput) error {
	if in == nil {
		return nil
	}
	m := &out.Inventory
	m2 := in.Model
	account := func(owner string) string {
		ids := make([]string, 0, len(m.Accounts))
		for id, a := range m.Accounts {
			if a.Owner == owner && a.Currency == "CRD" {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		if len(ids) == 0 {
			return ""
		}
		return ids[0]
	}
	shops := map[string]inv.Shopfront{}
	for _, s := range m.Shopfronts {
		shops[s.ID] = s
	}
	hero := m2.Hero.Actor
	if !m.Actors[hero] {
		return fmt.Errorf("world.crafting: hero %s is not an inventory actor", hero)
	}
	m2.Hero.WorkStore = craftingWorkStorePrefix + hero
	m2.Hero.Account = account(hero)
	roomy := inv.Capacity{Mass: 200000, Volume: 400000, Stacks: 64}
	addStore := func(st inv.Store) error {
		if _, exists := m.Stores[st.ID]; exists {
			return fmt.Errorf("world.crafting: duplicate store %s", st.ID)
		}
		m.Stores[st.ID] = st
		return nil
	}
	if err := addStore(inv.Store{ID: m2.Hero.WorkStore, Name: "crafting.store.work", Kind: "container", Anchor: inv.Anchor{Kind: "actor", Actor: hero}, Owner: hero, Locked: true, Capacity: roomy}); err != nil {
		return err
	}
	for i := range m2.Stations {
		s := &m2.Stations[i]
		if _, ok := shops[s.Shop]; !ok || !m.Actors[s.Owner] || shops[s.Shop].Owner != s.Owner {
			return fmt.Errorf("world.crafting: station %s needs the shopfront %s of its owner %s", s.ID, s.Shop, s.Owner)
		}
		s.Account = account(s.Owner)
	}
	for i := range m2.Masters {
		ms := &m2.Masters[i]
		sh, ok := shops[ms.Shop]
		if !ok || sh.Owner != ms.Actor {
			return fmt.Errorf("world.crafting: master %s needs the own shopfront %s", ms.Actor, ms.Shop)
		}
		// the master's working stock has its own locked box at the counter: the commerce runtime unpacks a shop's reserve onto
		// the counter at opening, so the reserve stays the shop's packing space (market_traders.cue)
		stock := craftingStockStorePrefix + ms.Actor
		ms.Stores = []string{sh.DisplayStore, sh.ReserveStore, stock}
		ms.Account = account(ms.Actor)
		ms.OrderStore = craftingOrderStorePrefix + ms.Actor
		for _, st := range []inv.Store{
			{ID: ms.OrderStore, Name: "crafting.store.orders", Kind: "container", Anchor: inv.Anchor{Kind: "world", Level: sh.Service.Level, At: sh.Service.At}, Owner: ms.Actor, Locked: true, Capacity: roomy},
			{ID: stock, Name: "crafting.store.stock", Kind: "container", Anchor: inv.Anchor{Kind: "world", Level: sh.Service.Level, At: sh.Service.At}, Owner: ms.Actor, Locked: true, Capacity: roomy},
		} {
			if err := addStore(st); err != nil {
				return err
			}
		}
	}
	for i, s := range in.Stock {
		sh, ok := shops[s.Shop]
		if !ok {
			return fmt.Errorf("world.crafting: stock at unknown shop %s", s.Shop)
		}
		store := sh.DisplayStore
		if s.Where == "reserve" {
			store = craftingStockStorePrefix + sh.Owner
			if _, ok := m.Stores[store]; !ok {
				return fmt.Errorf("world.crafting: working stock at %s, whose owner is not a master", s.Shop)
			}
		}
		m.Sources = append(m.Sources, inv.Source{ID: fmt.Sprintf("crafting.stock.%s.%03d", s.Shop, i), Definition: s.Item, Quantity: s.N, Condition: 1000000, Owner: sh.Owner, ForSale: s.Where == "display", Location: inv.Location{Kind: "store", Store: store}, Grade: inv.Grade{Quality: s.Quality, Provenance: s.Origin}})
	}
	// stage 2: carriers, caches, residents' things, the terminals of the rebinding, the balance of every part class
	if err := applyCraftingSources(out, &m2, m, shops); err != nil {
		return err
	}
	// the hero's kit goes into the carried bag (the hands have little volume left), else into the hands
	kitStore := inv.ActorStore(hero)
	for _, s := range m.Sources {
		if d := m.Definitions[s.Definition]; s.Owner == hero && d.ContainerVolume > 0 && s.Location == (inv.Location{Kind: "store", Store: inv.ActorStore(hero)}) && len(s.Loot) == 0 {
			kitStore = inv.ItemStore(inv.InstanceID(s.ID, 0, 0))
			break
		}
	}
	for i, k := range in.Kit {
		m.Sources = append(m.Sources, inv.Source{ID: fmt.Sprintf("crafting.kit.%s.%02d", hero, i), Definition: k.Item, Quantity: k.N, Condition: k.Condition, Owner: hero, Location: inv.Location{Kind: "store", Store: kitStore}, Grade: inv.Grade{Quality: k.Quality, Provenance: k.Origin}})
	}
	sort.Slice(m.Sources, func(i, j int) bool { return m.Sources[i].ID < m.Sources[j].ID })
	if _, err := inv.New(*m, 425); err != nil {
		return fmt.Errorf("world.crafting: inventory with the crafting stock: %w", err)
	}
	// CRD prices of every definition the catalog names (salt prices at the canon reference course)
	saltPerCRD := in.Prices["rate:SLT"]
	names := map[string]bool{}
	for _, p := range m2.Parts {
		for _, i := range p.Items {
			names[i.Item] = true
		}
	}
	for _, t := range m2.Tools {
		names[t.Item] = true
	}
	for _, c := range m2.Sources.Carriers {
		names[c.Item] = true
	}
	for _, r := range m2.Recipes {
		if r.Target != "" {
			names[r.Target] = true
		}
		if r.Output != nil {
			names[r.Output.Item] = true
		}
		for _, x := range r.Inputs {
			if x.Item != "" {
				names[x.Item] = true
			}
		}
	}
	m2.Prices = map[string]int64{}
	facts := map[string]crafting.WorldItem{}
	for id := range names {
		d, ok := m.Definitions[id]
		if !ok {
			continue
		}
		price := int64(-1)
		if d.PriceDenominator > 0 {
			p := big.NewRat(d.PriceNumerator, d.PriceDenominator)
			switch d.PriceUnit {
			case "CRD":
			case "SLT":
				if saltPerCRD > 0 {
					p.Mul(p, big.NewRat(m.Currencies["CRD"].Scale*1000, saltPerCRD*m.Currencies["SLT"].Scale))
				} else {
					p = nil
				}
			default:
				p = nil
			}
			if p != nil {
				f, _ := p.Float64()
				price = int64(f + .5)
			}
		}
		if price >= 0 {
			m2.Prices[id] = price
		}
		facts[id] = crafting.WorldItem{PriceMinor: price, StackLimit: d.StackLimit}
	}
	for _, s := range m.Sources {
		defs := []string{s.Definition}
		for _, l := range s.Loot {
			defs = append(defs, l.Definition)
		}
		for _, id := range defs {
			if f, ok := facts[id]; ok {
				f.Present = true
				f.Sold = f.Sold || s.ForSale
				facts[id] = f
			}
		}
	}
	// a master's "at work" action is one of the seller actions of the own craft stall (demo-hub v16 kits)
	if out.Craft != nil {
		for _, ms := range m2.Masters {
			if ms.WorkAction == "" {
				continue
			}
			found := false
			for _, s := range out.Craft.Stalls {
				for _, a := range s.Seller {
					found = found || (s.Trader == ms.Actor && a.ID == ms.WorkAction)
				}
			}
			if !found {
				return fmt.Errorf("world.crafting: master %s works with action %q that the own stall kit has not", ms.Actor, ms.WorkAction)
			}
		}
	}
	if err := m2.Validate(); err != nil {
		return fmt.Errorf("world.crafting: %w", err)
	}
	locales := out.Localization.Locales
	texts := map[string]map[string]string{}
	for _, msg := range out.Localization.Messages {
		texts[msg.Key] = msg.Texts
	}
	localized := func(key string) bool {
		t, ok := texts[key]
		if !ok {
			return false
		}
		for _, l := range locales {
			if t[l] == "" {
				return false
			}
		}
		return true
	}
	if err := crafting.Check(m2, crafting.World{Items: facts, Localized: localized}); err != nil {
		return err
	}
	out.Crafting = &m2
	return nil
}
