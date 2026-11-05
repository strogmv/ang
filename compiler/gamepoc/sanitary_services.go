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
}
type SanitaryCatalog struct {
	PrefabResource     string         `json:"prefab_resource"`
	CommonEntryCell    Point          `json:"common_entry_cell"`
	CommonEntryPose    MedicalPose    `json:"common_entry_pose"`
	ServiceAvailable   bool           `json:"service_available"`
	AvailabilitySource string         `json:"availability_source"`
	ManagedAffordance  string         `json:"managed_affordance"`
	DoorSeconds        float64        `json:"door_seconds"`
	EntrySeconds       float64        `json:"entry_seconds"`
	ExitSeconds        float64        `json:"exit_seconds"`
	QueueLimit         int            `json:"queue_limit"`
	NPCHygiene         bool           `json:"npc_hygiene"`
	Units              []SanitaryUnit `json:"units"`
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
	if err = out.validate(c); err != nil {
		return nil, fmt.Errorf("sanitary_services: %w", err)
	}
	return &out, nil
}
func (c SanitaryCatalog) validate(world Compiled) error {
	if c.AvailabilitySource != "configured_cold_supply" || c.ManagedAffordance != "hygiene.market" || len(c.Units) != 7 || c.QueueLimit < 1 || c.QueueLimit > 8 {
		return fmt.Errorf("invalid sanitary policy")
	}
	for _, v := range []float64{c.DoorSeconds, c.EntrySeconds, c.ExitSeconds} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 10 {
			return fmt.Errorf("invalid sanitary physical duration")
		}
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
	if !locationSDKSurfaceResourceKey.MatchString(c.PrefabResource) || len(c.PrefabResource) > 256 || c.CommonEntryCell != (Point{X: 11, Y: 44}) || c.CommonEntryPose != (MedicalPose{X: 5.75, Y: 0, Z: 22.25, YawDegrees: 270}) {
		return fmt.Errorf("invalid common sanitary entrance/resource")
	}
	ids, spots, counts := map[string]bool{}, map[string]bool{}, map[string]int{}
	visuals := map[string]bool{}
	for _, u := range c.Units {
		provider := map[string]string{"wc": "svc.latrine_use", "basin": "svc.basin_wash", "shower": "svc.shower_3min"}[u.Kind]
		if u.ID == "" || ids[u.ID] || provider == "" || u.Provider != provider || u.NameKey == "" || u.ServiceSeconds <= 0 || u.ServiceSeconds > 3600 || u.WaterML <= 0 || u.WaterML > 100000 || u.MaxWaitSeconds <= 0 || u.MaxWaitSeconds > 3600 || u.Door != (u.Kind != "basin") {
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
