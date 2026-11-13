package gamepoc

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestPaintTestAccepted(t *testing.T) {
	_, a, _, person := partsFixture(t)
	a.PaintTest = &PaintTest{Mode: "now", People: []string{"char.trader_a"}, TwoTone: &PaintTestTwoTone{Zone: "cloth", LimbsValue: 0.62}}
	if err := checkPaintTest(a, person); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(clientPaintTest(a.PaintTest))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"mode":"now","people":["char.trader_a"],"two_tone":{"limbs_value":0.62,"zone":"cloth"}}`; string(raw) != want {
		t.Fatalf("client row %s, want %s", raw, want)
	}
	for _, mode := range []string{"sprite", "painted3d"} {
		a.PaintTest.Mode = mode
		if err := checkPaintTest(a, person); err != nil {
			t.Fatal(mode, err)
		}
	}
	// without two values, and without the test at all
	a.PaintTest.TwoTone = nil
	if err := checkPaintTest(a, person); err != nil {
		t.Fatal(err)
	}
	a.PaintTest = nil
	if err := checkPaintTest(a, person); err != nil {
		t.Fatal(err)
	}
}

func TestPaintTestRejected(t *testing.T) {
	cases := []struct {
		name string
		edit func(a *Art, p *PaintTest)
		want string
	}{
		{"mode", func(a *Art, p *PaintTest) { p.Mode = "painted" }, "mode must be"},
		{"empty mode", func(a *Art, p *PaintTest) { p.Mode = "" }, "mode must be"},
		{"no people", func(a *Art, p *PaintTest) { p.People = nil }, "name the people"},
		{"unknown person", func(a *Art, p *PaintTest) { p.People = []string{"char.nobody"} }, "no person"},
		{"not a person id", func(a *Art, p *PaintTest) { p.People = []string{"trader_a"} }, "no person"},
		{"twice", func(a *Art, p *PaintTest) { p.People = []string{"char.trader_a", "char.trader_a"} }, "named twice"},
		{"no look", func(a *Art, p *PaintTest) { p.People = []string{"char.guard"} }, "wears no 3D look"},
		{"look without zones", func(a *Art, p *PaintTest) {
			l := a.Looks["look.trader"]
			l.Colourways = nil
			a.Looks["look.trader"] = l
		}, "no zone mask"},
		{"unknown zone", func(a *Art, p *PaintTest) { p.TwoTone.Zone = "apron" }, "is not a zone"},
		{"zone name", func(a *Art, p *PaintTest) { p.TwoTone.Zone = "Cloth!" }, "plain zone name"},
		{"too dark", func(a *Art, p *PaintTest) { p.TwoTone.LimbsValue = 0.2 }, "within 0.3..1"},
		{"too light", func(a *Art, p *PaintTest) { p.TwoTone.LimbsValue = 1.2 }, "within 0.3..1"},
		{"not a number", func(a *Art, p *PaintTest) { p.TwoTone.LimbsValue = math.NaN() }, "within 0.3..1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, a, _, person := partsFixture(t)
			a.PaintTest = &PaintTest{Mode: "sprite", People: []string{"char.trader_a"}, TwoTone: &PaintTestTwoTone{Zone: "cloth", LimbsValue: 0.62}}
			c.edit(&a, a.PaintTest)
			err := checkPaintTest(a, person)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error with %q", err, c.want)
			}
		})
	}
}
