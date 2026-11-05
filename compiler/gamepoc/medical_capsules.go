package gamepoc

import (
	"bytes"
	"cuelang.org/go/cue"
	"encoding/json"
	"fmt"
	"math"
)

type MedicalPose struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Z          float64 `json:"z"`
	YawDegrees float64 `json:"yaw_degrees"`
}
type MedicalUnit struct {
	VisualIndex int         `json:"visual_index"`
	ID          string      `json:"id"`
	LevelID     string      `json:"level_id"`
	InstanceID  string      `json:"instance_id"`
	Provider    string      `json:"provider"`
	NameKey     string      `json:"name_key"`
	ScanCell    Point       `json:"scan_cell"`
	ScanPose    MedicalPose `json:"scan_pose"`
	InsidePose  MedicalPose `json:"inside_pose"`
}
type MedicalCapsuleCatalog struct {
	PrefabResource  string        `json:"prefab_resource"`
	DoorSeconds     float64       `json:"door_seconds"`
	ReceiverAccount string        `json:"receiver_account"`
	Currency        string        `json:"currency"`
	ScanSeconds     float64       `json:"scan_seconds"`
	EntrySeconds    float64       `json:"entry_seconds"`
	ExitSeconds     float64       `json:"exit_seconds"`
	LeaseSeconds    int64         `json:"lease_seconds"`
	QueueLimit      int           `json:"queue_limit"`
	NPCSelfCare     bool          `json:"npc_self_care"`
	Units           []MedicalUnit `json:"units"`
}

func readMedicalCapsules(world cue.Value, c Compiled) (*MedicalCapsuleCatalog, error) {
	v := world.LookupPath(cue.ParsePath("medical_capsules"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var out MedicalCapsuleCatalog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("medical_capsules: %w", err)
	}
	if err := out.validate(c); err != nil {
		return nil, fmt.Errorf("medical_capsules: %w", err)
	}
	return &out, nil
}
func (c MedicalCapsuleCatalog) validate(world Compiled) error {
	finite := func(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) && math.Abs(x) < 1e6 }
	if len(c.PrefabResource) > 256 || !locationSDKSurfaceResourceKey.MatchString(c.PrefabResource) {
		return fmt.Errorf("invalid medical prefab Resources path")
	}
	if c.DoorSeconds > 2 {
		return fmt.Errorf("invalid door duration")
	}
	if c.Currency != "CRD" || len(c.Units) != 2 || c.QueueLimit < 1 || c.QueueLimit > 8 || c.LeaseSeconds < 10 || c.LeaseSeconds > 300 {
		return fmt.Errorf("invalid medical service policy")
	}
	for _, x := range []float64{c.ScanSeconds, c.EntrySeconds, c.ExitSeconds, c.DoorSeconds} {
		if !finite(x) || x <= 0 || x > 10 {
			return fmt.Errorf("invalid physical phase duration")
		}
	}
	receiver, ok := world.Inventory.Accounts[c.ReceiverAccount]
	if !ok || receiver.Currency != c.Currency {
		return fmt.Errorf("unknown CRD receiver account")
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
	ids, spots := map[string]bool{}, map[string]bool{}
	for i, u := range c.Units {
		if u.LevelID != "level.c7_market" || u.ScanCell.X != 9 || u.ScanCell.Y != 66+2*i || u.ScanPose.Y != 0 || u.ScanPose.YawDegrees != 270 || u.InsidePose.X != 3 || u.InsidePose.Y != 0 || u.InsidePose.Z != 33.25+float64(i) || u.InsidePose.YawDegrees != 90 {
			return fmt.Errorf("medical pose outside authorized pair geometry")
		}
		if u.VisualIndex != i {
			return fmt.Errorf("invalid visual unit order")
		}
		if u.ID == "" || ids[u.ID] || u.InstanceID != "wall.med.2" || u.Provider != "capsule" || u.NameKey == "" {
			return fmt.Errorf("invalid or duplicate capsule / forbidden intake replacement")
		}
		ids[u.ID] = true
		l, ok := life.Locations[u.LevelID]
		p := u.ScanCell
		key := fmt.Sprintf("%s/%d/%d", u.LevelID, p.X, p.Y)
		if !ok || spots[key] || p.X < 0 || p.Y < 0 || p.Y >= len(l.Terrain) || p.X >= len(l.Terrain[p.Y]) || l.Terrain[p.Y][p.X] == '#' || l.Terrain[p.Y][p.X] == ' ' {
			return fmt.Errorf("scan cell must be a distinct floor cell")
		}
		spots[key] = true
		cell := float64(l.CellMM) / 1000
		if cell <= 0 {
			cell = .5
		}
		if math.Abs(u.ScanPose.X-(float64(p.X)+.5)*cell) > 1e-6 || math.Abs(u.ScanPose.Z-(float64(p.Y)+.5)*cell) > 1e-6 {
			return fmt.Errorf("scan pose disagrees with navigation cell centre")
		}
		for _, p := range []MedicalPose{u.ScanPose, u.InsidePose} {
			if !finite(p.X) || !finite(p.Y) || !finite(p.Z) || !finite(p.YawDegrees) || p.YawDegrees < 0 || p.YawDegrees >= 360 {
				return fmt.Errorf("invalid medical pose")
			}
		}
		if d := math.Hypot(u.ScanPose.X-u.InsidePose.X, u.ScanPose.Z-u.InsidePose.Z); d < .5 || d > 2 {
			return fmt.Errorf("invalid entry stroke")
		}
		if u.InsidePose.X < 1.5 || u.InsidePose.X > 3.5 || u.InsidePose.Z < 32.5 || u.InsidePose.Z > 35 {
			return fmt.Errorf("patient outside authorized medical niche")
		}
	}
	return nil
}
