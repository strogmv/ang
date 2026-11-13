package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// partsFixture: a crowd model with the zone mask, two part surfaces of a kit, an apron part, two hair parts and a skin
// surface whose second UV set holds the older face; a crowd binding worn by visitors, a trader binding worn by persons.
func partsFixture(t *testing.T) (string, Art, *VisitorCatalog, func(string) bool) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "art-source/people/v1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"looks": {"woman": {"file": "art-source/people/v1/model/woman.glb", "sha256": "` + strings.Repeat("a", 64) + `",
		"surfaces": {"cloth": "M_cloth", "skin": "M_skin", "part_tools": "M_tools", "part_apron": "M_apron", "part_hair_bun": "M_bun",
			"part_hair_tail": "M_tail", "glow_lamp": "M_lamp"},
		"uv2_surfaces": ["cloth", "skin"],
		"zones": {"mask": "art-source/people/v1/model/woman_zones.png", "mask_sha256": "` + strings.Repeat("b", 64) + `",
			"reference_srgb": {"cloth": "#5B666B", "hair": "#332921"}}}}}`
	if err := os.WriteFile(filepath.Join(dir, "look.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	cws := map[string]LookColourway{"steel": {Zones: map[string]string{"cloth": "#5B666B", "hair": "#4A3426"}, Note: "brown hair"},
		"grey": {Zones: map[string]string{"cloth": "#666C6D", "hair": "#9A9690"}, Note: "grey hair"}}
	hair := map[string][]string{"short": {}, "bun": {"part_hair_bun"}, "tied": {"part_hair_tail"}}
	look := func(id string, wear ...string) Look {
		return Look{ID: id, Record: "art-source/people/v1/look.json", Model: "woman", Caste: "bronze", Glow: []LookGlow{},
			Colourway: "steel", Colourways: cws, Parts: &LookParts{Wear: wear, Choose: map[string]map[string][]string{"hair": hair}},
			Ageing: &LookAgeing{Surface: "skin", FromAge: 35, FullAge: 70}}
	}
	a := Art{
		Looks: map[string]Look{"look.woman": look("look.woman", "part_tools"), "look.trader": look("look.trader", "part_apron")},
		PopulationLooks: map[string]PopulationLook{
			"bronze_work": {Look: "look.woman", Colourways: map[string]int64{"steel": 1}},
			"bronze_trader_f": {Look: "look.trader", Colourways: map[string]int64{"steel": 2}, Ages: []PopulationAge{{FromAge: 55, Colourways: map[string]int64{"grey": 1},
				Choose: map[string][]string{"hair": {"tied", "bun"}}}}},
		},
		PersonPopulations: map[string]PersonPopulation{"char.trader_b": {Population: "bronze_trader_f", Age: 58}, "char.trader_a": {Population: "bronze_trader_f", Age: 27}},
	}
	visitors := &VisitorCatalog{Kinds: []VisitorKind{{ID: "bronze_fitter", Look: "bronze_work"}}}
	person := func(id string) bool { return id == "char.trader_a" || id == "char.trader_b" || id == "char.guard" }
	return root, a, visitors, person
}

func TestLookPartsAgesAndPersonPopulationsAccepted(t *testing.T) {
	root, a, visitors, person := partsFixture(t)
	if err := checkLooks(root, a, person, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := checkPopulationLooks(a, visitors); err != nil {
		t.Fatal(err)
	}
	if err := checkPersonPopulations(a, person); err != nil {
		t.Fatal(err)
	}
	looks, _ := clientLooks(a, nil)
	raw, err := json.Marshal(looks)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{
		`"ageing":{"surface":"skin","from_age":35,"full_age":70}`,
		`"parts":{"choose":[{"id":"hair","options":[{"id":"bun","parts":["part_hair_bun"]},{"id":"short","parts":[]},{"id":"tied","parts":["part_hair_tail"]}]}],"wear":["part_apron"]}`,
		`"parts":{"choose":[{"id":"hair","options":[{"id":"bun","parts":["part_hair_bun"]},{"id":"short","parts":[]},{"id":"tied","parts":["part_hair_tail"]}]}],"wear":["part_tools"]}`,
	} {
		if !strings.Contains(string(raw), part) {
			t.Fatalf("client looks lack %s:\n%s", part, raw)
		}
	}
	raw, err = json.Marshal(clientPopulationLooks(a))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"ages":[{"choose":{"hair":["bun","tied"]},"colourways":[{"id":"grey","share":1}],"from_age":55}],"colourways":[{"id":"steel","share":2}],"id":"bronze_trader_f","look":"look.trader"}`; !strings.Contains(string(raw), want) {
		t.Fatalf("client population looks lack the age band:\n%s", raw)
	}
	raw, err = json.Marshal(clientArt(Compiled{Art: a})["person_populations"])
	if err != nil {
		t.Fatal(err)
	}
	if want := `[{"age":27,"id":"char.trader_a","population":"bronze_trader_f"},{"age":58,"id":"char.trader_b","population":"bronze_trader_f"}]`; string(raw) != want {
		t.Fatalf("client person populations:\n got %s\nwant %s", raw, want)
	}
	if _, ok := clientArt(Compiled{})["person_populations"]; ok {
		t.Fatal("a world without person populations must not get art.person_populations")
	}
	// a look without parts or ageing keeps its row as before (drawn whole, no age marks)
	l := a.Looks["look.woman"]
	l.Parts, l.Ageing = nil, nil
	a.Looks["look.woman"] = l
	if err := checkLooks(root, a, person, nil, nil); err != nil {
		t.Fatal(err)
	}
	looks, _ = clientLooks(a, nil)
	for _, row := range looks {
		if row["id"] == "look.woman" {
			if _, ok := row["parts"]; ok {
				t.Fatal("a look without parts must not get parts")
			}
			if _, ok := row["ageing"]; ok {
				t.Fatal("a look without ageing must not get ageing")
			}
		}
	}
}

