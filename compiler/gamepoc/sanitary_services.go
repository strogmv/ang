package gamepoc

import (
	"bytes"
	"cuelang.org/go/cue"
	"encoding/json"
	"fmt"
	"math"
)

type SanitaryUnit struct {
	ID             string      `json:"id"`
	InstanceID     string      `json:"instance_id"`
	VisualIndex    int         `json:"visual_index"`
	Kind           string      `json:"kind"`
	Provider       string      `json:"provider"`
	NameKey        string      `json:"name_key"`
	LevelID        string      `json:"level_id"`
	EntryCell      Point       `json:"entry_cell"`
	EntryPose      MedicalPose `json:"entry_pose"`
	InsidePose     MedicalPose `json:"inside_pose"`
	DoorMotion     string      `json:"door_motion"`
	Door           bool        `json:"door"`
	ServiceSeconds int64       `json:"service_seconds"`
	WaterML        int64       `json:"water_ml"`
	MaxWaitSeconds int64       `json:"max_wait_seconds"`
	// A private cell: the authored SDK door of its instance. Every unit behind
	// one door shares its approach, door and occupancy; waiting people stand
	// on WaitCells in front of that door.
	PortID    string  `json:"port_id,omitempty"`
	WaitCells []Point `json:"wait_cells,omitempty"`
}
type SanitaryCatalog struct {
	// The primary Units keep either the strict legacy shared-entrance topology
	// (CommonEntry present) or private cells, each entered by its own SDK door.
	Levels             map[string][]SanitaryUnit `json:"levels,omitempty"`
	PrefabResource     string                    `json:"prefab_resource"`
	CommonEntryCell    *Point                    `json:"common_entry_cell,omitempty"`
	CommonEntryPose    *MedicalPose              `json:"common_entry_pose,omitempty"`
	ServiceAvailable   bool                      `json:"service_available"`
	AvailabilitySource string                    `json:"availability_source"`
	ManagedAffordance  string                    `json:"managed_affordance"`
	DoorSeconds        float64                   `json:"door_seconds"`
	EntrySeconds       float64                   `json:"entry_seconds"`
	ExitSeconds        float64                   `json:"exit_seconds"`
	QueueLimit         int                       `json:"queue_limit"`
	// DutyMarginSeconds: a break on duty fits the brain's budget less this (baseline fixes, 21.11: the people on the way
	// and a frame of ×30 at each end of the visit; doc 09 «санблок ≤10 мин» is the whole absence).
	DutyMarginSeconds int64          `json:"duty_margin_seconds"`
	NPCHygiene        bool           `json:"npc_hygiene"`
	Units             []SanitaryUnit `json:"units"`
}

