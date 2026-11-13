package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"

	"cuelang.org/go/cue"
)

// CameraCutaway is world.camera_cutaway (camera cutaway v1, design/camera-cutaway/v1): structures of an occluder kind on
// the camera side of the hero that hide him or the floor he sees around him from the fixed 35°/37° camera, or stand wholly
// between him and the camera near him on the screen, are drawn cut down to a low stub with a dark cap, smoothly; walls
// behind the hero stay whole. Presentation only: the walls keep blocking movement, sight and
// light on their full height. The client (client-godot/camera_cutaway.gd) reads the numbers from generated/presentation.json
// and the kind of each box from the scene's own metadata/structure_kind; nobody lists walls by hand.
type CameraCutaway struct {
	// StubM: the cut height over the floor the hero stands on (metres).
	StubM float64 `json:"stub_m"`
	// ReachM: the radius of floor around the hero (and the camera's focus) that must stay visible (plan metres), and the
	// same distance on the screen for a structure wholly in front of him.
	ReachM float64 `json:"reach_m"`
	// Seconds: how long a wall takes to go down or up; HoldSeconds: how long it stays down after it stops hiding anything.
	Seconds     float64 `json:"seconds"`
	HoldSeconds float64 `json:"hold_seconds"`
	// CapColor: the colour of the cut top edge (the dark cap).
	CapColor string               `json:"cap_color"`
	Levels   []CameraCutawayLevel `json:"levels"`
}

// CameraCutawayLevel: one level where the rule applies, with the structure kinds that count as occluders there.
type CameraCutawayLevel struct {
	LevelID          string   `json:"level_id"`
	OccluderTerrains []string `json:"occluder_terrains"`
	Why              string   `json:"why"`
}

var cutawayColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// readCameraCutaway reads world.camera_cutaway and checks it against the levels of the world. A level that is also a
// world.levels entry must name exactly its own view.occluder_terrains (one source for the hint).
func readCameraCutaway(world cue.Value, c Compiled) (*CameraCutaway, error) {
	v := world.LookupPath(cue.ParsePath("camera_cutaway"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.camera_cutaway: %w", err)
	}
	var out CameraCutaway
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.camera_cutaway: %w", err)
	}
	var life struct {
		Locations map[string]json.RawMessage `json:"locations"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return nil, err
	}
	views := map[string][]string{}
	if levels := world.LookupPath(cue.ParsePath("levels")); levels.Exists() {
		it, err := levels.Fields()
		if err != nil {
			return nil, fmt.Errorf("world.levels: %w", err)
		}
		for it.Next() {
			hint := it.Value().LookupPath(cue.ParsePath("view.occluder_terrains"))
			if !hint.Exists() {
				continue
			}
			var kinds []string
			if err := hint.Decode(&kinds); err != nil {
				return nil, fmt.Errorf("world.levels.%s.view.occluder_terrains: %w", it.Selector(), err)
			}
			views[it.Selector().Unquoted()] = kinds
		}
	}
	known := func(id string) bool { _, ok := life.Locations[id]; return ok }
	if err := checkCameraCutaway(out, known, views); err != nil {
		return nil, fmt.Errorf("world.camera_cutaway: %w", err)
	}
	return &out, nil
}

func checkCameraCutaway(c CameraCutaway, known func(string) bool, views map[string][]string) error {
	finite := func(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
	switch {
	case !finite(c.StubM) || c.StubM < 0.3 || c.StubM > 3:
		return fmt.Errorf("stub_m %v: the cut must leave 0.3..3 m of wall (a kerb to a waist-high parapet)", c.StubM)
	case !finite(c.ReachM) || c.ReachM < 2 || c.ReachM > 20:
		return fmt.Errorf("reach_m %v: 2..20 m of floor around the hero (the 12 m frame shows about 10 m of floor in front of him)", c.ReachM)
	case !finite(c.Seconds) || c.Seconds <= 0 || c.Seconds > 3:
		return fmt.Errorf("seconds %v: a wall goes down or up in more than 0 and at most 3 s (0 would pop)", c.Seconds)
	case !finite(c.HoldSeconds) || c.HoldSeconds < 0 || c.HoldSeconds > 10:
		return fmt.Errorf("hold_seconds %v: 0..10 s", c.HoldSeconds)
	case !cutawayColor.MatchString(c.CapColor):
		return fmt.Errorf("cap_color %q: #RRGGBB", c.CapColor)
	case len(c.Levels) == 0:
		return fmt.Errorf("no level: say where the rule applies")
	}
	seen := map[string]bool{}
	for _, l := range c.Levels {
		if seen[l.LevelID] {
			return fmt.Errorf("level %s twice", l.LevelID)
		}
		seen[l.LevelID] = true
		if !known(l.LevelID) {
			return fmt.Errorf("unknown level %s", l.LevelID)
		}
		if l.Why == "" {
			return fmt.Errorf("level %s: say why the rule applies there", l.LevelID)
		}
		if len(l.OccluderTerrains) == 0 {
			return fmt.Errorf("level %s: no occluder kind", l.LevelID)
		}
		kinds := map[string]bool{}
		for _, k := range l.OccluderTerrains {
			if kinds[k] {
				return fmt.Errorf("level %s: occluder kind %s twice", l.LevelID, k)
			}
			kinds[k] = true
			if k == "floor" {
				return fmt.Errorf("level %s: a floor is walked on, never cut away", l.LevelID)
			}
		}
		if hint, ok := views[l.LevelID]; ok {
			a, b := append([]string{}, hint...), append([]string{}, l.OccluderTerrains...)
			sort.Strings(a)
			sort.Strings(b)
			if fmt.Sprint(a) != fmt.Sprint(b) {
				return fmt.Errorf("level %s: occluder kinds %v differ from its view.occluder_terrains %v", l.LevelID, b, a)
			}
		}
	}
	return nil
}

// clientCameraCutaway: what the client needs — the numbers and, per level, the occluder kinds.
func clientCameraCutaway(c *CameraCutaway) map[string]any {
	levels := map[string]any{}
	for _, l := range c.Levels {
		levels[l.LevelID] = map[string]any{"occluder_terrains": l.OccluderTerrains}
	}
	return map[string]any{"stub_m": c.StubM, "reach_m": c.ReachM, "seconds": c.Seconds, "hold_seconds": c.HoldSeconds,
		"cap_color": c.CapColor, "levels": levels}
}
