package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"

	"cuelang.org/go/cue"
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
	Seller []CraftAction `json:"seller"`
	Buyer  []CraftAction `json:"buyer"`
}
type CraftCatalog struct {
	Level     string        `json:"level"`
	Passport  CraftPassport `json:"passport"`
	Kits      []CraftKit    `json:"kits"`
	Stalls    []CraftStall  `json:"stalls"`
	HandProps []string      `json:"hand_props"`
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
//   - people and places: the trader owns the shopfront, the buyers' brain place exists, the actions' props are known.
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
	}
	return nil
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
		stalls = append(stalls, map[string]any{"place": s.Place, "island": s.Island, "face": s.Face, "units": s.Units, "shop": s.Shop, "trader": s.Trader,
			"kit": s.Kit, "reference": s.Reference, "seller": s.Seller, "buyer": s.Buyer, "effects": s.Effects, "lights": s.Lights,
			"rows": s.Rows, "beacon": s.Beacon.Item})
	}
	kits := []map[string]any{}
	for _, k := range g.Kits {
		kits = append(kits, map[string]any{"id": k.ID, "atlas": k.Atlas, "closure": k.Closure.Mode, "phase_minutes": k.Closure.PhaseMinutes, "charge": k.Charge != nil})
	}
	return map[string]any{"level": g.Level, "stalls": stalls, "kits": kits, "hand_props": g.HandProps}
}