func readSanitaryServices(world cue.Value, c Compiled) (*SanitaryCatalog, error) {
	v := world.LookupPath(cue.ParsePath("sanitary_services"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var out SanitaryCatalog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Levels) != 0 {
		return nil, fmt.Errorf("sanitary levels must derive from validated SDK slots")
	}
	if err = out.validate(c); err != nil {
		return nil, fmt.Errorf("sanitary_services: %w", err)
	}
	if err := appendSDKSanitary(world, c, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (c SanitaryCatalog) validate(world Compiled) error {
	if c.AvailabilitySource != "configured_cold_supply" || c.ManagedAffordance != "hygiene.market" || len(c.Units) == 0 || len(c.Units) > 64 || c.QueueLimit < 1 || c.QueueLimit > 8 || c.DutyMarginSeconds < 0 || c.DutyMarginSeconds > 180 {
		return fmt.Errorf("invalid sanitary policy")
	}
	for _, v := range []float64{c.DoorSeconds, c.EntrySeconds, c.ExitSeconds} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 10 {
			return fmt.Errorf("invalid sanitary physical duration")
		}
	}
	if !locationSDKSurfaceResourceKey.MatchString(c.PrefabResource) || len(c.PrefabResource) > 256 {
		return fmt.Errorf("invalid common sanitary entrance/resource")
	}
	if (c.CommonEntryCell == nil) != (c.CommonEntryPose == nil) {
		return fmt.Errorf("invalid common sanitary entrance/resource")
	}
	if c.CommonEntryCell == nil {
		return validatePrivateCells(c.Units, c.Units[0].LevelID, c.QueueLimit, &world)
	}
	return c.validateSharedEntrance(world)
}

// validatePrivateCells checks units entered by their own SDK doors. A door is
// one cell: one WC, at most one basin and one shower behind it, one shared
// approach, door and short queue. Poses come from the compiled SDK door and
// stay inside the instance; nothing is accepted by coordinates alone.
func validatePrivateCells(units []SanitaryUnit, level string, queueLimit int, world *Compiled) error {
	type door struct {
		entry  Point
		visual int
		motion string
		waits  []Point
		kinds  map[string]bool
	}
	doors := map[string]*door{}
	ids, entries := map[string]bool{}, map[Point]string{}
	visuals := map[string]string{}
	for _, u := range units {
		provider := map[string]string{"wc": "svc.latrine_use", "basin": "svc.basin_wash", "shower": "svc.shower_3min"}[u.Kind]
		if u.ID == "" || ids[u.ID] || provider == "" || u.Provider != provider || u.NameKey == "" || u.ServiceSeconds <= 0 || u.ServiceSeconds > 3600 || u.WaterML <= 0 || u.WaterML > 100000 || u.MaxWaitSeconds <= 0 || u.MaxWaitSeconds > 3600 || u.LevelID != level || u.InstanceID == "" {
			return fmt.Errorf("invalid sanitary unit")
		}
		ids[u.ID] = true
		if u.PortID == "" || !u.Door || u.DoorMotion == "none" || u.DoorMotion == "" || u.VisualIndex < 0 || u.VisualIndex > 7 {
			return fmt.Errorf("private sanitary unit %s needs its own SDK door", u.ID)
		}
		key := u.InstanceID + "/" + u.PortID
		d := doors[key]
		if d == nil {
			d = &door{entry: u.EntryCell, visual: u.VisualIndex, motion: u.DoorMotion, waits: u.WaitCells, kinds: map[string]bool{}}
			doors[key] = d
			visual := fmt.Sprintf("%s/%d", u.InstanceID, u.VisualIndex)
			if visuals[visual] != "" {
				return fmt.Errorf("invalid sanitary visual binding")
			}
			visuals[visual] = key
		}
		if d.kinds[u.Kind] {
			return fmt.Errorf("sanitary cell %s repeats %s", key, u.Kind)
		}
		d.kinds[u.Kind] = true
		if d.entry != u.EntryCell || d.visual != u.VisualIndex || d.motion != u.DoorMotion || fmt.Sprint(d.waits) != fmt.Sprint(u.WaitCells) {
			return fmt.Errorf("units of sanitary cell %s disagree on door, approach or queue", key)
		}
		for _, pose := range []MedicalPose{u.EntryPose, u.InsidePose} {
			for _, v := range []float64{pose.X, pose.Y, pose.Z, pose.YawDegrees} {
				if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e5 {
					return fmt.Errorf("nonfinite sanitary pose")
				}
			}
			if pose.YawDegrees < 0 || pose.YawDegrees >= 360 {
				return fmt.Errorf("invalid sanitary orientation")
			}
		}
		if world == nil {
			continue
		}
		cell, yaw, floor, rect, _, err := sdkDoor(*world, u.LevelID, u.InstanceID, u.PortID)
		if err != nil {
			return err
		}
		const unit = .5
		if u.EntryCell != cell || math.Abs(u.EntryPose.X-(float64(cell.X)+.5)*unit) > 1e-6 || math.Abs(u.EntryPose.Z-(float64(cell.Y)+.5)*unit) > 1e-6 || u.EntryPose.Y != floor || u.EntryPose.YawDegrees != yaw {
			return fmt.Errorf("entry pose of %s is not its door's public approach", u.ID)
		}
		if u.InsidePose.Y != floor || u.InsidePose.X <= rect[0] || u.InsidePose.X >= rect[2] || u.InsidePose.Z <= rect[1] || u.InsidePose.Z >= rect[3] {
			return fmt.Errorf("inside pose of %s outside its instance", u.ID)
		}
		if distance := math.Hypot(u.EntryPose.X-u.InsidePose.X, u.EntryPose.Z-u.InsidePose.Z); distance < .5 || distance > 3 {
			return fmt.Errorf("inside pose of %s is not behind its door", u.ID)
		}
	}
	for key, d := range doors {
		if !d.kinds["wc"] {
			return fmt.Errorf("sanitary cell %s has no WC", key)
		}
		if other, ok := entries[d.entry]; ok {
			return fmt.Errorf("sanitary cells %s and %s share an approach", key, other)
		}
		entries[d.entry] = key
	}
	if world == nil {
		return nil
	}
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
		} `json:"locations"`
	}
	if err := json.Unmarshal(world.Life, &life); err != nil {
		return err
	}
	l, ok := life.Locations[level]
	if !ok {
		return fmt.Errorf("sanitary level %s has no Life location", level)
	}
	waiting := map[Point]string{}
	for key, d := range doors {
		if len(d.waits) >= queueLimit {
			return fmt.Errorf("sanitary cell %s has more wait cells than its queue", key)
		}
		previous := d.entry
		for _, p := range d.waits {
			if p.Y < 0 || p.Y >= len(l.Terrain) || p.X < 0 || p.X >= len(l.Terrain[p.Y]) || l.Terrain[p.Y][p.X] != '.' {
				return fmt.Errorf("wait cell %v of %s is not open floor", p, key)
			}
			if _, entry := entries[p]; entry || waiting[p] != "" {
				return fmt.Errorf("wait cell %v of %s blocks another door", p, key)
			}
			if step := (p.X-previous.X)*(p.X-previous.X) + (p.Y-previous.Y)*(p.Y-previous.Y); step != 1 {
				return fmt.Errorf("wait cells of %s do not form a line from its door", key)
			}
			waiting[p] = key
			previous = p
		}
	}
	return nil
}

