package gamepoc

import "fmt"

type DemoAgentDefinition struct {
	Vectors  []string `json:"vectors"`
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	At       Point    `json:"at"`
	PatrolTo Point    `json:"patrol_to"`
}

// TestHook is the explicit scripted network call for reproducible checks; normal games keep it off.
type TestHook struct {
	Enabled       bool  `json:"enabled"`
	CallAtSeconds int64 `json:"call_at_seconds"`
}

// HeroAction is the scripted start of the emergent scenario: a sound at the hero's cell.
type HeroAction struct {
	AtSeconds    int64  `json:"at_seconds"`
	Sound        string `json:"sound"`
	WitnessLevel int    `json:"witness_level"`
}

// Monitoring names the staffed post: an NPC operator who hears alarms and reads the level's cameras.
type Monitoring struct {
	Operator     string `json:"operator"`
	CameraSource string `json:"camera_source"`
}

type DemoConfig struct {
	TargetActor              string                `json:"target_actor"`
	TestHook                 TestHook              `json:"test_hook"`
	HeroAction               HeroAction            `json:"hero_action"`
	Monitoring               Monitoring            `json:"monitoring"`
	DangerLevel              int                   `json:"danger_level"`
	ResidentPositions        map[string]Point      `json:"resident_positions"`
	ResponderRole            string                `json:"responder_role"`
	ResidentRole             string                `json:"resident_role"`
	ResponderProfile         string                `json:"responder_profile"`
	ResidentProfile          string                `json:"resident_profile"`
	KnownFacts               map[string]bool       `json:"known_facts"`
	MissionRoot              string                `json:"mission_root"`
	Medblock                 Point                 `json:"medblock"`
	InitialTargetDead        bool                  `json:"initial_target_dead"`
	Enabled                  bool                  `json:"enabled"`
	ExtraWorkAtSeconds       int64                 `json:"extra_work_at_seconds"`
	ExtraWorkDurationSeconds int64                 `json:"extra_work_duration_seconds"`
	BusyActorIDs             []string              `json:"busy_actor_ids"`
	RequiredResponders       int                   `json:"required_responders"`
	ThinkTicks               int64                 `json:"think_ticks"`
	CapturePriority          int                   `json:"capture_priority"`
	ExtraWorkPriority        int                   `json:"extra_work_priority"`
	MoveEveryTicks           int64                 `json:"move_every_ticks"`
	ReplanEveryTicks         int64                 `json:"replan_every_ticks"`
	CaptureDistance          int                   `json:"capture_distance"`
	Agents                   []DemoAgentDefinition `json:"agents"`
}

func (d DemoConfig) validate() error {
	if !d.Enabled {
		return nil
	}
	if d.MissionRoot == "" || d.ResponderRole == "" || d.ResidentRole == "" {
		return fmt.Errorf("capture demo requires mission_root")
	}
	if d.ResponderProfile == "" || d.ResidentProfile == "" {
		return fmt.Errorf("capture demo requires responder_profile and resident_profile")
	}
	if d.TestHook.CallAtSeconds < 1 || d.TestHook.CallAtSeconds > 3600 || d.HeroAction.AtSeconds < 1 || d.HeroAction.AtSeconds > 3600 || d.HeroAction.Sound == "" || d.HeroAction.WitnessLevel < 0 || d.DangerLevel < 0 || d.Monitoring.CameraSource != "level.effective_cameras" {
		return fmt.Errorf("invalid capture demo scenario start, danger level or monitoring source")
	}
	if d.ExtraWorkAtSeconds < 0 || d.ExtraWorkAtSeconds >= d.TestHook.CallAtSeconds || d.ExtraWorkAtSeconds >= d.HeroAction.AtSeconds || d.ExtraWorkDurationSeconds < 1 || d.ExtraWorkDurationSeconds > 3600 || d.RequiredResponders < 1 || d.RequiredResponders > 4 || d.ThinkTicks < 1 || d.ThinkTicks > 100 || d.MoveEveryTicks < 1 || d.ReplanEveryTicks < 1 || d.CaptureDistance != 1 || len(d.Agents) < d.RequiredResponders || len(d.Agents) > 32 || d.CapturePriority < 1 || d.ExtraWorkPriority < 1 {
		return fmt.Errorf("invalid capture demo policy")
	}
	ids := map[string]bool{}
	positions := map[Point]bool{}
	for _, a := range d.Agents {
		if a.ID == "" || a.Name == "" || ids[a.ID] || positions[a.At] || a.At == a.PatrolTo {
			return fmt.Errorf("duplicate or invalid capture demo agent")
		}
		seenVectors := map[string]bool{}
		for _, vector := range a.Vectors {
			switch vector {
			case "skin", "anal", "urethral", "muscle", "visual", "sound", "oral", "olfactory":
			default:
				return fmt.Errorf("invalid NPC vector %q", vector)
			}
			if seenVectors[vector] {
				return fmt.Errorf("duplicate NPC vector %q", vector)
			}
			seenVectors[vector] = true
		}
		ids[a.ID] = true
		positions[a.At] = true
	}
	for _, id := range d.BusyActorIDs {
		if !ids[id] {
			return fmt.Errorf("unknown extra-work actor %s", id)
		}
	}
	return nil
}
func (c Compiled) validateDemoTerrain() error {
	if !c.Config.CaptureDemo.Enabled {
		return nil
	}
	locations := append([]DemoAgentDefinition(nil), c.Config.CaptureDemo.Agents...)
	locations = append(locations, DemoAgentDefinition{ID: "demo.medblock", At: c.Config.CaptureDemo.Medblock, PatrolTo: c.Config.CaptureDemo.Medblock})
	for _, a := range locations {
		for _, p := range []Point{a.At, a.PatrolTo} {
			if p.Y < 0 || p.Y >= len(c.Terrain) || p.X < 0 || p.X >= len(c.Terrain[0]) || !c.Walkable[string(c.Terrain[p.Y][p.X])] {
				return fmt.Errorf("demo agent %s on blocked terrain", a.ID)
			}
			for _, e := range c.Entities {
				if p == e.Position {
					return fmt.Errorf("demo agent %s overlaps %s", a.ID, e.ID)
				}
			}
		}
	}
	return nil
}
