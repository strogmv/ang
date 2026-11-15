package gamepoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// motionFixture: the look fixture's model with the apron's two swing shapes in its record, its swing and posture in data,
// and the age bands.
func motionFixture(t *testing.T) (string, Art, *TurretWatchCatalog) {
	t.Helper()
	root, a, turrets := lookFixture(t)
	writeSwing(t, root, `{"apron_swing_L": {"side": "L", "thigh": "DEF-thigh.L", "max_deg": 35, "length_m": 0.42, "thigh_share": 0.45, "hinge_m": [0.08, -0.12, 0.96], "hem_m": [0.08, -0.15, 0.52], "knee_front_m": 0.07, "belt": {"DEF-hips": 0.6, "DEF-spine.001": 0.4}, "contact": [`+contactRow(`{"DEF-hips": 0.2, "DEF-thigh.L": 0.8}`, `{"DEF-hips": 0.9, "DEF-thigh.L": 0.1}`, 0.9)+`]},
		"apron_swing_R": {"side": "R", "thigh": "DEF-thigh.R", "max_deg": 35, "length_m": 0.42, "thigh_share": 0.45, "hinge_m": [0.08, -0.12, 0.96], "hem_m": [0.08, -0.15, 0.52], "knee_front_m": 0.07}}`)
	l := a.Looks["look.guard"]
	l.Swing = &LookSwing{PeriodS: 1.1, DampingRatio: 0.3, ClearanceDeg: 4, PelvisGain: 1}
	l.Posture = &LookPosture{Walk: map[string]float64{"Neck": -8, "Head": -10}, Stand: map[string]float64{"Head": -6}}
	a.Looks["look.guard"] = l
	a.PostureAge = []LookPostureAge{{FromAge: 55, LookPosture: LookPosture{Walk: map[string]float64{"UpperChest": 6}}},
		{FromAge: 70, LookPosture: LookPosture{Walk: map[string]float64{"UpperChest": 10, "Neck": 6}}}}
	return root, a, turrets
}

// swingWith: the fixture's left swing shape with one more field (contact, belt).
func swingWith(field string) string {
	return `{"apron_swing_L": {"side": "L", "thigh": "DEF-thigh.L", "max_deg": 35, "length_m": 0.42, "thigh_share": 0.45, "hinge_m": [0.08, -0.12, 0.96], "hem_m": [0.08, -0.15, 0.52], "knee_front_m": 0.07, ` + field + `}}`
}

// contactRow: one contact point pair at mid-thigh with the given skins and swing share.
func contactRow(thighSkin, panelSkin string, swing float64) string {
	return fmt.Sprintf(`{"thigh_m": [0.08, -0.12, 0.75], "thigh_skin": %s, "panel_m": [0.08, -0.13, 0.75], "panel_skin": %s, "swing": %g}`, thighSkin, panelSkin, swing)
}

