package gamepoc

import (
	"fmt"
	"math"
	"sort"
)

type SpatialLightRule struct {
	FixtureID    string  `json:"fixture_id"`
	SpacingCells int     `json:"spacing_cells"`
	RangeMeters  float64 `json:"range_m"`
	Intensity    float64 `json:"intensity"`
	Color        string  `json:"color"`
}
type SpatialLighting struct {
	Rooms                SpatialLightRule `json:"rooms"`
	Corridors            SpatialLightRule `json:"corridors"`
	Circuits             []string         `json:"circuits"`
	EmergencyEvery       int              `json:"emergency_every"`
	MaxFixtures          int              `json:"max_fixtures"`
	RequireServiceAccess bool             `json:"require_service_access"`
}
type SpatialLight struct {
	ID          string  `json:"id"`
	At          Point   `json:"at"`
	Kind        string  `json:"kind"`
	FixtureID   string  `json:"fixture_id"`
	RangeMeters float64 `json:"range_m"`
	Intensity   float64 `json:"intensity"`
	Color       string  `json:"color"`
	Emergency   bool    `json:"emergency"`
	Circuit     string  `json:"circuit"`
	ServiceAt   *Point  `json:"service_at,omitempty"`
}

func (l *SpatialLevel) compileLights(floor []bool, unique func(string, string) error) error {
	cfg := l.Lighting
	if cfg == nil {
		l.Lights = nil
		return nil
	}
	for _, r := range []SpatialLightRule{cfg.Rooms, cfg.Corridors} {
		if !spatialID(r.FixtureID) || r.SpacingCells < 1 || r.SpacingCells > 64 || math.IsNaN(r.RangeMeters) || math.IsInf(r.RangeMeters, 0) || r.RangeMeters < 1 || r.RangeMeters > 32 || math.IsNaN(r.Intensity) || math.IsInf(r.Intensity, 0) || r.Intensity <= 0 || r.Intensity > 1 || !spatialColor(r.Color) {
			return fmt.Errorf("lighting: invalid fixture rule")
		}
	}
	if cfg.MaxFixtures < 1 || cfg.MaxFixtures > 4096 || cfg.EmergencyEvery < 1 || cfg.EmergencyEvery > 64 || len(cfg.Circuits) < 2 || len(cfg.Circuits) > 8 {
		return fmt.Errorf("lighting: invalid fixture/circuit/emergency limits")
	}
	circuits := map[string]bool{}
	for _, id := range cfg.Circuits {
		if !spatialID(id) || circuits[id] {
			return fmt.Errorf("lighting: invalid/duplicate circuit %q", id)
		}
		circuits[id] = true
	}
	forbidden := make([]bool, len(floor))
	corridor := make([]bool, len(floor))
	for _, d := range l.Doors {
		forbidden[d.At.Y*l.Width+d.At.X] = true
	}
	for _, e := range l.Exits {
		forbidden[e.At.Y*l.Width+e.At.X] = true
		forbidden[e.Landing.Y*l.Width+e.Landing.X] = true
	}
	for _, c := range l.Corridors {
		for y := c.Rect.Y; y < c.Rect.Y+c.Rect.H; y++ {
			for x := c.Rect.X; x < c.Rect.X+c.Rect.W; x++ {
				corridor[y*l.Width+x] = true
			}
		}
	}
	at := func(i int) Point { return Point{X: i % l.Width, Y: i / l.Width} }
	allowed := func(i int) (*Point, bool) {
		if i < 0 || i >= len(floor) || !floor[i] || forbidden[i] {
			return nil, false
		}
		if !cfg.RequireServiceAccess {
			return nil, true
		}
		p := at(i)
		for _, n := range spatialNeighbors(p) {
			if l.contains(n) {
				j := n.Y*l.Width + n.X
				if floor[j] && !forbidden[j] {
					v := n
					return &v, true
				}
			}
		}
		return nil, false
	}
	ruleAt := func(i int) (SpatialLightRule, string) {
		if corridor[i] {
			return cfg.Corridors, "corridor"
		}
		return cfg.Rooms, "room"
	}
	covered := make([]bool, len(floor))
	placed := map[int]SpatialLight{}
	budget := 8000000
	add := func(i int) error {
		if _, ok := placed[i]; ok {
			return nil
		}
		service, ok := allowed(i)
		if !ok {
			return nil
		}
		if len(placed) >= cfg.MaxFixtures {
			return fmt.Errorf("lighting: fixture budget exhausted before coverage")
		}
		r, kind := ruleAt(i)
		p := at(i)
		placed[i] = SpatialLight{ID: fmt.Sprintf("light.%d.%d", p.X, p.Y), At: p, Kind: kind, FixtureID: r.FixtureID, RangeMeters: r.RangeMeters, Intensity: r.Intensity, Color: r.Color, ServiceAt: service}
		return l.lightFlood(floor, i, int(math.Floor(r.RangeMeters)), &budget, func(cell, distance int) bool { covered[cell] = true; return false })
	}
	// Corridor rows run along their longer axis, including both end approaches.
	ordered := append([]SpatialCorridor(nil), l.Corridors...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	for _, c := range ordered {
		horizontal := c.Rect.W >= c.Rect.H
		length := c.Rect.H
		if horizontal {
			length = c.Rect.W
		}
		for offset := 0; offset < length; offset++ {
			if offset%cfg.Corridors.SpacingCells != 0 && offset != length-1 {
				continue
			}
			x, y := c.Rect.X+c.Rect.W/2, c.Rect.Y+offset
			if horizontal {
				x, y = c.Rect.X+offset, c.Rect.Y+c.Rect.H/2
			}
			if e := add(y*l.Width + x); e != nil {
				return e
			}
		}
	}
	// Stable level-coordinate grid for rooms; blocked candidates are omitted.
	step := cfg.Rooms.SpacingCells
	for y := step / 2; y < l.Height; y += step {
		for x := step / 2; x < l.Width; x += step {
			i := y*l.Width + x
			if !corridor[i] {
				if e := add(i); e != nil {
					return e
				}
			}
		}
	}
	// Coverage completion catches room edges, corners and obstacles. Search never
	// crosses walls, and each candidate must actually reach the uncovered cell.
	radius := int(math.Floor(math.Max(cfg.Rooms.RangeMeters, cfg.Corridors.RangeMeters)))
	for i, walk := range floor {
		if !walk || covered[i] {
			continue
		}
		candidate := -1
		if _, ok := allowed(i); ok {
			candidate = i
		} else {
			e := l.lightFlood(floor, i, radius, &budget, func(cell, distance int) bool {
				_, ok := allowed(cell)
				r, _ := ruleAt(cell)
				if ok && float64(distance) <= r.RangeMeters {
					candidate = cell
					return true
				}
				return false
			})
			if e != nil {
				return e
			}
		}
		if candidate < 0 {
			return fmt.Errorf("lighting: no serviceable fixture can cover %d,%d", i%l.Width, i/l.Width)
		}
		if e := add(candidate); e != nil {
			return e
		}
		if !covered[i] {
			return fmt.Errorf("lighting: coverage failure at %d,%d", i%l.Width, i/l.Width)
		}
	}
	indices := make([]int, 0, len(placed))
	for i := range placed {
		indices = append(indices, i)
	}
	sort.Ints(indices)
	lights := make([]SpatialLight, 0, len(indices))
	for ordinal, i := range indices {
		v := placed[i]
		v.Circuit = cfg.Circuits[ordinal%len(cfg.Circuits)]
		v.Emergency = ordinal%cfg.EmergencyEvery == 0
		if e := unique("light", v.ID); e != nil {
			return e
		}
		lights = append(lights, v)
	}
	l.Lights = lights
	return nil
}
func spatialNeighbors(p Point) [4]Point {
	return [4]Point{{X: p.X, Y: p.Y - 1}, {X: p.X + 1, Y: p.Y}, {X: p.X, Y: p.Y + 1}, {X: p.X - 1, Y: p.Y}}
}
func (l *SpatialLevel) lightFlood(floor []bool, start, radius int, budget *int, visit func(int, int) bool) error {
	type entry struct{ cell, distance int }
	queue := []entry{{start, 0}}
	seen := map[int]bool{start: true}
	for head := 0; head < len(queue); head++ {
		*budget--
		if *budget < 0 {
			return fmt.Errorf("lighting: search budget exceeded")
		}
		e := queue[head]
		if visit(e.cell, e.distance) {
			return nil
		}
		if e.distance >= radius {
			continue
		}
		p := Point{X: e.cell % l.Width, Y: e.cell / l.Width}
		for _, n := range spatialNeighbors(p) {
			if !l.contains(n) {
				continue
			}
			i := n.Y*l.Width + n.X
			if floor[i] && !seen[i] {
				seen[i] = true
				queue = append(queue, entry{i, e.distance + 1})
			}
		}
	}
	return nil
}
