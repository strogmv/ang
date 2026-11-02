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

// Peddlers (demo-hub v23, world.peddlers): people who walk a level with their goods and trade at the stops of a round.
// Each peddler owns a MOBILE shopfront (inventory Shopfront.Stops): the tray (display), the pack (reserve) and the bag
// (the rare piece until its day) are carried by him (stores anchored to the actor), the shop opens wherever he stands at a
// stop in his hours. The finite stock is graded (quality, origin, condition); the price is law.price_market's (the crafting
// economy's scarcity of the class × the legality risk) × the peddler's appraisal of the grade; one of them may buy junk by
// weight (never under a share of the law's price). The host (peddlers.go.tmpl) trades through ordinary inventory
// transactions; the masters' restock buys off a peddler whose stop is near their counter (crafting_market.go.tmpl).
type PeddlerStop struct {
	ID string   `json:"id"`
	At inv.Cell `json:"at"`
}
type PeddlerStock struct {
	Item         string `json:"item"`
	N            int64  `json:"n"`
	Quality      int    `json:"quality"`
	Origin       string `json:"origin"`
	ConditionPPM int64  `json:"condition_ppm"`
}
type PeddlerRare struct {
	Item           string `json:"item"`
	Quality        int    `json:"quality"`
	Origin         string `json:"origin"`
	ChancePermille int64  `json:"chance_permille"`
}
type PeddlerBuys struct {
	MinorPerKg    int64    `json:"minor_per_kg"`
	FloorPermille int64    `json:"floor_permille"`
	Classes       []string `json:"classes"`
	Items         []string `json:"items"`
}
type Peddler struct {
	Actor       string         `json:"actor"`
	Shop        string         `json:"shop"`
	Kind        string         `json:"kind"`
	Name        string         `json:"name"`
	Permit      bool           `json:"permit"`
	Account     string         `json:"account"`
	OpenSecond  int64          `json:"open_second"`
	CloseSecond int64          `json:"close_second"`
	Stops       []PeddlerStop  `json:"stops"`
	Stock       []PeddlerStock `json:"stock"`
	Rare        PeddlerRare    `json:"rare"`
	Buys        *PeddlerBuys   `json:"buys,omitempty"`
	// filled by ANG: the carried stores
	Tray string `json:"tray,omitempty"`
	Pack string `json:"pack,omitempty"`
	Bag  string `json:"bag,omitempty"`
}
type PeddlerCatalog struct {
	Level      string `json:"level"`
	ReachCells int    `json:"reach_cells"`
	DealCells  int    `json:"deal_cells"`
	Appraisal  struct {
		QualityPermille []int64          `json:"quality_permille"`
		OriginPermille  map[string]int64 `json:"origin_permille"`
	} `json:"appraisal"`
	RareWindow struct {
		FirstDay int64 `json:"first_day"`
		LastDay  int64 `json:"last_day"`
		FromHour int64 `json:"from_hour"`
		ToHour   int64 `json:"to_hour"`
	} `json:"rare_window"`
	FactSeconds int64             `json:"fact_seconds"`
	Legality    map[string]string `json:"legality"` // definitions that are not legal in the dome (law.price_market risk)
	Peddlers    []Peddler         `json:"peddlers"`
}

const peddlerStorePrefix = "peddler/"

func readPeddlers(world cue.Value) (*PeddlerCatalog, error) {
	v := world.LookupPath(cue.ParsePath("peddlers"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.peddlers: %w", err)
	}
	var out PeddlerCatalog
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.peddlers: %w", err)
	}
	return &out, nil
}

