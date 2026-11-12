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
	// Derived, never authored: the module whose berthed envelope holds the berth edge's exit cell.
	RiderModule string `json:"rider_module,omitempty"`
}

// A place inside one car: metres from the car centre, x along the car, y across (+ toward the side of the
// authored "far" door leaves). Seated places face the aisle; standing ones face along the car.
type TrainCabinSpot struct {
	XM float64 `json:"x_m"`
	YM float64 `json:"y_m"`
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
}

type TrainCabinCrowdLook struct {
	Look  string `json:"look"`
	Share int    `json:"share"`
}

var trainCabinColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
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
