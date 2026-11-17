package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"cuelang.org/go/cue"

	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// Craft places of the hub islands (demo-hub v16-goods, world.market_goods): a craft kit puts rows of goods, fixtures,
// devices and lights onto the mount levels of an unchanged island module (passport hub.island.loop). The compiler checks
// every placed kit against the passport and the compiled world; the Go runtime (craft.go.tmpl) publishes the public state
// of each place (open, closing / opening phase, charge of a charging rack, the seller's action, the buyers queueing or
// examining) and Unity shows it (models, light, particles, clips).
type CraftLevel struct {
	ID       string `json:"id"`
	ZMM      int    `json:"z_mm"`
	ZMaxMM   int    `json:"z_max_mm"`
	DMM      [2]int `json:"d_mm"`
	XMM      []int  `json:"x_mm,omitempty"`
	HeightMM int    `json:"height_mm"`
	Hang     bool   `json:"hang"`
	RU       string `json:"ru"`
}
type CraftBus struct {
	ID  string `json:"id"`
	DMM [2]int `json:"d_mm"`
	ZMM [2]int `json:"z_mm"`
	RU  string `json:"ru"`
}
type CraftPassport struct {
	ID     string `json:"id"`
	SizeMM struct {
		W int `json:"w"`
		D int `json:"d"`
		H int `json:"h"`
	} `json:"size_mm"`
	Cells struct {
		W int `json:"w"`
		D int `json:"d"`
	} `json:"cells"`
	Levels    []CraftLevel `json:"levels"`
	Bus       []CraftBus   `json:"bus"`
	LampTopMM int          `json:"lamp_top_mm"`
}
type CraftCable struct {
	RouteMM [][3]int `json:"route_mm"`
	Braided bool     `json:"braided"`
}
type CraftFixture struct {
	ID        string      `json:"id"`
	Kind      string      `json:"kind"`
	Mount     string      `json:"mount"`
	XMM       [2]int      `json:"x_mm"`
	DMM       [2]int      `json:"d_mm"`
	ZMM       [2]int      `json:"z_mm"`
	ShelvesMM []int       `json:"shelves_mm,omitempty"`
	Watts     float64     `json:"watts"`
	Cable     *CraftCable `json:"cable,omitempty"`
}
type CraftRow struct {
	ID      string   `json:"id"`
	Mount   string   `json:"mount"`
	On      string   `json:"on,omitempty"`
	Item    string   `json:"item"`
	N       int      `json:"n"`
	PitchMM int      `json:"pitch_mm"`
	X0MM    int      `json:"x0_mm"`
	DMM     int      `json:"d_mm"`
	ZMM     int      `json:"z_mm"`
	WMM     int      `json:"w_mm"`
	HMM     int      `json:"h_mm"`
	Var     []string `json:"var"`
	Hang    bool     `json:"hang"`
	Sway    float64  `json:"sway,omitempty"`
	Stack   []int    `json:"stack,omitempty"`
}
type CraftDevice struct {
	ID     string      `json:"id"`
	Item   string      `json:"item"`
	Mount  string      `json:"mount"`
	AtMM   [3]int      `json:"at_mm"`
	WMM    int         `json:"w_mm"`
	HMM    int         `json:"h_mm"`
	Watts  float64     `json:"watts"`
	Cable  *CraftCable `json:"cable,omitempty"`
	Beacon bool        `json:"beacon"`
}
type CraftLight struct {
	ID     string     `json:"id"`
	Kind   string     `json:"kind"`
	RGB    [3]float64 `json:"rgb"`
	Watts  float64    `json:"watts"`
	AtMM   [3]int     `json:"at_mm"`
	SizeMM []int      `json:"size_mm,omitempty"`
	Timer  *struct {
		OnMinute  int `json:"on_minute"`
		OffMinute int `json:"off_minute"`
	} `json:"timer,omitempty"`
	Of string `json:"of"`
}
type CraftEffect struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	AtMM      *[3]int  `json:"at_mm,omitempty"`
	Particles int      `json:"particles"`
	During    []string `json:"during"`
}
type CraftAction struct {
	ID      string  `json:"id"`
	Clip    string  `json:"clip"`
	Src     string  `json:"src"`
	Seconds float64 `json:"seconds"`
	Weight  float64 `json:"weight"`
	Prop    string  `json:"prop,omitempty"`
	Hand    string  `json:"hand"`
}

// demo-hub v26-alive: what a seller does in the closing and opening phases (market_hub/closing.cue)
type CraftClosure struct {
	Mode         string   `json:"mode"`
	PhaseMinutes int      `json:"phase_minutes"`
	Order        string   `json:"order"`
	Night        []string `json:"night"`
}
type CraftKit struct {
	ID        string       `json:"id"`
	Archetype string       `json:"archetype"`
	Atlas     string       `json:"atlas"`
	Closure   CraftClosure `json:"closure"`
	Charge    *struct {
		FullHours float64 `json:"full_hours"`
		StartPPM  int64   `json:"start_ppm"`
	} `json:"charge,omitempty"`
}

