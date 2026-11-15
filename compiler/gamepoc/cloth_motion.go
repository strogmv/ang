package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"

	"cuelang.org/go/cue"
)

// ClothMotion is world.cloth_motion (cloth v1, the game's design/market-hub/hot-food/strip-curtain-v1/BRIEF.md): heavy
// hanging strips (the PVC strip curtain of a yatai) hang still and swing only when a drawn person pushes them — each strip
// a damped pendulum hinged on its rail that bends a little along its length and nudges the strips beside it; cloth the
// air reaches (a banner) only drifts, slowly and a little, in a draft named here. There is no wind in a dome: a piece
// without a draft does not move by itself. Presentation only: the server never sees it. The client
// (client-godot/cloth_motion.gd) reads generated/presentation.json cloth_motion.levels[<level>] and the shared
// deformation (generated/world_text/world_text_deformation.gdshaderinc) draws the strips and the lettering on them alike.
type ClothMotion struct {
	Drafts   []ClothDraft   `json:"drafts"`
	Curtains []StripCurtain `json:"curtains"`
	Drapes   []ClothDrape   `json:"drapes"`
}

// ClothDraft: an air current that reaches cloth (a supply outlet nearby, the convection of the hall), with its drift: a
// tilt of DriftDeg degrees at most, over PeriodS seconds.
type ClothDraft struct {
	ID       string  `json:"id"`
	LevelID  string  `json:"level_id"`
	Source   string  `json:"source"`
	Ru       string  `json:"ru"`
	DriftDeg float64 `json:"drift_deg"`
	PeriodS  float64 `json:"period_s"`
}

// StripCurtain: the strips of one host (a level's instance), in rows along their rails (neighbours in a row touch at
// their edges). Strip: the body of one strip (metres; thickness mm; density kg/m3). ClampM: the clamp bar holds the top,
// over this length a strip turns from the vertical to its swing. Swing: the pendulum (its period and damping ratio, the
// largest angle); Bend: the strip's bend along its length (a faster, more damped mode; Share: how much further than the
// hinge the free end leans in a free swing — a hanging chain about 1.3, a stiff board 0 — the end lagging behind a push
// and then whipping); NeighbourShare: the share of an angle
// difference that the touching edges pass to a neighbour; ContactMarginM: clothing and hair over the bones of a body.
type StripCurtain struct {
	ID             string      `json:"id"`
	LevelID        string      `json:"level_id"`
	Instance       string      `json:"instance"`
	Rows           []StripRow  `json:"rows"`
	Strip          StripBody   `json:"strip"`
	ClampM         float64     `json:"clamp_m"`
	Swing          ClothSwing  `json:"swing"`
	Bend           ClothBend   `json:"bend"`
	NeighbourShare float64     `json:"neighbour_share"`
	ContactMarginM float64     `json:"contact_margin_m"`
	Draft          string      `json:"draft,omitempty"`
	Why            string      `json:"why"`
	Clear          *ClearSheet `json:"clear,omitempty"`
}

// ClearSheet: the strips as a clear sheet (the game's design/painted-location/startup-v1/BRIEF.md: clear industrial PVC, the
// kitchen seen through it), presentation only. Tint: the light through the sheet (linear, 1: no tint); Body: the share of
// the straight view through it that takes its tint and bend; Edge: how far a grazing view and the cut edges go over to the
// sheet's gloss; Roughness, Specular: its gloss; DistortionM: how far the view behind bends at a sheet turned fully
// side-on; Ripple: the sheet's waviness (tilt, waves a metre); Frost: the scuffed band where hands and shoulders push it
// (from/to above the strip's lower edge, how matte, scuffs a metre across).
type ClearSheet struct {
	Tint        [3]float64  `json:"tint"`
	Body        float64     `json:"body"`
	Edge        float64     `json:"edge"`
	Roughness   float64     `json:"roughness"`
	Specular    float64     `json:"specular"`
	DistortionM float64     `json:"distortion_m"`
	Ripple      SheetRipple `json:"ripple"`
	Frost       SheetFrost  `json:"frost"`
	Why         string      `json:"why"`
}

