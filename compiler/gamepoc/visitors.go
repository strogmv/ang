package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// Hall visitors (world.visitors): people of the other sectors who come to a market level for a few minutes. They are not
// persons: a visitor is a kind, a purse paid out of one outside account, a shopping list and a route entry -> stalls -> exit.
// The host (visitors.go.tmpl) admits them by the calendar and the seed, walks a capped number of bodies over the static
// floor (they block nobody) and buys through the ordinary inventory purchase. ANG adds here what the inventory needs:
//   - the pool of buyer slots (inventory actors with a bag and a CRD account; slot 0 buys for the visitors nobody sees);
//   - the outside account the purses come from and go back to;
//   - the consignment of every stall and peddler of the level: `days` x the starting stock of its showcase behind a
//     lock, with the lots the host lays out again every morning (goods only move, nothing is created).
type VisitorRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}
type VisitorShare struct {
	ID    string `json:"id"`
	Share int64  `json:"share"`
}
type VisitorWant struct {
	Category       string       `json:"category"`
	ChancePermille int64        `json:"chance_permille"`
	Units          VisitorRange `json:"units"`
}
type VisitorKind struct {
	ID      string         `json:"id"`
	Look    string         `json:"look"`
	Share   int64          `json:"share"`
	Purse   VisitorRange   `json:"purse"`
	Entries []VisitorShare `json:"entries"`
	Wants   []VisitorWant  `json:"wants"`
}
type VisitorEntry struct {
	ID string   `json:"id"`
	At inv.Cell `json:"at"`
}
type VisitorTarget struct {
	ID       string   `json:"id"`
	Shop     string   `json:"shop"`
	Category string   `json:"category"`
	Store    string   `json:"store"`
	At       inv.Cell `json:"at"`
	// torgomat v2: a place with its own standing cells (the panel row of a terminal, in preference order; none = the
	// customer's cell or a cell beside it along the counter), the way a walker faces there (plan x, y) and how many ticks he
	// waits after paying (the dispense of a terminal) before he walks on
	Spots     []inv.Cell   `json:"spots,omitempty"`
	Face      *VisitorFace `json:"face,omitempty"`
	TakeTicks int64        `json:"take_ticks,omitempty"`
}
type VisitorFace struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type VisitorLot struct {
	Item string `json:"item"`
	Full int64  `json:"full"`
}
type VisitorSupply struct {
	Shop  string       `json:"shop"`
	Store string       `json:"store"`
	Lots  []VisitorLot `json:"lots"`
}
type VisitorCatalog struct {
	Level       string       `json:"level"`
	OpenSecond  int64        `json:"open_second"`
	CloseSecond int64        `json:"close_second"`
	PerDay      int64        `json:"per_day"`
	Hours       []int64      `json:"hours"`
	MaxBodies   int          `json:"max_bodies"`
	MoveTicks   int64        `json:"move_ticks"`
	DwellTicks  VisitorRange `json:"dwell_ticks"`
	LeaveSecond int64        `json:"leave_second"`
	MaxStops    int          `json:"max_stops"`
	Outside     struct {
		Owner   string `json:"owner"`
		Account string `json:"account"`
		Balance int64  `json:"balance"`
	} `json:"outside"`
	Entries         []VisitorEntry  `json:"entries"`
	Kinds           []VisitorKind   `json:"kinds"`
	Targets         []VisitorTarget `json:"targets"`
	PeddlerCategory string          `json:"peddler_category"`
	Supply          struct {
		Days          int64    `json:"days"`
		WindowMinutes int64    `json:"window_minutes"`
		Skip          []string `json:"skip"`
		// filled by ANG: one rule a stall or peddler with a consignment
		Rules []VisitorSupply `json:"rules,omitempty"`
	} `json:"supply"`
	// filled by ANG: the buyer slots (slot 0 is the unseen buyer)
	Slots []string `json:"slots,omitempty"`
}

const visitorSlotPrefix = "visitor."

func readVisitors(world cue.Value) (*VisitorCatalog, error) {
	v := world.LookupPath(cue.ParsePath("visitors"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.visitors: %w", err)
	}
	var out VisitorCatalog
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.visitors: %w", err)
	}
	return &out, nil
}