// Market goods v2 (phase 3): a model of the produce kit, a heap slot of an open stall, the morning supply of a stall and
// the counter of an island place (its units own the shut covers and the goods blocks of the island model).
type CraftModel struct {
	Kind      string `json:"kind"`
	Scene     string `json:"scene"`
	SizeMM    [3]int `json:"size_mm"`
	Triangles int    `json:"triangles"`
	Produce   string `json:"produce,omitempty"`
	Fill      string `json:"fill,omitempty"`
	Container string `json:"container,omitempty"`
}
type CraftHeap struct {
	ID    string `json:"id"`
	Item  string `json:"item"`
	Mount string `json:"mount"`
	Tare  string `json:"tare"`
	Heaps struct {
		Full string `json:"full"`
		Half string `json:"half"`
		Few  string `json:"few"`
	} `json:"heaps"`
	XMM  int   `json:"x_mm"`
	DMM  int   `json:"d_mm"`
	ZMM  int   `json:"z_mm"`
	Full int64 `json:"full"`
}
type CraftSupply struct {
	Store         string `json:"store"`
	Days          int64  `json:"days"`
	WindowMinutes int64  `json:"window_minutes"`
}
type CraftCounter struct {
	Place  string `json:"place"`
	Island string `json:"island"`
	Shop   string `json:"shop"`
	At     struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"at"`
	Side     string `json:"side"`
	Units    []int  `json:"units"`
	PackAway bool   `json:"pack_away"`
	// market food court v1: the plan depth of the counter's side of its island module (mm); 0 = half of the passport's
	// depth (the loop). A half-island (one straight against a stall's wall) is its whole depth.
	DepthMM int `json:"depth_mm,omitempty"`
}
type CraftStall struct {
	Place      string         `json:"place"`
	Island     string         `json:"island"`
	Face       string         `json:"face"`
	Units      []string       `json:"units"`
	XMM        [2]int         `json:"x_mm"`
	Trader     string         `json:"trader"`
	Shop       string         `json:"shop"`
	Kit        string         `json:"kit"`
	Reference  bool           `json:"reference"`
	BrainPlace string         `json:"brain_place"`
	Signs      []string       `json:"signs"`
	Fixtures   []CraftFixture `json:"fixtures"`
	Rows       []CraftRow     `json:"rows"`
	Devices    []CraftDevice  `json:"devices"`
	Lights     []CraftLight   `json:"lights"`
	Effects    []CraftEffect  `json:"effects"`
	Beacon     struct {
		Item string `json:"item"`
		RU   string `json:"ru"`
	} `json:"beacon"`
	Seller   []CraftAction `json:"seller"`
	Buyer    []CraftAction `json:"buyer"`
	Heaps    []CraftHeap   `json:"heaps"`
	Replaces []string      `json:"replaces"`
	Supply   *CraftSupply  `json:"supply,omitempty"`
}
type CraftCatalog struct {
	Level     string        `json:"level"`
	Passport  CraftPassport `json:"passport"`
	Kits      []CraftKit    `json:"kits"`
	Stalls    []CraftStall  `json:"stalls"`
	HandProps []string      `json:"hand_props"`
	// demo-hub v26-alive: what a seller does in the closing and opening phases of his place (market_hub/closing.cue)
	ClosingActions []CraftAction `json:"closing_actions,omitempty"`
	OpeningActions []CraftAction `json:"opening_actions,omitempty"`
	// market goods v2: the produce kit models and the counters of the island places
	Models   map[string]CraftModel `json:"models,omitempty"`
	Counters []CraftCounter        `json:"counters,omitempty"`
	// market food court v1: modules whose instances carry counters too (a half-island: one straight of the loop, the
	// counters of one side only); the craft kits' mount levels stay the passport's module alone.
	CounterModules []string `json:"counter_modules,omitempty"`
	// phase 3d: the dressed selling places (pavilion interiors, the griddle set, the benches, the decks)
	Dress []CraftDress `json:"dress,omitempty"`
}

// A thing the server dresses a selling place with (market_goods schema #Prop) and a dressed place (#Dress).
type CraftPropStock struct {
	Items []string `json:"items"`
	Full  int64    `json:"full"`
	Half  string   `json:"half"`
	Few   string   `json:"few"`
}
type CraftProp struct {
	ID    string          `json:"id"`
	Model string          `json:"model"`
	XMM   int             `json:"x_mm"`
	DMM   int             `json:"d_mm"`
	ZMM   int             `json:"z_mm"`
	Yaw   int             `json:"yaw"`
	Show  string          `json:"show"`
	Lit   bool            `json:"lit"`
	Stock *CraftPropStock `json:"stock,omitempty"`
	// market goods stage 2: a stock row of a protected display (the shop does not pack away) stays on the counter behind the
	// night cage — shown at the closed place too, at its real level
	BehindCage bool `json:"behind_cage,omitempty"`
}
type CraftDress struct {
	Place    string      `json:"place"`
	Shop     string      `json:"shop"`
	Frame    string      `json:"frame"`
	AtMM     []int       `json:"at_mm,omitempty"`
	Yaw      int         `json:"yaw,omitempty"`
	Props    []CraftProp `json:"props"`
	Replaces []string    `json:"replaces"`
	Shut     []string    `json:"shut"`
}

func readCraft(world cue.Value) (*CraftCatalog, error) {
	v := world.LookupPath(cue.ParsePath("market_goods"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.market_goods: %w", err)
	}
	var out CraftCatalog
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields() // inscriptions only through slots: a kit has no free text field to carry them
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.market_goods: %w", err)
	}
	return &out, nil
}

// validateCraft: every placed kit against the island passport and the compiled world.
//   - mount: the level of every row / fixture / device is a level of the passport; a fixed level fixes z, a range holds it;
//   - envelope: every item (x ± w/2 at the pitch, d, z .. z + h or z - h hanging) stays inside the stall's own units and the
//     level's depth band and free height — the module itself is never changed (the passport size is the compiled module);
//   - nothing on the floor: no level below the counter top;
//   - wired (doc 57): every consumer (watts > 0) has a cable from its body (first point within 0.25 m) to the island bus
//     (last point inside a bus band); a light belongs to a fixture or device of the same stall;
//   - inscriptions: only world-text slots of the island instance, by id;
//   - people and places: the trader owns the shopfront, the buyers' brain place exists, the actions' props are known;
//   - heap slots (market goods v2): the tare and the three heaps are models of the kit and the heaps lie in that tare; the
//     slot stands on its level inside the stall's units, on a front deck when the level is the deck, and no two slots
//     overlap; the stall's display holds the item; a supplied stall has its locked consignment with every slot's item.
func validateCraft(c Compiled) error {
	g := c.Craft
	if g == nil {
		return nil
	}
	p := g.Passport
	levels := map[string]CraftLevel{}
	for _, l := range p.Levels {
		if l.ZMM < 900 {
			return fmt.Errorf("market_goods: level %q below the counter top (the hall floor stays free)", l.ID)
		}
		levels[l.ID] = l
	}
	if len(levels) == 0 || len(p.Bus) == 0 {
		return fmt.Errorf("market_goods: passport %s has no mount levels or no bus", p.ID)
	}
	kits := map[string]CraftKit{}
	for _, k := range g.Kits {
		if k.Closure.PhaseMinutes < 2 || (k.Closure.Mode != "pack_away" && k.Closure.Mode != "mesh_cover") {
			return fmt.Errorf("market_goods: kit %s closure", k.ID)
		}
		kits[k.ID] = k
	}
	props := map[string]bool{}
	for _, h := range g.HandProps {
		props[h] = true
	}
	// the module of the passport as compiled (island instances of the level): never changed by a kit
	instances := map[string]bool{}
	slots := map[string]string{}
	if c.LocationSDK != nil {
		for _, lv := range c.LocationSDK.Levels {
			for _, in := range lv.Instances {
				if in.ModuleID != "kit.geo."+p.ID {
					continue
				}
				if in.Width*500 != p.SizeMM.W || in.Depth*500 != p.SizeMM.D || in.Height*500 < p.SizeMM.H || in.Width != p.Cells.W || in.Depth != p.Cells.D {
					return fmt.Errorf("market_goods: island %s is %dx%dx%d cells, the passport %s says %dx%dx%d mm", in.ID, in.Width, in.Depth, in.Height, p.ID, p.SizeMM.W, p.SizeMM.D, p.SizeMM.H)
				}
				instances[in.ID] = true
			}
			for _, s := range lv.TextSlots {
				slots[s.ID] = s.Instance
			}
		}
	}
	// counters also stand on the instances of the counter modules of the data (half-islands)
	counterIslands := map[string]bool{}
	for id := range instances {
		counterIslands[id] = true
	}
	if c.LocationSDK != nil && len(g.CounterModules) > 0 {
		extra := map[string]bool{}
		for _, m := range g.CounterModules {
			extra[m] = true
		}
		for _, lv := range c.LocationSDK.Levels {
			for _, in := range lv.Instances {
				if extra[in.ModuleID] {
					counterIslands[in.ID] = true
				}
			}
		}
	}
	var life struct {
		Brain struct {
			Places []struct {
				ID string `json:"id"`
			} `json:"places"`
		} `json:"brain"`
	}
	_ = json.Unmarshal(c.Life, &life)
	brain := map[string]bool{}
	for _, b := range life.Brain.Places {
		brain[b.ID] = true
	}
	owners := map[string]string{}
	for _, sh := range c.Inventory.Shopfronts {
		owners[sh.ID] = sh.Owner
	}
	onBus := func(q [3]int) bool {
		for _, b := range p.Bus {
			if q[1] >= b.DMM[0] && q[1] <= b.DMM[1] && q[2] >= b.ZMM[0] && q[2] <= b.ZMM[1] {
				return true
			}
		}
		return false
	}
	dist := func(a, b [3]int) float64 {
		return math.Sqrt(float64((a[0]-b[0])*(a[0]-b[0]) + (a[1]-b[1])*(a[1]-b[1]) + (a[2]-b[2])*(a[2]-b[2])))
	}
	wired := func(where, id string, body [3]int, cab *CraftCable) error {
		if cab == nil || len(cab.RouteMM) < 2 {
			return fmt.Errorf("market_goods %s: consumer %s has no cable to the island bus (doc 57)", where, id)
		}
		if dist(cab.RouteMM[0], body) > 250 {
			return fmt.Errorf("market_goods %s: the cable of %s does not start at its body", where, id)
		}
		if !onBus(cab.RouteMM[len(cab.RouteMM)-1]) {
			return fmt.Errorf("market_goods %s: the cable of %s does not end on the island bus", where, id)
		}
		for _, q := range cab.RouteMM {
			if q[2] < 820 {
				return fmt.Errorf("market_goods %s: the cable of %s runs below the counter (nothing on the floor)", where, id)
			}
		}
		return nil
	}
	place := func(where, id, mount string, x0, x1, d, z0, z1 int, x [2]int, hang bool) error {
		l, ok := levels[mount]
		if !ok {
			return fmt.Errorf("market_goods %s: %s on level %q, not a level of the passport %s", where, id, mount, p.ID)
		}
		if x0 < x[0] || x1 > x[1] || x0 < 0 || x1 > p.SizeMM.W {
			return fmt.Errorf("market_goods %s: %s spans x %d..%d outside the stall's units %d..%d", where, id, x0, x1, x[0], x[1])
		}
		if len(l.XMM) == 2 && (x0 < l.XMM[0] || x1 > l.XMM[1]) {
			return fmt.Errorf("market_goods %s: %s spans x %d..%d beyond level %s", where, id, x0, x1, mount)
		}
		if d < l.DMM[0] || d > l.DMM[1] {
			return fmt.Errorf("market_goods %s: %s at depth %d outside level %s %v", where, id, d, mount, l.DMM)
		}
		// a standing level holds z .. z + free height; a hanging level holds the hooks' range down to z_max - length
		low, top := l.ZMM, l.ZMM+l.HeightMM
		if l.Hang {
			low, top = l.ZMaxMM-l.HeightMM, l.ZMaxMM
		}
		if z0 < low || z1 > top || z0 < 900 {
			return fmt.Errorf("market_goods %s: %s spans z %d..%d outside level %s (%d..%d)", where, id, z0, z1, mount, low, top)
		}
		if !l.Hang && hang {
			return fmt.Errorf("market_goods %s: %s hangs from level %s that carries no hooks", where, id, mount)
		}
		return nil
	}
	seen := map[string]bool{}
	for _, s := range g.Stalls {
		where := s.Place
		if seen[s.Place] {
			return fmt.Errorf("market_goods: place %s twice", s.Place)
		}
		seen[s.Place] = true
		if _, ok := kits[s.Kit]; !ok {
			return fmt.Errorf("market_goods %s: unknown kit %s", where, s.Kit)
		}
		if len(instances) > 0 && !instances["island."+s.Island] {
			return fmt.Errorf("market_goods %s: island.%s is not an island module of the level", where, s.Island)
		}
		if s.XMM[0] < 0 || s.XMM[1] > p.SizeMM.W || s.XMM[1]-s.XMM[0] != 1500*len(s.Units) {
			return fmt.Errorf("market_goods %s: x %v does not match its %d units", where, s.XMM, len(s.Units))
		}
		if owners[s.Shop] != s.Trader {
			return fmt.Errorf("market_goods %s: shopfront %s is not run by %s", where, s.Shop, s.Trader)
		}
		if s.BrainPlace != "" && !brain[s.BrainPlace] {
			return fmt.Errorf("market_goods %s: the buyers' brain place %s does not exist", where, s.BrainPlace)
		}
		for _, sl := range s.Signs {
			if in, ok := slots[sl]; !ok || in != "island."+s.Island {
				return fmt.Errorf("market_goods %s: inscription %s is not a world-text slot of island.%s", where, sl, s.Island)
			}
		}
		fixtures := map[string]CraftFixture{}
		carriers := map[string]bool{}
		for _, f := range s.Fixtures {
			fixtures[f.ID] = f
			carriers[f.ID] = true
			if err := place(where, f.ID, f.Mount, s.XMM[0]+f.XMM[0], s.XMM[0]+f.XMM[1], (f.DMM[0]+f.DMM[1])/2, f.ZMM[0], f.ZMM[1], s.XMM, levels[f.Mount].Hang); err != nil {
				return err
			}
			if f.Watts > 0 {
				if err := wiredFixture(where, f, onBus); err != nil {
					return err
				}
			}
		}
		for _, r := range s.Rows {
			if r.N < 1 || r.PitchMM < 1 || r.WMM < 1 || r.HMM < 1 {
				return fmt.Errorf("market_goods %s: row %s is empty", where, r.ID)
			}
			x0 := s.XMM[0] + r.X0MM - r.WMM/2
			x1 := s.XMM[0] + r.X0MM + (r.N-1)*r.PitchMM + r.WMM/2
			z0, z1 := r.ZMM, r.ZMM+r.HMM
			if r.Hang {
				z0, z1 = r.ZMM-r.HMM, r.ZMM
			}
			if _, ok := levels[r.Mount]; !ok {
				return fmt.Errorf("market_goods %s: %s on level %q, not a level of the passport %s", where, r.ID, r.Mount, p.ID)
			}
			if r.On != "" {
				f, ok := fixtures[r.On]
				if !ok || f.Mount != r.Mount {
					return fmt.Errorf("market_goods %s: row %s on %q, not a fixture of the stall on level %s", where, r.ID, r.On, r.Mount)
				}
				shelf := false
				for _, z := range f.ShelvesMM {
					shelf = shelf || z == r.ZMM
				}
				if !shelf {
					return fmt.Errorf("market_goods %s: row %s at z %d is not on a shelf of %s", where, r.ID, r.ZMM, r.On)
				}
			} else if l := levels[r.Mount]; !l.Hang && r.ZMM != l.ZMM {
				return fmt.Errorf("market_goods %s: row %s at z %d, level %s is at %d", where, r.ID, r.ZMM, r.Mount, l.ZMM)
			}
			if err := place(where, r.ID, r.Mount, x0, x1, r.DMM, z0, z1, s.XMM, r.Hang); err != nil {
				return err
			}
		}
		for _, d := range s.Devices {
			carriers[d.ID] = true
			if err := place(where, d.ID, d.Mount, s.XMM[0]+d.AtMM[0]-d.WMM/2, s.XMM[0]+d.AtMM[0]+d.WMM/2, d.AtMM[1], d.AtMM[2], d.AtMM[2]+d.HMM, s.XMM, false); err != nil {
				return err
			}
			if d.Watts > 0 {
				if err := wired(where, d.ID, d.AtMM, d.Cable); err != nil {
					return err
				}
			}
		}
		for _, l := range s.Lights {
			if !carriers[l.Of] {
				return fmt.Errorf("market_goods %s: light %s belongs to %q, not a fixture or device of the stall (its feed)", where, l.ID, l.Of)
			}
		}
		actions := map[string]bool{}
		for _, list := range [][]CraftAction{s.Seller, s.Buyer} {
			for _, a := range list {
				if a.Clip == "" || a.Seconds <= 0 || a.Weight <= 0 {
					return fmt.Errorf("market_goods %s: action %s", where, a.ID)
				}
				if a.Prop != "" && !props[a.Prop] {
					return fmt.Errorf("market_goods %s: action %s holds unknown prop %s", where, a.ID, a.Prop)
				}
			}
		}
		for _, a := range s.Seller {
			actions[a.ID] = true
		}
		for _, e := range s.Effects {
			for _, d := range e.During {
				if !actions[d] {
					return fmt.Errorf("market_goods %s: effect %s runs during %s, not a seller action", where, e.ID, d)
				}
			}
		}
		if len(s.Seller) == 0 {
			return fmt.Errorf("market_goods %s: the seller does nothing", where)
		}
		if err := validateCraftHeaps(c, s, levels, place); err != nil {
			return err
		}
	}
	if err := validateCraftCounters(c, counterIslands); err != nil {
		return err
	}
	return validateCraftDress(c)
}

// validateCraftDress: every dressed place names a shopfront and, on an island, the counter of that shop; every prop is a
// model of the kit inside the counter's own units and half of the island; a prop that follows a stock names models of
// its levels and goods the shop starts with, and `full` is exactly that starting stock (the lot the stall is topped up to).
func validateCraftDress(c Compiled) error {
	g := c.Craft
	fronts := map[string]int{}
	for i, sh := range c.Inventory.Shopfronts {
		fronts[sh.ID] = i + 1
	}
	counters := map[string]CraftCounter{}
	for _, k := range g.Counters {
		counters[k.Place] = k
	}
	seen := map[string]bool{}
	for _, d := range g.Dress {
		fail := func(format string, args ...any) error {
			return fmt.Errorf("market_goods dress %s: %s", d.Place, fmt.Sprintf(format, args...))
		}
		if d.Place == "" || seen[d.Place] || len(d.Props) == 0 {
			return fail("repeated place or no props")
		}
		seen[d.Place] = true
		// market goods stage 2: a shuttered place of an island has no shop — only bolted-on props stand there (its
		// shutter), never a stock, a lamp or a thing that comes and goes with opening hours it does not keep
		if d.Shop == "" {
			k, ok := counters[d.Place]
			if d.Frame != "counter" || !ok || k.Shop != "" {
				return fail("a dress without a shop is a shuttered island counter (no shop at this place)")
			}
			ids := map[string]bool{}
			for _, q := range d.Props {
				if q.ID == "" || ids[q.ID] {
					return fail("prop id %q repeats", q.ID)
				}
				ids[q.ID] = true
				if m, ok := g.Models[q.Model]; !ok || (m.Kind != "fixture" && m.Kind != "goods" && m.Kind != "prop") {
					return fail("prop %s: %q is not a fixture, goods or prop model of the kit", q.ID, q.Model)
				}
				if q.Show != "always" || q.Stock != nil || q.Lit {
					return fail("prop %s: a shuttered place shows only bolted-on props (show always, no stock, no lamp)", q.ID)
				}
				length, depth := len(k.Units)*1500, g.Passport.SizeMM.D/2
				if k.DepthMM > 0 {
					depth = k.DepthMM
				}
				if q.XMM < 0 || q.XMM > length || q.DMM < 0 || q.DMM > depth || q.ZMM > 2600 {
					return fail("prop %s stands outside the counter's own units (x %d of %d, d %d of %d, z %d)", q.ID, q.XMM, length, q.DMM, depth, q.ZMM)
				}
			}
			continue
		}
		if fronts[d.Shop] == 0 {
			return fail("%s is not a shopfront", d.Shop)
		}
		front := c.Inventory.Shopfronts[fronts[d.Shop]-1]
		length, depth := 0, 0
		switch d.Frame {
		case "counter":
			k, ok := counters[d.Place]
			if !ok || k.Shop != d.Shop {
				return fail("no counter of shop %s at this place", d.Shop)
			}
			length, depth = len(k.Units)*1500, g.Passport.SizeMM.D/2
			if k.DepthMM > 0 {
				depth = k.DepthMM
			}
		case "bay":
			if len(d.AtMM) != 3 || d.AtMM[0] < 0 || d.AtMM[1] < 0 || d.AtMM[2] < 0 {
				return fail("a bay needs its plan origin [x, y, z]")
			}
			for _, name := range append(append([]string(nil), d.Replaces...), d.Shut...) {
				if i := strings.Index(name, ":"); i < 1 || i == len(name)-1 {
					return fail("a bay replaces nodes of a named module: %q is not \"<instance>:<node>\"", name)
				}
			}
		default:
			return fail("frame %q", d.Frame)
		}
		if d.Frame != "bay" && len(d.Shut) != 0 {
			return fail("only a bay has nodes shown while it is shut")
		}
		// the starting stock of the shop's showcase and reserve, by definition
		start := map[string]int64{}
		var whole int64
		for _, src := range c.Inventory.Sources {
			if src.Location.Kind == "store" && (src.Location.Store == front.DisplayStore || src.Location.Store == front.ReserveStore) && src.Owner == front.Owner && src.ForSale {
				start[src.Definition] += src.Quantity
				whole += src.Quantity
			}
		}
		ids := map[string]bool{}
		for _, q := range d.Props {
			if q.ID == "" || ids[q.ID] {
				return fail("prop id %q repeats", q.ID)
			}
			ids[q.ID] = true
			model := func(id string) error {
				m, ok := g.Models[id]
				if !ok || (m.Kind != "fixture" && m.Kind != "goods" && m.Kind != "prop") {
					return fail("prop %s: %q is not a fixture, goods or prop model of the kit", q.ID, id)
				}
				return nil
			}
			if err := model(q.Model); err != nil {
				return err
			}
			if q.Show != "open" && q.Show != "always" && q.Show != "shut" {
				return fail("prop %s: show %q", q.ID, q.Show)
			}
			if d.Frame == "counter" && (q.XMM < 0 || q.XMM > length || q.DMM < 0 || q.DMM > depth || q.ZMM > 2600) {
				return fail("prop %s stands outside the counter's own units (x %d of %d, d %d of %d, z %d)", q.ID, q.XMM, length, q.DMM, depth, q.ZMM)
			}
			if q.BehindCage && (q.Stock == nil || q.Show != "open") {
				return fail("prop %s: only a stock row of the open place stays behind the night cage", q.ID)
			}
			if q.BehindCage && (d.Shop == "" || front.PackAway) {
				return fail("prop %s: a shop that packs away at night keeps nothing behind a cage", q.ID)
			}
			if st := q.Stock; st != nil {
				if q.Show != "open" {
					return fail("prop %s: a stock is shown only at the open place", q.ID)
				}
				for _, id := range []string{st.Half, st.Few} {
					if err := model(id); err != nil {
						return err
					}
				}
				want := whole
				if len(st.Items) > 0 {
					want = 0
					for _, item := range st.Items {
						if start[item] == 0 {
							return fail("prop %s: shop %s does not start with %s", q.ID, d.Shop, item)
						}
						want += start[item]
					}
				}
				if st.Full != want {
					return fail("prop %s: full %d, the shop starts with %d", q.ID, st.Full, want)
				}
			}
		}
	}
	return nil
}

// validateCraftHeaps: the heap slots of one stall (see validateCraft).
func validateCraftHeaps(c Compiled, s CraftStall, levels map[string]CraftLevel, place func(where, id, mount string, x0, x1, d, z0, z1 int, x [2]int, hang bool) error) error {
	g, where := c.Craft, s.Place
	if len(s.Heaps) == 0 {
		if s.Supply != nil {
			return fmt.Errorf("market_goods %s: a supply without heap slots", where)
		}
		return nil
	}
	var front *inv.Shopfront
	for i := range c.Inventory.Shopfronts {
		if c.Inventory.Shopfronts[i].ID == s.Shop {
			front = &c.Inventory.Shopfronts[i]
		}
	}
	if front == nil {
		return fmt.Errorf("market_goods %s: heap slots need the shopfront %s", where, s.Shop)
	}
	stocked := map[string]map[string]bool{}
	for _, src := range c.Inventory.Sources {
		if src.Location.Kind == "store" {
			if stocked[src.Location.Store] == nil {
				stocked[src.Location.Store] = map[string]bool{}
			}
			stocked[src.Location.Store][src.Definition] = true
		}
	}
	if s.Supply != nil {
		st, ok := c.Inventory.Stores[s.Supply.Store]
		if !ok || !st.Locked || st.Owner != s.Trader || s.Supply.Days < 1 {
			return fmt.Errorf("market_goods %s: the consignment %s is not a locked store of %s", where, s.Supply.Store, s.Trader)
		}
	}
	type span struct {
		id     string
		x0, x1 int
	}
	var spans []span
	for _, h := range s.Heaps {
		l, ok := levels[h.Mount]
		if !ok || l.Hang {
			return fmt.Errorf("market_goods %s: heap %s on level %q, not a standing level of the passport", where, h.ID, h.Mount)
		}
		if h.ZMM != l.ZMM {
			return fmt.Errorf("market_goods %s: heap %s at z %d, level %s is at %d", where, h.ID, h.ZMM, h.Mount, l.ZMM)
		}
		tare, ok := g.Models[h.Tare]
		if !ok || tare.Kind != "container" {
			return fmt.Errorf("market_goods %s: heap %s stands in %q, not a tare of the produce kit", where, h.ID, h.Tare)
		}
		w, depth, height, produce := tare.SizeMM[0], tare.SizeMM[1], tare.SizeMM[2], ""
		for _, level := range [][2]string{{"full", h.Heaps.Full}, {"half", h.Heaps.Half}, {"few", h.Heaps.Few}} {
			m, ok := g.Models[level[1]]
			if !ok || m.Kind != "heap" || m.Fill != level[0] {
				return fmt.Errorf("market_goods %s: heap %s level %s is %q, not a %s heap of the produce kit", where, h.ID, level[0], level[1], level[0])
			}
			if h.Tare != m.Container && !strings.HasPrefix(h.Tare, m.Container+".") {
				return fmt.Errorf("market_goods %s: heap %s: %s lies in %s, the slot's tare is %s", where, h.ID, level[1], m.Container, h.Tare)
			}
			if produce != "" && m.Produce != produce {
				return fmt.Errorf("market_goods %s: heap %s mixes %s and %s", where, h.ID, produce, m.Produce)
			}
			produce = m.Produce
			w, depth, height = max(w, m.SizeMM[0]), max(depth, m.SizeMM[1]), max(height, m.SizeMM[2])
		}
		x0, x1 := s.XMM[0]+h.XMM-(w+1)/2, s.XMM[0]+h.XMM+(w+1)/2
		if err := place(where, "heap "+h.ID, h.Mount, x0, x1, h.DMM, h.ZMM, h.ZMM+height, s.XMM, false); err != nil {
			return err
		}
		if h.DMM-(depth+1)/2 < l.DMM[0] || h.DMM+(depth+1)/2 > l.DMM[1] {
			return fmt.Errorf("market_goods %s: heap %s is %d mm deep at depth %d, level %s holds %v", where, h.ID, depth, h.DMM, h.Mount, l.DMM)
		}
		if h.Mount == "deck" {
			// the decks of adjacent units carry a tray together: at most 60 mm of its length (the joint) is unsupported
			carried := 0
			for _, f := range s.Fixtures {
				if f.Kind == "front_deck" && f.ZMM[1] == l.ZMM {
					carried += max(0, min(x1, s.XMM[0]+f.XMM[1])-max(x0, s.XMM[0]+f.XMM[0]))
				}
			}
			if carried < x1-x0-60 {
				return fmt.Errorf("market_goods %s: heap %s stands on the deck level without a front deck under it", where, h.ID)
			}
		}
		for _, o := range spans {
			if x0 < o.x1 && o.x0 < x1 {
				return fmt.Errorf("market_goods %s: heaps %s and %s overlap", where, o.id, h.ID)
			}
		}
		spans = append(spans, span{h.ID, x0, x1})
		if _, ok := c.Inventory.Definitions[h.Item]; !ok || !stocked[front.DisplayStore][h.Item] {
			return fmt.Errorf("market_goods %s: heap %s shows %s, which the stall's display does not stock", where, h.ID, h.Item)
		}
		if h.Full < 1 {
			return fmt.Errorf("market_goods %s: heap %s has no full stock", where, h.ID)
		}
		if s.Supply != nil && !stocked[s.Supply.Store][h.Item] {
			return fmt.Errorf("market_goods %s: the consignment %s holds no %s for heap %s", where, s.Supply.Store, h.Item, h.ID)
		}
	}
	for _, name := range s.Replaces {
		if name == "" || strings.Contains(name, ":") {
			return fmt.Errorf("market_goods %s: replaced node %q", where, name)
		}
	}
	return nil
}

// validateCraftCounters: every counter is 1..4 adjacent units of one side of an island of the level; a counter with a
// shop names a shopfront; no unit belongs to two counters.
func validateCraftCounters(c Compiled, instances map[string]bool) error {
	fronts := map[string]bool{}
	for _, sh := range c.Inventory.Shopfronts {
		fronts[sh.ID] = true
	}
	taken := map[string]string{}
	for _, k := range c.Craft.Counters {
		if len(instances) > 0 && !instances[k.Island] {
			return fmt.Errorf("market_goods counter %s: %s is not an island module of the level", k.Place, k.Island)
		}
		if (k.Side != "north" && k.Side != "south") || len(k.Units) < 1 || len(k.Units) > 4 {
			return fmt.Errorf("market_goods counter %s: side %q, units %v", k.Place, k.Side, k.Units)
		}
		if k.DepthMM < 0 || k.DepthMM > c.Craft.Passport.SizeMM.D {
			return fmt.Errorf("market_goods counter %s: depth %d mm", k.Place, k.DepthMM)
		}
		for i, u := range k.Units {
			if u < 0 || u > 3 || (i > 0 && u != k.Units[i-1]+1) {
				return fmt.Errorf("market_goods counter %s: units %v are not adjacent units of the island", k.Place, k.Units)
			}
			key := fmt.Sprintf("%s/%s/%d", k.Island, k.Side, u)
			if other, ok := taken[key]; ok {
				return fmt.Errorf("market_goods counter %s: unit %s also belongs to %s", k.Place, key, other)
			}
			taken[key] = k.Place
		}
		if k.Shop != "" && !fronts[k.Shop] && len(fronts) > 0 {
			return fmt.Errorf("market_goods counter %s: %s is not a shopfront", k.Place, k.Shop)
		}
	}
	return nil
}

// clientCraftCounters: the counters for the client — the plan rectangle of the counter's half of the island (metres) and
// the name prefixes of its goods blocks in the island model ("<island>-<side letter><unit>_").
func clientCraftCounters(g *CraftCatalog) []map[string]any {
	out := []map[string]any{}
	for _, k := range g.Counters {
		x, y := float64(k.At.X)*0.5, float64(k.At.Y)*0.5
		half := float64(g.Passport.SizeMM.D) / 2000
		if k.DepthMM > 0 {
			half = float64(k.DepthMM) / 1000
		}
		rect := []float64{x + float64(k.Units[0])*1.5, y, x + float64(k.Units[len(k.Units)-1]+1)*1.5, y + half}
		letter := "n"
		if k.Side == "south" {
			rect[1], rect[3], letter = y+half, y+2*half, "s"
		}
		prefixes := []string{}
		for _, u := range k.Units {
			prefixes = append(prefixes, fmt.Sprintf("%s-%s%d_", strings.TrimPrefix(k.Island, "island."), letter, u))
		}
		out = append(out, map[string]any{"place": k.Place, "island": k.Island, "shop": k.Shop, "pack_away": k.PackAway, "rect_m": rect, "goods_prefixes": prefixes})
	}
	return out
}

// wiredFixture: a powered fixture's cable starts on the fixture's box (x, d, z bands widened by 0.25 m) and ends on the bus.
func wiredFixture(where string, f CraftFixture, onBus func([3]int) bool) error {
	if f.Cable == nil || len(f.Cable.RouteMM) < 2 {
		return fmt.Errorf("market_goods %s: consumer %s has no cable to the island bus (doc 57)", where, f.ID)
	}
	a := f.Cable.RouteMM[0]
	in := func(v int, b [2]int) bool { return v >= b[0]-250 && v <= b[1]+250 }
	if !in(a[0], f.XMM) || !in(a[1], f.DMM) || !in(a[2], f.ZMM) {
		return fmt.Errorf("market_goods %s: the cable of %s does not start at its body", where, f.ID)
	}
	if !onBus(f.Cable.RouteMM[len(f.Cable.RouteMM)-1]) {
		return fmt.Errorf("market_goods %s: the cable of %s does not end on the island bus", where, f.ID)
	}
	return nil
}

// clientCraft: the presentation contract of PocConfig — the places with their kit layout (Unity binds the stall prefab of
// the place and maps the published action ids to clips / props / effects). Stock and money stay on the server.
func clientCraft(g *CraftCatalog) map[string]any {
	if g == nil {
		return nil
	}
	stalls := []map[string]any{}
	for _, s := range g.Stalls {
		// v26: the client maps every action a seller can get to its clip; the server picks the closing ones only in those phases
		seller := append(append(append([]CraftAction(nil), s.Seller...), g.ClosingActions...), g.OpeningActions...)
		stalls = append(stalls, map[string]any{"place": s.Place, "island": s.Island, "face": s.Face, "units": s.Units, "shop": s.Shop, "trader": s.Trader,
			"kit": s.Kit, "reference": s.Reference, "seller": seller, "buyer": s.Buyer, "effects": s.Effects, "lights": s.Lights,
			"rows": s.Rows, "beacon": s.Beacon.Item})
	}
	kits := []map[string]any{}
	for _, k := range g.Kits {
		kits = append(kits, map[string]any{"id": k.ID, "atlas": k.Atlas, "closure": k.Closure.Mode, "phase_minutes": k.Closure.PhaseMinutes, "charge": k.Charge != nil})
	}
	view := map[string]any{"level": g.Level, "stalls": stalls, "kits": kits, "hand_props": g.HandProps}
	if len(g.Counters) > 0 {
		view["counters"] = clientCraftCounters(g)
	}
	return view
}
