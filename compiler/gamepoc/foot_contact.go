package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"

	"cuelang.org/go/cue"
)

// FootContact is world.foot_contact (foot contact v1, the game's art-source/hero/v2/briefs/foot-contact.md): after the clip
// pose the client keeps a person's soles on the floor — a foot whose sole points (the boot's outline, measured when the
// pack is built) go into the floor is lifted in any phase; in the clip's contact phase of that foot (measured from the clip
// when the pack is built) a foot hovering over a lower floor is lowered onto it and the pelvis comes down only as far as the
// longer leg needs; a boot the clip pitches toe down while its heel is near the floor is turned toe up toward flat (a rigid
// boot cannot bend at the toes as the clip's bare foot does). Two-bone IK on the shared UAL legs, the knee in the clip's
// plane, the foot keeps the clip's orientation but for that tilt.
// Presentation only: the body's place, speed and stride stay the server's and the clip's. The client
// (client-godot/foot_contact.gd) reads these numbers from generated/presentation.json foot_contact.
type FootContact struct {
	// Hero: the step is on for the hero (people in looks: not in v1 — their soles keep the floor).
	Hero bool `json:"hero"`
	// BlendS: seconds the step takes to come in and out at the edges of a contact phase (0 would pop).
	BlendS float64 `json:"blend_s"`
	// MaxLiftM: how far a foot may be lifted out of the floor; MaxDropM: how far a foot in contact may be lowered onto a
	// lower floor; MaxPelvisM: how far the pelvis may come down for it (never more than the foot may go down).
	MaxLiftM   float64 `json:"max_lift_m"`
	MaxDropM   float64 `json:"max_drop_m"`
	MaxPelvisM float64 `json:"max_pelvis_m"`
	// Reach: the share of a leg's length it may straighten to (below 1: the knee never snaps straight).
	Reach float64 `json:"reach"`
	// MaxTiltDeg: how far a boot the clip pitches toe down is turned toe up toward the floor in its contact phase (a rigid
	// boot stands flat), only while its heel is within FlatBandM of the floor.
	MaxTiltDeg float64 `json:"max_tilt_deg"`
	FlatBandM  float64 `json:"flat_band_m"`
	// TiltFromM: a front lower than the heel by up to this is the clip's own pitch (a standing foot, mid-stance): untouched.
	TiltFromM float64 `json:"tilt_from_m"`
}

func readFootContact(world cue.Value) (*FootContact, error) {
	v := world.LookupPath(cue.ParsePath("foot_contact"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.foot_contact: %w", err)
	}
	var out FootContact
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.foot_contact: %w", err)
	}
	if err := checkFootContact(out); err != nil {
		return nil, fmt.Errorf("world.foot_contact: %w", err)
	}
	return &out, nil
}

func checkFootContact(c FootContact) error {
	finite := func(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
	switch {
	case !finite(c.BlendS) || c.BlendS <= 0 || c.BlendS > 0.5:
		return fmt.Errorf("blend_s %v: more than 0 (0 would pop) and at most 0.5 s (a few frames, not a slow drift)", c.BlendS)
	case !finite(c.MaxLiftM) || c.MaxLiftM < 0 || c.MaxLiftM > 0.2:
		return fmt.Errorf("max_lift_m %v: 0..0.2 m (more is a step, not a sole on the floor)", c.MaxLiftM)
	case !finite(c.MaxDropM) || c.MaxDropM < 0 || c.MaxDropM > 0.2:
		return fmt.Errorf("max_drop_m %v: 0..0.2 m", c.MaxDropM)
	case !finite(c.MaxPelvisM) || c.MaxPelvisM < 0 || c.MaxPelvisM > c.MaxDropM:
		return fmt.Errorf("max_pelvis_m %v: 0..max_drop_m %v (the pelvis comes down only as far as a foot must)", c.MaxPelvisM, c.MaxDropM)
	case !finite(c.Reach) || c.Reach < 0.9 || c.Reach >= 1:
		return fmt.Errorf("reach %v: 0.9 to below 1 of the leg's length (1 lets the knee snap straight)", c.Reach)
	case !finite(c.MaxTiltDeg) || c.MaxTiltDeg < 0 || c.MaxTiltDeg > 30:
		return fmt.Errorf("max_tilt_deg %v: 0..30 degrees (more turns the clip's foot, not a boot onto the floor)", c.MaxTiltDeg)
	case !finite(c.FlatBandM) || c.FlatBandM <= 0 || c.FlatBandM > 0.1:
		return fmt.Errorf("flat_band_m %v: more than 0 and at most 0.1 m of heel over the floor", c.FlatBandM)
	case !finite(c.TiltFromM) || c.TiltFromM < 0 || c.TiltFromM > 0.05:
		return fmt.Errorf("tilt_from_m %v: 0..0.05 m of the clip's own toe-down pitch left alone", c.TiltFromM)
	}
	return nil
}

// clientFootContact: the numbers as the client reads them.
func clientFootContact(c *FootContact) map[string]any {
	return map[string]any{"hero": c.Hero, "blend_s": c.BlendS, "max_lift_m": c.MaxLiftM, "max_drop_m": c.MaxDropM,
		"max_pelvis_m": c.MaxPelvisM, "reach": c.Reach, "max_tilt_deg": c.MaxTiltDeg, "flat_band_m": c.FlatBandM,
		"tilt_from_m": c.TiltFromM}
}
