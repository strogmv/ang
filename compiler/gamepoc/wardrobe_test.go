package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wardrobeFixture: a game tree with the wardrobe build's record of one body (a head, a hair style; a coverall, an apron with
// swing shapes and a knit cap with a cover plane), the CUE data that matches it, a rule and a person's own outfit.
func wardrobeFixture(t *testing.T) (string, Art, func(string) bool, wardrobeWorld) {
	t.Helper()
	root := t.TempDir()
	sha := strings.Repeat("a", 64)
	contact := []map[string]any{{"thigh_m": []float64{0.08, -0.12, 0.7}, "thigh_skin": map[string]float64{"DEF-thigh.L": 1},
		"panel_m": []float64{0.08, -0.13, 0.7}, "panel_skin": map[string]float64{"DEF-hips": 0.5, "DEF-thigh.L": 0.5}, "swing": 0.5}}
	swing := func(side string) map[string]any {
		return map[string]any{"side": side, "thigh": "DEF-thigh." + side, "max_deg": 60, "length_m": 0.44, "thigh_share": 0.9,
			"hinge_m": []float64{0.08, -0.13, 0.96}, "hem_m": []float64{0.09, -0.13, 0.52}, "knee_front_m": 0.1, "contact": contact,
			"belt": map[string]float64{"DEF-hips": 0.6, "DEF-spine.001": 0.4}}
	}
	record := map[string]any{"bodies": map[string]any{"female": map[string]any{
		"atlas":    map[string]any{"file": "art-source/clothing/v1/model/female/atlas.png", "sha256": sha, "px": 4096},
		"body_glb": map[string]any{"file": "art-source/clothing/v1/model/female/body.glb", "sha256": sha},
		"shapes":   []string{"body_heavy", "body_thin"},
		"slots": []map[string]string{{"owner": "apron_waist", "zone": "apron"}, {"owner": "coverall", "zone": "cloth"},
			{"owner": "hair@braid@polina", "zone": "hair"}, {"owner": "head@polina", "zone": "skin"}, {"owner": "knit_cap", "zone": "cap"}, {"owner": "skin", "zone": "skin"}},
		"materials": []map[string]string{{"owner": "apron_waist", "surface": "apron"}, {"owner": "coverall", "surface": "cloth"},
			{"owner": "hair@braid@polina", "surface": "hair"}, {"owner": "head@polina", "surface": "skin"}, {"owner": "head@polina", "surface": "eye"},
			{"owner": "knit_cap", "surface": "cap"}, {"owner": "skin", "surface": "skin"}},
		"regions": []string{"chest_back", "chest_front", "hand_L", "head", "thigh_L", "thigh_R", "waist_back", "waist_front"},
		"heads":   map[string]any{"polina": map[string]any{"skin": "young"}},
		"hair":    map[string]any{"braid": map[string]any{"heads": []string{"polina"}}},
		"parts": map[string]any{"head@polina": map[string]any{"triangles": map[string]int{"near": 2500, "crowd": 1200}},
			"hair@braid@polina": map[string]any{"triangles": map[string]int{"near": 2700, "crowd": 1300}},
			"skin":              map[string]any{"triangles": map[string]int{"near": 2300, "crowd": 1100}}},
		"items": map[string]any{
			"coverall": map[string]any{"file": "art-source/clothing/v1/model/female/coverall.glb", "sha256": sha, "triangles": map[string]int{"near": 3500, "crowd": 1700},
				"regions": []string{"chest_front", "thigh_L"}, "covered": map[string]float64{"chest_back": 1, "chest_front": 1, "thigh_L": 1, "thigh_R": 0.99, "waist_back": 1, "waist_front": 1},
				"covered_over": map[string]any{"chest_front": map[string]float64{"apron_waist": 0.2}}},
			"apron_waist": map[string]any{"file": "art-source/clothing/v1/model/female/apron_waist.glb", "sha256": sha, "triangles": map[string]int{"near": 1000, "crowd": 500},
				"covered": map[string]float64{"thigh_L": 0.3}, "swing_shapes": map[string]any{"apron_swing_L": swing("L"), "apron_swing_R": swing("R")}},
			"knit_cap": map[string]any{"file": "art-source/clothing/v1/model/female/knit_cap.glb", "sha256": sha, "triangles": map[string]int{"near": 1000, "crowd": 500},
				"cover": map[string]any{"point": []float64{0, -0.14, 1.58}, "normal": []float64{0, 0.5, 0.86}}},
		},
	}}}
	raw, _ := json.Marshal(record)
	path := filepath.Join(root, "art-source/clothing/v1/wardrobe.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	mat := LookMaterial{Roughness: 0.8}
	item := func(id, layer string, slots, covers []string, mats ...string) WardrobeItem {
		it := WardrobeItem{ID: id, Items: []string{"item." + id}, Surfaces: map[string]string{}, Slots: slots, Layer: layer, Covers: covers,
			Bodies: []string{"female"}, Materials: map[string]LookMaterial{}, Note: "why it looks so"}
		for _, m := range mats {
			it.Materials[m] = mat
			it.Surfaces[m] = "outer"
		}
		return it
	}
	apron := item("apron_waist", "over", []string{"over_waist"}, nil, "apron")
	apron.Motion = &WardrobeMotion{Swing: &LookSwing{PeriodS: 1.1, DampingRatio: 0.3, ClearanceDeg: 3, PelvisGain: 0.6}}
	cap := item("knit_cap", "head", []string{"head"}, nil, "cap")
	cap.HidesHair = true
	coverall := item("coverall", "one_piece", []string{"torso", "legs"}, []string{"chest_front", "chest_back", "waist_front", "waist_back", "thigh_L", "thigh_R"}, "cloth")
	idle := "calm"
	a := Art{
		Wardrobe: &Wardrobe{Record: "art-source/clothing/v1/wardrobe.json", Layers: append([]string{}, wardrobeLayers...), Steps: LookSteps{NearUpToFrameM: 5, HysteresisM: 0.5},
			Bodies: map[string]WardrobeBody{"female": {ID: "female", Sex: "f", Heads: map[string]WardrobeHead{"polina": {Note: "her"}},
				Hair: map[string]WardrobeHair{"braid": {Heads: []string{"polina"}}}, Materials: map[string]LookMaterial{"skin": mat, "hair": mat, "eye": mat}, Note: "a body"}},
			Items: map[string]WardrobeItem{"coverall": coverall, "apron_waist": apron, "knit_cap": cap}},
		OutfitRules: map[string]OutfitRule{"bronze_trader_f": {Body: "female", Caste: "bronze", Heads: map[string]int64{"polina": 1}, Hair: map[string]int64{"braid": 1},
			Wear: []string{"coverall"}, Choose: map[string]map[string]OutfitOption{"apron": {"waist": {Items: []string{"apron_waist"}, Share: 1}, "none": {Share: 1}},
				"head": {"cap": {Items: []string{"knit_cap"}, Share: 1}, "bare": {Share: 2}}},
			Colours: map[string]map[string]int64{"cloth": {"#5B666B": 3}, "apron": {"#7A5C42": 1}, "skin": {"#C99A80": 1}, "hair": {"#3A2C22": 1}},
			Ages:    []OutfitAge{{FromAge: 55, Colours: map[string]map[string]int64{"hair": {"#4E4D4B": 1}}}},
			HeightM: &PopulationHeight{Min: 1.56, Max: 1.72}, Shapes: map[string]PopulationShape{"none": {Share: 1}, "body_heavy": {Share: 1, Min: 0.4, Max: 1}}, Idle: &idle}},
		PersonOutfits: map[string]PersonOutfit{"char.polina": {Body: "female", Caste: "bronze", Head: "polina", Hair: "braid", Items: []string{"coverall", "apron_waist"},
			Colours: map[string]string{"cloth": "#5B666B", "apron_waist.apron": "#3E4234", "skin": "#D6AC92"}, Use: "look"}},
		PersonLooks:       map[string]string{"char.polina": "look.polina"},
		PersonPopulations: map[string]PersonPopulation{"char.trader": {Population: "bronze_trader_f", Age: 40}},
	}
	person := func(id string) bool { return id == "char.polina" || id == "char.trader" }
	ww := wardrobeWorld{Garments: map[string][]string{"clothing.bronze_coverall": {"lining", "outer"}, "clothing.apron": {"outer"}, "clothing.cap": {"outer"}},
		Items:       map[string]bool{"item.coverall": true, "item.apron_waist": true, "item.knit_cap": true, "item.knife": true},
		ItemGarment: map[string]string{"item.coverall": "clothing.bronze_coverall", "item.apron_waist": "clothing.apron", "item.knit_cap": "clothing.cap"}}
	return root, a, person, ww
}

func TestWardrobeAcceptedAndProjected(t *testing.T) {
	root, a, person, ww := wardrobeFixture(t)
	if err := checkWardrobe(root, a, person, ww); err != nil {
		t.Fatal(err)
	}
	if err := checkPopulationLooks(a, nil); err != nil {
		t.Fatal(err)
	}
	if err := checkPersonPopulations(a, person); err != nil {
		t.Fatal(err)
	}
	w, rules, people := clientWardrobe(a)
	raw, err := json.Marshal(map[string]any{"wardrobe": w, "rules": rules, "people": people})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"pack":"res://actors-assets/wardrobe/female.scn"`, `"hides_hair":true`, `"motion":{"swing":{"period_s":1.1`,
		`"choose":[{"id":"apron","options":[{"id":"none","items":[],"share":1},{"id":"waist","items":["apron_waist"],"share":1}]}`,
		`"colours":[{"key":"apron","options":[{"colour":"#7A5C42","share":1}]}`, `"idles":["calm"]`, `"use":"look"`,
		`"shapes":[{"id":"body_heavy","max":1,"min":0.4,"share":1},{"id":"none","max":0,"min":0,"share":1}]`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("the projection lacks %s:\n%s", want, raw)
		}
	}
}

func TestWardrobeRejected(t *testing.T) {
	cases := map[string]func(a *Art){
		"unknown body": func(a *Art) {
			r := a.OutfitRules["bronze_trader_f"]
			r.Body = "male"
			a.OutfitRules["bronze_trader_f"] = r
		},
		"head of no body": func(a *Art) {
			p := a.PersonOutfits["char.polina"]
			p.Head = "woman"
			a.PersonOutfits["char.polina"] = p
		},
		"hair on another head": func(a *Art) {
			r := a.OutfitRules["bronze_trader_f"]
			r.Hair = map[string]int64{"bun": 1}
			a.OutfitRules["bronze_trader_f"] = r
		},
		"two items in one slot": func(a *Art) {
			p := a.PersonOutfits["char.polina"]
			p.Items = []string{"coverall", "coverall"}
			a.PersonOutfits["char.polina"] = p
		},
		"nothing on the trunk": func(a *Art) {
			p := a.PersonOutfits["char.polina"]
			p.Items = []string{"apron_waist"}
			a.PersonOutfits["char.polina"] = p
		},
		"colour of no zone": func(a *Art) {
			p := a.PersonOutfits["char.polina"]
			p.Colours = map[string]string{"knit": "#111111"}
			a.PersonOutfits["char.polina"] = p
		},
		"covers more than built": func(a *Art) {
			it := a.Wardrobe.Items["coverall"]
			it.Covers = append(it.Covers, "hand_L")
			a.Wardrobe.Items["coverall"] = it
		},
		"covers too little": func(a *Art) {
			it := a.Wardrobe.Items["apron_waist"]
			it.Covers = []string{"thigh_L"}
			a.Wardrobe.Items["apron_waist"] = it
		},
		"covers an unknown place": func(a *Art) {
			it := a.Wardrobe.Items["coverall"]
			it.Covers = []string{"wing"}
			a.Wardrobe.Items["coverall"] = it
		},
		"material not in record": func(a *Art) {
			it := a.Wardrobe.Items["coverall"]
			it.Materials["glass"] = LookMaterial{}
			a.Wardrobe.Items["coverall"] = it
		},
		"material missing": func(a *Art) {
			it := a.Wardrobe.Items["coverall"]
			it.Materials = map[string]LookMaterial{}
			a.Wardrobe.Items["coverall"] = it
		},
		"swing without shapes": func(a *Art) {
			it := a.Wardrobe.Items["knit_cap"]
			it.Motion = a.Wardrobe.Items["apron_waist"].Motion
			a.Wardrobe.Items["knit_cap"] = it
		},
		"shapes without swing": func(a *Art) {
			it := a.Wardrobe.Items["apron_waist"]
			it.Motion = nil
			a.Wardrobe.Items["apron_waist"] = it
		},
		"hides hair without plane": func(a *Art) {
			it := a.Wardrobe.Items["coverall"]
			it.HidesHair = true
			a.Wardrobe.Items["coverall"] = it
		},
		"layer order changed": func(a *Art) { a.Wardrobe.Layers = []string{"skin", "over"} },
		"item on no body": func(a *Art) {
			it := a.Wardrobe.Items["coverall"]
			it.Bodies = []string{"male"}
			a.Wardrobe.Items["coverall"] = it
		},
		"use look without a look": func(a *Art) { a.PersonLooks = nil },
		"rule shares to nothing": func(a *Art) {
			r := a.OutfitRules["bronze_trader_f"]
			r.Heads = map[string]int64{"polina": 0}
			a.OutfitRules["bronze_trader_f"] = r
		},
		"rule and population": func(a *Art) { a.PopulationLooks = map[string]PopulationLook{"bronze_trader_f": {Look: "look.x"}} },
		"shape of no body": func(a *Art) {
			r := a.OutfitRules["bronze_trader_f"]
			r.Shapes = map[string]PopulationShape{"body_tall": {Share: 1}}
			a.OutfitRules["bronze_trader_f"] = r
		},
		"ages not ascending": func(a *Art) {
			r := a.OutfitRules["bronze_trader_f"]
			r.Ages = append(r.Ages, OutfitAge{FromAge: 40})
			a.OutfitRules["bronze_trader_f"] = r
		},
	}
	item := func(a *Art, id string, change func(it *WardrobeItem)) {
		it := a.Wardrobe.Items[id]
		change(&it)
		a.Wardrobe.Items[id] = it
	}
	// the link to the world: the physical items (world.items with a garment) and the clothing SDK's garments and layers
	world := map[string]func(a *Art, ww *wardrobeWorld){
		"view draws no item": func(a *Art, _ *wardrobeWorld) { item(a, "coverall", func(it *WardrobeItem) { it.Items = nil }) },
		"no such world item": func(a *Art, _ *wardrobeWorld) {
			item(a, "coverall", func(it *WardrobeItem) { it.Items = []string{"item.none"} })
		},
		"world item without garment": func(_ *Art, ww *wardrobeWorld) { delete(ww.ItemGarment, "item.coverall") },
		"garment not in the catalog": func(_ *Art, ww *wardrobeWorld) { ww.ItemGarment["item.coverall"] = "clothing.none" },
		"surface shows no such layer": func(a *Art, _ *wardrobeWorld) {
			item(a, "coverall", func(it *WardrobeItem) { it.Surfaces = map[string]string{"cloth": "liner"} })
		},
		"surface without a layer": func(a *Art, _ *wardrobeWorld) {
			item(a, "coverall", func(it *WardrobeItem) { it.Surfaces = map[string]string{} })
		},
		"layer of no surface": func(a *Art, _ *wardrobeWorld) {
			item(a, "coverall", func(it *WardrobeItem) { it.Surfaces = map[string]string{"cloth": "outer", "zip": "outer"} })
		},
		"item drawn by two views": func(a *Art, _ *wardrobeWorld) {
			item(a, "apron_waist", func(it *WardrobeItem) { it.Items = []string{"item.apron_waist", "item.coverall"} })
		},
		"worn item without a view": func(_ *Art, ww *wardrobeWorld) { ww.ItemGarment["item.knife"] = "clothing.cap" },
		"view without a note":      func(a *Art, _ *wardrobeWorld) { item(a, "coverall", func(it *WardrobeItem) { it.Note = "" }) },
	}
	for name, mutate := range cases {
		world[name] = func(a *Art, _ *wardrobeWorld) { mutate(a) }
	}
	for name, mutate := range world {
		t.Run(name, func(t *testing.T) {
			root, a, person, ww := wardrobeFixture(t)
			mutate(&a, &ww)
			err := checkWardrobe(root, a, person, ww)
			if err == nil {
				t.Fatalf("accepted %s", name)
			}
			t.Logf("%s: %v", name, err)
		})
	}
	// a rule nobody wears is a typo; a person may name a rule as his population
	_, a, _, _ := wardrobeFixture(t)
	a.PersonPopulations = nil
	if err := checkPopulationLooks(a, nil); err == nil {
		t.Fatal("accepted a rule nobody wears")
	}
	_, a, person, _ := wardrobeFixture(t)
	a.PersonPopulations["char.trader"] = PersonPopulation{Population: "bronze_nobody", Age: 40}
	if err := checkPersonPopulations(a, person); err == nil {
		t.Fatal("accepted a person of an unknown rule")
	}
}
