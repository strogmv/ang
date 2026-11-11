package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"cuelang.org/go/cue"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// Vending terminals of the dome system (demo-hub v12-vending, world.vending): each unit has a display store (finite stock
// for sale, owner = the system), a locked cassette (the system's reserve, refilled into the display at night) and a locked
// intake bin (buy-back). The host sells through the ordinary inventory purchase, buys back at the data price with a daily
// limit per account and drives the presentation states (status band, panel lamp by the presence sensor, dock).
type VendingOffer struct {
	Item            string   `json:"item"`
	Par             int64    `json:"par"`
	Reserve         int64    `json:"reserve"`
	RequiresProfile []string `json:"requires_profile"`
	Food            bool     `json:"food"`
}
type VendingBuy struct {
	Item       string `json:"item"`
	PriceMinor int64  `json:"price_minor"`
}
type VendingSignal struct {
	Color   string  `json:"color"`
	PulseHz float64 `json:"pulse_hz,omitempty"`
	FlashHz float64 `json:"flash_hz,omitempty"`
	RU      string  `json:"ru"`
}

// torgomat v2 (owner 11.11.2026): a pose in plan metres of the level and the shelves, models, timings and the night robot.
type VendingPose struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}
type VendingShelf struct {
	ID         string      `json:"id"`
	Item       string      `json:"item"`
	Units      int64       `json:"units"`
	Kind       string      `json:"kind"`
	At         VendingPose `json:"at"`
	YawDegrees float64     `json:"yaw_degrees"`
}
type VendingModel struct {
	Item string `json:"item"`
	Full string `json:"full,omitempty"`
	Half string `json:"half,omitempty"`
	Few  string `json:"few,omitempty"`
}
type VendingLift struct {
	RestZ   float64 `json:"rest_z"`
	WindowZ float64 `json:"window_z"`
}

// A leg of the night cabinet, derived by ANG from the waypoints of its route: every cell it drives over, their plan points
// (the dock cell's point is the coupled pose beside the port) and its game seconds when nothing stands in the way.
type VendingLeg struct {
	Cells   []inv.Cell   `json:"cells"`
	Points  [][2]float64 `json:"points"`
	Seconds int64        `json:"seconds"`
}
type VendingDock struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	YawDegrees float64 `json:"yaw_degrees"`
}

