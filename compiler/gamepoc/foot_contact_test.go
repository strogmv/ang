package gamepoc

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func footContactFixture() FootContact {
	return FootContact{Hero: true, BlendS: 0.08, MaxLiftM: 0.12, MaxDropM: 0.06, MaxPelvisM: 0.06, Reach: 0.995, MaxTiltDeg: 15, FlatBandM: 0.03, TiltFromM: 0.01}
}

func TestFootContactAcceptedAndProjected(t *testing.T) {
	c := footContactFixture()
	if err := checkFootContact(c); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(clientFootContact(&c))
	want := `{"blend_s":0.08,"flat_band_m":0.03,"hero":true,"max_drop_m":0.06,"max_lift_m":0.12,"max_pelvis_m":0.06,"max_tilt_deg":15,"reach":0.995,"tilt_from_m":0.01}`
	if string(raw) != want {
		t.Fatalf("client foot contact:\n got %s\nwant %s", raw, want)
	}
	// Edges that are still a sole on the floor: no lift at all, no drop and no pelvis, the longest reach below 1.
	for _, edge := range []func(c *FootContact){
		func(c *FootContact) { c.MaxLiftM = 0 },
		func(c *FootContact) { c.MaxDropM, c.MaxPelvisM = 0, 0 },
		func(c *FootContact) { c.MaxLiftM, c.MaxDropM, c.MaxPelvisM = 0.2, 0.2, 0.2 },
		func(c *FootContact) { c.Reach = 0.9 },
		func(c *FootContact) { c.BlendS = 0.5 },
		func(c *FootContact) { c.Hero = false },
		func(c *FootContact) { c.MaxTiltDeg = 0 },
		func(c *FootContact) { c.MaxTiltDeg, c.FlatBandM = 30, 0.1 },
		func(c *FootContact) { c.TiltFromM = 0 },
		func(c *FootContact) { c.TiltFromM = 0.05 },
	} {
		c := footContactFixture()
		edge(&c)
		if err := checkFootContact(c); err != nil {
			t.Fatalf("%+v: %v", c, err)
		}
	}
}

