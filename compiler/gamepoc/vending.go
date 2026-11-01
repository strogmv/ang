package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"

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
		FromSecond     int64 `json:"from_second"`
		SecondsPerUnit int64 `json:"seconds_per_unit"`
	} `json:"refill"`
	Signals struct {
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
		units = append(units, map[string]any{"id": u.ID, "n": u.N, "kind": u.Kind, "face": u.Face, "dock": u.Dock})
	}
	return map[string]any{"instance": v.Instance, "level": v.Level, "units": units, "light": v.Light, "status": v.Signals.Status, "dock": v.Signals.Dock}
}
