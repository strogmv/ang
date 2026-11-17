package gamepoc

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// TrainCabin is the inside of every passenger car (train v3, owner 11.11): who rides and how the car is lit.
// The server publishes per-car counts only: the real Life riders who boarded at the berth (in the car of their
// door, derived here from the berth edge's exit cell) plus background passengers by the hour of the run's
// departure, varied by the seed, the run and the car. Background passengers have no names and no saved life.
// Places and look are presentation data for the client's cheap crowd and the car's own cold light.
type TrainCabin struct {
	FloorM                 float64          `json:"floor_m"`
	SeatM                  float64          `json:"seat_m"`
	Seats                  []TrainCabinSpot `json:"seats"`
	Standing               []TrainCabinSpot `json:"standing"`
	BackgroundPerCarByHour []int            `json:"background_per_car_by_hour"`
	SpreadPermille         int              `json:"spread_permille"`
	Look                   TrainCabinLook   `json:"look"`
	// Cozy v1 (design/market-train/cozy-v1): who rides each run (the server's per-car list), the riders' dressing of the
	// car, the client's pooled people and painted cards, the painted outside. All optional: a cabin without them is v3.
	Riders   *TrainCabinRiders   `json:"riders,omitempty"`
	Dressing *TrainCabinDressing `json:"dressing,omitempty"`
	People   *TrainCabinPeople   `json:"people,omitempty"`
	Paint    *TrainCabinPaint    `json:"paint,omitempty"`
	// Derived, never authored: the module whose berthed envelope holds the berth edge's exit cell.
	RiderModule string `json:"rider_module,omitempty"`
}

// A place inside one car: metres from the car centre, x along the car, y across (+ toward the side of the
// authored "far" door leaves). Seated places face the aisle; standing ones face along the car.
type TrainCabinSpot struct {
	XM float64 `json:"x_m"`
	YM float64 `json:"y_m"`
	// Market polish v1 (cozy v2): the way a bay seat's rider faces along the car ("ahead": +X, "back": -X); empty: across,
	// toward the aisle (the v1 benches) or, for a standing place, along the aisle.
	Face string `json:"face,omitempty"`
}

type TrainCabinLook struct {
	// Source material names (substring match) of the stock's glass, ceiling luminaires and lit interior.
	GlassMaterials    []string `json:"glass_materials"`
	StripMaterials    []string `json:"strip_materials"`
	InteriorMaterials []string `json:"interior_materials"`
	StripColor        string   `json:"strip_color"`
	StripEnergy       float64  `json:"strip_energy"`
	InteriorEmission  float64  `json:"interior_emission"`
	LightColor        string   `json:"light_color"`
	LightEnergy       float64  `json:"light_energy"`
	LightRangeM       float64  `json:"light_range_m"`
	LightAttenuation  float64  `json:"light_attenuation"`
	LightHeightM      float64  `json:"light_height_m"`
	GlassTint         string   `json:"glass_tint"`
	GlassAlpha        float64  `json:"glass_alpha"`
	GlassSpecular     float64  `json:"glass_specular"`
	GlassRoughness    float64  `json:"glass_roughness"`
	// Cloth colours of the crowd: the hall visitors' kinds (market_visitors_link.gd), by share.
	Looks []TrainCabinCrowdLook `json:"looks"`
	// Base colours that replace a lit interior material's own (the stock keeps its mesh; the look is data).
	Recolors []TrainCabinRecolor `json:"recolors"`
}

// TrainCabinRecolor names one interior source material (substring match, like the roles) and its new base colour.
type TrainCabinRecolor struct {
	Material string `json:"material"`
	Albedo   string `json:"albedo"`
}

type TrainCabinCrowdLook struct {
	Look  string `json:"look"`
	Share int    `json:"share"`
}