// applyVisitors checks the catalog against the compiled Life, inventory and peddlers and adds the slots, the outside
// account and the consignments. It runs after the peddlers (their trays are supplied too) and before crafting (the stock
// behind the locks counts in the starting balance of law.price_market like every other stock for sale).
func applyVisitors(c *Compiled) error {
	p := c.Visitors
	if p == nil {
		return nil
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("world.visitors: %s", fmt.Sprintf(format, args...))
	}
	m := &c.Inventory
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
		} `json:"locations"`
		Actors []struct {
			ID    string `json:"id"`
			Steps []struct {
				Level    string   `json:"level"`
				At       inv.Cell `json:"at"`
				Activity string   `json:"activity"`
			} `json:"steps"`
		} `json:"actors"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return fail("life: %v", err)
	}
	loc, ok := life.Locations[p.Level]
	if !ok {
		return fail("unknown level %s", p.Level)
	}
	walk := func(at inv.Cell) bool {
		return at.Y >= 0 && at.Y < len(loc.Terrain) && at.X >= 0 && at.X < len(loc.Terrain[at.Y]) && c.Walkable[string(loc.Terrain[at.Y][at.X])]
	}
	if p.OpenSecond < 0 || p.CloseSecond > 86400 || p.OpenSecond >= p.CloseSecond || p.OpenSecond%3600 != 0 || p.CloseSecond%3600 != 0 ||
		int64(len(p.Hours)) != (p.CloseSecond-p.OpenSecond)/3600 || p.LeaveSecond < p.CloseSecond || p.LeaveSecond >= 86400 {
		return fail("hours: whole hours inside one day, one share an hour, the last body leaves before midnight")
	}
	var hours int64
	for _, h := range p.Hours {
		if h < 0 {
			return fail("negative share of an hour")
		}
		hours += h
	}
	if hours < 1 || p.PerDay < 1 || p.PerDay > 20000 || p.MaxBodies < 1 || p.MaxBodies > 200 || p.MoveTicks < 1 || p.MoveTicks > 30 ||
		p.DwellTicks.Min < 1 || p.DwellTicks.Max < p.DwellTicks.Min || p.DwellTicks.Max > 6000 || p.MaxStops < 1 || p.MaxStops > 8 {
		return fail("per_day, max_bodies, move_ticks, dwell_ticks or max_stops out of range")
	}
	if len(p.Entries) < 2 || len(p.Kinds) < 1 || len(p.Targets) < 1 || p.PeddlerCategory == "" {
		return fail("needs two entries, a kind, a target and the peddlers' category")
	}
	entries := map[string]bool{}
	for _, e := range p.Entries {
		if e.ID == "" || entries[e.ID] || !walk(e.At) {
			return fail("entry %q: duplicate or not on the floor %v", e.ID, e.At)
		}
		entries[e.ID] = true
	}
	shops := map[string]inv.Shopfront{}
	for _, sh := range m.Shopfronts {
		shops[sh.ID] = sh
	}
	categories := map[string]bool{p.PeddlerCategory: c.Peddlers != nil && c.Peddlers.Level == p.Level && len(c.Peddlers.Peddlers) > 0}
	targets := map[string]bool{}
	for _, t := range p.Targets {
		if t.ID == "" || targets[t.ID] || t.Category == "" || t.Category == p.PeddlerCategory || !walk(t.At) {
			return fail("target %q: duplicate, without a category or not on the floor %v", t.ID, t.At)
		}
		if t.TakeTicks < 0 || t.TakeTicks > 600 || (t.Face != nil && t.Face.X == 0 && t.Face.Y == 0) {
			return fail("target %q: the wait after paying must be 0..600 ticks and a facing must point somewhere", t.ID)
		}
		for _, c := range t.Spots {
			if !walk(c) || manhattanCells(c, t.At) > 2 {
				return fail("target %q: standing cell %v is not on the floor within two cells of the place", t.ID, c)
			}
		}
		targets[t.ID] = true
		st, ok := m.Stores[t.Store]
		if !ok || st.Locked || st.Anchor.Kind != "world" || st.Anchor.Level != p.Level {
			return fail("target %s: showcase %q is not an open store of the level", t.ID, t.Store)
		}
		if t.Shop != "" {
			sh, ok := shops[t.Shop]
			if !ok || sh.DisplayStore != t.Store || sh.Mobile() {
				return fail("target %s: shop %q does not keep showcase %q", t.ID, t.Shop, t.Store)
			}
		}
		categories[t.Category] = true
	}
	kinds := map[string]bool{}
	var share int64
	for _, k := range p.Kinds {
		if k.ID == "" || kinds[k.ID] || k.Look == "" || k.Share < 1 || k.Purse.Min < 1 || k.Purse.Max < k.Purse.Min || len(k.Entries) == 0 || len(k.Wants) == 0 {
			return fail("kind %q: duplicate, or without look, share, purse, entries or wants", k.ID)
		}
		kinds[k.ID] = true
		share += k.Share
		for _, e := range k.Entries {
			if !entries[e.ID] || e.Share < 1 {
				return fail("kind %s: unknown entry %q or no share", k.ID, e.ID)
			}
		}
		seen := map[string]bool{}
		for _, w := range k.Wants {
			if !categories[w.Category] || seen[w.Category] || w.ChancePermille < 1 || w.ChancePermille > 1000 || w.Units.Min < 1 || w.Units.Max < w.Units.Min || w.Units.Max > 12 {
				return fail("kind %s: want %q has no selling place, repeats or has an invalid chance or units", k.ID, w.Category)
			}
			seen[w.Category] = true
		}
	}
	if share < 1 {
		return fail("kinds have no share")
	}
	// the outside account and the buyer slots
	o := p.Outside
	if o.Owner == "" || o.Account == "" || o.Balance < p.PerDay*p.Kinds[0].Purse.Max || m.Owners[o.Owner] || m.Actors[o.Owner] {
		return fail("outside: a new owner, an account and a balance for at least a day")
	}
	if _, dup := m.Accounts[o.Account]; dup {
		return fail("outside account %s exists", o.Account)
	}
	m.Owners[o.Owner] = true
	m.Accounts[o.Account] = inv.Account{ID: o.Account, Owner: o.Owner, Currency: "CRD", Balance: o.Balance}
	p.Slots = nil
	for i := 0; i <= p.MaxBodies; i++ {
		id := fmt.Sprintf("%s%03d", visitorSlotPrefix, i)
		if m.Actors[id] || m.Owners[id] {
			return fail("slot %s exists", id)
		}
		m.Actors[id] = true
		m.Owners[id] = true
		store := inv.ActorStore(id)
		m.Stores[store] = inv.Store{ID: store, Name: "market.store.visitor", Kind: "actor", Anchor: inv.Anchor{Kind: "actor", Actor: id}, Owner: id, Capacity: inv.Capacity{Mass: 25000, Volume: 25000, Stacks: 32}}
		account := "account." + id + ".crd"
		m.Accounts[account] = inv.Account{ID: account, Owner: id, Currency: "CRD"}
		p.Slots = append(p.Slots, id)
	}
	// the consignments
	s := &p.Supply
	if s.Days < 1 || s.Days > 400 || s.WindowMinutes < 10 || s.WindowMinutes > 180 {
		return fail("supply: days 1..400, window 10..180 minutes")
	}
	skip := map[string]bool{}
	for _, id := range s.Skip {
		if _, ok := shops[id]; !ok {
			return fail("supply skips unknown shop %q", id)
		}
		skip[id] = true
	}
	sleeps := map[string]inv.DeliveryPoint{}
	for _, a := range life.Actors {
		for _, st := range a.Steps {
			if _, ok := sleeps[a.ID]; !ok && st.Activity == "sleep" && st.Level != p.Level {
				sleeps[a.ID] = inv.DeliveryPoint{Level: st.Level, At: st.At}
			}
		}
	}
	s.Rules = nil
	var added []inv.Source
	for _, sh := range m.Shopfronts {
		if skip[sh.ID] || sh.Service.Level != p.Level {
			continue
		}
		rule := VisitorSupply{Shop: sh.ID, Store: "commerce/" + sh.ID + "/consignment"}
		anchor := inv.Anchor{Kind: "world", Level: sh.Service.Level, At: sh.Service.At}
		if sh.Mobile() {
			// a peddler keeps his where he sleeps; one who sleeps in the hall carries it in the locked pack
			rule.Store = peddlerStorePrefix + sh.ID + "/stock"
			if at, ok := sleeps[sh.Owner]; ok {
				anchor = inv.Anchor{Kind: "world", Level: at.Level, At: at.At}
			} else {
				anchor = inv.Anchor{Kind: "actor", Actor: sh.Owner}
			}
		}
		if _, dup := m.Stores[rule.Store]; dup {
			return fail("supply store %s exists", rule.Store)
		}
		lots := map[string]int64{}
		for _, src := range m.Sources {
			if src.Location.Kind != "store" || (src.Location.Store != sh.DisplayStore && src.Location.Store != sh.ReserveStore) || src.Owner != sh.Owner || !src.ForSale || src.Rolls != 0 || len(src.Loot) != 0 || src.Quantity < 1 {
				continue
			}
			lots[src.Definition] += src.Quantity
			copy := src
			copy.ID = "consignment." + src.ID
			copy.Quantity = src.Quantity * s.Days
			copy.Location = inv.Location{Kind: "store", Store: rule.Store}
			added = append(added, copy)
		}
		if len(lots) == 0 {
			continue
		}
		for item, full := range lots {
			rule.Lots = append(rule.Lots, VisitorLot{Item: item, Full: full})
		}
		sort.Slice(rule.Lots, func(i, j int) bool { return rule.Lots[i].Item < rule.Lots[j].Item })
		m.Stores[rule.Store] = inv.Store{ID: rule.Store, Name: "market.store.consignment", Kind: "container", Anchor: anchor, Owner: sh.Owner, Locked: true, Capacity: inv.Capacity{Mass: 400000000, Volume: 600000000, Stacks: 20000}}
		s.Rules = append(s.Rules, rule)
	}
	sort.Slice(s.Rules, func(i, j int) bool { return s.Rules[i].Shop < s.Rules[j].Shop })
	m.Sources = append(m.Sources, added...)
	sort.Slice(m.Sources, func(i, j int) bool { return m.Sources[i].ID < m.Sources[j].ID })
	if _, err := inv.New(*m, 425); err != nil {
		return fail("inventory with the visitors and the consignments: %v", err)
	}
	return nil
}

// visitorSlot reports whether an inventory actor is a buyer slot of the visitors (tools that list people skip them).
func visitorSlot(id string) bool { return strings.HasPrefix(id, visitorSlotPrefix) }

func manhattanCells(a, b inv.Cell) int {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}