func TestLookPartsAgesAndPersonPopulationsRejected(t *testing.T) {
	look := func(f func(l *Look)) func(*Art, **VisitorCatalog) {
		return func(a *Art, _ **VisitorCatalog) {
			l := a.Looks["look.trader"]
			p := *l.Parts
			p.Choose = map[string]map[string][]string{}
			for g, opts := range l.Parts.Choose {
				p.Choose[g] = map[string][]string{}
				for o, parts := range opts {
					p.Choose[g][o] = append([]string{}, parts...)
				}
			}
			p.Wear = append([]string{}, l.Parts.Wear...)
			ag := *l.Ageing
			l.Parts, l.Ageing = &p, &ag
			f(&l)
			a.Looks["look.trader"] = l
		}
	}
	pop := func(f func(p *PopulationLook)) func(*Art, **VisitorCatalog) {
		return func(a *Art, _ **VisitorCatalog) {
			p := a.PopulationLooks["bronze_trader_f"]
			p.Ages = append([]PopulationAge{}, p.Ages...)
			f(&p)
			a.PopulationLooks["bronze_trader_f"] = p
		}
	}
	cases := map[string]func(*Art, **VisitorCatalog){
		"part_unknown":        look(func(l *Look) { l.Parts.Wear = []string{"part_cape"} }),
		"part_not_part":       look(func(l *Look) { l.Parts.Wear = []string{"cloth"} }),
		"part_glow":           look(func(l *Look) { l.Parts.Wear = []string{"glow_lamp"} }),
		"part_twice":          look(func(l *Look) { l.Parts.Wear = []string{"part_apron", "part_hair_bun"} }),
		"group_name":          look(func(l *Look) { l.Parts.Choose["Hair"] = l.Parts.Choose["hair"] }),
		"group_one_option":    look(func(l *Look) { l.Parts.Choose["hair"] = map[string][]string{"bun": {"part_hair_bun"}} }),
		"option_name":         look(func(l *Look) { l.Parts.Choose["hair"]["Tied Back"] = []string{} }),
		"option_unknown_part": look(func(l *Look) { l.Parts.Choose["hair"]["tied"] = []string{"part_hair_braid"} }),
		"ageing_unknown":      look(func(l *Look) { l.Ageing.Surface = "face" }),
		"ageing_no_uv2":       look(func(l *Look) { l.Ageing.Surface = "part_apron" }),
		"ageing_glow":         look(func(l *Look) { l.Ageing.Surface = "glow_lamp" }),
		"ageing_order":        look(func(l *Look) { l.Ageing.FromAge, l.Ageing.FullAge = 70, 35 }),
		"ageing_child":        look(func(l *Look) { l.Ageing.FromAge = 10 }),
		"ageing_old":          look(func(l *Look) { l.Ageing.FullAge = 120 }),
		"band_age":            pop(func(p *PopulationLook) { p.Ages[0].FromAge = 12 }),
		"band_twice":          pop(func(p *PopulationLook) { p.Ages = append(p.Ages, p.Ages[0]) }),
		"band_empty":          pop(func(p *PopulationLook) { p.Ages[0] = PopulationAge{FromAge: 60} }),
		"band_colourway": pop(func(p *PopulationLook) {
			p.Ages[0] = PopulationAge{FromAge: 55, Colourways: map[string]int64{"white": 1}}
		}),
		"band_share": pop(func(p *PopulationLook) {
			p.Ages[0] = PopulationAge{FromAge: 55, Colourways: map[string]int64{"grey": 0}}
		}),
		"band_choose_group": pop(func(p *PopulationLook) {
			p.Ages[0] = PopulationAge{FromAge: 55, Colourways: map[string]int64{"grey": 1}, Choose: map[string][]string{"beard": {"bun"}}}
		}),
		"band_choose_option": pop(func(p *PopulationLook) {
			p.Ages[0] = PopulationAge{FromAge: 55, Colourways: map[string]int64{"grey": 1}, Choose: map[string][]string{"hair": {"braid"}}}
		}),
		"band_choose_empty": pop(func(p *PopulationLook) {
			p.Ages[0] = PopulationAge{FromAge: 55, Colourways: map[string]int64{"grey": 1}, Choose: map[string][]string{"hair": {}}}
		}),
		"band_choose_twice": pop(func(p *PopulationLook) {
			p.Ages[0] = PopulationAge{FromAge: 55, Colourways: map[string]int64{"grey": 1}, Choose: map[string][]string{"hair": {"bun", "bun"}}}
		}),
		"person_unknown": func(a *Art, _ **VisitorCatalog) {
			a.PersonPopulations["char.nobody"] = PersonPopulation{Population: "bronze_trader_f", Age: 30}
		},
		"person_has_look": func(a *Art, _ **VisitorCatalog) { a.PersonLooks = map[string]string{"char.trader_a": "look.trader"} },
		"population_unknown": func(a *Art, _ **VisitorCatalog) {
			a.PersonPopulations["char.trader_a"] = PersonPopulation{Population: "bronze_trader_m", Age: 30}
		},
		"person_child": func(a *Art, _ **VisitorCatalog) {
			a.PersonPopulations["char.trader_a"] = PersonPopulation{Population: "bronze_trader_f", Age: 12}
		},
		"person_too_old": func(a *Art, _ **VisitorCatalog) {
			a.PersonPopulations["char.trader_a"] = PersonPopulation{Population: "bronze_trader_f", Age: 91}
		},
		"population_not_worn": func(a *Art, _ **VisitorCatalog) { a.PersonPopulations = nil },
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			root, a, visitors, person := partsFixture(t)
			a.PersonPopulations = map[string]PersonPopulation{"char.trader_b": a.PersonPopulations["char.trader_b"], "char.trader_a": a.PersonPopulations["char.trader_a"]}
			spoil(&a, &visitors)
			err := checkLooks(root, a, person, nil, nil)
			if err == nil {
				err = checkPopulationLooks(a, visitors)
			}
			if err == nil {
				err = checkPersonPopulations(a, person)
			}
			if err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