// The night service cabinet (addendum 1, owner 11.11): it couples to the docking port on the block's end; the host keeps its
// state (where it is, coupled or not, which module it loads).
type VendingRobot struct {
	Resource       string      `json:"resource"`
	Home           inv.Cell    `json:"home"`
	Out            []inv.Cell  `json:"out"`
	Back           []inv.Cell  `json:"back"`
	DockCell       inv.Cell    `json:"dock_cell"`
	Dock           VendingDock `json:"dock"`
	CellSeconds    int64       `json:"cell_seconds"`
	CoupleSeconds  int64       `json:"couple_seconds"`
	FillSeconds    int64       `json:"fill_seconds"`
	GiveWaySeconds int64       `json:"give_way_seconds"`
	ClearanceCells int         `json:"clearance_cells"`
	OutLeg         *VendingLeg `json:"out_leg,omitempty"`
	BackLeg        *VendingLeg `json:"back_leg,omitempty"`
}
type VendingUnit struct {
	ID            string         `json:"id"`
	N             int            `json:"n"`
	Kind          string         `json:"kind"`
	Face          string         `json:"face"`
	Panel         inv.Cell       `json:"panel"`
	Spine         inv.Cell       `json:"spine"`
	Approach      []inv.Cell     `json:"approach"`
	Lane          []inv.Cell     `json:"lane"`
	DisplayStore  string         `json:"display_store"`
	CassetteStore string         `json:"cassette_store"`
	IntakeStore   string         `json:"intake_store"`
	Sells         []VendingOffer `json:"sells"`
	Buys          []VendingBuy   `json:"buys"`
	Services      []string       `json:"services"`
	Dock          string         `json:"dock"`
	LabelKey      string         `json:"label_key"`
	Shelves       []VendingShelf `json:"shelves"`
	YawDegrees    float64        `json:"yaw_degrees"`
	Lift          *VendingLift   `json:"lift,omitempty"`
	Window        VendingPose    `json:"window"`
	Sash          bool           `json:"sash"`
	Fan           bool           `json:"fan"`
}
type VendingFixture struct {
	ID            string  `json:"id"`
	Kelvin        int     `json:"kelvin"`
	Watts         float64 `json:"watts"`
	Lumens        float64 `json:"lumens"`
	BeamDeg       float64 `json:"beam_deg"`
	TiltToFaceDeg float64 `json:"tilt_to_face_deg"`
	AimAheadM     float64 `json:"aim_ahead_m"`
	LipZM         float64 `json:"lip_z_m"`
}
type VendingLight struct {
	DayPPM       int64          `json:"day_ppm"`
	NightPPM     int64          `json:"night_ppm"`
	SensorPPM    int64          `json:"sensor_ppm"`
	NightFrom    int64          `json:"night_from"`
	NightTo      int64          `json:"night_to"`
	Fixture      VendingFixture `json:"fixture"`
	ScreenLumens float64        `json:"screen_lumens"`
}
type VendingCatalog struct {
	Instance               string       `json:"instance"`
	Level                  string       `json:"level"`
	Owner                  string       `json:"owner"`
	Account                string       `json:"account"`
	BuybackDailyLimitMinor int64        `json:"buyback_daily_limit_minor"`
	ServingTicks           int64        `json:"serving_ticks"`
	RefusalTicks           int64        `json:"refusal_ticks"`
	Light                  VendingLight `json:"light"`
	Refill                 struct {
		FromSecond     int64    `json:"from_second"`
		SecondsPerUnit int64    `json:"seconds_per_unit"`
		Order          []string `json:"order"`
	} `json:"refill"`
	Dispense struct {
		RingTicks   int64 `json:"ring_ticks"`
		LiftTicks   int64 `json:"lift_ticks"`
		WindowTicks int64 `json:"window_ticks"`
		OpenTicks   int64 `json:"open_ticks"`
		TakeTicks   int64 `json:"take_ticks"`
		CloseTicks  int64 `json:"close_ticks"`
	} `json:"dispense"`
	Intake struct {
		LightTicks int64 `json:"light_ticks"`
		OpenTicks  int64 `json:"open_ticks"`
		TakeTicks  int64 `json:"take_ticks"`
		CloseTicks int64 `json:"close_ticks"`
	} `json:"intake"`
	Models       map[string]VendingModel `json:"models"`
	NeutralModel string                  `json:"neutral_model"`
	Kit          []string                `json:"kit"`
	Robot        VendingRobot            `json:"robot"`
	Look         map[string]any          `json:"look"`
	Signals      struct {
		Status map[string]VendingSignal `json:"status"`
		Dock   map[string]VendingSignal `json:"dock"`
	} `json:"signals"`
	Units []VendingUnit `json:"units"`
}

var vendingColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func readVending(world cue.Value) (*VendingCatalog, error) {
	v := world.LookupPath(cue.ParsePath("vending"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.vending: %w", err)
	}
	var out VendingCatalog
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.vending: %w", err)
	}
	return &out, nil
}

