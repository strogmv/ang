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

func sdkServiceSlots(world cue.Value, kind string) ([]SDKServiceSlot, error) {
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
	doors := map[string]bool{}
	for _, s := range slots {
		if s.ID == "" || ids[s.ID] || (s.Kind != "wc" && s.Kind != "medical") || s.Reason == "" {
			return nil, fmt.Errorf("invalid/duplicate SDK service slot")
		}
		ids[s.ID] = true
		key := s.LevelID + "/" + s.InstanceID + "/" + s.PortID
		if doors[key] {
			return nil, fmt.Errorf("duplicate SDK service door")
		}
		doors[key] = true
		if s.Kind == kind {
			out = append(out, s)
		}
	}
	return out, nil
}
func sdkServicePoses(c Compiled, s SDKServiceSlot) (Point, MedicalPose, MedicalPose, error) {
	var level *locationsdk.RuntimeLevel
	for _, b := range c.ModularLocations {
		if b.LevelID == s.LevelID && c.LocationSDK != nil {
			for i := range c.LocationSDK.Levels {
				l := &c.LocationSDK.Levels[i]
				if l.ID == b.LayoutID {
					level = l
				}
			}
		}
	}
	fail := func() (Point, MedicalPose, MedicalPose, error) {
		return Point{}, MedicalPose{}, MedicalPose{}, fmt.Errorf("SDK service %s needs an authored bound door, clear approach and inside pose", s.ID)
	}
	if level == nil || level.UnitMM != 500 || s.VisualIndex < 0 || s.VisualIndex > 3 || s.InsideYaw < 0 || s.InsideYaw >= 360 {
		return fail()
	}
	var instance *locationsdk.CompiledInstance
	for i := range level.Instances {
		if level.Instances[i].ID == s.InstanceID {
			instance = &level.Instances[i]
		}
	}
	if instance == nil {
		return fail()
	}
	var port *locationsdk.CompiledPort
	for i := range level.Ports {
		p := &level.Ports[i]
		if p.ID == s.PortID && p.InstanceID == s.InstanceID && p.Kind == "door" {
			port = p
		}
	}
	if port == nil || len(port.Opening) == 0 || len(port.Approach) == 0 {
		return fail()
	}
	cell := port.Approach[len(port.Approach)-1]
	if cell.Y < 0 || cell.Y >= len(level.Terrain) || cell.X < 0 || cell.X >= len(level.Terrain[cell.Y]) || level.Terrain[cell.Y][cell.X] != '.' {
		return fail()
	}
	unit := float64(level.UnitMM) / 1000
	floor := float64(level.Elevation) * unit
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
	yaw := map[locationsdk.Side]float64{"north": 0, "east": 270, "south": 180, "west": 90}[port.Side]
	entry := MedicalPose{X: (float64(cell.X) + .5) * unit, Y: floor, Z: (float64(cell.Y) + .5) * unit, YawDegrees: yaw}
	inner := MedicalPose{X: inside[0], Y: inside[1], Z: -inside[2], YawDegrees: math.Mod(s.InsideYaw-float64(instance.Rotation)+360, 360)}
	if distance := math.Hypot(entry.X-inner.X, entry.Z-inner.Z); distance < .5 || distance > 3 {
		return fail()
	}
	return Point{X: cell.X, Y: cell.Y}, entry, inner, nil
}
func appendSDKSanitary(world cue.Value, c Compiled, out *SanitaryCatalog) error {
	slots, err := sdkServiceSlots(world, "wc")
	if err != nil {
		return err
	}
	original := append([]SanitaryUnit(nil), out.Units...)
	ids := map[string]bool{}
	doors := map[string]bool{}
	for _, u := range original {
		ids[u.ID] = true
	}
	for _, s := range slots {
		if len(original) == 0 || s.LevelID == original[0].LevelID {
			return fmt.Errorf("SDK sanitary installation must be separate from the validated primary topology")
		}
		if ids[s.ID] || doors[s.LevelID+"/"+s.InstanceID+"/"+s.PortID] {
			return fmt.Errorf("duplicate SDK sanitary door")
		}
		var donor *SanitaryUnit
		for i := range original {
			if original[i].ID == s.SourceUnit && original[i].Kind == "wc" {
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
		u.DoorMotion = "telescopic_pair"
		if out.Levels == nil {
			out.Levels = map[string][]SanitaryUnit{}
		}
		out.Levels[s.LevelID] = append(out.Levels[s.LevelID], u)
		ids[s.ID] = true
		doors[s.LevelID+"/"+s.InstanceID+"/"+s.PortID] = true
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