// TrainCabinRiders: the server picks every passenger's look and pose from these lists (seed, departure, car, place) and
// sends per car only indices into them; look index len(Looks) is the child.
type TrainCabinRiders struct {
	Looks []TrainCabinRiderLook `json:"looks"`
	Child TrainCabinChild       `json:"child"`
	Poses []TrainCabinPose      `json:"poses"`
	Home  TrainCabinHome        `json:"home"`
}
type TrainCabinRiderLook struct {
	Look  string `json:"look"`
	Share int    `json:"share"`
	Body  string `json:"body"`
	Tint  string `json:"tint"`
}
type TrainCabinChild struct {
	Look      string `json:"look"`
	AfterLook string `json:"after_look"`
	Permille  int    `json:"permille"`
	Pose      string `json:"pose"`
	Tint      string `json:"tint"`
}
type TrainCabinPose struct {
	ID      string     `json:"id"`
	At      string     `json:"at"`
	Share   int        `json:"share"`
	HeadDeg [3]float64 `json:"head_deg"`
	Prop    string     `json:"prop"`
	// Market polish v1 (cozy v2): the head's tilt and turn lean toward the window at the rider's side ("window"), and the
	// arms over the seated clip: "point" (the window-side arm out at the window), "eat" (both forearms to the table),
	// "hold" (a slate held up in both hands).
	Toward string `json:"toward,omitempty"`
	Arms   string `json:"arms,omitempty"`
	// the painted card of another pose drawn for this one (far and fast cars: the silhouettes are alike at that size)
	CardAs string `json:"card_as,omitempty"`
}

// TrainCabinHome is the hook for real people going home by this train: residents of Levels boarding by Edges.
type TrainCabinHome struct {
	Levels []string `json:"levels"`
	Edges  []string `json:"edges"`
}
type TrainCabinDressing struct {
	Kit   string `json:"kit"`
	Shade struct {
		Material string  `json:"material"`
		Emission string  `json:"emission"`
		Energy   float64 `json:"energy"`
	} `json:"shade"`
	Curtains struct {
		Material  string   `json:"material"`
		Palette   []string `json:"palette"`
		Backlight string   `json:"backlight"`
		Glow      float64  `json:"glow"`
	} `json:"curtains"`
	WindowLight struct {
		Color       string  `json:"color"`
		Energy      float64 `json:"energy"`
		RangeM      float64 `json:"range_m"`
		AngleDeg    float64 `json:"angle_deg"`
		HeightM     float64 `json:"height_m"`
		DownDeg     float64 `json:"down_deg"`
		Attenuation float64 `json:"attenuation"`
		PatternPx   [2]int  `json:"pattern_px"`
		PatternSoft float64 `json:"pattern_soft"`
	} `json:"window_light"`
	Glass struct {
		Tint       string  `json:"tint"`
		Alpha      float64 `json:"alpha"`
		Fog        float64 `json:"fog"`
		FogHeightM float64 `json:"fog_height_m"`
		FogTint    string  `json:"fog_tint"`
		Drops      float64 `json:"drops"`
		Sheen      float64 `json:"sheen"`
		Rim        float64 `json:"rim"`
	} `json:"glass"`
	Haze struct {
		Color   string  `json:"color"`
		Alpha   float64 `json:"alpha"`
		WidthM  float64 `json:"width_m"`
		HeightM float64 `json:"height_m"`
		RiseMPS float64 `json:"rise_mps"`
	} `json:"haze"`
	// Market polish v1 (cozy v2): the stock's own materials the dressing replaces (hidden in the cabin: the v1 benches), and
	// the table lamps of the bays — the bulb's glow and one small light each, car metres (x along, y across, z up).
	HideMaterials []string         `json:"hide_materials,omitempty"`
	Lamps         *TrainCabinLamps `json:"lamps,omitempty"`
	// the share of the cabin's own glow (shades, bulbs, curtains, lit liner, its lights) kept in full daylight: by day the
	// hall's light outshines it and the windows read calm (0 or absent: the same glow day and night, as v1)
	DayKeep float64 `json:"day_keep,omitempty"`
	// the glow of the riders' things (seats, tables, what is on them) lit by the cabin's light; absent: the liner's
	KitGlow *float64 `json:"kit_glow,omitempty"`
}
type TrainCabinLamps struct {
	Bulb struct {
		Material string  `json:"material"`
		Emission string  `json:"emission"`
		Energy   float64 `json:"energy"`
	} `json:"bulb"`
	Light struct {
		Color       string  `json:"color"`
		Energy      float64 `json:"energy"`
		RangeM      float64 `json:"range_m"`
		Attenuation float64 `json:"attenuation"`
	} `json:"light"`
	At [][3]float64 `json:"at"`
}
type TrainCabinPeople struct {
	Pool []struct {
		Look  string `json:"look"`
		Count int    `json:"count"`
	} `json:"pool"`
	RealWithinM  float64 `json:"real_within_m"`
	RealBelowMPS float64 `json:"real_below_mps"`
	Clips        struct {
		Seated   string `json:"seated"`
		Standing string `json:"standing"`
	} `json:"clips"`
	AnimateEvery int `json:"animate_every"`
	Cards        struct {
		Atlas string `json:"atlas"`
		Mask  string `json:"mask"`
		Grid  [2]int `json:"grid"`
		Cells []struct {
			Pose string `json:"pose"`
			Body string `json:"body"`
			View string `json:"view"`
			Cell [2]int `json:"cell"`
		} `json:"cells"`
		SizeM     [2]float64 `json:"size_m"`
		SeatDropM float64    `json:"seat_drop_m"`
		Breath    struct {
			Amp float64 `json:"amp"`
			Hz  float64 `json:"hz"`
		} `json:"breath"`
		TintKeep  float64 `json:"tint_keep"`
		BackLight float64 `json:"back_light"`
	} `json:"cards"`
}
type TrainCabinPaint struct {
	SidePos     string   `json:"side_pos"`
	SideNeg     string   `json:"side_neg"`
	Roof        string   `json:"roof"`
	HalfLengthM float64  `json:"half_length_m"`
	HalfWidthM  float64  `json:"half_width_m"`
	Z0M         float64  `json:"z0_m"`
	Z1M         float64  `json:"z1_m"`
	Materials   []string `json:"materials"`
	Strength    float64  `json:"strength"`
	Tint        string   `json:"tint"`
	// market polish v1: the share of its painted colour the car keeps at the least in the painting's light (night)
	NightFloor float64 `json:"night_floor,omitempty"`
}

var trainCabinColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
var trainCabinRes = regexp.MustCompile(`^res://[A-Za-z0-9_./-]+$`)
var trainCabinPoseID = regexp.MustCompile(`^[a-z][a-z_]{0,23}$`)
var trainCabinLookID = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

func (c TrainMotionCatalog) validateCabin() error {
	k := c.Cabin
	if k == nil {
		return nil
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	halfLength, halfWidth := math.Inf(1), math.Inf(1)
	for _, m := range c.Modules {
		halfLength = math.Min(halfLength, m.LengthM/2)
		halfWidth = math.Min(halfWidth, m.WidthM/2)
	}
	if !finite(k.FloorM) || k.FloorM < -1 || k.FloorM > 1 {
		return fmt.Errorf("cabin floor must be a finite height within a metre of the stock datum")
	}
	if !finite(k.SeatM) || k.SeatM <= k.FloorM+.2 || k.SeatM > k.FloorM+1 {
		return fmt.Errorf("cabin seat top must stand 0.2..1 m above the cabin floor")
	}
	if len(k.Seats) == 0 || len(k.Seats) > 64 || len(k.Standing) > 64 {
		return fmt.Errorf("cabin needs 1..64 seats and at most 64 standing places")
	}
	places := append(append([]TrainCabinSpot{}, k.Seats...), k.Standing...)
	for i, p := range places {
		if p.Face != "" && p.Face != "ahead" && p.Face != "back" {
			return fmt.Errorf("cabin place %d faces %q: ahead, back or across (empty)", i, p.Face)
		}
		if !finite(p.XM) || !finite(p.YM) || math.Abs(p.XM) > halfLength-.2 || math.Abs(p.YM) > halfWidth-.2 {
			return fmt.Errorf("cabin place %d lies outside the car body", i)
		}
		for j := 0; j < i; j++ {
			if math.Hypot(p.XM-places[j].XM, p.YM-places[j].YM) < .35 {
				return fmt.Errorf("cabin places %d and %d overlap", j, i)
			}
		}
	}
	if len(k.BackgroundPerCarByHour) != 24 {
		return fmt.Errorf("cabin background passengers need 24 hourly values")
	}
	for h, n := range k.BackgroundPerCarByHour {
		if n < 0 || n > len(places) {
			return fmt.Errorf("cabin background passengers at hour %d exceed the car", h)
		}
	}
	if k.SpreadPermille < 0 || k.SpreadPermille > 1000 {
		return fmt.Errorf("cabin spread must be 0..1000 permille")
	}
	l := k.Look
	for _, set := range [][]string{l.GlassMaterials, l.StripMaterials, l.InteriorMaterials} {
		if len(set) == 0 || len(set) > 16 {
			return fmt.Errorf("cabin look needs 1..16 source material names per role")
		}
		for _, name := range set {
			if strings.TrimSpace(name) == "" || len(name) > 96 {
				return fmt.Errorf("cabin look has an empty source material name")
			}
		}
	}
	for _, color := range []string{l.StripColor, l.LightColor, l.GlassTint} {
		if !trainCabinColor.MatchString(color) {
			return fmt.Errorf("cabin look colour %q is not #RRGGBB", color)
		}
	}
	ranges := []struct {
		v, lo, hi float64
		name      string
	}{
		{l.StripEnergy, 0, 16, "strip energy"}, {l.InteriorEmission, 0, 2, "interior emission"},
		{l.LightEnergy, 0, 16, "light energy"}, {l.LightRangeM, .1, 20, "light range"},
		{l.LightAttenuation, .1, 8, "light attenuation"}, {l.LightHeightM, 0, 3, "light height"},
		{l.GlassAlpha, .01, 1, "glass alpha"}, {l.GlassSpecular, 0, 1, "glass specular"}, {l.GlassRoughness, 0, 1, "glass roughness"},
	}
	for _, r := range ranges {
		if !finite(r.v) || r.v < r.lo || r.v > r.hi {
			return fmt.Errorf("cabin look %s out of range", r.name)
		}
	}
	if len(l.Looks) == 0 || len(l.Looks) > 16 {
		return fmt.Errorf("cabin crowd needs 1..16 looks")
	}
	for _, look := range l.Looks {
		if !trainCabinLookID.MatchString(look.Look) || look.Share <= 0 || look.Share > 1000 {
			return fmt.Errorf("cabin crowd look %q needs an id and a positive share", look.Look)
		}
	}
	if len(l.Recolors) > 16 {
		return fmt.Errorf("cabin look has more than 16 recolours")
	}
	seen := map[string]bool{}
	for _, r := range l.Recolors {
		interior := false
		for _, name := range l.InteriorMaterials {
			interior = interior || name == r.Material
		}
		if !interior {
			return fmt.Errorf("cabin recolour %q is not one of the interior materials", r.Material)
		}
		if seen[r.Material] {
			return fmt.Errorf("cabin recolour %q is given twice", r.Material)
		}
		seen[r.Material] = true
		if !trainCabinColor.MatchString(r.Albedo) {
			return fmt.Errorf("cabin recolour colour %q is not #RRGGBB", r.Albedo)
		}
	}
	return k.validateCozy(halfLength, halfWidth)
}

// validateCozy checks the cozy v1 blocks: the riders' lists the server indexes, the dressing, the people and the paint.
func (k *TrainCabin) validateCozy(halfLength, halfWidth float64) error {
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	within := func(v, lo, hi float64, name string) error {
		if !finite(v) || v < lo || v > hi {
			return fmt.Errorf("cabin %s out of range", name)
		}
		return nil
	}
	colors := func(set ...string) error {
		for _, c := range set {
			if !trainCabinColor.MatchString(c) {
				return fmt.Errorf("cabin colour %q is not #RRGGBB", c)
			}
		}
		return nil
	}
	looks := map[string]bool{}
	poses := map[string]TrainCabinPose{}
	if r := k.Riders; r != nil {
		if len(r.Looks) == 0 || len(r.Looks) > 15 {
			return fmt.Errorf("cabin riders need 1..15 looks")
		}
		for _, l := range r.Looks {
			if !trainCabinLookID.MatchString(l.Look) || l.Share <= 0 || l.Share > 1000 || looks[l.Look] {
				return fmt.Errorf("cabin rider look %q needs a unique id and a positive share", l.Look)
			}
			if l.Body != "m" && l.Body != "f" {
				return fmt.Errorf("cabin rider look %q has no card body m or f", l.Look)
			}
			if err := colors(l.Tint); err != nil {
				return err
			}
			looks[l.Look] = true
		}
		if len(r.Poses) < 2 || len(r.Poses) > 16 {
			return fmt.Errorf("cabin riders need 2..16 poses")
		}
		shares := map[string]int{}
		for _, p := range r.Poses {
			if !trainCabinPoseID.MatchString(p.ID) || poses[p.ID].ID != "" || (p.At != "seat" && p.At != "standing") || p.Share < 0 || p.Share > 1000 {
				return fmt.Errorf("cabin pose %q needs a unique id, a place kind and a share", p.ID)
			}
			if p.Prop != "" && p.Prop != "reader" {
				return fmt.Errorf("cabin pose %q has an unknown prop %q", p.ID, p.Prop)
			}
			if (p.Toward != "" && p.Toward != "window") || (p.Arms != "" && p.Arms != "point" && p.Arms != "eat" && p.Arms != "hold") {
				return fmt.Errorf("cabin pose %q leans %q with arms %q: toward window or none; arms point, eat, hold or none", p.ID, p.Toward, p.Arms)
			}
			for _, d := range p.HeadDeg {
				if !finite(d) || math.Abs(d) > 45 {
					return fmt.Errorf("cabin pose %q turns the head beyond 45 degrees", p.ID)
				}
			}
			poses[p.ID] = p
			shares[p.At] += p.Share
		}
		if shares["seat"] <= 0 || (len(k.Standing) > 0 && shares["standing"] <= 0) {
			return fmt.Errorf("cabin poses need a positive share for seats and for standing places")
		}
		ch := r.Child
		if !trainCabinLookID.MatchString(ch.Look) || looks[ch.Look] || !looks[ch.AfterLook] || ch.Permille < 0 || ch.Permille > 1000 {
			return fmt.Errorf("cabin child needs its own look and an adult look among the riders")
		}
		if p, ok := poses[ch.Pose]; !ok || p.At != "seat" {
			return fmt.Errorf("cabin child pose %q is not a seat pose", ch.Pose)
		}
		if err := colors(ch.Tint); err != nil {
			return err
		}
		for _, l := range r.Home.Levels {
			if strings.TrimSpace(l) == "" {
				return fmt.Errorf("cabin home level is empty")
			}
		}
		for _, e := range r.Home.Edges {
			if strings.TrimSpace(e) == "" {
				return fmt.Errorf("cabin home edge is empty")
			}
		}
	}
	if d := k.Dressing; d != nil {
		if !trainCabinRes.MatchString(d.Kit) {
			return fmt.Errorf("cabin dressing needs a res:// path for the kit")
		}
		if d.WindowLight.PatternPx[0] < 16 || d.WindowLight.PatternPx[1] < 16 || d.WindowLight.PatternPx[0] > 1024 || d.WindowLight.PatternPx[1] > 1024 {
			return fmt.Errorf("cabin window light pattern needs 16..1024 pixels a side")
		}
		if strings.TrimSpace(d.Shade.Material) == "" || strings.TrimSpace(d.Curtains.Material) == "" || len(d.Curtains.Palette) == 0 || len(d.Curtains.Palette) > 16 {
			return fmt.Errorf("cabin dressing needs the shade and curtain materials and 1..16 curtain colours")
		}
		if err := colors(append([]string{d.Shade.Emission, d.Curtains.Backlight, d.WindowLight.Color, d.Glass.Tint, d.Glass.FogTint, d.Haze.Color}, d.Curtains.Palette...)...); err != nil {
			return err
		}
		w, g, h := d.WindowLight, d.Glass, d.Haze
		for _, e := range []error{within(d.Shade.Energy, 0, 16, "shade energy"), within(w.Energy, 0, 16, "window light energy"),
			within(w.RangeM, .1, 20, "window light range"), within(w.AngleDeg, 1, 89, "window light angle"), within(w.HeightM, 0, 3, "window light height"),
			within(w.DownDeg, 0, 89, "window light aim"), within(w.Attenuation, .1, 8, "window light attenuation"), within(w.PatternSoft, 0, 1, "window light softness"),
			within(d.Curtains.Glow, 0, 4, "curtain glow"), within(g.Alpha, .01, 1, "glass alpha"), within(g.Fog, 0, 1, "glass fog"), within(g.FogHeightM, 0, 1.3, "glass fog height"),
			within(g.Drops, 0, 1, "glass drops"), within(g.Sheen, 0, 1, "glass sheen"), within(g.Rim, 0, 1, "glass rim"),
			within(h.Alpha, 0, 1, "haze alpha"), within(h.WidthM, .05, 3, "haze width"), within(h.HeightM, .05, 3, "haze height"), within(h.RiseMPS, 0, 2, "haze rise")} {
			if e != nil {
				return e
			}
		}
		if err := within(d.DayKeep, 0, 1, "cabin glow kept by day"); err != nil {
			return err
		}
		if d.KitGlow != nil {
			if err := within(*d.KitGlow, 0, 2, "cabin kit glow"); err != nil {
				return err
			}
		}
		for _, m := range d.HideMaterials {
			if strings.TrimSpace(m) == "" {
				return fmt.Errorf("cabin dressing hides a material without a name")
			}
		}
		if l := d.Lamps; l != nil {
			if strings.TrimSpace(l.Bulb.Material) == "" || len(l.At) == 0 || len(l.At) > 32 {
				return fmt.Errorf("cabin lamps need the bulb material and 1..32 places")
			}
			if err := colors(l.Bulb.Emission, l.Light.Color); err != nil {
				return err
			}
			for _, e := range []error{within(l.Bulb.Energy, 0, 16, "lamp bulb energy"), within(l.Light.Energy, 0, 8, "lamp light energy"),
				within(l.Light.RangeM, .1, 4, "lamp light range"), within(l.Light.Attenuation, .1, 8, "lamp light attenuation")} {
				if e != nil {
					return e
				}
			}
			for i, a := range l.At {
				if !finite(a[0]) || !finite(a[1]) || !finite(a[2]) || math.Abs(a[0]) > halfLength || math.Abs(a[1]) > halfWidth || a[2] < k.FloorM || a[2] > k.FloorM+2.5 {
					return fmt.Errorf("cabin lamp %d lies outside the car body", i)
				}
			}
		}
	}
	if p := k.People; p != nil {
		if k.Riders == nil {
			return fmt.Errorf("cabin people need the riders' lists")
		}
		pooled := map[string]bool{}
		for _, row := range p.Pool {
			if !looks[row.Look] || pooled[row.Look] || row.Count < 0 || row.Count > 24 {
				return fmt.Errorf("cabin pool look %q is not a rider look, repeats or holds more than 24", row.Look)
			}
			pooled[row.Look] = true
		}
		if err := within(p.RealWithinM, 0, 60, "real people distance"); err != nil {
			return err
		}
		if err := within(p.RealBelowMPS, 0, 30, "real people speed"); err != nil {
			return err
		}
		if strings.TrimSpace(p.Clips.Seated) == "" || strings.TrimSpace(p.Clips.Standing) == "" || p.AnimateEvery < 1 || p.AnimateEvery > 8 {
			return fmt.Errorf("cabin people need seated and standing clips and an animation step 1..8")
		}
		c := p.Cards
		if !trainCabinRes.MatchString(c.Atlas) || !trainCabinRes.MatchString(c.Mask) || c.Grid[0] < 1 || c.Grid[1] < 1 || c.Grid[0] > 16 || c.Grid[1] > 16 {
			return fmt.Errorf("cabin cards need res:// atlas and mask and a grid of 1..16")
		}
		used := map[[2]int]bool{}
		covered := map[string]bool{}
		for _, cell := range c.Cells {
			if _, ok := poses[cell.Pose]; !ok {
				return fmt.Errorf("cabin card pose %q is not a rider pose", cell.Pose)
			}
			if (cell.Body != "m" && cell.Body != "f" && cell.Body != "child") || (cell.View != "front" && cell.View != "back") {
				return fmt.Errorf("cabin card %s needs a body m/f/child and a view front/back", cell.Pose)
			}
			if cell.Cell[0] < 0 || cell.Cell[1] < 0 || cell.Cell[0] >= c.Grid[0] || cell.Cell[1] >= c.Grid[1] || used[cell.Cell] {
				return fmt.Errorf("cabin card cell %v is outside the grid or used twice", cell.Cell)
			}
			used[cell.Cell] = true
			covered[cell.Pose+"/"+cell.Body+"/"+cell.View] = true
		}
		// every pose a rider can take has a card for every body that can take it, from both sides
		for _, pose := range k.Riders.Poses {
			bodies := []string{}
			if pose.ID == k.Riders.Child.Pose {
				bodies = []string{"child"}
			} else {
				for _, l := range k.Riders.Looks {
					bodies = append(bodies, l.Body)
				}
			}
			if pose.Share == 0 && pose.ID != k.Riders.Child.Pose {
				continue
			}
			card := pose.ID
			if pose.CardAs != "" {
				if _, ok := poses[pose.CardAs]; !ok || pose.CardAs == pose.ID {
					return fmt.Errorf("cabin pose %s draws the card of %q, not another rider pose", pose.ID, pose.CardAs)
				}
				card = pose.CardAs
			}
			for _, b := range bodies {
				for _, v := range []string{"front", "back"} {
					if !covered[card+"/"+b+"/"+v] {
						return fmt.Errorf("cabin card missing for pose %s, body %s, %s view", pose.ID, b, v)
					}
				}
			}
		}
		for _, e := range []error{within(c.SizeM[0], .1, 3, "card width"), within(c.SizeM[1], .1, 3, "card height"), within(c.SeatDropM, 0, 1, "card seat drop"),
			within(c.Breath.Amp, 0, .05, "card breath"), within(c.Breath.Hz, .01, 2, "card breath rate"), within(c.TintKeep, 0, 1, "card tint"), within(c.BackLight, 0, 1, "card back light")} {
			if e != nil {
				return e
			}
		}
	}
	if p := k.Paint; p != nil {
		if !trainCabinRes.MatchString(p.SidePos) || !trainCabinRes.MatchString(p.SideNeg) || !trainCabinRes.MatchString(p.Roof) || len(p.Materials) == 0 || len(p.Materials) > 16 {
			return fmt.Errorf("cabin paint needs res:// pictures and 1..16 source materials")
		}
		if !trainCabinColor.MatchString(p.Tint) {
			return fmt.Errorf("cabin paint tint %q is not #RRGGBB", p.Tint)
		}
		for _, e := range []error{within(p.HalfLengthM, .5, 20, "paint length"), within(p.HalfWidthM, .2, 5, "paint width"),
			within(p.Z0M, -3, 0, "paint bottom"), within(p.Z1M, .5, 5, "paint top"), within(p.Strength, 0, 1, "paint strength"),
			within(p.NightFloor, 0, .5, "paint night floor")} {
			if e != nil {
				return e
			}
		}
	}
	return nil
}

// deriveCabinRiderModule finds the car of the boarding door: the berth edge's exit cell (plan metres from the
// berth level's cell size) against every module's envelope while the train stands at the berth.
func (c *TrainMotionCatalog) deriveCabinRiderModule(life json.RawMessage) error {
	if c.Cabin == nil {
		return nil
	}
	c.Cabin.RiderModule = ""
	if c.BerthEdgeID == "" {
		return nil
	}
	var l struct {
		Locations map[string]struct {
			CellMM int `json:"cell_mm"`
		} `json:"locations"`
		Edges []struct {
			ID   string `json:"id"`
			From string `json:"from"`
			Exit struct {
				X int `json:"x"`
				Y int `json:"y"`
			} `json:"exit"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(life, &l); err != nil {
		return fmt.Errorf("cabin: Life edges: %w", err)
	}
	cell := float64(l.Locations[c.BerthLevelID].CellMM) / 1000
	if cell <= 0 {
		return fmt.Errorf("cabin: berth level %s has no cell size", c.BerthLevelID)
	}
	var binding *TrainBinding
	for i := range c.Bindings {
		if c.Bindings[i].LevelID == c.BerthLevelID {
			binding = &c.Bindings[i]
		}
	}
	if binding == nil {
		return fmt.Errorf("cabin: berth level has no binding")
	}
	for _, e := range l.Edges {
		if e.ID != c.BerthEdgeID || e.From != c.BerthLevelID {
			continue
		}
		exitX, exitY := (float64(e.Exit.X)+.5)*cell, (float64(e.Exit.Y)+.5)*cell
		best, distance := "", math.Inf(1)
		for _, m := range c.Modules {
			x, z, yaw := c.berthedPose(m.OffsetM, *binding)
			// Along/across the car at the berth; the door is on the car's long side.
			dx, dz := exitX-x, exitY-z
			along := math.Abs(dx*math.Sin(yaw) + dz*math.Cos(yaw))
			if along <= m.LengthM/2 && along < distance {
				best, distance = m.ID, along
			}
		}
		if best == "" {
			return fmt.Errorf("cabin: boarding exit (%d,%d) of %s is beside no car of the berthed train", e.Exit.X, e.Exit.Y, e.ID)
		}
		c.Cabin.RiderModule = best
		return nil
	}
	return fmt.Errorf("cabin: berth edge %s is missing", c.BerthEdgeID)
}

// berthedPose projects a module centre of the berthed train into the berth level (plan metres, heading in radians;
// yaw 0 points along +Z, +90 degrees along +X), with the same sampling the generated server uses.
func (c TrainMotionCatalog) berthedPose(offset float64, b TrainBinding) (x, z, yaw float64) {
	length := 0.0
	for i := 1; i < len(c.Points); i++ {
		length += math.Hypot(c.Points[i].X-c.Points[i-1].X, c.Points[i].Z-c.Points[i-1].Z)
	}
	distance := math.Mod(c.BerthDistanceM+offset, length)
	if distance < 0 {
		distance += length
	}
	var px, pz, heading float64
	for i := 1; i < len(c.Points); i++ {
		a, p := c.Points[i-1], c.Points[i]
		dx, dz := p.X-a.X, p.Z-a.Z
		segment := math.Hypot(dx, dz)
		if distance < segment || i == len(c.Points)-1 {
			t := distance / segment
			px, pz, heading = a.X+dx*t, a.Z+dz*t, math.Atan2(dx, dz)
			break
		}
		distance -= segment
	}
	r := b.YawDegrees * math.Pi / 180
	co, si := math.Cos(r), math.Sin(r)
	return b.X + co*px + si*pz, b.Z - si*px + co*pz, heading + r
}
