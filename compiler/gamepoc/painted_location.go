package gamepoc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
)

// PaintedLocation is world.painted_location (painted location v1, the game's design/painted-location/v1/BRIEF.md): a corner
// of a level drawn the Pillars of Eternity / Disco Elysium way — its albedo rendered from the fixed game camera, painted
// over (by the coordinator with an image model, or the deterministic stand-in made from the render), and projected back
// from that camera onto the corner's own geometry; normals, the live light, the cutaway and people stay the game's. Mode
// "now" (the default) draws every level as before; "painted" turns the corners on. Presentation only; no light is baked.
// The client (client-godot/painted_location.gd) reads generated/presentation.json painted_location.
type PaintedLocation struct {
	Mode    string          `json:"mode"`
	Corners []PaintedCorner `json:"corners"`
	// Painted market v1: whole levels (painted_level.go).
	Levels []PaintedLevel `json:"levels,omitempty"`
}

// PaintedCorner: one corner. FocusPlan is the frame's middle on the level's plan (metres: x east, y south, z up); SizeM the
// frame's height (the game's ortho size); RenderPx the render's (and the painting's) size. Picture chooses the stand-in or
// the coordinator's painting; Depth is the render's depth map (16 bit along the view, blue: may be painted). A fragment
// takes the painting where it agrees with the depth map within DepthToleranceM; EdgeM is the soft edge inside the frame,
// Strength the painting's share. LiveParts: the corner's parts whose state the server changes (doors, flaps, lifts, fans,
// signal lamps, screens, stock): matched by node name (or a pattern with * and ?), each with its reason; they stay real-time
// 3D and are left out of the render (the static / live split; later a flag of the kit and level data).
type PaintedCorner struct {
	ID              string            `json:"id"`
	LevelID         string            `json:"level_id"`
	Why             string            `json:"why"`
	FocusPlan       [3]float64        `json:"focus_plan"`
	SizeM           float64           `json:"size_m"`
	RenderPx        [2]int            `json:"render_px"`
	Picture         string            `json:"picture"`
	StandIn         string            `json:"stand_in"`
	Painted         string            `json:"painted"`
	Depth           string            `json:"depth"`
	DepthToleranceM float64           `json:"depth_tolerance_m"`
	EdgeM           float64           `json:"edge_m"`
	Strength        float64           `json:"strength"`
	LiveParts       []PaintedLivePart `json:"live_parts"`
	// v3 (the game's design/painted-location/v3/README.md): Look is how the painted static surfaces are shown — "lit"
	// (v1/v2: the picture is an albedo under the live light; the default when absent) or "painting" (the picture is the
	// final image: emitted through the game's tonemapper backwards, no ambient, no key-light Lambert, no gloss; lamps add,
	// people's shadows multiply, the hour is a grade). Painting is that picture (merge mode "paint": the painting at all
	// frequencies, harmonised to the style key); PaintingLook its numbers.
	Look         string        `json:"look,omitempty"`
	Painting     string        `json:"painting,omitempty"`
	PaintingLook *PaintingLook `json:"painting_look,omitempty"`
}

