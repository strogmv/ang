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
	// SeeThrough: the see-through circle around the hero (optional).
	SeeThrough *CutawaySeeThrough `json:"see_through,omitempty"`
}

// CutawaySeeThrough is the see-through circle (owner 16.11, «аура видения»: standing behind the yatai the hero was not to
// be seen): on the screen around the hero, whatever stands on the camera side of him over his floor dissolves (dithered),
// with a thin dark edge; the floor and the people are never touched, walls keep their cut. The client draws it in the
// painted surfaces' shader (client-godot/shaders/see_through.gdshaderinc) from these numbers.
type CutawaySeeThrough struct {
	// RadiusM: the circle's radius on the view plane (metres), around the hero's middle (CentreUpM over his feet).
	RadiusM   float64 `json:"radius_m"`
	CentreUpM float64 `json:"centre_up_m"`
	// FadeM: the dissolving band inside the circle's edge; EdgeM: the dark edge outside it, EdgeColor its colour.
	FadeM     float64 `json:"fade_m"`
	EdgeM     float64 `json:"edge_m"`
	EdgeColor string  `json:"edge_color"`
	// NearM: a thing dissolves when it stands this much nearer the camera than the hero over the floor plan;
	// FloorKeepM: below this height over his floor nothing dissolves (the floor, the marking, what lies on it).
	NearM      float64 `json:"near_m"`
	FloorKeepM float64 `json:"floor_keep_m"`
	// Hidden (v1.1, owner 16.11: the circle opened the gantry beam over the hero when nothing hid him): the circle opens
	// only while something hides him — rays from points of his body (HeightsM over his feet) toward the camera, at least
	// MinCovered of them blocked; it grows open in GrowSeconds and closes HoldSeconds after he is free.
	Hidden SeeThroughHidden `json:"hidden"`
}

// SeeThroughHidden: when the see-through circle opens (client-godot/see_through.gd tests it).
type SeeThroughHidden struct {
	HeightsM    []float64 `json:"heights_m"`
	MinCovered  int       `json:"min_covered"`
	HoldSeconds float64   `json:"hold_seconds"`
	GrowSeconds float64   `json:"grow_seconds"`
}

// CameraCutawayLevel: one level where the rule applies, with the structure kinds that count as occluders there.
type CameraCutawayLevel struct {
	LevelID          string              `json:"level_id"`
	OccluderTerrains []string            `json:"occluder_terrains"`
	Why              string              `json:"why"`
	Parts            []CameraCutawayPart `json:"parts,omitempty"`
}

// CameraCutawayPart (the see-through zone, owner 15.11: what the isometric view hides is opened, not moved): a region of a
// module's model that the cutaway hides whole. By module, not by instance: every instance of the module on the level carries
// that region as separate meshes '<name>.cut' (its model build cuts them along the region), the scene builder gives them
// Kind and cut_whole, the client hides them whole while they hide the hero or the floor he sees.
type CameraCutawayPart struct {
	ModuleID string        `json:"module_id"`
	Kind     string        `json:"kind"`
	RegionM  CutawayRegion `json:"region_m"`
	Why      string        `json:"why"`
}

// CutawayRegion: model-local metres from the model's corner — x east, up, south (from its north side).
type CutawayRegion struct {
	X     [2]float64 `json:"x"`
	Up    [2]float64 `json:"up"`
	South [2]float64 `json:"south"`
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
	// the modules placed on each level of the rule (the instances of the level's location SDK layout, as the exit zones
	// find it: level.c7_market -> example.market_hub): a cut part names one of them
	placed := map[string]map[string]bool{}
	for _, l := range out.Levels {
		if layout := exitZoneLayout(c, l.LevelID); layout != nil {
			placed[l.LevelID] = map[string]bool{}
			for _, in := range layout.Instances {
				placed[l.LevelID][in.ModuleID] = true
			}
		}
	}
	if err := checkCameraCutaway(out, known, views, placed); err != nil {
		return nil, fmt.Errorf("world.camera_cutaway: %w", err)
	}
	return &out, nil
}

func checkCameraCutaway(c CameraCutaway, known func(string) bool, views map[string][]string, placed map[string]map[string]bool) error {
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
	if err := checkSeeThrough(c.SeeThrough); err != nil {
		return fmt.Errorf("see_through: %w", err)
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
		if err := checkCutawayParts(c, l, kinds, placed[l.LevelID]); err != nil {
			return fmt.Errorf("level %s: %w", l.LevelID, err)
		}
	}
	return nil
}