// validateVending checks the catalog against the compiled Life geometry and the inventory model: the panel, approach and
// lane cells are walkable cells of the level, the spine is not, every store exists with the system as owner at the right
// anchor and lock, every item is defined and sold for the account currency, the system account exists.
func validateVending(c Compiled) error {
	v := c.Vending
	if v == nil {
		return nil
	}
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
		} `json:"locations"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	rows, ok := life.Locations[v.Level]
	if !ok || len(rows.Terrain) == 0 {
		return fmt.Errorf("vending: level %q is not a Life location", v.Level)
	}
	walk := func(p inv.Cell) bool {
		t := rows.Terrain
		return p.Y >= 0 && p.Y < len(t) && p.X >= 0 && p.X < len(t[p.Y]) && c.Walkable[string(t[p.Y][p.X])]
	}
	if v.Owner == "" || !c.Inventory.Owners[v.Owner] {
		return fmt.Errorf("vending: owner %q is not an inventory owner", v.Owner)
	}
	account, ok := c.Inventory.Accounts[v.Account]
	if !ok || account.Owner != v.Owner || c.Inventory.Currencies[account.Currency].Carrier != "account" {
		return fmt.Errorf("vending: system account %q missing, foreign or not an account currency", v.Account)
	}
	if v.BuybackDailyLimitMinor < 1 || v.ServingTicks < 1 || v.RefusalTicks < 1 || v.Refill.SecondsPerUnit < 60 || v.Refill.FromSecond < 0 || v.Refill.FromSecond >= 86400 {
		return fmt.Errorf("vending: invalid limits or refill window")
	}
	if v.Refill.FromSecond+int64(len(v.Units))*v.Refill.SecondsPerUnit > 86400 {
		return fmt.Errorf("vending: the refill window crosses midnight")
	}
	l := v.Light
	if l.DayPPM < 0 || l.DayPPM > 1000000 || l.NightPPM < 0 || l.NightPPM > 1000000 || l.SensorPPM < l.NightPPM || l.SensorPPM > 1000000 || l.NightFrom < 0 || l.NightFrom >= 86400 || l.NightTo < 0 || l.NightTo >= 86400 || l.Fixture.Lumens <= 0 || l.Fixture.BeamDeg <= 0 {
		return fmt.Errorf("vending: invalid panel light")
	}
	for _, name := range []string{"working", "serving", "refused", "service", "fault", "lockdown"} {
		if s, ok := v.Signals.Status[name]; !ok || !vendingColor.MatchString(s.Color) {
			return fmt.Errorf("vending: status signal %q missing or not #RRGGBB", name)
		}
	}
	for _, name := range []string{"off", "charging", "unlocking", "locked"} {
		if s, ok := v.Signals.Dock[name]; !ok || !vendingColor.MatchString(s.Color) {
			return fmt.Errorf("vending: dock signal %q missing or not #RRGGBB", name)
		}
	}
	if len(v.Units) == 0 {
		return fmt.Errorf("vending: no units")
	}
	seen := map[string]bool{}
	taken := map[inv.Cell]string{}
	for _, u := range v.Units {
		if u.ID == "" || seen[u.ID] {
			return fmt.Errorf("vending: empty or duplicate unit %q", u.ID)
		}
		seen[u.ID] = true
		if u.Face != "south" && u.Face != "north" {
			return fmt.Errorf("vending %s: face %q", u.ID, u.Face)
		}
		if !walk(u.Panel) || walk(u.Spine) {
			return fmt.Errorf("vending %s: the panel cell must be walkable and the spine blocked", u.ID)
		}
		inApproach := false
		for _, p := range u.Approach {
			if !walk(p) {
				return fmt.Errorf("vending %s: approach cell %v is not walkable", u.ID, p)
			}
			if other, dup := taken[p]; dup {
				return fmt.Errorf("vending %s: approach cell %v shared with %s", u.ID, p, other)
			}
			taken[p] = u.ID
			inApproach = inApproach || p == u.Panel
		}
		if !inApproach {
			return fmt.Errorf("vending %s: the panel cell is not in the approach", u.ID)
		}
		for _, p := range u.Lane {
			if !walk(p) {
				return fmt.Errorf("vending %s: lane cell %v is not walkable", u.ID, p)
			}
		}
		for kind, id := range map[string]string{"display": u.DisplayStore, "cassette": u.CassetteStore, "intake": u.IntakeStore} {
			st, ok := c.Inventory.Stores[id]
			want := u.Spine
			if kind == "display" {
				want = u.Panel
			}
			if !ok || st.Owner != v.Owner || st.Anchor.Kind != "world" || st.Anchor.Level != v.Level || st.Anchor.At != want || st.Locked != (kind != "display") {
				return fmt.Errorf("vending %s: %s store %q missing, foreign, misplaced or wrongly locked", u.ID, kind, id)
			}
		}
		if len(u.Sells) == 0 {
			return fmt.Errorf("vending %s: sells nothing", u.ID)
		}
		for _, o := range u.Sells {
			d, ok := c.Inventory.Definitions[o.Item]
			// par 0 = listed but never stocked by this unit: only a profile-gated offer (the refusal is the point)
			if !ok || d.Carrier != "physical" || d.PriceUnit != account.Currency || d.PriceDenominator < 1 || o.Par < 0 || o.Reserve < 0 || (o.Par == 0 && (len(o.RequiresProfile) == 0 || o.Reserve != 0)) {
				return fmt.Errorf("vending %s: offer %q is not a physical item priced in %s", u.ID, o.Item, account.Currency)
			}
		}
		for _, b := range u.Buys {
			d, ok := c.Inventory.Definitions[b.Item]
			if !ok || d.Carrier != "physical" || b.PriceMinor < 1 || b.PriceMinor > v.BuybackDailyLimitMinor {
				return fmt.Errorf("vending %s: buy-back %q undefined or priced out of the daily limit", u.ID, b.Item)
			}
		}
		for _, s := range u.Services {
			if s != "charge" {
				return fmt.Errorf("vending %s: unknown service %q", u.ID, s)
			}
		}
		if u.Dock != "none" {
			if _, ok := v.Signals.Dock[u.Dock]; !ok {
				return fmt.Errorf("vending %s: unknown dock state %q", u.ID, u.Dock)
			}
		}
	}
	return validateVendingLive(v, walk)
}

// validateVendingLive (torgomat v2): the shelves of every unit hold its stocked goods and their shares add up to the par level;
// every good has its scenes in the kit; the dispense and buy-back timings fit the band; the refill order names every unit once;
// the robot's route runs over walkable cells in straight runs, passes the docks in the refill order and fits the night window.
// It derives the robot's legs (plan points and game seconds) for the host.
func validateVendingLive(v *VendingCatalog, walk func(inv.Cell) bool) error {
	d := v.Dispense
	if d.RingTicks < 1 || d.LiftTicks < 1 || d.WindowTicks < 1 || d.OpenTicks < 1 || d.TakeTicks < 1 || d.CloseTicks < 1 ||
		v.ServingTicks != d.RingTicks+d.LiftTicks+d.WindowTicks+d.OpenTicks+d.TakeTicks+d.CloseTicks {
		return fmt.Errorf("vending: the dispense phases must be positive and add up to serving_ticks")
	}
	in := v.Intake
	if in.LightTicks < 1 || in.OpenTicks < 1 || in.TakeTicks < 1 || in.CloseTicks < 1 || in.LightTicks+in.OpenTicks+in.TakeTicks+in.CloseTicks > v.ServingTicks {
		return fmt.Errorf("vending: the buy-back phases must be positive and fit serving_ticks")
	}
	kit := map[string]bool{}
	for _, r := range v.Kit {
		kit[r] = true
	}
	inKit := func(r string) bool { return r != "" && kit[r] }
	if !inKit(v.NeutralModel) || !inKit(v.Robot.Resource) {
		return fmt.Errorf("vending: the neutral package or the robot is not in the kit")
	}
	if len(v.Look) == 0 {
		return fmt.Errorf("vending: the look of the block is missing")
	}
	byID := map[string]int{}
	for i, u := range v.Units {
		byID[u.ID] = i
		share := map[string]int64{}
		seen := map[string]bool{}
		for _, sh := range u.Shelves {
			if sh.ID == "" || seen[sh.ID] || sh.Units < 1 || (sh.Kind != "shelf" && sh.Kind != "bay") || (sh.Kind == "bay") != (u.Kind == "equipment") {
				return fmt.Errorf("vending %s: shelf %q empty, duplicate or of the wrong kind", u.ID, sh.ID)
			}
			seen[sh.ID] = true
			m, ok := v.Models[sh.Item]
			if !ok || !inKit(m.Item) || !inKit(m.Full) || !inKit(m.Half) || !inKit(m.Few) {
				return fmt.Errorf("vending %s: shelf %s holds %q without its scenes in the kit", u.ID, sh.ID, sh.Item)
			}
			share[sh.Item] += sh.Units
		}
		for _, o := range u.Sells {
			if o.Par > 0 && share[o.Item] != o.Par {
				return fmt.Errorf("vending %s: the shelves of %s hold %d, the par level is %d", u.ID, o.Item, share[o.Item], o.Par)
			}
			delete(share, o.Item)
		}
		if len(share) > 0 {
			extra := make([]string, 0, len(share))
			for item := range share {
				extra = append(extra, item)
			}
			sort.Strings(extra)
			return fmt.Errorf("vending %s: a shelf holds %s the unit does not stock", u.ID, extra[0])
		}
		for _, b := range u.Buys {
			if m, ok := v.Models[b.Item]; ok && !inKit(m.Item) {
				return fmt.Errorf("vending %s: the buy-back scene of %s is not in the kit", u.ID, b.Item)
			}
		}
		if (u.Kind != "equipment") != (u.Lift != nil) || (u.Lift != nil && u.Lift.WindowZ >= u.Lift.RestZ) {
			return fmt.Errorf("vending %s: a showcase has a lift from its floor down to the window, an equipment module none", u.ID)
		}
	}
	order := v.Refill.Order
	if len(order) != len(v.Units) {
		return fmt.Errorf("vending: the refill order must name every unit once")
	}
	placed := map[string]bool{}
	for _, id := range order {
		if _, ok := byID[id]; !ok || placed[id] {
			return fmt.Errorf("vending: the refill order must name every unit once")
		}
		placed[id] = true
	}
	// the cabinet: straight runs between the waypoints over walkable cells, out from home to the dock cell and back
	r := &v.Robot
	if r.CellSeconds < 1 || r.CoupleSeconds < 1 || r.FillSeconds < 1 || r.FillSeconds > v.Refill.SecondsPerUnit || r.GiveWaySeconds < 1 || r.ClearanceCells < 0 || !walk(r.DockCell) {
		return fmt.Errorf("vending: the robot needs positive timings, a fill within a module's window and a dock cell on the floor")
	}
	leg := func(name string, way []inv.Cell, from, to inv.Cell) (*VendingLeg, error) {
		if len(way) < 2 || way[0] != from || way[len(way)-1] != to {
			return nil, fmt.Errorf("vending: the robot's %s route must run from %v to %v", name, from, to)
		}
		cells := []inv.Cell{way[0]}
		for k := 1; k < len(way); k++ {
			a, b := way[k-1], way[k]
			if a.X != b.X && a.Y != b.Y {
				return nil, fmt.Errorf("vending: the robot's run %v -> %v is not straight", a, b)
			}
			for a != b {
				switch {
				case a.X < b.X:
					a.X++
				case a.X > b.X:
					a.X--
				case a.Y < b.Y:
					a.Y++
				default:
					a.Y--
				}
				cells = append(cells, a)
			}
		}
		out := &VendingLeg{Cells: cells, Seconds: int64(len(cells)-1) * r.CellSeconds}
		for _, c := range cells {
			// its body covers the cell and clearance_cells round it: the whole lane must be floor
			for dx := -r.ClearanceCells; dx <= r.ClearanceCells; dx++ {
				for dy := -r.ClearanceCells; dy <= r.ClearanceCells; dy++ {
					if !walk(inv.Cell{X: c.X + dx, Y: c.Y + dy}) {
						return nil, fmt.Errorf("vending: the robot's %s lane at %v runs into the blocked cell %v", name, c, inv.Cell{X: c.X + dx, Y: c.Y + dy})
					}
				}
			}
			out.Points = append(out.Points, [2]float64{float64(c.X)*0.5 + 0.25, float64(c.Y)*0.5 + 0.25})
		}
		return out, nil
	}
	var err error
	if r.OutLeg, err = leg("out", r.Out, r.Home, r.DockCell); err != nil {
		return err
	}
	if r.BackLeg, err = leg("back", r.Back, r.DockCell, r.Home); err != nil {
		return err
	}
	dock := [2]float64{r.Dock.X, r.Dock.Y}
	r.OutLeg.Points[len(r.OutLeg.Points)-1] = dock
	r.BackLeg.Points[0] = dock
	// the coupled pose stands beside its dock cell (within a cell of its middle)
	if dx, dy := r.Dock.X-(float64(r.DockCell.X)*0.5+0.25), r.Dock.Y-(float64(r.DockCell.Y)*0.5+0.25); dx*dx+dy*dy > 0.5*0.5 {
		return fmt.Errorf("vending: the coupled pose is more than a cell from the dock cell")
	}
	// the night: it leaves home so that the first module's window opens as it has coupled; it is home before midnight
	if v.Refill.FromSecond-r.OutLeg.Seconds-r.CoupleSeconds < 0 ||
		v.Refill.FromSecond+int64(len(order))*v.Refill.SecondsPerUnit+r.CoupleSeconds+r.BackLeg.Seconds >= 86400 {
		return fmt.Errorf("vending: the robot's round does not fit the night")
	}
	return nil
}

// clientVending is the presentation contract of PocConfig: units (to find their prefab nodes), the colours of the
// signals and the panel lamp. Stock, prices and accounts stay on the server (the snapshot carries only what the hero sees).
func clientVending(v *VendingCatalog) map[string]any {
	if v == nil {
		return nil
	}
	units := []map[string]any{}
	for _, u := range v.Units {
		row := map[string]any{"id": u.ID, "n": u.N, "kind": u.Kind, "face": u.Face, "dock": u.Dock,
			// torgomat v2: the live parts of the model the client moves (the scenes of the goods come with the view)
			"yaw_degrees": u.YawDegrees, "window": u.Window, "sash": u.Sash, "fan": u.Fan}
		if u.Lift != nil {
			row["lift"] = u.Lift
		}
		units = append(units, row)
	}
	return map[string]any{"instance": v.Instance, "level": v.Level, "units": units, "light": v.Light, "status": v.Signals.Status, "dock": v.Signals.Dock,
		"dispense": v.Dispense, "intake": v.Intake, "look": v.Look, "robot": map[string]any{"resource": v.Robot.Resource}}
}