// PaintingLook: the numbers of the look "painting". LampGain / LampWrap: the live lamps' pools on the painting (wrapped
// N.L, tinted by the painted colour). HighlightCeiling: see the field. Shadow: the key's shadows of people and live parts multiply the painting toward
// Tint (linear multipliers) with Strength at the key's day energy. StaticShadows: whether the corner's static meshes keep
// casting the key's shadow onto it. Grade: the painting by the server's day / dusk / night weights. PeopleReadKeep: the
// share of the people-read background grade the painted surfaces keep. Live: the live parts (painted-live material) in
// the painting's palette.
type PaintingLook struct {
	LampGain float64 `json:"lamp_gain"`
	LampWrap float64 `json:"lamp_wrap"`
	// HighlightCeiling: the painting's brightest colours roll off toward this tonemapper input (exposure included; 0:
	// none), under the game's glow threshold.
	HighlightCeiling float64       `json:"highlight_ceiling"`
	Shadow           PaintingShade `json:"shadow"`
	StaticShadows    bool          `json:"static_shadows"`
	// LiveShadows: whether the corner's live parts (the insides of static things: fans, flaps, goods behind glass) keep
	// casting the key's shadow while the painting is shown; people and things outside the corner always cast.
	LiveShadows bool `json:"live_shadows"`
	Grade       struct {
		Day   PaintingGrade `json:"day"`
		Dusk  PaintingGrade `json:"dusk"`
		Night PaintingGrade `json:"night"`
	} `json:"grade"`
	PeopleReadKeep float64      `json:"people_read_keep"`
	Live           PaintingLive `json:"live"`
}

// PaintingShade: the key shadow's strength and the painting's shadow colour.
type PaintingShade struct {
	Strength float64    `json:"strength"`
	Tint     [3]float64 `json:"tint"`
}

// PaintingGrade: one phase of the hour — value and tint (display multipliers) and saturation of the painting.
type PaintingGrade struct {
	Value      float64    `json:"value"`
	Tint       [3]float64 `json:"tint"`
	Saturation float64    `json:"saturation"`
}

// PaintingLive: the live parts in the painting's palette — their colour's tint and saturation, the key's and the lamps' gain.
type PaintingLive struct {
	Tint       [3]float64 `json:"tint"`
	Saturation float64    `json:"saturation"`
	KeyGain    float64    `json:"key_gain"`
	LampGain   float64    `json:"lamp_gain"`
}

// PaintedLivePart: a node name or a name pattern of the corner's live parts and why it is live.
// Painted market v1: or a level instance (Instance) — the whole instance, or only its descendants named by Parts.
type PaintedLivePart struct {
	Match    string   `json:"match,omitempty"`
	Instance string   `json:"instance,omitempty"`
	Parts    []string `json:"parts,omitempty"`
	Why      string   `json:"why"`
}

var (
	paintedCornerID   = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)
	paintedNodeName   = regexp.MustCompile(`^[A-Za-z0-9_.*?-]+$`)
	paintedPicturePth = regexp.MustCompile(`^client-godot/assets/painted-location/[a-z0-9_.-]+/[a-z0-9_.-]+\.png$`)
)