func (c SanitaryCatalog) validateSharedEntrance(world Compiled) error {
	if len(c.Units) != 7 {
		return fmt.Errorf("invalid sanitary policy")
	}
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
			CellMM  int      `json:"cell_mm"`
		} `json:"locations"`
	}
	if err := json.Unmarshal(world.Life, &life); err != nil {
		return err
	}
	if *c.CommonEntryCell != (Point{X: 11, Y: 44}) || *c.CommonEntryPose != (MedicalPose{X: 5.75, Y: 0, Z: 22.25, YawDegrees: 270}) {
		return fmt.Errorf("invalid common sanitary entrance/resource")
	}
	ids, spots, counts := map[string]bool{}, map[string]bool{}, map[string]int{}
	visuals := map[string]bool{}
	for _, u := range c.Units {
		provider := map[string]string{"wc": "svc.latrine_use", "basin": "svc.basin_wash", "shower": "svc.shower_3min"}[u.Kind]
		if u.ID == "" || ids[u.ID] || provider == "" || u.Provider != provider || u.NameKey == "" || u.ServiceSeconds <= 0 || u.ServiceSeconds > 3600 || u.WaterML <= 0 || u.WaterML > 100000 || u.MaxWaitSeconds <= 0 || u.MaxWaitSeconds > 3600 || u.Door != (u.Kind != "basin") || u.PortID != "" || len(u.WaitCells) != 0 {
			return fmt.Errorf("invalid sanitary unit")
		}
		visual := fmt.Sprintf("%s/%d", u.InstanceID, u.VisualIndex)
		if visuals[visual] || u.Kind == "wc" && (u.VisualIndex < 0 || u.VisualIndex > 3) || u.Kind == "basin" && (u.VisualIndex < 4 || u.VisualIndex > 5) || u.Kind == "shower" && u.VisualIndex != 6 {
			return fmt.Errorf("invalid sanitary visual binding")
		}
		visuals[visual] = true
		if u.DoorMotion != map[string]string{"wc": "bifold_inward", "basin": "none", "shower": "slide_left"}[u.Kind] {
			return fmt.Errorf("invalid sanitary door motion")
		}
		ids[u.ID] = true
		counts[u.Kind]++
		if u.LevelID != "level.c7_market" || u.InstanceID != "wall.sanitary" {
			return fmt.Errorf("invalid sanitary binding")
		}
		l, ok := life.Locations[u.LevelID]
		p := u.EntryCell
		k := fmt.Sprintf("%s/%d/%d", u.LevelID, p.X, p.Y)
		if !ok || spots[k] || p.X < 0 || p.Y < 0 || p.Y >= len(l.Terrain) || p.X >= len(l.Terrain[p.Y]) || l.Terrain[p.Y][p.X] == '#' || l.Terrain[p.Y][p.X] == ' ' {
			return fmt.Errorf("invalid sanitary entry cell")
		}
		spots[k] = true
		cell := float64(l.CellMM) / 1000
		if cell <= 0 {
			cell = .5
		}
		if math.Abs(u.EntryPose.X-(float64(p.X)+.5)*cell) > 1e-6 || math.Abs(u.EntryPose.Z-(float64(p.Y)+.5)*cell) > 1e-6 {
			return fmt.Errorf("entry pose is not its navigation cell")
		}
		for _, pose := range []MedicalPose{u.EntryPose, u.InsidePose} {
			for _, v := range []float64{pose.X, pose.Y, pose.Z, pose.YawDegrees} {
				if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e5 {
					return fmt.Errorf("nonfinite sanitary pose")
				}
			}
			if pose.Y != 0 || pose.YawDegrees < 0 || pose.YawDegrees >= 360 {
				return fmt.Errorf("invalid sanitary orientation")
			}
		}
		if u.Kind == "shower" && (u.InsidePose.X != 4.6 || u.InsidePose.Z != 20.95) || u.Kind == "wc" && (u.InsidePose.X != 2.5 || u.InsidePose.Z < 23.625 || u.InsidePose.Z > 27.375) || u.Kind == "basin" && (u.InsidePose.X != 3.75 || u.InsidePose.Z < 21.8 || u.InsidePose.Z > 22.7) {
			return fmt.Errorf("inside pose outside authorized footprint")
		}
	}
	if counts["wc"] != 4 || counts["basin"] != 2 || counts["shower"] != 1 {
		return fmt.Errorf("sanitary capacity changed")
	}
	return nil
}
