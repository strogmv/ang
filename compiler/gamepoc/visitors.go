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
	// walkers v1, addendum 1: how long a walker looks here before he buys (a terminal: a short look; none = the catalog's)
	DwellTicks *VisitorRange `json:"dwell_ticks,omitempty"`
	// market people part C: what his hands do with the machine of the place in a phase (a terminal's panel, its window); the
	// view names the act and the touches while he stands on a standing cell of the place
	Acts []VisitorAct `json:"acts,omitempty"`
}

// VisitorAct: in the phase `phase` of a place (stand: the look before paying; take: the wait after paying) the walker does
// the act `act` (a general name of what a person does with a machine: use_panel, take_from_window), his hand meeting the
// machine at the touches.
type VisitorAct struct {
	Phase   string         `json:"phase"`
	Act     string         `json:"act"`
	Touches []VisitorTouch `json:"touches"`
	// how far across the plan a hand reaches from the middle of a standing cell: the acts of a place are done only on its
	// standing cells whose middle is within reach_m of every touch of every act of the place — the whole purchase from one
	// cell (a walker on another standing cell of the place waits beside the one who uses the machine)
	ReachM float64 `json:"reach_m"`
	// filled by ANG: those standing cells (the same for every act of the place)
	Cells []inv.Cell `json:"cells,omitempty"`
}

// VisitorTouch: the point the hand meets (plan metres of the level: x east, y south, z up from the floor), which hand and
// which part of it, from and to which tick of the phase — counted from its start (from, to) or back from its end
// (from_end, to_end; the look lasts as long as the walker's own draw). Exactly one of each pair.
type VisitorTouch struct {
	At      VisitorPoint `json:"at"`
	Hand    string       `json:"hand"`
	Part    string       `json:"part"`
	From    *int64       `json:"from,omitempty"`
	FromEnd *int64       `json:"from_end,omitempty"`
	To      *int64       `json:"to,omitempty"`
	ToEnd   *int64       `json:"to_end,omitempty"`
}
type VisitorPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// visitorActNames: a general name of what a person does with a machine (lower case words joined by "_").
func visitorActName(s string) bool {
	if s == "" || s[0] == '_' || s[len(s)-1] == '_' {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && r != '_' {
			return false
		}
	}
	return true
}

// checkVisitorActs: the acts of a place against its phases (the look of `dwell` ticks, the dispense of take ticks) and its
// standing cells (cell_mm: the size of a cell of the level): every touch lies within the phase at its shortest and longest,
// in reach of a standing cell (1.2 m across the plan, 0..2.2 m high), with a known hand and part.
func checkVisitorActs(t *VisitorTarget, dwell VisitorRange, cellMM int) error {
	if len(t.Acts) == 0 {
		return nil
	}
	if cellMM <= 0 {
		return fmt.Errorf("its level has no cell size for the touches of its acts")
	}
	cells := t.Spots
	if len(cells) == 0 {
		cells = []inv.Cell{t.At}
	}
	cell := float64(cellMM) / 1000
	seen := map[string]bool{}
	for ai := range t.Acts {
		a := &t.Acts[ai]
		lengths := []int64{}
		switch a.Phase {
		case "stand":
			lengths = append(lengths, dwell.Min, dwell.Max)
		case "take":
			if t.TakeTicks < 1 {
				return fmt.Errorf("act %q in the take phase of a place without a wait after paying", a.Act)
			}
			lengths = append(lengths, t.TakeTicks)
		default:
			return fmt.Errorf("act %q: phase %q is not stand or take", a.Act, a.Phase)
		}
		if seen[a.Phase] || !visitorActName(a.Act) || len(a.Touches) < 1 || len(a.Touches) > 4 {
			return fmt.Errorf("act %q: a second act of phase %s, a name that is not lower-case words, or not 1..4 touches", a.Act, a.Phase)
		}
		seen[a.Phase] = true
		for k, h := range a.Touches {
			if (h.Hand != "right" && h.Hand != "left") || (h.Part != "fingertips" && h.Part != "palm") {
				return fmt.Errorf("act %s, touch %d: hand %q / part %q", a.Act, k, h.Hand, h.Part)
			}
			if (h.From == nil) == (h.FromEnd == nil) || (h.To == nil) == (h.ToEnd == nil) {
				return fmt.Errorf("act %s, touch %d: give from or from_end and to or to_end", a.Act, k)
			}
			for _, n := range lengths {
				from, to := VisitorTouchSpan(h, n)
				if from < 0 || to <= from || to > n {
					return fmt.Errorf("act %s, touch %d: ticks %d..%d are outside a phase of %d ticks", a.Act, k, from, to, n)
				}
			}
			if h.At.Z < 0 || h.At.Z > 2.2 {
				return fmt.Errorf("act %s, touch %d: point %+v is out of reach of the place's standing cells", a.Act, k, h.At)
			}
		}
		if a.ReachM < 0.3 || a.ReachM > 1.2 {
			return fmt.Errorf("act %s: reach_m %.2f is not a hand's reach (0.3..1.2 m)", a.Act, a.ReachM)
		}
	}
	// the cells the whole use of the machine is done from: every touch of every act within its act's reach
	var in []inv.Cell
	for _, c := range cells {
		ok := true
		for _, a := range t.Acts {
			for _, h := range a.Touches {
				dx, dy := h.At.X-(float64(c.X)+0.5)*cell, h.At.Y-(float64(c.Y)+0.5)*cell
				ok = ok && dx*dx+dy*dy <= a.ReachM*a.ReachM
			}
		}
		if ok {
			in = append(in, c)
		}
	}
	if len(in) == 0 {
		return fmt.Errorf("no standing cell of the place reaches all the touches of its acts (out of reach)")
	}
	for ai := range t.Acts {
		t.Acts[ai].Cells = in
	}
	return nil
}