func readPaintedLocation(world cue.Value, c Compiled, root string) (*PaintedLocation, error) {
	v := world.LookupPath(cue.ParsePath("painted_location"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.painted_location: %w", err)
	}
	var out PaintedLocation
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.painted_location: %w", err)
	}
	// Go fills a short list into a fixed array with zeros: every list must have exactly its length.
	if err := paintedLocationLengths(raw); err != nil {
		return nil, fmt.Errorf("world.painted_location: %w", err)
	}
	var life struct {
		Locations map[string]json.RawMessage `json:"locations"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return nil, err
	}
	known := func(id string) bool { _, ok := life.Locations[id]; return ok }
	if err := checkPaintedLocation(out, known, root); err != nil {
		return nil, fmt.Errorf("world.painted_location: %w", err)
	}
	return &out, nil
}

func paintedLocationLengths(raw []byte) error {
	type grade struct {
		Tint []json.RawMessage `json:"tint"`
	}
	var doc struct {
		Corners []struct {
			ID           string            `json:"id"`
			FocusPlan    []json.RawMessage `json:"focus_plan"`
			RenderPx     []json.RawMessage `json:"render_px"`
			PaintingLook *struct {
				Shadow grade `json:"shadow"`
				Grade  struct {
					Day   grade `json:"day"`
					Dusk  grade `json:"dusk"`
					Night grade `json:"night"`
				} `json:"grade"`
				Live grade `json:"live"`
			} `json:"painting_look"`
		} `json:"corners"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	var levels struct {
		Levels []struct {
			ID     string `json:"id"`
			Frames []struct {
				Name string            `json:"name"`
				Plan []json.RawMessage `json:"plan"`
			} `json:"frames"`
		} `json:"levels"`
	}
	if err := json.Unmarshal(raw, &levels); err != nil {
		return err
	}
	for _, l := range levels.Levels {
		for _, f := range l.Frames {
			if len(f.Plan) != 3 {
				return fmt.Errorf("level painting %s: frames %s: plan needs exactly 3 numbers (x east, y south, z up)", l.ID, f.Name)
			}
		}
	}
	for _, c := range doc.Corners {
		if len(c.FocusPlan) != 3 {
			return fmt.Errorf("corner %s: focus_plan needs exactly 3 numbers (x east, y south, z up)", c.ID)
		}
		if len(c.RenderPx) != 2 {
			return fmt.Errorf("corner %s: render_px needs exactly 2 numbers (width, height)", c.ID)
		}
		if l := c.PaintingLook; l != nil {
			for name, tint := range map[string][]json.RawMessage{"shadow": l.Shadow.Tint, "grade.day": l.Grade.Day.Tint,
				"grade.dusk": l.Grade.Dusk.Tint, "grade.night": l.Grade.Night.Tint, "live": l.Live.Tint} {
				if len(tint) != 3 {
					return fmt.Errorf("corner %s: painting_look.%s.tint needs exactly 3 numbers (R G B)", c.ID, name)
				}
			}
		}
	}
	return nil
}

func checkPaintedLocation(p PaintedLocation, known func(string) bool, root string) error {
	if p.Mode != "now" && p.Mode != "painted" {
		return fmt.Errorf("mode must be now or painted, not %q", p.Mode)
	}
	if len(p.Corners) == 0 {
		return fmt.Errorf("no corner: name the corner to paint")
	}
	ids := map[string]bool{}
	levels := map[string]bool{}
	for _, c := range p.Corners {
		if !paintedCornerID.MatchString(c.ID) {
			return fmt.Errorf("corner id %q: <place>.<corner>", c.ID)
		}
		if ids[c.ID] {
			return fmt.Errorf("corner %s twice", c.ID)
		}
		ids[c.ID] = true
		// One corner per level for now: two would fight over the same surfaces (one painting per surface).
		if levels[c.LevelID] {
			return fmt.Errorf("corner %s: level %s has a corner already", c.ID, c.LevelID)
		}
		levels[c.LevelID] = true
		if err := checkPaintedCorner(c, known, root); err != nil {
			return fmt.Errorf("corner %s: %w", c.ID, err)
		}
	}
	return nil
}

func checkPaintedCorner(c PaintedCorner, known func(string) bool, root string) error {
	if !known(c.LevelID) {
		return fmt.Errorf("unknown level %s", c.LevelID)
	}
	if strings.TrimSpace(c.Why) == "" {
		return fmt.Errorf("say why this corner is painted")
	}
	nums := []float64{c.SizeM, c.DepthToleranceM, c.EdgeM, c.Strength}
	nums = append(nums, c.FocusPlan[:]...)
	for _, x := range nums {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("non-finite number")
		}
	}
	w, h := c.RenderPx[0], c.RenderPx[1]
	half := math.Min(c.SizeM*float64(w)/float64(max(h, 1)), c.SizeM) / 2
	switch {
	case c.SizeM < 9 || c.SizeM > 16:
		return fmt.Errorf("size_m %v: the game's frame is 9..16 m (the wheel)", c.SizeM)
	case h < 400 || w < h || w > 8192 || h > 8192:
		return fmt.Errorf("render_px %v: at least 400 px high, not narrower than high, at most 8192", c.RenderPx)
	case float64(h)/c.SizeM < 50:
		return fmt.Errorf("render_px %v over %v m: under 50 px a metre the painting is coarser than the window at 16 m", c.RenderPx, c.SizeM)
	case c.DepthToleranceM < 0.005 || c.DepthToleranceM > 0.2:
		return fmt.Errorf("depth_tolerance_m %v: 0.005..0.2 (wider paints over what the render did not see)", c.DepthToleranceM)
	case c.EdgeM < 0 || c.EdgeM >= half:
		return fmt.Errorf("edge_m %v: 0 .. half the frame (%v m)", c.EdgeM, half)
	case c.Strength <= 0 || c.Strength > 1:
		return fmt.Errorf("strength %v: above 0, at most 1", c.Strength)
	case c.Picture != "stand_in" && c.Picture != "painted":
		return fmt.Errorf("picture must be stand_in or painted, not %q", c.Picture)
	}
	seen := map[string]bool{}
	for _, part := range c.LiveParts {
		if !paintedNodeName.MatchString(part.Match) || seen[part.Match] {
			return fmt.Errorf("live_parts %q: a node name or a name pattern (* ?), once", part.Match)
		}
		if strings.Trim(part.Match, "*?") == "" {
			return fmt.Errorf("live_parts %q: a pattern of only wildcards makes the whole level live", part.Match)
		}
		if strings.TrimSpace(part.Why) == "" {
			return fmt.Errorf("live_parts %q: say what the server changes in it", part.Match)
		}
		seen[part.Match] = true
	}
	switch c.Look {
	case "", "lit":
	case "painting":
		if c.Painting == "" || c.PaintingLook == nil {
			return fmt.Errorf("look painting needs painting (the paint-mode picture) and painting_look")
		}
	default:
		return fmt.Errorf("look must be lit or painting, not %q", c.Look)
	}
	if c.PaintingLook != nil {
		if err := checkPaintingLook(*c.PaintingLook); err != nil {
			return fmt.Errorf("painting_look: %w", err)
		}
	}
	if c.Painting != "" {
		if !paintedPicturePth.MatchString(c.Painting) {
			return fmt.Errorf("painting %q: a .png under client-godot/assets/painted-location/<corner>/", c.Painting)
		}
		if c.Painting == c.StandIn || c.Painting == c.Painted || c.Painting == c.Depth {
			return fmt.Errorf("painting must be a file of its own")
		}
		if err := checkPaintedPNG(root, c.Painting, w, h, false); err != nil {
			return fmt.Errorf("painting: %w", err)
		}
	}
	for _, f := range []struct{ name, path string }{{"stand_in", c.StandIn}, {"painted", c.Painted}, {"depth", c.Depth}} {
		if !paintedPicturePth.MatchString(f.path) {
			return fmt.Errorf("%s %q: a .png under client-godot/assets/painted-location/<corner>/", f.name, f.path)
		}
	}
	if c.StandIn == c.Painted || c.StandIn == c.Depth || c.Painted == c.Depth {
		return fmt.Errorf("stand_in, painted and depth must be three files")
	}
	chosen := c.StandIn
	if c.Picture == "painted" {
		chosen = c.Painted
	}
	// The depth map's bytes are the depth: plain 8-bit RGB, the render's size.
	if err := checkPaintedPNG(root, c.Depth, w, h, true); err != nil {
		return fmt.Errorf("depth: %w", err)
	}
	if err := checkPaintedPNG(root, chosen, w, h, false); err != nil {
		return fmt.Errorf("%s: %w", c.Picture, err)
	}
	return nil
}

