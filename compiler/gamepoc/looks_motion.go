package gamepoc

import (
	"fmt"
	"math"
	"regexp"
	"sort"
)

// The look's own motion over the shared clips (owner 15.11, two shots of Polina walking: the apron did not part between the
// legs, and the walk read hunched). Only the view: the server never sees it; the client draws it every frame.
//
// Swing: the cloth panels of a model (the two panels of a waist apron, build_character.py) carry a swing shape each — the panel
// turned forward about its hinge under the belt (the record's swing_shapes: side, thigh bone, the shape's angle, the share of
// its own thigh the panel's skin already takes at the hem, the contact heights along the thigh). The client drives each as a
// damped pendulum: it lags when the pelvis speeds up, the thigh of its side pushes it so that at every contact height the
// panel stays ahead of the thigh's front (never closer than ClearanceDeg), and it settles when the wearer stands. Posture: spine, neck and head pitch offsets after the clips, per clip group (walk: walk and run; stand: idle
// and the stances), degrees forward (negative: more upright) — «походка следует телу» (doc 09, 10.11): a look's own carriage,
// and world.art.posture_age adds a stoop by the wearer's age.

// LookSwing: the dynamics of a look's swing shapes (world.art.looks[].swing).
type LookSwing struct {
	PeriodS      float64 `json:"period_s"`
	DampingRatio float64 `json:"damping_ratio"`
	ClearanceDeg float64 `json:"clearance_deg"`
	PelvisGain   float64 `json:"pelvis_gain"`
}

// lookRecordSwing: one swing shape of a model in its record (build_character.py).
type lookRecordSwing struct {
	Side       string  `json:"side"`
	Thigh      string  `json:"thigh"`
	MaxDeg     float64 `json:"max_deg"`
	LengthM    float64 `json:"length_m"`
	ThighShare float64 `json:"thigh_share"`
	// The contact (rest, Blender axes, metres): the hinge's middle, the hem's middle, the trouser leg's front before the knee.
	HingeM     []float64 `json:"hinge_m"`
	HemM       []float64 `json:"hem_m"`
	KneeFrontM float64   `json:"knee_front_m"`
	// Contact along the thigh: at a few heights the trouser leg's most forward vertices and the panel's vertex before each,
	// with their skins; the client skins both and keeps each panel point in front of its trouser point. A record without them
	// falls back to the knee's front against the hem.
	Contact []lookSwingContact `json:"contact"`
	// The belt: the bones the panel's top hangs from and their shares (its skin at the hinge: the pelvis and the lower spine,
	// the build's names); the client turns the hinge with them. Empty: the hips.
	Belt map[string]float64 `json:"belt"`
}

// lookSwingContact: one contact of a swing shape (rest, Blender axes, metres): a trouser vertex of the leg's front and the
// panel's vertex before it, each with its skin (bone -> weight, the build's names).
type lookSwingContact struct {
	ThighM    []float64          `json:"thigh_m"`
	ThighSkin map[string]float64 `json:"thigh_skin"`
	PanelM    []float64          `json:"panel_m"`
	PanelSkin map[string]float64 `json:"panel_skin"`
	// Swing: the share of the swing shape's turn this panel point takes (what its skin lacks; 0.15..1).
	Swing float64 `json:"swing"`
}

// LookPosture: pitch offsets of the spine, neck and head per clip group (degrees forward; negative: more upright).
type LookPosture struct {
	Walk  map[string]float64 `json:"walk,omitempty"`
	Stand map[string]float64 `json:"stand,omitempty"`
}

// LookPostureAge: what a wearer of this age or older adds to his look's posture (world.art.posture_age; the highest band
// reached counts).
type LookPostureAge struct {
	FromAge int64 `json:"from_age"`
	LookPosture
}

var (
	// The bones a posture may pitch (Godot's humanoid names of the shared rig: the trunk, the neck and the head).
	lookPostureBones  = map[string]bool{"Spine": true, "Chest": true, "UpperChest": true, "Neck": true, "Head": true}
	lookPostureMaxDeg = 30.0
	// The swing: a cloth panel's period (a 0.4 m panel swings at about 1.1 s), its damping, the clearance from the thigh.
	lookSwingPeriod  = [2]float64{0.3, 3.0}
	lookSwingDamping = [2]float64{0.05, 1.0}
	lookSwingClear   = [2]float64{0.0, 20.0}
	lookSwingGain    = [2]float64{0.0, 3.0}
	// The bones a swing shape's belt may name (the build's names of the trunk the client maps: actor_looks.gd SWING_BELT_BONES).
	lookSwingBelt = map[string]bool{"DEF-hips": true, "DEF-spine.001": true, "DEF-spine.002": true, "DEF-spine.003": true}
	// The bones a contact point's skin may name (the trunk and the legs the client maps: actor_looks.gd SWING_BONES).
	lookSwingSkin = map[string]bool{"DEF-hips": true, "DEF-spine.001": true, "DEF-spine.002": true, "DEF-spine.003": true,
		"DEF-thigh.L": true, "DEF-thigh.R": true, "DEF-shin.L": true, "DEF-shin.R": true}
	// A swing shape's name: plain, its side last (apron_swing_L).
	lookSwingShape = regexp.MustCompile(`^[a-z][a-z0-9_]*_[LR]$`)
)