type SheetRipple struct {
	TiltDeg   float64 `json:"tilt_deg"`
	WavesPerM float64 `json:"waves_per_m"`
}

type SheetFrost struct {
	FromM      float64 `json:"from_m"`
	ToM        float64 `json:"to_m"`
	Amount     float64 `json:"amount"`
	ScuffsPerM float64 `json:"scuffs_per_m"`
}

type StripRow struct {
	ID    string   `json:"id"`
	Nodes []string `json:"nodes"`
}

type StripBody struct {
	LengthM      float64 `json:"length_m"`
	WidthM       float64 `json:"width_m"`
	ThicknessMM  float64 `json:"thickness_mm"`
	DensityKgM3  float64 `json:"density_kg_m3"`
	MaterialNote string  `json:"material"`
}

type ClothSwing struct {
	PeriodS      float64 `json:"period_s"`
	DampingRatio float64 `json:"damping_ratio"`
	MaxDeg       float64 `json:"max_deg"`
}

type ClothBend struct {
	PeriodS      float64 `json:"period_s"`
	DampingRatio float64 `json:"damping_ratio"`
	Share        float64 `json:"share"`
	MaxDeg       float64 `json:"max_deg"`
}

// ClothDrape: cloth that only the air moves, a node of the level's scene (a MultiMesh of banners): its hinge length,
// its clamp and the draft it hangs in.
type ClothDrape struct {
	ID      string  `json:"id"`
	LevelID string  `json:"level_id"`
	Node    string  `json:"node"`
	LengthM float64 `json:"length_m"`
	ClampM  float64 `json:"clamp_m"`
	Draft   string  `json:"draft"`
	Why     string  `json:"why"`
}

const clothGravity = 9.81

var clothID = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$`)
var clothNode = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]*$`)