// checkPaintingLook: every number finite and in its range (the CUE schema says the same; ANG is the gate for data that
// bypasses it).
func checkPaintingLook(l PaintingLook) error {
	in := func(name string, v, lo, hi float64) error {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < lo || v > hi {
			return fmt.Errorf("%s %v: %v..%v", name, v, lo, hi)
		}
		return nil
	}
	checks := []error{in("lamp_gain", l.LampGain, 0, 4), in("lamp_wrap", l.LampWrap, 0, 1), in("highlight_ceiling", l.HighlightCeiling, 0, 16), in("shadow.strength", l.Shadow.Strength, 0, 1),
		in("people_read_keep", l.PeopleReadKeep, 0, 1), in("live.saturation", l.Live.Saturation, 0, 2),
		in("live.key_gain", l.Live.KeyGain, 0, 2), in("live.lamp_gain", l.Live.LampGain, 0, 2)}
	for i, v := range l.Shadow.Tint {
		checks = append(checks, in(fmt.Sprintf("shadow.tint[%d]", i), v, 0, 1))
	}
	for i, v := range l.Live.Tint {
		checks = append(checks, in(fmt.Sprintf("live.tint[%d]", i), v, 0, 2))
	}
	for name, g := range map[string]PaintingGrade{"day": l.Grade.Day, "dusk": l.Grade.Dusk, "night": l.Grade.Night} {
		if g.Value <= 0 {
			checks = append(checks, fmt.Errorf("grade.%s.value %v: above 0", name, g.Value))
		}
		checks = append(checks, in("grade."+name+".value", g.Value, 0, 2), in("grade."+name+".saturation", g.Saturation, 0, 2))
		for i, v := range g.Tint {
			checks = append(checks, in(fmt.Sprintf("grade.%s.tint[%d]", name, i), v, 0, 2))
		}
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}

// checkPaintedPNG: the file is a PNG of exactly w x h; rgb8 asks for 8-bit RGB without alpha (colour type 2).
func checkPaintedPNG(root, rel string, w, h int, rgb8 bool) error {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	defer f.Close()
	head := make([]byte, 29)
	if _, err := f.Read(head); err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	if !bytes.Equal(head[:8], []byte("\x89PNG\r\n\x1a\n")) || string(head[12:16]) != "IHDR" {
		return fmt.Errorf("%s is not a PNG", rel)
	}
	gw, gh := int(binary.BigEndian.Uint32(head[16:20])), int(binary.BigEndian.Uint32(head[20:24]))
	if gw != w || gh != h {
		return fmt.Errorf("%s is %dx%d, the render is %dx%d", rel, gw, gh, w, h)
	}
	if rgb8 && (head[24] != 8 || head[25] != 2) {
		return fmt.Errorf("%s: 8-bit RGB without alpha (bit depth %d, colour type %d)", rel, head[24], head[25])
	}
	return nil
}

// clientPaintedLocation: the mode and the corners with the chosen picture as a res:// path, without the reasons.
func clientPaintedLocation(p *PaintedLocation) map[string]any {
	res := func(rel string) string { return "res://" + strings.TrimPrefix(rel, "client-godot/") }
	corners := []any{}
	for _, c := range p.Corners {
		picture := c.StandIn
		if c.Picture == "painted" {
			picture = c.Painted
		}
		live := []string{}
		for _, part := range c.LiveParts {
			live = append(live, part.Match)
		}
		look := c.Look
		if look == "" {
			look = "lit"
		}
		pictures := map[string]any{"lit": res(picture)}
		if c.Painting != "" {
			pictures["painting"] = res(c.Painting)
		}
		row := map[string]any{"id": c.ID, "level_id": c.LevelID, "focus_plan": c.FocusPlan, "size_m": c.SizeM,
			"render_px": c.RenderPx, "picture_kind": c.Picture, "picture": res(picture), "depth": res(c.Depth),
			"depth_tolerance_m": c.DepthToleranceM, "edge_m": c.EdgeM, "strength": c.Strength, "live_parts": live,
			"look": look, "pictures": pictures}
		if c.PaintingLook != nil {
			row["painting_look"] = c.PaintingLook
		}
		corners = append(corners, row)
	}
	view := map[string]any{"mode": p.Mode, "corners": corners}
	if len(p.Levels) > 0 {
		view["levels"] = clientPaintedLevels(p.Levels)
	}
	return view
}