// applyPeddlers checks the catalog against the compiled Life and inventory and adds the carried stores, the mobile
// shopfronts and the finite stock (before crafting: the stock counts in law.price_market's balance of the classes).
func applyPeddlers(c *Compiled) error {
	p := c.Peddlers
	if p == nil {
		return nil
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
		return fmt.Errorf("world.peddlers: life: %w", err)
	}
	loc, ok := life.Locations[p.Level]
	if !ok {
		return fmt.Errorf("world.peddlers: unknown level %s", p.Level)
	}
	walk := func(at inv.Cell) bool {
		return at.Y >= 0 && at.Y < len(loc.Terrain) && at.X >= 0 && at.X < len(loc.Terrain[at.Y]) && c.Walkable[string(loc.Terrain[at.Y][at.X])]
	}
	if p.ReachCells < 1 || p.ReachCells > 4 || p.DealCells < 1 || p.DealCells > 30 || len(p.Appraisal.QualityPermille) != 5 || p.FactSeconds < 30 || p.RareWindow.FirstDay < 1 || p.RareWindow.LastDay < p.RareWindow.FirstDay || p.RareWindow.FromHour < 0 || p.RareWindow.ToHour > 24 || p.RareWindow.FromHour >= p.RareWindow.ToHour {
		return fmt.Errorf("world.peddlers: invalid reach, deal, appraisal, fact or rare window")
	}
	for _, o := range []string{"heirloom", "system", "handmade"} {
		if p.Appraisal.OriginPermille[o] < 1 {
			return fmt.Errorf("world.peddlers: appraisal has no origin %s", o)
		}
	}
	steps := map[string]map[inv.Cell]bool{}
	for _, a := range life.Actors {
		for _, s := range a.Steps {
			if s.Level == p.Level && s.Activity == "service" {
				if steps[a.ID] == nil {
					steps[a.ID] = map[inv.Cell]bool{}
				}
				steps[a.ID][s.At] = true
			}
		}
	}
	shops := map[string]bool{}
	for _, sh := range m.Shopfronts {
		shops[sh.ID] = true
	}
	taken := map[inv.Cell]string{}
	known := func(def string) bool { d, ok := m.Definitions[def]; return ok && d.Carrier == "physical" }
	priced := func(def string) bool { q, err := inv.Quote(m.Definitions[def], 1); return err == nil && q > 0 }
	for i := range p.Peddlers {
		pd := &p.Peddlers[i]
		fail := func(format string, args ...any) error {
			return fmt.Errorf("world.peddlers %s: %s", pd.Actor, fmt.Sprintf(format, args...))
		}
		if !m.Actors[pd.Actor] || shops[pd.Shop] || !strings.HasPrefix(pd.Shop, "peddler.") {
			return fail("needs an inventory actor and a new shop id (%s)", pd.Shop)
		}
		shops[pd.Shop] = true
		switch pd.Kind {
		case "tray", "cart", "bag", "sack":
		default:
			return fail("unknown kind %q", pd.Kind)
		}
		if a, ok := m.Accounts[pd.Account]; !ok || a.Owner != pd.Actor || a.Currency != "CRD" {
			return fail("needs its own CRD account %s", pd.Account)
		}
		if pd.OpenSecond < 0 || pd.CloseSecond > 86400 || pd.OpenSecond >= pd.CloseSecond || len(pd.Stops) < 2 {
			return fail("hours or round invalid")
		}
		points := []inv.DeliveryPoint{}
		for _, s := range pd.Stops {
			if !walk(s.At) {
				return fail("stop %s %v is not walkable", s.ID, s.At)
			}
			if who, dup := taken[s.At]; dup {
				return fail("stop %s %v is taken by %s", s.ID, s.At, who)
			}
			if !steps[pd.Actor][s.At] {
				return fail("stop %s %v is not a service step of his Life routine", s.ID, s.At)
			}
			taken[s.At] = pd.Actor
			points = append(points, inv.DeliveryPoint{Level: p.Level, At: s.At})
		}
		pd.Tray, pd.Pack, pd.Bag = peddlerStorePrefix+pd.Shop+"/tray", peddlerStorePrefix+pd.Shop+"/pack", peddlerStorePrefix+pd.Shop+"/bag"
		carried := inv.Anchor{Kind: "actor", Actor: pd.Actor}
		room := inv.Capacity{Mass: 40000, Volume: 60000, Stacks: 48}
		if pd.Kind == "cart" {
			room = inv.Capacity{Mass: 160000, Volume: 240000, Stacks: 64}
		}
		for _, st := range []inv.Store{
			{ID: pd.Tray, Name: "peddler.store." + pd.Shop + ".tray", Kind: "container", Anchor: carried, Owner: pd.Actor, Capacity: room},
			{ID: pd.Pack, Name: "peddler.store." + pd.Shop + ".pack", Kind: "container", Anchor: carried, Owner: pd.Actor, Locked: true, Capacity: room},
			{ID: pd.Bag, Name: "peddler.store." + pd.Shop + ".bag", Kind: "container", Anchor: carried, Owner: pd.Actor, Locked: true, Capacity: inv.Capacity{Mass: 5000, Volume: 5000, Stacks: 4}},
		} {
			if _, dup := m.Stores[st.ID]; dup {
				return fail("duplicate store %s", st.ID)
			}
			m.Stores[st.ID] = st
		}
		m.Shopfronts = append(m.Shopfronts, inv.Shopfront{ID: pd.Shop, Owner: pd.Actor, Service: points[0], Stops: points, DisplayStore: pd.Tray, ReserveStore: pd.Pack, OpenSecond: pd.OpenSecond, CloseSecond: pd.CloseSecond})
		grade := func(q int, o string) error {
			if q < 1 || q > 5 || p.Appraisal.OriginPermille[o] < 1 {
				return fail("grade q%d %q", q, o)
			}
			return nil
		}
		for k, s := range pd.Stock {
			if !known(s.Item) || s.N < 1 || s.ConditionPPM < 1 || s.ConditionPPM > 1000000 || !priced(s.Item) {
				return fail("stock %s: unknown, unpriced or invalid", s.Item)
			}
			if err := grade(s.Quality, s.Origin); err != nil {
				return err
			}
			m.Sources = append(m.Sources, inv.Source{ID: fmt.Sprintf("peddler.stock.%s.%02d", pd.Shop, k), Definition: s.Item, Quantity: s.N, Condition: s.ConditionPPM, Owner: pd.Actor, ForSale: true, Location: inv.Location{Kind: "store", Store: pd.Tray}, Grade: inv.Grade{Quality: s.Quality, Provenance: s.Origin}})
		}
		if !known(pd.Rare.Item) || pd.Rare.ChancePermille < 0 || pd.Rare.ChancePermille > 1000 || (pd.Rare.Origin != "heirloom" && pd.Rare.Origin != "system") {
			return fail("rare piece %s invalid (an old factory-made or system piece)", pd.Rare.Item)
		}
		if err := grade(pd.Rare.Quality, pd.Rare.Origin); err != nil {
			return err
		}
		m.Sources = append(m.Sources, inv.Source{ID: fmt.Sprintf("peddler.rare.%s", pd.Shop), Definition: pd.Rare.Item, Quantity: 1, Condition: 1000000, Owner: pd.Actor, ForSale: true, Location: inv.Location{Kind: "store", Store: pd.Bag}, Grade: inv.Grade{Quality: pd.Rare.Quality, Provenance: pd.Rare.Origin}})
		if b := pd.Buys; b != nil {
			if b.MinorPerKg < 1 || b.FloorPermille < 0 || b.FloorPermille > 1000 || len(b.Classes)+len(b.Items) == 0 {
				return fail("buy-back rule invalid")
			}
			for _, def := range b.Items {
				if !known(def) {
					return fail("buys unknown item %s", def)
				}
			}
			sort.Strings(b.Classes)
			sort.Strings(b.Items)
		}
	}
	sort.Slice(m.Sources, func(i, j int) bool { return m.Sources[i].ID < m.Sources[j].ID })
	if _, err := inv.New(*m, 425); err != nil {
		return fmt.Errorf("world.peddlers: inventory with the peddlers: %w", err)
	}
	return nil
}

// checkPeddlerClasses: the bought classes are part classes of the crafting catalog (after it is applied).
func checkPeddlerClasses(c *Compiled) error {
	if c.Peddlers == nil {
		return nil
	}
	if c.Crafting == nil {
		for _, pd := range c.Peddlers.Peddlers {
			if pd.Buys != nil && len(pd.Buys.Classes) > 0 {
				return fmt.Errorf("world.peddlers %s: buys part classes without a crafting catalog", pd.Actor)
			}
		}
		return nil
	}
	for _, pd := range c.Peddlers.Peddlers {
		if pd.Buys == nil {
			continue
		}
		for _, cl := range pd.Buys.Classes {
			if _, ok := c.Crafting.Part(cl); !ok {
				return fmt.Errorf("world.peddlers %s: buys unknown part class %s", pd.Actor, cl)
			}
		}
	}
	return nil
}
