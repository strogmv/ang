package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"cuelang.org/go/cue"
)

// EdgeFog is world.edge_fog (edge fog v1, owner 16.11: «чёрный туман или размытие локаций это для всей игры правило, что бы
// некрасивые обрезки скрыть»): every level fades softly into black where the space the hero can reach ends — exits, the rail
// canyon, drops below the deck, the void outside. A rule of the whole game: every level has it (the shared numbers here, a
// level may override them with a reason, never switch it off). The edge is the level's own data, measured by the client
// (client-godot/edge_fog.gd) over the level's plan: the distance from its floor (the floor cells of every walkable surface),
// the distance from where its walkable floor runs out over the level's bound (an exit: the floor fades to black before the
// bound and everything beyond it is black), and the depth under its lowest walkable floor. One screen term darkens
// everything by them (the painted look and plain 3D, live parts, trains, people). Presentation only: sight, light and
// movement are untouched.
type EdgeFog struct {
	EdgeFogNumbers
	Levels []EdgeFogLevel `json:"levels"`
}

// EdgeFogNumbers: StartM, WidthM — from this distance (plan metres) beyond the floor the fog begins, and over this width it
// reaches full black; ExitM — over how many metres before the level's bound the floor that runs out of the level (an exit)
// fades to full black; DepthStartM, DepthM — where the fog begins and where it is full under the lowest walkable floor
// (metres down); Color — the black's colour (#RRGGBB, near-black and warm, from the key's palette).
type EdgeFogNumbers struct {
	StartM      float64 `json:"start_m"`
	WidthM      float64 `json:"width_m"`
	ExitM       float64 `json:"exit_m"`
	DepthStartM float64 `json:"depth_start_m"`
	DepthM      float64 `json:"depth_m"`
	Color       string  `json:"color"`
}

// EdgeFogLevel: a level's own numbers (any of them), with the reason.
type EdgeFogLevel struct {
	LevelID     string   `json:"level_id"`
	Why         string   `json:"why"`
	StartM      *float64 `json:"start_m,omitempty"`
	WidthM      *float64 `json:"width_m,omitempty"`
	ExitM       *float64 `json:"exit_m,omitempty"`
	DepthStartM *float64 `json:"depth_start_m,omitempty"`
	DepthM      *float64 `json:"depth_m,omitempty"`
	Color       *string  `json:"color,omitempty"`
}

// readEdgeFog reads world.edge_fog. A world with levels must have it (an unfogged level shows its cut edges against
// black), and every level must have floor for the edge to be measured from.
func readEdgeFog(world cue.Value, c Compiled) (*EdgeFog, error) {
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
		} `json:"locations"`
	}
	if len(c.Life) > 0 {
		if err := json.Unmarshal(c.Life, &life); err != nil {
			return nil, err
		}
	}
	v := world.LookupPath(cue.ParsePath("edge_fog"))
	if !v.Exists() {
		if len(life.Locations) > 0 {
			return nil, fmt.Errorf("world.edge_fog: missing — every level fades into black at the edge of the hero's reach (a rule of the whole game, owner 16.11)")
		}
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.edge_fog: %w", err)
	}
	var out EdgeFog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.edge_fog: %w", err)
	}
	known := func(id string) bool { _, ok := life.Locations[id]; return ok }
	if err := checkEdgeFog(out, known); err != nil {
		return nil, fmt.Errorf("world.edge_fog: %w", err)
	}
	ids := make([]string, 0, len(life.Locations))
	for id := range life.Locations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !hasFloor(life.Locations[id].Terrain, c.Walkable) {
			return nil, fmt.Errorf("world.edge_fog: level %s has no walkable cell: its fog has no edge to start from", id)
		}
	}
	return &out, nil
}

func hasFloor(terrain []string, walkable map[string]bool) bool {
	for _, row := range terrain {
		for _, glyph := range row {
			if walkable[string(glyph)] {
				return true
			}
		}
	}
	return false
}

