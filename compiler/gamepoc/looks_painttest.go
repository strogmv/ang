package gamepoc

import (
	"fmt"
	"math"
)

// The painted people test (the game's art-source/market-people/v1/briefs/04-paint-test.md): one person of the world drawn
// three ways for the owner to choose the path by real frames. Which people and which version are data
// (world.art.paint_test): "now" — the installed 3D look (the default: the game does not change); "sprite" — the person's
// frames baked from his own 3D look in the game camera, a camera-facing quad lit by the game's light through the baked
// normals; "painted3d" — the same 3D body with a matte painted material. TwoTone: a trial colouring of the test only (not
// canon): in the named zone of the look's mask, sleeves and legs drawn darker by LimbsValue (by the body's arm and leg bones).

// PaintTest is world.art.paint_test.
type PaintTest struct {
	Mode    string            `json:"mode"`
	People  []string          `json:"people"`
	TwoTone *PaintTestTwoTone `json:"two_tone,omitempty"`
}

// PaintTestTwoTone: the zone of the look's mask drawn in two values and the value of its sleeves and legs (1: one value).
type PaintTestTwoTone struct {
	Zone       string  `json:"zone"`
	LimbsValue float64 `json:"limbs_value"`
}

var paintTestModes = map[string]bool{"now": true, "sprite": true, "painted3d": true}

// paintTestLook: the look a person is drawn in by data — his own (art.person_looks) or his crowd binding's
// (art.person_populations); "" without one.
func paintTestLook(a Art, id string) string {
	if look, ok := a.PersonLooks[id]; ok {
		return look
	}
	if pp, ok := a.PersonPopulations[id]; ok {
		if p, ok := a.PopulationLooks[pp.Population]; ok {
			return p.Look
		}
	}
	return ""
}

// checkPaintTest: a known mode; at least one person, each once, who exists and is drawn in a 3D look with colourways (a zone
// mask: the painter paints its zones); two_tone names a zone every colourway of each such look colours, and a value 0.3..1.
func checkPaintTest(a Art, person func(string) bool) error {
	p := a.PaintTest
	if p == nil {
		return nil
	}
	if !paintTestModes[p.Mode] {
		return fmt.Errorf("art.paint_test: mode must be now, sprite or painted3d, not %q", p.Mode)
	}
	if len(p.People) == 0 {
		return fmt.Errorf("art.paint_test: name the people of the test")
	}
	seen := map[string]bool{}
	for _, id := range p.People {
		if !personID.MatchString(id) || !person(id) {
			return fmt.Errorf("art.paint_test: %s is no person of world.persons", id)
		}
		if seen[id] {
			return fmt.Errorf("art.paint_test: %s is named twice", id)
		}
		seen[id] = true
		key := paintTestLook(a, id)
		l, ok := a.Looks[key]
		if !ok {
			return fmt.Errorf("art.paint_test: %s wears no 3D look (art.person_looks or art.person_populations)", id)
		}
		if len(l.Colourways) == 0 {
			return fmt.Errorf("art.paint_test: %s wears %s, which has no zone mask and colourways to paint", id, key)
		}
		if t := p.TwoTone; t != nil {
			if !lookPlainName.MatchString(t.Zone) {
				return fmt.Errorf("art.paint_test: two_tone zone %q is not a plain zone name", t.Zone)
			}
			for cw, c := range l.Colourways {
				if _, ok := c.Zones[t.Zone]; !ok {
					return fmt.Errorf("art.paint_test: two_tone zone %s is not a zone of %s (colourway %s)", t.Zone, key, cw)
				}
			}
		}
	}
	if t := p.TwoTone; t != nil && (math.IsNaN(t.LimbsValue) || t.LimbsValue < 0.3 || t.LimbsValue > 1) {
		return fmt.Errorf("art.paint_test: two_tone limbs_value %g must be within 0.3..1", t.LimbsValue)
	}
	return nil
}

// clientPaintTest is the Godot projection: the mode, the people (in data order) and the two values.
func clientPaintTest(p *PaintTest) map[string]any {
	row := map[string]any{"mode": p.Mode, "people": p.People}
	if p.TwoTone != nil {
		row["two_tone"] = map[string]any{"zone": p.TwoTone.Zone, "limbs_value": p.TwoTone.LimbsValue}
	}
	return row
}
