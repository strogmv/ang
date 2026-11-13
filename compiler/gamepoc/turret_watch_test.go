package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func turretWatchLifeFixture() json.RawMessage {
	row := strings.Repeat(".", 40)
	rows := make([]string, 30)
	for i := range rows {
		rows[i] = row
	}
	raw, _ := json.Marshal(map[string]any{
		"locations": map[string]any{"hub": map[string]any{"cell_mm": 500, "terrain": rows}},
		"actors": []any{
			map[string]any{"id": "char.guard", "steps": []any{map[string]any{"level": "hub", "label": "duty.post"}}},
			map[string]any{"id": "char.walker", "steps": []any{map[string]any{"level": "hub", "label": "walk"}}},
		},
	})
	return raw
}

func turretWatchCatalogFixture() TurretWatchCatalog {
	c := TurretWatchCatalog{
		Rule: TurretWatchRule{RangeM: 5, HoldMS: 2000, SwitchMarginM: .5, AimHeightM: 1.3, PanSlewDegS: 90, TiltSlewDegS: 60, ReturnSlewDegS: 30,
			PanHalfRangeDeg: 100, TiltMinDeg: -75, TiltMaxDeg: 15, EyeRiseMS: 300, EyeFallMS: 1500},
		Look: TurretWatchLook{EyeRestEnergy: .6, EyeWatchEnergy: 4, EyeLightEnergy: .6, EyeLightRangeM: .7, EyeLightColor: "#FF2A1A"},
	}
	p := TurretWatchPost{ID: "th.turret.a", LevelID: "hub", InstanceID: "turret.a", GuardPost: "post", StaffLabel: "duty.post", Guards: []string{"char.guard"},
		PivotM: []float64{8, 3.7, 2.98}, FacingDeg: 90, RestAimM: []float64{10, 4.5, 1}, PanNode: "a.pan", TiltNode: "a.tilt", EyeNodes: []string{"a.eye"}}
	p.Object.ID, p.Object.XM, p.Object.YM = "door", 10, 2.75
	c.Posts = []TurretWatchPost{p}
	return c
}

func TestTurretWatchDerivesTheRestAim(t *testing.T) {
	c := turretWatchCatalogFixture()
	if err := c.validate(turretWatchLifeFixture()); err != nil {
		t.Fatal(err)
	}
	// From (8, 3.7, 2.98) to (10, 4.5, 1): 21.8 degrees south of east, 42.6 degrees down.
	if p := c.Posts[0]; p.RestHeadingMdeg != 21801 || p.RestPitchMdeg != -42589 {
		t.Fatalf("rest aim heading %d pitch %d", p.RestHeadingMdeg, p.RestPitchMdeg)
	}
	if turretWatchWrap(190000) != -170000 || turretWatchWrap(-180000) != 180000 || turretWatchWrap(540000) != 180000 {
		t.Fatal("angle wrap")
	}
}

func TestTurretWatchRejectsPostsItCannotRun(t *testing.T) {
	cases := map[string]func(*TurretWatchCatalog){
		"range too short":        func(c *TurretWatchCatalog) { c.Rule.RangeM = .5 },
		"no slew":                func(c *TurretWatchCatalog) { c.Rule.PanSlewDegS = 0 },
		"tilt up only":           func(c *TurretWatchCatalog) { c.Rule.TiltMinDeg = 5 },
		"eye never lights":       func(c *TurretWatchCatalog) { c.Rule.EyeRiseMS = 0 },
		"look colour":            func(c *TurretWatchCatalog) { c.Look.EyeLightColor = "red" },
		"unknown level":          func(c *TurretWatchCatalog) { c.Posts[0].LevelID = "nowhere" },
		"pivot outside":          func(c *TurretWatchCatalog) { c.Posts[0].PivotM[0] = 40 },
		"pivot below the people": func(c *TurretWatchCatalog) { c.Posts[0].PivotM[2] = 1 },
		"object out of range":    func(c *TurretWatchCatalog) { c.Posts[0].Object.XM = 18 },
		"no guards":              func(c *TurretWatchCatalog) { c.Posts[0].Guards = nil },
		"guard without the duty": func(c *TurretWatchCatalog) { c.Posts[0].Guards = []string{"char.walker"} },
		"duplicate node":         func(c *TurretWatchCatalog) { c.Posts[0].TiltNode = "a.pan" },
		"no eye":                 func(c *TurretWatchCatalog) { c.Posts[0].EyeNodes = nil },
		"rest behind the wall":   func(c *TurretWatchCatalog) { c.Posts[0].RestAimM = []float64{8, .5, 1} },
		"rest aim straight down": func(c *TurretWatchCatalog) { c.Posts[0].RestAimM = []float64{8.05, 3.7, 0} },
		"duplicate post": func(c *TurretWatchCatalog) {
			c.Posts = append(c.Posts, c.Posts[0])
			c.Posts[1].PivotM, c.Posts[1].RestAimM = append([]float64(nil), c.Posts[0].PivotM...), append([]float64(nil), c.Posts[0].RestAimM...)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := turretWatchCatalogFixture()
			mutate(&c)
			if err := c.validate(turretWatchLifeFixture()); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
