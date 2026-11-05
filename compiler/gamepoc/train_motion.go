package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"

	"cuelang.org/go/cue"
)

// TrainMotionCatalog is independent of the legacy C7 passenger timetable.
// Distances are metres on one closed, ordered path; bindings only project it.
type TrainMotionCatalog struct {
	PrefabResource   string            `json:"prefab_resource"`
	ClockDomain      string            `json:"clock_domain"`
	TrainID          string            `json:"train_id"`
	StockID          string            `json:"stock_id"`
	RouteID          string            `json:"route_id"`
	RouteVersion     int               `json:"route_version"`
	EpochSeconds     int64             `json:"epoch_seconds"`
	InitialDistanceM float64           `json:"initial_distance_m"`
	SpeedMPS         float64           `json:"speed_mps"`
	DwellSeconds     float64           `json:"dwell_seconds"`
	Points           []TrainRoutePoint `json:"points"`
	Modules          []TrainModule     `json:"modules"`
	Bindings         []TrainBinding    `json:"bindings"`
}
type TrainRoutePoint struct {
	X float64 `json:"x"`
	Z float64 `json:"z"`
}
type TrainModule struct {
	ID            string    `json:"id"`
	OffsetM       float64   `json:"offset_m"`
	LengthM       float64   `json:"length_m"`
	BogieYM       float64   `json:"bogie_y_m"`
	BogieOffsetsM []float64 `json:"bogie_offsets_m"`
}
type TrainBinding struct {
	PassageID  string  `json:"passage_id,omitempty"`
	LevelID    string  `json:"level_id"`
	StartM     float64 `json:"start_m"`
	EndM       float64 `json:"end_m"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Z          float64 `json:"z"`
	YawDegrees float64 `json:"yaw_degrees"`
	// All v1 bindings are exterior pass-throughs, not boarding or cargo interfaces.
	EntryAllowed bool `json:"entry_allowed"`
}

func readTrainMotion(world cue.Value, life json.RawMessage) (*TrainMotionCatalog, error) {
	v := world.LookupPath(cue.ParsePath("train_motion"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.train_motion: %w", err)
	}
	var c TrainMotionCatalog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return nil, fmt.Errorf("world.train_motion: %w", err)
	}
	if err = c.validate(life); err != nil {
		return nil, fmt.Errorf("world.train_motion: %w", err)
	}
	return &c, nil
}
func (c TrainMotionCatalog) validate(life json.RawMessage) error {
	if len(c.PrefabResource) > 256 || !locationSDKSurfaceResourceKey.MatchString(c.PrefabResource) {
		return fmt.Errorf("unsafe extensionless train prefab Resources path")
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= 1e6 }
	if c.ClockDomain != "simulation" || c.TrainID == "" || c.StockID == "" || c.RouteID == "" || c.RouteVersion < 1 || c.EpochSeconds < 0 || !finite(c.SpeedMPS) || c.SpeedMPS <= 0 || c.SpeedMPS > 30 || !finite(c.DwellSeconds) || c.DwellSeconds < 0 || c.DwellSeconds > 3600 {
		return fmt.Errorf("invalid identity, epoch or motion")
	}
	if len(c.Points) < 4 || len(c.Points) > 256 || c.Points[0] != c.Points[len(c.Points)-1] {
		return fmt.Errorf("route must be a closed ordered metric path")
	}
	length := 0.0
	cuts := []float64{0}
	for i, p := range c.Points {
		if !finite(p.X) || !finite(p.Z) {
			return fmt.Errorf("non-finite route point")
		}
		if i > 0 {
			q := c.Points[i-1]
			d := math.Hypot(p.X-q.X, p.Z-q.Z)
			if d < 1 {
				return fmt.Errorf("degenerate route segment")
			}
			length += d
			cuts = append(cuts, length)
		}
	}
	if length > 1e6 || len(c.Modules) == 0 || len(c.Modules) > 16 {
		return fmt.Errorf("invalid route length or modules")
	}
	ids := map[string]bool{}
	tail := 0.0
	for i, m := range c.Modules {
		if m.ID == "" || ids[m.ID] || !finite(m.OffsetM) || m.OffsetM > 0 || !finite(m.LengthM) || m.LengthM <= 0 || m.LengthM > 100 || len(m.BogieOffsetsM) != 2 || !finite(m.BogieYM) {
			return fmt.Errorf("invalid or duplicate module")
		}
		if i == 0 && m.OffsetM != 0 {
			return fmt.Errorf("leading module must define distance zero")
		}
		if i > 0 {
			previous := c.Modules[i-1]
			if previous.OffsetM-m.OffsetM < (previous.LengthM+m.LengthM)/2 {
				return fmt.Errorf("modules overlap or are out of order")
			}
		}
		ids[m.ID] = true
		for _, b := range m.BogieOffsetsM {
			if !finite(b) || math.Abs(b) >= m.LengthM/2 {
				return fmt.Errorf("bogie outside module")
			}
		}
		if m.BogieOffsetsM[0] <= m.BogieOffsetsM[1] {
			return fmt.Errorf("bogies must be ordered front to rear")
		}
		tail = math.Max(tail, -m.OffsetM+m.LengthM/2)
	}
	if !finite(c.InitialDistanceM) || c.InitialDistanceM < 0 || c.InitialDistanceM >= length {
		return fmt.Errorf("invalid initial route distance")
	}
	if tail*3 >= length {
		return fmt.Errorf("train is too long for route")
	}
	var l struct {
		Locations map[string]json.RawMessage `json:"locations"`
	}
	if err := json.Unmarshal(life, &l); err != nil {
		return fmt.Errorf("Life locations required: %w", err)
	}
	levels := map[string]bool{}
	for i, b := range c.Bindings {
		if b.LevelID == "" || levels[b.LevelID] || l.Locations[b.LevelID] == nil || b.EntryAllowed {
			return fmt.Errorf("binding must reference distinct existing exterior level")
		}
		if i > 0 && c.Bindings[i-1].EndM > b.StartM {
			return fmt.Errorf("bindings must follow the ordered route without overlap")
		}
		levels[b.LevelID] = true
		if !finite(b.StartM) || !finite(b.EndM) || b.StartM < 0 || b.EndM <= b.StartM || b.EndM > length || b.EndM-b.StartM < tail || !finite(b.X) || !finite(b.Y) || !finite(b.Z) || !finite(b.YawDegrees) {
			return fmt.Errorf("invalid metric binding")
		}
		// V1 exterior views cannot contain an unengineered sharp corner.
		for _, cut := range cuts {
			if cut > b.StartM && cut < b.EndM {
				return fmt.Errorf("visible binding crosses route corner")
			}
		}
	}
	if len(levels) < 3 {
		return fmt.Errorf("at least three distinct level bindings required")
	}
	return nil
}

// Painted bindings name their own rail aperture; a berth is never an exterior
// motion target. Native SDK bindings have no painted passage identifier.
func (c TrainMotionCatalog) validatePainted(scenes []PaintedScene) error {
	bound := map[string]bool{}
	for _, b := range c.Bindings {
		var scene *PaintedScene
		for i := range scenes {
			if scenes[i].LevelID == b.LevelID {
				scene = &scenes[i]
				break
			}
		}
		if scene == nil {
			if b.PassageID != "" {
				return fmt.Errorf("native binding %s cannot name a painted passage", b.LevelID)
			}
			continue
		}
		count := 0
		for _, p := range scene.AmbientPassages {
			if p.ID != b.PassageID {
				continue
			}
			count++
			if p.MetricRail == nil || p.StopProgress != 0 || p.DwellSeconds != 0 || len(p.BerthDoors) > 0 {
				return fmt.Errorf("train binding %s must target a metric pass-through, never passenger berth", b.LevelID)
			}
			if scene.FloorElevationM == nil || math.Abs(*scene.FloorElevationM+b.Y-p.ElevationM) > .001 {
				return fmt.Errorf("train binding %s has inconsistent relative floor elevation", b.LevelID)
			}
		}
		if b.PassageID == "" || count != 1 {
			return fmt.Errorf("train binding %s requires one exact existing passage_id", b.LevelID)
		}
		bound[b.LevelID+"/"+b.PassageID] = true
	}
	for _, s := range scenes {
		for _, p := range s.AmbientPassages {
			if p.MotionSource == "train_motion" && !bound[s.LevelID+"/"+p.ID] {
				return fmt.Errorf("external rail %s/%s has no train owner", s.LevelID, p.ID)
			}
		}
	}
	return nil
}