func TestFootContactRejected(t *testing.T) {
	cases := map[string]func(c *FootContact){
		"blend_zero":      func(c *FootContact) { c.BlendS = 0 },
		"blend_negative":  func(c *FootContact) { c.BlendS = -0.1 },
		"blend_slow":      func(c *FootContact) { c.BlendS = 0.6 },
		"blend_nan":       func(c *FootContact) { c.BlendS = math.NaN() },
		"lift_negative":   func(c *FootContact) { c.MaxLiftM = -0.01 },
		"lift_step":       func(c *FootContact) { c.MaxLiftM = 0.25 },
		"lift_inf":        func(c *FootContact) { c.MaxLiftM = math.Inf(1) },
		"drop_negative":   func(c *FootContact) { c.MaxDropM = -0.01 },
		"drop_step":       func(c *FootContact) { c.MaxDropM, c.MaxPelvisM = 0.3, 0.05 },
		"drop_nan":        func(c *FootContact) { c.MaxDropM = math.NaN() },
		"pelvis_negative": func(c *FootContact) { c.MaxPelvisM = -0.01 },
		"pelvis_deeper":   func(c *FootContact) { c.MaxPelvisM = 0.07 },
		"pelvis_inf":      func(c *FootContact) { c.MaxPelvisM = math.Inf(1) },
		"reach_one":       func(c *FootContact) { c.Reach = 1 },
		"reach_short":     func(c *FootContact) { c.Reach = 0.8 },
		"reach_nan":       func(c *FootContact) { c.Reach = math.NaN() },
		"tilt_negative":   func(c *FootContact) { c.MaxTiltDeg = -1 },
		"tilt_steep":      func(c *FootContact) { c.MaxTiltDeg = 45 },
		"tilt_nan":        func(c *FootContact) { c.MaxTiltDeg = math.NaN() },
		"band_zero":       func(c *FootContact) { c.FlatBandM = 0 },
		"band_high":       func(c *FootContact) { c.FlatBandM = 0.2 },
		"band_inf":        func(c *FootContact) { c.FlatBandM = math.Inf(1) },
		"from_negative":   func(c *FootContact) { c.TiltFromM = -0.01 },
		"from_large":      func(c *FootContact) { c.TiltFromM = 0.06 },
		"from_nan":        func(c *FootContact) { c.TiltFromM = math.NaN() },
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			c := footContactFixture()
			spoil(&c)
			if err := checkFootContact(c); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

// The block as CUE writes it: read, checked and refused by its type, unknown fields and bad numbers alike.
func TestFootContactRead(t *testing.T) {
	ctx := cuecontext.New()
	good := ctx.CompileString(`foot_contact: {hero: true, blend_s: 0.08, max_lift_m: 0.12, max_drop_m: 0.06, max_pelvis_m: 0.06, reach: 0.995, max_tilt_deg: 15, flat_band_m: 0.03, tilt_from_m: 0.01}`)
	c, err := readFootContact(good)
	if err != nil || c == nil || *c != footContactFixture() {
		t.Fatalf("read %+v, %v", c, err)
	}
	if c, err := readFootContact(ctx.CompileString(`other: 1`)); err != nil || c != nil {
		t.Fatalf("a world without the block: %+v, %v", c, err)
	}
	for name, src := range map[string]string{
		"unknown_field": `foot_contact: {hero: true, blend_s: 0.08, max_lift_m: 0.12, max_drop_m: 0.06, max_pelvis_m: 0.06, reach: 0.995, max_tilt_deg: 15, flat_band_m: 0.03, tilt_from_m: 0.01, looks: true}`,
		"string_blend":  `foot_contact: {hero: true, blend_s: "fast", max_lift_m: 0.12, max_drop_m: 0.06, max_pelvis_m: 0.06, reach: 0.995, max_tilt_deg: 15, flat_band_m: 0.03, tilt_from_m: 0.01}`,
		"bool_lift":     `foot_contact: {hero: true, blend_s: 0.08, max_lift_m: true, max_drop_m: 0.06, max_pelvis_m: 0.06, reach: 0.995, max_tilt_deg: 15, flat_band_m: 0.03, tilt_from_m: 0.01}`,
		"list_reach":    `foot_contact: {hero: true, blend_s: 0.08, max_lift_m: 0.12, max_drop_m: 0.06, max_pelvis_m: 0.06, reach: [0.99], max_tilt_deg: 15, flat_band_m: 0.03, tilt_from_m: 0.01}`,
		"string_hero":   `foot_contact: {hero: "yes", blend_s: 0.08, max_lift_m: 0.12, max_drop_m: 0.06, max_pelvis_m: 0.06, reach: 0.995, max_tilt_deg: 15, flat_band_m: 0.03, tilt_from_m: 0.01}`,
		"bad_number":    `foot_contact: {hero: true, blend_s: 0.08, max_lift_m: 0.5, max_drop_m: 0.06, max_pelvis_m: 0.06, reach: 0.995, max_tilt_deg: 15, flat_band_m: 0.03, tilt_from_m: 0.01}`,
		"string_tilt":   `foot_contact: {hero: true, blend_s: 0.08, max_lift_m: 0.12, max_drop_m: 0.06, max_pelvis_m: 0.06, reach: 0.995, max_tilt_deg: "flat", flat_band_m: 0.03, tilt_from_m: 0.01}`,
		"missing_band":  `foot_contact: {hero: true, blend_s: 0.08, max_lift_m: 0.12, max_drop_m: 0.06, max_pelvis_m: 0.06, reach: 0.995, max_tilt_deg: 15}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readFootContact(ctx.CompileString(src)); err == nil || !strings.Contains(err.Error(), "world.foot_contact") {
				t.Fatalf("accepted or unnamed: %v", err)
			}
		})
	}
}
