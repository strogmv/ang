package gamepoc

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func clothMotionFixture() (ClothMotion, func(string) bool) {
	c := ClothMotion{
		Drafts: []ClothDraft{{ID: "draft.hall_convection", LevelID: "level.market", Source: "convection",
			Ru: "warm air rises to the dome and sinks", DriftDeg: 0.6, PeriodS: 12}},
		Curtains: []StripCurtain{{ID: "curtain.yatai", LevelID: "level.market", Instance: "yatai.F5",
			Rows:   []StripRow{{ID: "south", Nodes: []string{"yatai.strip.S00", "yatai.strip.S01"}}, {ID: "east", Nodes: []string{"yatai.strip.E0"}}},
			Strip:  StripBody{LengthM: 1.08, WidthM: 0.296, ThicknessMM: 6, DensityKgM3: 1350, MaterialNote: "welding PVC"},
			ClampM: 0.12, Swing: ClothSwing{PeriodS: 1.7, DampingRatio: 0.14, MaxDeg: 75},
			Bend:           ClothBend{PeriodS: 0.74, DampingRatio: 0.3, Share: 0.6, MaxDeg: 25},
			NeighbourShare: 0.06, ContactMarginM: 0.015, Why: "the yatai's noren",
			Clear: &ClearSheet{Tint: [3]float64{0.96, 0.88, 0.7}, Body: 0.5, Edge: 0.6, Roughness: 0.2, Specular: 0.5, DistortionM: 0.015,
				Ripple: SheetRipple{TiltDeg: 2, WavesPerM: 2.5}, Frost: SheetFrost{FromM: 0, ToM: 0.55, Amount: 0.5, ScuffsPerM: 11},
				Why: "clear like a cold-store door: the cook sees who comes"}}},
		Drapes: []ClothDrape{{ID: "drape.banners", LevelID: "level.market", Node: "DomeBanners", LengthM: 2.2, ClampM: 0.1,
			Draft: "draft.hall_convection", Why: "banners under the dome"}},
	}
	return c, func(id string) bool { return id == "level.market" || id == "level.hub" }
}