func within(x float64, r [2]float64) bool { return !math.IsNaN(x) && x >= r[0] && x <= r[1] }

// swingSkin: a contact point's skin — bones the client maps (actor_looks.gd SWING_BONES), shares 0..1 adding up to 1.
func swingSkin(w map[string]float64) bool {
	total := 0.0
	for b, x := range w {
		if !lookSwingSkin[b] || !within(x, [2]float64{0, 1}) {
			return false
		}
		total += x
	}
	return len(w) > 0 && math.Abs(total-1) <= 0.02
}

// checkLookSwing: a model with swing shapes is drawn with a swing and a swing needs them; each shape a side's panel with a
// thigh, an angle and a share; the dynamics within their ranges.
func checkLookSwing(key string, l Look, shapes map[string]lookRecordSwing, record string) error {
	if len(shapes) == 0 {
		if l.Swing != nil {
			return fmt.Errorf("art.looks %s: swing needs swing shapes in the model's record %s", key, record)
		}
		return nil
	}
	if l.Swing == nil {
		return fmt.Errorf("art.looks %s: the model has swing shapes in %s; give their swing (period, damping, clearance, pelvis gain)", key, record)
	}
	names := make([]string, 0, len(shapes))
	for n := range shapes {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		s := shapes[n]
		if !lookSwingShape.MatchString(n) || (s.Side != "L" && s.Side != "R") || s.Thigh == "" || !within(s.MaxDeg, [2]float64{5, 60}) ||
			!within(s.LengthM, [2]float64{0.05, 1.2}) || !within(s.ThighShare, [2]float64{0, 1}) || len(s.HingeM) != 3 || len(s.HemM) != 3 ||
			!within(s.KneeFrontM, [2]float64{0, 0.25}) || s.HemM[2] >= s.HingeM[2] {
			return fmt.Errorf("art.looks %s: record %s: swing shape %q needs a side L or R, its thigh, an angle of 5..60 degrees, a length, a share 0..1, its hinge above its hem and the knee's front 0..0.25 m", key, record, n)
		}
		total := 0.0
		for b, w := range s.Belt {
			if !lookSwingBelt[b] || !within(w, [2]float64{0, 1}) {
				return fmt.Errorf("art.looks %s: record %s: swing shape %q belt: %q must be the hips or a lower spine bone (DEF-hips, DEF-spine.001..003) with a share 0..1", key, record, n, b)
			}
			total += w
		}
		if len(s.Belt) > 0 && math.Abs(total-1) > 0.02 {
			return fmt.Errorf("art.looks %s: record %s: swing shape %q belt shares add up to %g, not 1", key, record, n, total)
		}
		for i, c := range s.Contact {
			if len(c.ThighM) != 3 || len(c.PanelM) != 3 || c.PanelM[2] > s.HingeM[2] || c.PanelM[2] < s.HemM[2]-0.05 ||
				math.Abs(c.ThighM[2]-c.PanelM[2]) > 0.2 || !swingSkin(c.ThighSkin) || !swingSkin(c.PanelSkin) || !within(c.Swing, [2]float64{0.15, 1}) {
				return fmt.Errorf("art.looks %s: record %s: swing shape %q contact %d needs a trouser point and a panel point (3 numbers each, the panel's between its hinge and its hem, within 0.2 m of the trouser point's height), each with a skin of the hips, the lower spine, the thighs and the shins adding up to 1, and the panel point's swing share 0.15..1", key, record, n, i)
			}
		}
	}
	w := l.Swing
	if !within(w.PeriodS, lookSwingPeriod) || !within(w.DampingRatio, lookSwingDamping) || !within(w.ClearanceDeg, lookSwingClear) ||
		!within(w.PelvisGain, lookSwingGain) {
		return fmt.Errorf("art.looks %s: swing: period %g..%g s, damping %g..%g, clearance %g..%g deg, pelvis gain %g..%g", key,
			lookSwingPeriod[0], lookSwingPeriod[1], lookSwingDamping[0], lookSwingDamping[1], lookSwingClear[0], lookSwingClear[1],
			lookSwingGain[0], lookSwingGain[1])
	}
	return nil
}

func checkPostureGroups(where string, p LookPosture) error {
	for group, m := range map[string]map[string]float64{"walk": p.Walk, "stand": p.Stand} {
		for bone, deg := range m {
			if !lookPostureBones[bone] {
				return fmt.Errorf("%s: posture %s: %q is not a trunk, neck or head bone (Spine, Chest, UpperChest, Neck, Head)", where, group, bone)
			}
			if math.IsNaN(deg) || math.Abs(deg) > lookPostureMaxDeg {
				return fmt.Errorf("%s: posture %s %s: %g degrees is beyond %g", where, group, bone, deg, lookPostureMaxDeg)
			}
		}
	}
	return nil
}

// checkPostureAge: bands from an adult age up, ascending, each with valid offsets.
func checkPostureAge(bands []LookPostureAge) error {
	last := int64(-1)
	for i, b := range bands {
		if b.FromAge < 16 || b.FromAge > 100 || b.FromAge <= last {
			return fmt.Errorf("art.posture_age[%d]: from_age %d must be 16..100 and above the band before", i, b.FromAge)
		}
		last = b.FromAge
		if err := checkPostureGroups(fmt.Sprintf("art.posture_age[%d]", i), b.LookPosture); err != nil {
			return err
		}
	}
	return nil
}