// VisitorTouchSpan: the ticks of a touch within a phase of n ticks (the runtime uses the same rule).
func VisitorTouchSpan(h VisitorTouch, n int64) (int64, int64) {
	var from, to int64
	if h.From != nil {
		from = *h.From
	} else {
		from = n - *h.FromEnd
	}
	if h.To != nil {
		to = *h.To
	} else {
		to = n - *h.ToEnd
	}
	return from, to
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
	// walkers v1, addendum 1: the places a visible walker walks to; the rest of his list he buys at once, unseen
	WalkStops int `json:"walk_stops"`
	Outside   struct {
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
			CellMM  int      `json:"cell_mm"`
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
		p.DwellTicks.Min < 1 || p.DwellTicks.Max < p.DwellTicks.Min || p.DwellTicks.Max > 6000 || p.MaxStops < 1 || p.MaxStops > 8 ||
		p.WalkStops < 1 || p.WalkStops > p.MaxStops {
		return fail("per_day, max_bodies, move_ticks, dwell_ticks, max_stops or walk_stops out of range")
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
	for ti, t := range p.Targets {
		if t.ID == "" || targets[t.ID] || t.Category == "" || t.Category == p.PeddlerCategory || !walk(t.At) {
			return fail("target %q: duplicate, without a category or not on the floor %v", t.ID, t.At)
		}
		if t.TakeTicks < 0 || t.TakeTicks > 600 || (t.Face != nil && t.Face.X == 0 && t.Face.Y == 0) {
			return fail("target %q: the wait after paying must be 0..600 ticks and a facing must point somewhere", t.ID)
		}
		if d := t.DwellTicks; d != nil && (d.Min < 1 || d.Max < d.Min || d.Max > 6000) {
			return fail("target %q: its own look must be 1..6000 ticks", t.ID)
		}
		for _, c := range t.Spots {
			if !walk(c) || manhattanCells(c, t.At) > 2 {
				return fail("target %q: standing cell %v is not on the floor within two cells of the place", t.ID, c)
			}
		}
		dwell := p.DwellTicks
		if t.DwellTicks != nil {
			dwell = *t.DwellTicks
		}
		if err := checkVisitorActs(&p.Targets[ti], dwell, loc.CellMM); err != nil {
			return fail("target %s: %v", t.ID, err)
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