func TestClothMotionAcceptedAndProjected(t *testing.T) {
	c, known := clothMotionFixture()
	if err := checkClothMotion(c, known); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(clientClothMotion(&c))
	s := string(raw)
	for _, want := range []string{`"level.market":{`, `"instance":"yatai.F5"`, `"nodes":["yatai.strip.S00","yatai.strip.S01"]`,
		`"period_s":1.7`, `"damping_ratio":0.14`, `"share":0.6`, `"length_m":1.08`, `"clamp_m":0.12`,
		`"node":"DomeBanners"`, `"drift_deg":0.6`, `"drift_period_s":12`,
		`"clear":{`, `"tint":[0.96,0.88,0.7]`, `"body":0.5`, `"distortion_m":0.015`, `"tilt_deg":2`, `"to_m":0.55`, `"scuffs_per_m":11`} {
		if !strings.Contains(s, want) {
			t.Fatalf("client cloth lacks %s: %s", want, s)
		}
	}
	// The curtain names no draft: it hangs in still air (drift 0), only people move it.
	var view struct {
		Levels map[string]struct {
			Curtains []map[string]any `json:"curtains"`
		} `json:"levels"`
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	if got := view.Levels["level.market"].Curtains[0]["drift_deg"]; got != 0.0 {
		t.Fatalf("a curtain without a draft drifts: %v", got)
	}
	if strings.Contains(s, `"why"`) || strings.Contains(s, `"ru"`) {
		t.Fatalf("the reasons stay in CUE: %s", s)
	}
}

// The yatai's 1.08 m strip swings on its top edge in about 1.7 s, and a damping ratio of 0.14 settles it in about 3.3 swings.
func TestClothPendulumNumbers(t *testing.T) {
	if p := stripPendulumPeriod(1.08); math.Abs(p-1.705) > 0.01 {
		t.Fatalf("period of a 1.08 m strip: %.3f s", p)
	}
	if n := swingsToSettle(0.14); n < 3.2 || n > 3.5 {
		t.Fatalf("swings to settle at 0.14: %.2f", n)
	}
}

func TestClothMotionRejected(t *testing.T) {
	cases := map[string]func(c *ClothMotion){
		"wind":            func(c *ClothMotion) { c.Drafts[0].Source = "wind" },
		"draft_no_reason": func(c *ClothMotion) { c.Drafts[0].Ru = "" },
		"draft_strong":    func(c *ClothMotion) { c.Drafts[0].DriftDeg = 8 },
		"draft_fast":      func(c *ClothMotion) { c.Drafts[0].PeriodS = 1.1 },
		"draft_level":     func(c *ClothMotion) { c.Drafts[0].LevelID = "level.nowhere" },
		"id_twice":        func(c *ClothMotion) { c.Drapes[0].ID = "curtain.yatai" },
		"curtain_level":   func(c *ClothMotion) { c.Curtains[0].LevelID = "level.nowhere" },
		"no_host":         func(c *ClothMotion) { c.Curtains[0].Instance = "" },
		"no_rows":         func(c *ClothMotion) { c.Curtains[0].Rows = nil },
		"empty_row":       func(c *ClothMotion) { c.Curtains[0].Rows[1].Nodes = nil },
		"node_twice":      func(c *ClothMotion) { c.Curtains[0].Rows[1].Nodes = []string{"yatai.strip.S00"} },
		"row_twice":       func(c *ClothMotion) { c.Curtains[0].Rows[1].ID = "south" },
		"strip_long":      func(c *ClothMotion) { c.Curtains[0].Strip.LengthM = 5 },
		"no_material":     func(c *ClothMotion) { c.Curtains[0].Strip.MaterialNote = "" },
		"density":         func(c *ClothMotion) { c.Curtains[0].Strip.DensityKgM3 = 300 },
		"clamp_long":      func(c *ClothMotion) { c.Curtains[0].ClampM = 0.8 },
		"period_fast":     func(c *ClothMotion) { c.Curtains[0].Swing.PeriodS = 1.1 },
		"period_slow":     func(c *ClothMotion) { c.Curtains[0].Swing.PeriodS = 2.5 },
		"undamped":        func(c *ClothMotion) { c.Curtains[0].Swing.DampingRatio = 0.01 },
		"overdamped":      func(c *ClothMotion) { c.Curtains[0].Swing.DampingRatio = 0.9 },
		"swing_max":       func(c *ClothMotion) { c.Curtains[0].Swing.MaxDeg = 120 },
		"bend_slow":       func(c *ClothMotion) { c.Curtains[0].Bend.PeriodS = 1.6 },
		"bend_large":      func(c *ClothMotion) { c.Curtains[0].Bend.MaxDeg = 80 },
		"bend_share":      func(c *ClothMotion) { c.Curtains[0].Bend.Share = 2 },
		"neighbours":      func(c *ClothMotion) { c.Curtains[0].NeighbourShare = 0.8 },
		"margin":          func(c *ClothMotion) { c.Curtains[0].ContactMarginM = 0.2 },
		"unknown_draft":   func(c *ClothMotion) { c.Curtains[0].Draft = "draft.fan" },
		"draft_other_lvl": func(c *ClothMotion) {
			c.Curtains[0].LevelID = "level.hub"
			c.Curtains[0].Draft = "draft.hall_convection"
		},
		"no_why":           func(c *ClothMotion) { c.Curtains[0].Why = "" },
		"drape_no_draft":   func(c *ClothMotion) { c.Drapes[0].Draft = "" },
		"drape_node":       func(c *ClothMotion) { c.Drapes[0].Node = "" },
		"drape_clamp":      func(c *ClothMotion) { c.Drapes[0].ClampM = 2 },
		"bad_id":           func(c *ClothMotion) { c.Curtains[0].ID = "Yatai" },
		"nan_period":       func(c *ClothMotion) { c.Curtains[0].Swing.PeriodS = math.NaN() },
		"negative_density": func(c *ClothMotion) { c.Curtains[0].Strip.DensityKgM3 = -1 },
		"clear_opaque":     func(c *ClothMotion) { c.Curtains[0].Clear.Body = 1 },
		"clear_dark_tint":  func(c *ClothMotion) { c.Curtains[0].Clear.Tint = [3]float64{0.2, 0.2, 0.2} },
		"clear_mirror":     func(c *ClothMotion) { c.Curtains[0].Clear.Specular = 3 },
		"clear_warp":       func(c *ClothMotion) { c.Curtains[0].Clear.DistortionM = 0.3 },
		"clear_ripple":     func(c *ClothMotion) { c.Curtains[0].Clear.Ripple.TiltDeg = 30 },
		"frost_off_strip":  func(c *ClothMotion) { c.Curtains[0].Clear.Frost.ToM = 2 },
		"frost_upside":     func(c *ClothMotion) { c.Curtains[0].Clear.Frost.FromM = 0.6 },
		"frost_opaque":     func(c *ClothMotion) { c.Curtains[0].Clear.Frost.Amount = 1 },
		"clear_no_why":     func(c *ClothMotion) { c.Curtains[0].Clear.Why = "" },
		"clear_nan":        func(c *ClothMotion) { c.Curtains[0].Clear.Edge = math.NaN() },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c, known := clothMotionFixture()
			clear := *c.Curtains[0].Clear
			c.Curtains[0].Clear = &clear
			c.Curtains[0].Rows = append([]StripRow{}, c.Curtains[0].Rows...)
			c.Curtains[0].Rows[1].Nodes = append([]string{}, c.Curtains[0].Rows[1].Nodes...)
			mutate(&c)
			if err := checkClothMotion(c, known); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