func readClothMotion(world cue.Value, c Compiled) (*ClothMotion, error) {
	v := world.LookupPath(cue.ParsePath("cloth_motion"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.cloth_motion: %w", err)
	}
	var out ClothMotion
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.cloth_motion: %w", err)
	}
	var life struct {
		Locations map[string]json.RawMessage `json:"locations"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return nil, err
	}
	known := func(id string) bool { _, ok := life.Locations[id]; return ok }
	if err := checkClothMotion(out, known); err != nil {
		return nil, fmt.Errorf("world.cloth_motion: %w", err)
	}
	return &out, nil
}

// stripPendulumPeriod: the period of a strip of length l swinging on its top edge as a rigid board, 2π√(2l/3g). A heavy
// hanging strip (a chain with a little bending stiffness) swings within a few per cent of it.
func stripPendulumPeriod(l float64) float64 {
	return 2 * math.Pi * math.Sqrt(2*l/(3*clothGravity))
}

// swingsToSettle: how many full swings a damped pendulum of damping ratio z takes to fall to 5 % of its amplitude.
func swingsToSettle(z float64) float64 {
	return math.Log(20) / (2 * math.Pi * z / math.Sqrt(1-z*z))
}

func checkClothMotion(c ClothMotion, known func(string) bool) error {
	finite := func(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
	in := func(x, lo, hi float64) bool { return finite(x) && x >= lo && x <= hi }
	ids := map[string]bool{}
	unique := func(id string) error {
		if !clothID.MatchString(id) {
			return fmt.Errorf("%q: an id like curtain.yatai_f5", id)
		}
		if ids[id] {
			return fmt.Errorf("%s twice", id)
		}
		ids[id] = true
		return nil
	}
	drafts := map[string]ClothDraft{}
	for _, d := range c.Drafts {
		if err := unique(d.ID); err != nil {
			return fmt.Errorf("draft %w", err)
		}
		switch {
		case !known(d.LevelID):
			return fmt.Errorf("draft %s: unknown level %s", d.ID, d.LevelID)
		case d.Source != "convection" && d.Source != "vent":
			return fmt.Errorf("draft %s: source %q is convection or vent (a supply outlet nearby); there is no wind in a dome", d.ID, d.Source)
		case d.Ru == "":
			return fmt.Errorf("draft %s: say where the air comes from and why it reaches the cloth", d.ID)
		case !in(d.DriftDeg, 0.05, 3):
			return fmt.Errorf("draft %s: drift_deg %v is 0.05..3 degrees (indoor air only sways cloth a little)", d.ID, d.DriftDeg)
		case !in(d.PeriodS, 4, 30):
			return fmt.Errorf("draft %s: period_s %v is 4..30 s (a draft is slow; faster is a twitch)", d.ID, d.PeriodS)
		}
		drafts[d.ID] = d
	}
	draftOn := func(id, level, who string) error {
		if id == "" {
			return nil
		}
		d, ok := drafts[id]
		if !ok {
			return fmt.Errorf("%s: unknown draft %s", who, id)
		}
		if d.LevelID != level {
			return fmt.Errorf("%s: draft %s blows on %s, not on its level %s", who, id, d.LevelID, level)
		}
		return nil
	}
	for _, s := range c.Curtains {
		if err := unique(s.ID); err != nil {
			return fmt.Errorf("curtain %w", err)
		}
		if !known(s.LevelID) {
			return fmt.Errorf("curtain %s: unknown level %s", s.ID, s.LevelID)
		}
		if s.Instance == "" || s.Why == "" {
			return fmt.Errorf("curtain %s: name its host instance and say why it hangs there", s.ID)
		}
		if len(s.Rows) == 0 {
			return fmt.Errorf("curtain %s: no row of strips", s.ID)
		}
		nodes := map[string]bool{}
		rows := map[string]bool{}
		for _, r := range s.Rows {
			if r.ID == "" || rows[r.ID] || len(r.Nodes) == 0 {
				return fmt.Errorf("curtain %s: every row has its own id and at least one strip", s.ID)
			}
			rows[r.ID] = true
			for _, n := range r.Nodes {
				if !clothNode.MatchString(n) || nodes[n] {
					return fmt.Errorf("curtain %s: strip node %q is a model node name, once per curtain", s.ID, n)
				}
				nodes[n] = true
			}
		}
		b := s.Strip
		switch {
		case !in(b.LengthM, 0.2, 3) || !in(b.WidthM, 0.05, 1) || !in(b.ThicknessMM, 1, 20):
			return fmt.Errorf("curtain %s: a strip is 0.2..3 m long, 0.05..1 m wide, 1..20 mm thick", s.ID)
		case !in(b.DensityKgM3, 900, 2000) || b.MaterialNote == "":
			return fmt.Errorf("curtain %s: name the strip's material and its density 900..2000 kg/m3 (PVC 1250..1450)", s.ID)
		case !in(s.ClampM, 0.01, b.LengthM/2):
			return fmt.Errorf("curtain %s: clamp_m %v is 0.01 m..half the strip", s.ID, s.ClampM)
		}
		rod := stripPendulumPeriod(b.LengthM)
		w := s.Swing
		switch {
		case !in(w.PeriodS, 0.85*rod, 1.15*rod):
			return fmt.Errorf("curtain %s: swing period_s %v: a %.2f m strip hanging from its top swings in %.2f s (gravity); 0.85..1.15 of it", s.ID, w.PeriodS, b.LengthM, rod)
		case !in(w.DampingRatio, 0.05, 0.4):
			return fmt.Errorf("curtain %s: swing damping_ratio %v is 0.05..0.4 (a heavy strip settles in a few swings)", s.ID, w.DampingRatio)
		case swingsToSettle(w.DampingRatio) > 8 || swingsToSettle(w.DampingRatio) < 1:
			return fmt.Errorf("curtain %s: damping ratio %v settles to 5 %% in %.1f swings; 1..8", s.ID, w.DampingRatio, swingsToSettle(w.DampingRatio))
		case !in(w.MaxDeg, 10, 85):
			return fmt.Errorf("curtain %s: swing max_deg %v is 10..85 degrees", s.ID, w.MaxDeg)
		}
		n := s.Bend
		switch {
		case !in(n.PeriodS, 0.2, 0.75*w.PeriodS):
			return fmt.Errorf("curtain %s: bend period_s %v is 0.2 s..0.75 of the swing's (the bend is the faster mode)", s.ID, n.PeriodS)
		case !in(n.DampingRatio, 0.1, 0.9):
			return fmt.Errorf("curtain %s: bend damping_ratio %v is 0.1..0.9", s.ID, n.DampingRatio)
		case !in(n.Share, 0, 1.5):
			return fmt.Errorf("curtain %s: bend share %v is 0 (a board)..1.5 (a hanging chain is about 1.3)", s.ID, n.Share)
		case !in(n.MaxDeg, 0, 45):
			return fmt.Errorf("curtain %s: bend max_deg %v is 0..45 degrees (a strip bends a little)", s.ID, n.MaxDeg)
		}
		switch {
		case !in(s.NeighbourShare, 0, 0.3):
			return fmt.Errorf("curtain %s: neighbour_share %v is 0..0.3 (touching edges nudge a neighbour slightly)", s.ID, s.NeighbourShare)
		case !in(s.ContactMarginM, 0, 0.05):
			return fmt.Errorf("curtain %s: contact_margin_m %v is 0..0.05 m", s.ID, s.ContactMarginM)
		}
		if err := draftOn(s.Draft, s.LevelID, "curtain "+s.ID); err != nil {
			return err
		}
		if s.Clear != nil {
			if err := checkClearSheet(*s.Clear, b.LengthM, in); err != nil {
				return fmt.Errorf("curtain %s: clear: %w", s.ID, err)
			}
		}
	}
	for _, d := range c.Drapes {
		if err := unique(d.ID); err != nil {
			return fmt.Errorf("drape %w", err)
		}
		switch {
		case !known(d.LevelID):
			return fmt.Errorf("drape %s: unknown level %s", d.ID, d.LevelID)
		case !clothNode.MatchString(d.Node) || d.Why == "":
			return fmt.Errorf("drape %s: name its scene node and say why it hangs there", d.ID)
		case !in(d.LengthM, 0.2, 10) || !in(d.ClampM, 0.01, d.LengthM/2):
			return fmt.Errorf("drape %s: length 0.2..10 m, clamp 0.01 m..half of it", d.ID)
		case d.Draft == "":
			return fmt.Errorf("drape %s: cloth only the air moves needs the draft that moves it (without one it is not a drape)", d.ID)
		}
		if err := draftOn(d.Draft, d.LevelID, "drape "+d.ID); err != nil {
			return err
		}
	}
	return nil
}

// checkClearSheet: a clear sheet still reads as a sheet (some tint, some gloss) and shows what is behind it (never opaque,
// never a mirror); its frost lies on the strip.
func checkClearSheet(c ClearSheet, length float64, in func(x, lo, hi float64) bool) error {
	for _, t := range c.Tint {
		if !in(t, 0.5, 1) {
			return fmt.Errorf("tint %v: each channel 0.5..1 (light through a clear sheet, 1 is no tint)", c.Tint)
		}
	}
	switch {
	case !in(c.Body, 0.1, 0.8):
		return fmt.Errorf("body %v is 0.1..0.8 (the share of the view through the sheet that takes its tint; more is not clear)", c.Body)
	case !in(c.Edge, 0, 1):
		return fmt.Errorf("edge %v is 0..1", c.Edge)
	case !in(c.Roughness, 0.02, 0.8) || !in(c.Specular, 0, 1):
		return fmt.Errorf("roughness %v is 0.02..0.8 and specular %v 0..1", c.Roughness, c.Specular)
	case !in(c.DistortionM, 0, 0.05):
		return fmt.Errorf("distortion_m %v is 0..0.05 m (a little)", c.DistortionM)
	case !in(c.Ripple.TiltDeg, 0, 10) || !in(c.Ripple.WavesPerM, 0, 20):
		return fmt.Errorf("ripple: tilt 0..10 degrees, 0..20 waves a metre")
	case !in(c.Frost.FromM, 0, length) || !in(c.Frost.ToM, c.Frost.FromM, length):
		return fmt.Errorf("frost %v..%v m: a band on the %v m strip, from below to", c.Frost.FromM, c.Frost.ToM, length)
	case !in(c.Frost.Amount, 0, 0.9) || !in(c.Frost.ScuffsPerM, 0, 60):
		return fmt.Errorf("frost amount %v is 0..0.9 (a frosted band still shows light through), scuffs 0..60 a metre", c.Frost.Amount)
	case c.Why == "":
		return fmt.Errorf("say why the strips are clear (the world's reason)")
	}
	return nil
}

// clientClothMotion: per level, the curtains and drapes with their draft's drift resolved (0: still air). The reasons
// stay in CUE.
func clientClothMotion(c *ClothMotion) map[string]any {
	drafts := map[string]ClothDraft{}
	for _, d := range c.Drafts {
		drafts[d.ID] = d
	}
	drift := func(id string) (float64, float64) {
		if d, ok := drafts[id]; ok {
			return d.DriftDeg, d.PeriodS
		}
		return 0, 0
	}
	levels := map[string]map[string][]any{}
	level := func(id string) map[string][]any {
		if levels[id] == nil {
			levels[id] = map[string][]any{"curtains": {}, "drapes": {}}
		}
		return levels[id]
	}
	for _, s := range c.Curtains {
		rows := []any{}
		for _, r := range s.Rows {
			rows = append(rows, map[string]any{"id": r.ID, "nodes": r.Nodes})
		}
		deg, period := drift(s.Draft)
		l := level(s.LevelID)
		l["curtains"] = append(l["curtains"], map[string]any{"id": s.ID, "instance": s.Instance, "rows": rows,
			"length_m": s.Strip.LengthM, "width_m": s.Strip.WidthM, "clamp_m": s.ClampM,
			"swing":           map[string]any{"period_s": s.Swing.PeriodS, "damping_ratio": s.Swing.DampingRatio, "max_deg": s.Swing.MaxDeg},
			"bend":            map[string]any{"period_s": s.Bend.PeriodS, "damping_ratio": s.Bend.DampingRatio, "share": s.Bend.Share, "max_deg": s.Bend.MaxDeg},
			"neighbour_share": s.NeighbourShare, "contact_margin_m": s.ContactMarginM, "drift_deg": deg, "drift_period_s": period})
		if s.Clear != nil {
			k := s.Clear
			curtains := l["curtains"]
			curtains[len(curtains)-1].(map[string]any)["clear"] = map[string]any{"tint": k.Tint[:], "body": k.Body, "edge": k.Edge,
				"roughness": k.Roughness, "specular": k.Specular, "distortion_m": k.DistortionM,
				"ripple": map[string]any{"tilt_deg": k.Ripple.TiltDeg, "waves_per_m": k.Ripple.WavesPerM},
				"frost":  map[string]any{"from_m": k.Frost.FromM, "to_m": k.Frost.ToM, "amount": k.Frost.Amount, "scuffs_per_m": k.Frost.ScuffsPerM}}
		}
	}
	for _, d := range c.Drapes {
		deg, period := drift(d.Draft)
		l := level(d.LevelID)
		l["drapes"] = append(l["drapes"], map[string]any{"id": d.ID, "node": d.Node, "length_m": d.LengthM, "clamp_m": d.ClampM,
			"drift_deg": deg, "drift_period_s": period})
	}
	out := map[string]any{}
	for id, l := range levels {
		out[id] = l
	}
	return map[string]any{"levels": out}
}