func writeSwing(t *testing.T, root, swing string) {
	t.Helper()
	path := filepath.Join(root, "art-source/caste/v1/look.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), `"surfaces"`, `"swing_shapes": `+swing+`, "surfaces"`, 1))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLookMotionAcceptedAndProjected(t *testing.T) {
	root, a, turrets := motionFixture(t)
	person := func(id string) bool { return id == "char.guard" }
	if err := checkLooks(root, a, person, turrets, nil); err != nil {
		t.Fatal(err)
	}
	looks, _ := clientLooks(a, turrets)
	raw, _ := json.Marshal(looks)
	for _, want := range []string{`"swing":{"period_s":1.1,"damping_ratio":0.3,"clearance_deg":4,"pelvis_gain":1}`,
		`"posture":{"walk":{"Head":-10,"Neck":-8},"stand":{"Head":-6}}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("client looks lack %s:\n%s", want, raw)
		}
	}
}

func TestLookMotionRejected(t *testing.T) {
	person := func(id string) bool { return id == "char.guard" }
	look := func(f func(l *Look)) func(*Art) {
		return func(a *Art) {
			l := a.Looks["look.guard"]
			f(&l)
			a.Looks["look.guard"] = l
		}
	}
	cases := map[string]func(a *Art){
		"swing period too short":   look(func(l *Look) { l.Swing.PeriodS = 0.1 }),
		"swing damping above 1":    look(func(l *Look) { l.Swing.DampingRatio = 1.5 }),
		"swing clearance negative": look(func(l *Look) { l.Swing.ClearanceDeg = -1 }),
		"pelvis gain too high":     look(func(l *Look) { l.Swing.PelvisGain = 4 }),
		"model swings, look not":   look(func(l *Look) { l.Swing = nil }),
		"posture bone unknown":     look(func(l *Look) { l.Posture.Walk["LeftUpperArm"] = -5 }),
		"posture beyond 30 deg":    look(func(l *Look) { l.Posture.Stand["Head"] = -45 }),
		"age band not ascending":   func(a *Art) { a.PostureAge[1].FromAge = 50 },
		"age band a child":         func(a *Art) { a.PostureAge[0].FromAge = 10 },
		"age band bone unknown": func(a *Art) {
			a.PostureAge[0].Walk = map[string]float64{"Hips": 5}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			root, a, turrets := motionFixture(t)
			mutate(&a)
			if err := checkLooks(root, a, person, turrets, nil); err == nil {
				t.Fatalf("%s: accepted", name)
			}
		})
	}
	records := map[string]string{
		"side unknown":              `{"apron_swing_L": {"side": "X", "thigh": "DEF-thigh.L", "max_deg": 35, "length_m": 0.42, "thigh_share": 0.45, "hinge_m": [0.08, -0.12, 0.96], "hem_m": [0.08, -0.15, 0.52], "knee_front_m": 0.07}}`,
		"angle too small":           `{"apron_swing_L": {"side": "L", "thigh": "DEF-thigh.L", "max_deg": 2, "length_m": 0.42, "thigh_share": 0.45, "hinge_m": [0.08, -0.12, 0.96], "hem_m": [0.08, -0.15, 0.52], "knee_front_m": 0.07}}`,
		"no thigh":                  `{"apron_swing_L": {"side": "L", "thigh": "", "max_deg": 35, "length_m": 0.42, "thigh_share": 0.45, "hinge_m": [0.08, -0.12, 0.96], "hem_m": [0.08, -0.15, 0.52], "knee_front_m": 0.07}}`,
		"share above one":           `{"apron_swing_L": {"side": "L", "thigh": "DEF-thigh.L", "max_deg": 35, "length_m": 0.42, "thigh_share": 1.5}}`,
		"hem above hinge":           `{"apron_swing_L": {"side": "L", "thigh": "DEF-thigh.L", "max_deg": 35, "length_m": 0.42, "thigh_share": 0, "hinge_m": [0.08, -0.12, 0.5], "hem_m": [0.08, -0.15, 0.9], "knee_front_m": 0.07}}`,
		"shape name plain":          `{"Apron Swing": {"side": "L", "thigh": "DEF-thigh.L", "max_deg": 35, "length_m": 0.42, "thigh_share": 0.45, "hinge_m": [0.08, -0.12, 0.96], "hem_m": [0.08, -0.15, 0.52], "knee_front_m": 0.07}}`,
		"contact skin unknown bone": swingWith(`"contact": [` + contactRow(`{"DEF-hand.L": 1}`, `{"DEF-hips": 1}`, 0.5) + `]`),
		"contact skin not one":      swingWith(`"contact": [` + contactRow(`{"DEF-thigh.L": 0.5}`, `{"DEF-hips": 1}`, 0.5) + `]`),
		"contact swing too small":   swingWith(`"contact": [` + contactRow(`{"DEF-thigh.L": 1}`, `{"DEF-hips": 1}`, 0.05) + `]`),
		"contact above the hinge":   swingWith(`"contact": [{"thigh_m": [0.08, -0.12, 0.95], "thigh_skin": {"DEF-thigh.L": 1}, "panel_m": [0.08, -0.13, 0.99], "panel_skin": {"DEF-hips": 1}, "swing": 0.5}]`),
		"belt of the legs":          swingWith(`"belt": {"DEF-thigh.L": 1}`),
		"belt not one":              swingWith(`"belt": {"DEF-hips": 0.5, "DEF-spine.001": 0.2}`),
	}
	for name, swing := range records {
		t.Run(name, func(t *testing.T) {
			root, a, turrets := lookFixture(t)
			writeSwing(t, root, swing)
			l := a.Looks["look.guard"]
			l.Swing = &LookSwing{PeriodS: 1.1, DampingRatio: 0.3, ClearanceDeg: 4, PelvisGain: 1}
			a.Looks["look.guard"] = l
			if err := checkLooks(root, a, person, turrets, nil); err == nil {
				t.Fatalf("%s: accepted", name)
			}
		})
	}
	t.Run("look swings, model has no swing shapes", func(t *testing.T) {
		root, a, turrets := lookFixture(t)
		l := a.Looks["look.guard"]
		l.Swing = &LookSwing{PeriodS: 1.1, DampingRatio: 0.3, ClearanceDeg: 4, PelvisGain: 1}
		a.Looks["look.guard"] = l
		if err := checkLooks(root, a, person, turrets, nil); err == nil {
			t.Fatal("accepted")
		}
	})
}
