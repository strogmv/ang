package gamepoc

import (
	"fmt"
	"math"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

// SDKServiceSlot adds a provider at an authored SDK door. Existing service
// policies are inherited by source ID; prices, durations and market guards are
// preserved. It cannot create geometry or turn an arbitrary cell into a door.
type SDKServiceSlot struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"`
	LevelID      string         `json:"level_id"`
	InstanceID   string         `json:"instance_id"`
	PortID       string         `json:"port_id"`
	SourceUnit   string         `json:"source_unit"`
	InsideLocalM placement.Vec3 `json:"inside_local_m"`
	InsideYaw    float64        `json:"inside_yaw_degrees"`
	VisualIndex  int            `json:"visual_index"`
	Reason       string         `json:"reason"`
}

func sdkServiceSlots(world cue.Value, kinds ...string) ([]SDKServiceSlot, error) {
	v := world.LookupPath(cue.ParsePath("sdk_service_slots"))
	if !v.Exists() {
		return nil, nil
	}
	var slots []SDKServiceSlot
	if err := v.Decode(&slots); err != nil {
		return nil, err
	}
	var out []SDKServiceSlot
	ids := map[string]bool{}
	// One door is one occupancy: a private sanitary cell may host one WC, one
	// basin and one shower behind it, never a second family or a repeated kind.
	doors := map[string]map[string]bool{}
	for _, s := range slots {
		family := map[string]string{"wc": "sanitary", "basin": "sanitary", "shower": "sanitary", "medical": "medical"}[s.Kind]
		if s.ID == "" || ids[s.ID] || family == "" || s.Reason == "" {
			return nil, fmt.Errorf("invalid/duplicate SDK service slot")
		}
		ids[s.ID] = true
		key := s.LevelID + "/" + s.InstanceID + "/" + s.PortID
		held := doors[key]
		if held == nil {
			held = map[string]bool{}
			doors[key] = held
		}
		if held[s.Kind] || held["medical"] || family == "medical" && len(held) > 0 {
			return nil, fmt.Errorf("duplicate SDK service door")
		}
		held[s.Kind] = true
		for _, kind := range kinds {
			if s.Kind == kind {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

// sdkDoor resolves an authored SDK door of a modular level: its public
// approach cell, the walking yaw toward the door, the floor height and the
// instance's metric plan rectangle [x0, z0, x1, z1].
func sdkDoor(c Compiled, levelID, instanceID, portID string) (Point, float64, float64, [4]float64, *locationsdk.CompiledInstance, error) {
	var level *locationsdk.RuntimeLevel
	for _, b := range c.ModularLocations {
		if b.LevelID == levelID && c.LocationSDK != nil {
			for i := range c.LocationSDK.Levels {
				l := &c.LocationSDK.Levels[i]
				if l.ID == b.LayoutID {
					level = l
				}
			}
		}
	}
	fail := func() (Point, float64, float64, [4]float64, *locationsdk.CompiledInstance, error) {
		return Point{}, 0, 0, [4]float64{}, nil, fmt.Errorf("%s/%s/%s is not an authored bound door with a clear approach", levelID, instanceID, portID)
	}
	if level == nil || level.UnitMM != 500 {
		return fail()
	}
	var instance *locationsdk.CompiledInstance
	for i := range level.Instances {
		if level.Instances[i].ID == instanceID {
			instance = &level.Instances[i]
		}
	}
	if instance == nil {
		return fail()
	}
	var port *locationsdk.CompiledPort
	for i := range level.Ports {
		p := &level.Ports[i]
		if p.ID == portID && p.InstanceID == instanceID && p.Kind == "door" {
			port = p
		}
	}
	if port == nil || len(port.Opening) == 0 || len(port.Approach) == 0 {
		return fail()
	}
	cell, ok := locationsdk.PublicServiceApproach(*port)
	if !ok {
		return fail()
	}
	if cell.Y < 0 || cell.Y >= len(level.Terrain) || cell.X < 0 || cell.X >= len(level.Terrain[cell.Y]) || level.Terrain[cell.Y][cell.X] != '.' {
		return fail()
	}
	unit := float64(level.UnitMM) / 1000
	w, d := float64(instance.Width)*unit, float64(instance.Depth)*unit
	if instance.Rotation == 90 || instance.Rotation == 270 {
		w, d = d, w
	}
	x0, z0 := float64(instance.At.X)*unit, float64(instance.At.Y)*unit
	yaw := map[locationsdk.Side]float64{"north": 0, "east": 270, "south": 180, "west": 90}[port.Side]
	return Point{X: cell.X, Y: cell.Y}, yaw, float64(level.Elevation) * unit, [4]float64{x0, z0, x0 + w, z0 + d}, instance, nil
}

func sdkServicePoses(c Compiled, s SDKServiceSlot) (Point, MedicalPose, MedicalPose, error) {
	fail := func() (Point, MedicalPose, MedicalPose, error) {
		return Point{}, MedicalPose{}, MedicalPose{}, fmt.Errorf("SDK service %s needs an authored bound door, clear approach and inside pose", s.ID)
	}
	if s.VisualIndex < 0 || s.VisualIndex > 3 || s.InsideYaw < 0 || s.InsideYaw >= 360 {
		return fail()
	}
	cell, yaw, floor, _, instance, err := sdkDoor(c, s.LevelID, s.InstanceID, s.PortID)
	if err != nil {
		return fail()
	}
	unit := float64(locationsdk.UnitMM) / 1000
	pose := placement.Pose(instance.At, instance.Width, instance.Depth, floor, instance.Rotation)
	local := s.InsideLocalM
	for _, n := range local {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return fail()
		}
	}
	w, d := float64(instance.Width)*unit, float64(instance.Depth)*unit
	if instance.Rotation == 90 || instance.Rotation == 270 {
		w, d = d, w
	}
	if local[0] < 0 || local[0] > w || local[2] < 0 || local[2] > d || math.Abs(local[1]) > .1 {
		return fail()
	}
	basis := pose.BasisColumns()
	inside := pose.Position
	for j := range 3 {
		for k := range 3 {
			inside[j] += basis[k][j] * local[k]
		}
	}
	entry := MedicalPose{X: (float64(cell.X) + .5) * unit, Y: floor, Z: (float64(cell.Y) + .5) * unit, YawDegrees: yaw}
	inner := MedicalPose{X: inside[0], Y: inside[1], Z: -inside[2], YawDegrees: math.Mod(s.InsideYaw-float64(instance.Rotation)+360, 360)}
	if distance := math.Hypot(entry.X-inner.X, entry.Z-inner.Z); distance < .5 || distance > 3 {
		return fail()
	}
	return cell, entry, inner, nil
}
func appendSDKSanitary(world cue.Value, c Compiled, out *SanitaryCatalog) error {
	slots, err := sdkServiceSlots(world, "wc", "basin", "shower")
	if err != nil {
		return err
	}
	original := append([]SanitaryUnit(nil), out.Units...)
	ids := map[string]bool{}
	for _, u := range original {
		ids[u.ID] = true
	}
	for _, s := range slots {
		if len(original) == 0 || s.LevelID == original[0].LevelID {
			return fmt.Errorf("SDK sanitary installation must be separate from the validated primary topology")
		}
		if ids[s.ID] {
			return fmt.Errorf("duplicate SDK sanitary door")
		}
		var donor *SanitaryUnit
		for i := range original {
			if original[i].ID == s.SourceUnit && original[i].Kind == s.Kind {
				donor = &original[i]
			}
		}
		if donor == nil {
			return fmt.Errorf("unknown SDK sanitary provider %s", s.SourceUnit)
		}
		cell, entry, inside, err := sdkServicePoses(c, s)
		if err != nil {
			return err
		}
		u := *donor
		u.ID = s.ID
		u.LevelID = s.LevelID
		u.InstanceID = s.InstanceID
		u.VisualIndex = s.VisualIndex
		u.EntryCell = cell
		u.EntryPose = entry
		u.InsidePose = inside
		u.Door = true
		u.DoorMotion = "telescopic_pair"
		// Every slot behind one SDK door shares that cell's door and occupancy.
		u.PortID = s.PortID
		u.WaitCells = nil
		if out.Levels == nil {
			out.Levels = map[string][]SanitaryUnit{}
		}
		out.Levels[s.LevelID] = append(out.Levels[s.LevelID], u)
		ids[s.ID] = true
	}
	for level, units := range out.Levels {
		if err := validatePrivateCells(units, level, out.QueueLimit, nil); err != nil {
			return fmt.Errorf("sanitary %s: %w", level, err)
		}
	}
	return nil
}
func appendSDKMedical(world cue.Value, c Compiled, out *MedicalCapsuleCatalog) error {
	slots, err := sdkServiceSlots(world, "medical")
	if err != nil {
		return err
	}
	original := append([]MedicalUnit(nil), out.Units...)
	ids := map[string]bool{}
	doors := map[string]bool{}
	for _, u := range original {
		ids[u.ID] = true
	}
	for _, s := range slots {
		if len(original) == 0 || s.LevelID == original[0].LevelID {
			return fmt.Errorf("SDK medical installation must be separate from the validated primary topology")
		}
		if ids[s.ID] || doors[s.LevelID+"/"+s.InstanceID+"/"+s.PortID] {
			return fmt.Errorf("duplicate SDK medical door")
		}
		var donor *MedicalUnit
		for i := range original {
			if original[i].ID == s.SourceUnit {
				donor = &original[i]
			}
		}
		if donor == nil {
			return fmt.Errorf("unknown SDK medical provider %s", s.SourceUnit)
		}
		cell, entry, inside, err := sdkServicePoses(c, s)
		if err != nil {
			return err
		}
		u := *donor
		u.ID = s.ID
		u.LevelID = s.LevelID
		u.InstanceID = s.InstanceID
		u.VisualIndex = s.VisualIndex
		u.ScanCell = cell
		u.ScanPose = entry
		u.InsidePose = inside
		if out.Levels == nil {
			out.Levels = map[string][]MedicalUnit{}
		}
		out.Levels[s.LevelID] = append(out.Levels[s.LevelID], u)
		ids[s.ID] = true
		doors[s.LevelID+"/"+s.InstanceID+"/"+s.PortID] = true
	}
	return nil
}