// checkSeeThrough: the circle is a readable patch around the hero, not the screen and not a dot.
func checkSeeThrough(s *CutawaySeeThrough) error {
	if s == nil {
		return nil
	}
	finite := func(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
	switch {
	case !finite(s.RadiusM) || s.RadiusM < 0.5 || s.RadiusM > 4:
		return fmt.Errorf("radius_m %v: 0.5..4 m around the hero (the 12 m frame is 6 m from the centre to the top)", s.RadiusM)
	case !finite(s.CentreUpM) || s.CentreUpM < 0 || s.CentreUpM > 2:
		return fmt.Errorf("centre_up_m %v: the circle's centre 0..2 m over the hero's feet", s.CentreUpM)
	case !finite(s.FadeM) || s.FadeM <= 0 || s.FadeM > s.RadiusM:
		return fmt.Errorf("fade_m %v: more than 0 (a hard hole pops) and at most the radius %v", s.FadeM, s.RadiusM)
	case !finite(s.EdgeM) || s.EdgeM < 0 || s.EdgeM > 0.2:
		return fmt.Errorf("edge_m %v: a thin edge, 0..0.2 m", s.EdgeM)
	case !cutawayColor.MatchString(s.EdgeColor):
		return fmt.Errorf("edge_color %q: #RRGGBB", s.EdgeColor)
	case !finite(s.NearM) || s.NearM < 0 || s.NearM > 2:
		return fmt.Errorf("near_m %v: 0..2 m nearer the camera than the hero", s.NearM)
	case !finite(s.FloorKeepM) || s.FloorKeepM < 0 || s.FloorKeepM > 0.5:
		return fmt.Errorf("floor_keep_m %v: the floor and what lies on it stay, 0..0.5 m", s.FloorKeepM)
	}
	h := s.Hidden
	if len(h.HeightsM) < 2 || len(h.HeightsM) > 8 {
		return fmt.Errorf("hidden.heights_m %v: 2..8 points of the body (one point opens the circle for a beam over his head)", h.HeightsM)
	}
	for i, x := range h.HeightsM {
		if !finite(x) || x < 0.1 || x > 2.2 || i > 0 && x <= h.HeightsM[i-1] {
			return fmt.Errorf("hidden.heights_m %v: rising, 0.1..2.2 m over the feet", h.HeightsM)
		}
	}
	switch {
	case h.MinCovered < 1 || h.MinCovered > len(h.HeightsM):
		return fmt.Errorf("hidden.min_covered %d: 1..%d (the body points)", h.MinCovered, len(h.HeightsM))
	case !finite(h.HoldSeconds) || h.HoldSeconds < 0 || h.HoldSeconds > 5:
		return fmt.Errorf("hidden.hold_seconds %v: 0..5 s", h.HoldSeconds)
	case !finite(h.GrowSeconds) || h.GrowSeconds <= 0 || h.GrowSeconds > 2:
		return fmt.Errorf("hidden.grow_seconds %v: more than 0 (it would pop) and at most 2 s", h.GrowSeconds)
	}
	return nil
}

// checkCutawayParts: each cut part names a module placed on the level, once; its kind is one the level cuts (else the
// client would never hide it); its region is a real box of the model above the stub (what stays is at least the stub, as
// for walls); it says why.
func checkCutawayParts(c CameraCutaway, l CameraCutawayLevel, kinds map[string]bool, placed map[string]bool) error {
	finite := func(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
	seen := map[string]bool{}
	for _, p := range l.Parts {
		switch {
		case p.ModuleID == "":
			return fmt.Errorf("a cut part without a module")
		case seen[p.ModuleID]:
			return fmt.Errorf("cut part of module %s twice", p.ModuleID)
		case !placed[p.ModuleID]:
			return fmt.Errorf("cut part of module %s: no instance of it on the level", p.ModuleID)
		case !kinds[p.Kind]:
			return fmt.Errorf("cut part of module %s: kind %q is not an occluder kind of the level %v", p.ModuleID, p.Kind, l.OccluderTerrains)
		case p.Why == "":
			return fmt.Errorf("cut part of module %s: say why it is cut", p.ModuleID)
		}
		seen[p.ModuleID] = true
		for name, r := range map[string][2]float64{"x": p.RegionM.X, "up": p.RegionM.Up, "south": p.RegionM.South} {
			if !finite(r[0]) || !finite(r[1]) || r[0] < 0 || r[1] <= r[0] || r[1] > 30 {
				return fmt.Errorf("cut part of module %s: region %s %v must be 0 <= from < to <= 30 m", p.ModuleID, name, r)
			}
		}
		if p.RegionM.Up[0] < c.StubM {
			return fmt.Errorf("cut part of module %s: region starts %v m up, below the stub %v m (a cut leaves at least the stub)", p.ModuleID, p.RegionM.Up[0], c.StubM)
		}
	}
	return nil
}

// clientCameraCutaway: what the client needs — the numbers and, per level, the occluder kinds and the cut parts (the
// scene builder tags their meshes; the reasons stay in CUE).
func clientCameraCutaway(c *CameraCutaway) map[string]any {
	levels := map[string]any{}
	for _, l := range c.Levels {
		lv := map[string]any{"occluder_terrains": l.OccluderTerrains}
		if len(l.Parts) > 0 {
			parts := make([]map[string]any, 0, len(l.Parts))
			for _, p := range l.Parts {
				parts = append(parts, map[string]any{"module_id": p.ModuleID, "kind": p.Kind, "region_m": p.RegionM})
			}
			lv["parts"] = parts
		}
		levels[l.LevelID] = lv
	}
	out := map[string]any{"stub_m": c.StubM, "reach_m": c.ReachM, "seconds": c.Seconds, "hold_seconds": c.HoldSeconds,
		"cap_color": c.CapColor, "levels": levels}
	if c.SeeThrough != nil {
		out["see_through"] = c.SeeThrough
	}
	return out
}