func checkEdgeFogNumbers(n EdgeFogNumbers) error {
	finite := func(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
	switch {
	case !finite(n.StartM) || n.StartM < 0 || n.StartM > 4:
		return fmt.Errorf("start_m %v: the fog begins 0..4 m beyond the reach (the room's own edge stays clear)", n.StartM)
	case !finite(n.WidthM) || n.WidthM < 1 || n.WidthM > 16:
		return fmt.Errorf("width_m %v: a soft gradient of 1..16 m (under 1 m it is a hard black shape)", n.WidthM)
	case !finite(n.ExitM) || n.ExitM < 1 || n.ExitM > 8:
		return fmt.Errorf("exit_m %v: an exit fades over 1..8 m (under 1 m the floor's cut end shows)", n.ExitM)
	case !finite(n.DepthStartM) || n.DepthStartM < 0 || n.DepthStartM > 12:
		return fmt.Errorf("depth_start_m %v: 0..12 m under the lowest walkable floor", n.DepthStartM)
	case !finite(n.DepthM) || n.DepthM < n.DepthStartM+1 || n.DepthM > 30:
		return fmt.Errorf("depth_m %v: at least 1 m deeper than depth_start_m and at most 30 m", n.DepthM)
	case !cutawayColor.MatchString(n.Color):
		return fmt.Errorf("color %q: #RRGGBB", n.Color)
	}
	var r, g, b int
	fmt.Sscanf(n.Color[1:], "%02x%02x%02x", &r, &g, &b)
	if r > 0x30 || g > 0x30 || b > 0x30 || r < b {
		return fmt.Errorf("color %s: the fog is near-black (every channel at most #30) and warm (red not under blue)", n.Color)
	}
	return nil
}

func checkEdgeFog(e EdgeFog, known func(string) bool) error {
	if err := checkEdgeFogNumbers(e.EdgeFogNumbers); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, l := range e.Levels {
		if seen[l.LevelID] {
			return fmt.Errorf("level %s twice", l.LevelID)
		}
		seen[l.LevelID] = true
		if !known(l.LevelID) {
			return fmt.Errorf("unknown level %s", l.LevelID)
		}
		if l.Why == "" {
			return fmt.Errorf("level %s: say why its fog differs", l.LevelID)
		}
		if l.StartM == nil && l.WidthM == nil && l.ExitM == nil && l.DepthStartM == nil && l.DepthM == nil && l.Color == nil {
			return fmt.Errorf("level %s: an override that changes nothing", l.LevelID)
		}
		if err := checkEdgeFogNumbers(e.levelNumbers(l)); err != nil {
			return fmt.Errorf("level %s: %w", l.LevelID, err)
		}
	}
	return nil
}

// levelNumbers: the shared numbers with a level's own over them.
func (e *EdgeFog) levelNumbers(l EdgeFogLevel) EdgeFogNumbers {
	n := e.EdgeFogNumbers
	if l.StartM != nil {
		n.StartM = *l.StartM
	}
	if l.WidthM != nil {
		n.WidthM = *l.WidthM
	}
	if l.ExitM != nil {
		n.ExitM = *l.ExitM
	}
	if l.DepthStartM != nil {
		n.DepthStartM = *l.DepthStartM
	}
	if l.DepthM != nil {
		n.DepthM = *l.DepthM
	}
	if l.Color != nil {
		n.Color = *l.Color
	}
	return n
}

// clientEdgeFog: the shared numbers ("default") and every level that has its own ("levels", merged); a level not listed
// takes the default — no level is without fog.
func clientEdgeFog(e *EdgeFog) map[string]any {
	numbers := func(n EdgeFogNumbers) map[string]any {
		return map[string]any{"start_m": n.StartM, "width_m": n.WidthM, "exit_m": n.ExitM, "depth_start_m": n.DepthStartM,
			"depth_m": n.DepthM, "color": n.Color}
	}
	levels := map[string]any{}
	for _, l := range e.Levels {
		levels[l.LevelID] = numbers(e.levelNumbers(l))
	}
	return map[string]any{"default": numbers(e.EdgeFogNumbers), "levels": levels}
}
