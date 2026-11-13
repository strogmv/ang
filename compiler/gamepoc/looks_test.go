package gamepoc

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func lookFixture(t *testing.T) (string, Art, *TurretWatchCatalog) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "art-source/caste/v1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"looks": {"mark": {"file": "art-source/caste/v1/model/mark.glb", "sha256": "` + strings.Repeat("a", 64) + `",
		"surfaces": {"main": "M_main", "glow_visor": "M_glow_visor", "glow_wrist": "M_glow_wrist"}}}}`
	if err := os.WriteFile(filepath.Join(dir, "look.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	brighter := 12.0
	a := Art{
		Looks: map[string]Look{"look.guard": {ID: "look.guard", Record: "art-source/caste/v1/look.json", Model: "mark", Caste: "silver",
			Glow: []LookGlow{{Surface: "glow_wrist", Energy: 5, When: "always"}, {Surface: "glow_visor", Energy: 8, When: "on_duty", TrackingEnergy: &brighter}}}},
		PersonLooks: map[string]string{"char.guard": "look.guard"},
	}
	turrets := &TurretWatchCatalog{Posts: []TurretWatchPost{
		{ID: "th.turret.right", LevelID: "level.hub", GuardPost: "post.lift", Guards: []string{"char.guard"}},
		{ID: "th.turret.left", LevelID: "level.hub", GuardPost: "post.lift", Guards: []string{"char.guard", "char.other"}},
		{ID: "th.turret.far", LevelID: "level.far", GuardPost: "post.far", Guards: []string{"char.other"}},
	}}
	return root, a, turrets
}

func TestLooksAcceptedAndProjected(t *testing.T) {
	root, a, turrets := lookFixture(t)
	person := func(id string) bool { return id == "char.guard" }
	if err := checkLooks(root, a, person, turrets, nil); err != nil {
		t.Fatal(err)
	}
	looks, people := clientLooks(a, turrets)
	raw, err := json.Marshal(map[string]any{"looks": looks, "person_looks": people})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"looks":[{"caste":"silver","glow":[{"surface":"glow_visor","energy":8,"when":"on_duty","tracking_energy":12},{"surface":"glow_wrist","energy":5,"when":"always"}],` +
		`"id":"look.guard","model":"mark","pack":"res://actors-assets/looks/mark.scn","record":"art-source/caste/v1/look.json"}],` +
		`"person_looks":[{"duty":[{"post":"post.lift","level":"level.hub","turrets":["th.turret.left","th.turret.right"]}],"id":"char.guard","look":"look.guard"}]}`
	if string(raw) != want {
		t.Fatalf("client looks:\n got %s\nwant %s", raw, want)
	}
	// A world without looks keeps its presentation resource unchanged.
	if _, ok := clientArt(Compiled{})["looks"]; ok {
		t.Fatal("a world without looks must not get art.looks")
	}
}

func TestLooksRejected(t *testing.T) {
	person := func(id string) bool { return id == "char.guard" || id == "char.free" }
	cases := map[string]func(root string, a *Art, turrets **TurretWatchCatalog){
		"id": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.ID = "look.other"
			a.Looks["look.guard"] = l
		},
		"caste": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Caste = "iron"
			a.Looks["look.guard"] = l
		},
		"model_name": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Model = "../mark"
			a.Looks["look.guard"] = l
		},
		"record_traversal": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Record = "art-source/../look.json"
			a.Looks["look.guard"] = l
		},
		"record_outside": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Record = "content/look.json"
			a.Looks["look.guard"] = l
		},
		"record_missing": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Record = "art-source/none/look.json"
			a.Looks["look.guard"] = l
		},
		"model_missing": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Model = "other"
			a.Looks["look.guard"] = l
		},
		"glow_not_glow": func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].Surface = "main" },
		"glow_unknown":  func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].Surface = "glow_chest" },
		"glow_twice":    func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].Surface = "glow_visor" },
		"glow_when":     func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].When = "night" },
		"glow_energy":   func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].Energy = 17 },
		"glow_negative": func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].Energy = -1 },
		"tracking_always": func(_ string, a *Art, _ **TurretWatchCatalog) {
			v := 9.0
			a.Looks["look.guard"].Glow[0].TrackingEnergy = &v
		},
		"tracking_dimmer": func(_ string, a *Art, _ **TurretWatchCatalog) {
			v := 7.0
			a.Looks["look.guard"].Glow[1].TrackingEnergy = &v
		},
		"unknown_look":       func(_ string, a *Art, _ **TurretWatchCatalog) { a.PersonLooks["char.guard"] = "look.none" },
		"unknown_person":     func(_ string, a *Art, _ **TurretWatchCatalog) { a.PersonLooks["char.nobody"] = "look.guard" },
		"on_duty_not_guard":  func(_ string, a *Art, _ **TurretWatchCatalog) { a.PersonLooks["char.free"] = "look.guard" },
		"on_duty_no_turrets": func(_ string, _ *Art, turrets **TurretWatchCatalog) { *turrets = nil },
		"record_bad_sha": func(root string, _ *Art, _ **TurretWatchCatalog) {
			_ = os.WriteFile(filepath.Join(root, "art-source/caste/v1/look.json"), []byte(`{"looks":{"mark":{"file":"art-source/caste/v1/model/mark.glb","sha256":"x","surfaces":{"glow_visor":"a","glow_wrist":"b"}}}}`), 0o644)
		},
		"record_bad_file": func(root string, _ *Art, _ **TurretWatchCatalog) {
			_ = os.WriteFile(filepath.Join(root, "art-source/caste/v1/look.json"), []byte(`{"looks":{"mark":{"file":"/tmp/mark.glb","sha256":"`+strings.Repeat("a", 64)+`","surfaces":{"glow_visor":"a","glow_wrist":"b"}}}}`), 0o644)
		},
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			root, a, turrets := lookFixture(t)
			spoil(root, &a, &turrets)
			if err := checkLooks(root, a, person, turrets, nil); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

// zonedLookFixture: a model with a zone mask (two tinted zones), a second UV set on its biomonitor row, and a look worn
// in colourway B with materials from data and the biomonitor row.
func zonedLookFixture(t *testing.T) (string, Art, *BiomonitorCatalog) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "art-source/caste/v1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"looks": {"mark": {"file": "art-source/caste/v1/model/mark.glb", "sha256": "` + strings.Repeat("a", 64) + `",
		"surfaces": {"plate": "M_plate", "glass": "M_glass", "glow_visor": "M_visor", "glow_health": "M_health"},
		"uv2_surfaces": ["glow_health", "plate"],
		"zones": {"mask": "art-source/caste/v1/model/mark_zones.png", "mask_sha256": "` + strings.Repeat("b", 64) + `",
			"reference_srgb": {"plate": "#D9D9D5", "pad": "#1D1F22"}}}}}`
	if err := os.WriteFile(filepath.Join(dir, "look.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	coat := 0.3
	a := Art{
		Looks: map[string]Look{"look.guard": {ID: "look.guard", Record: "art-source/caste/v1/look.json", Model: "mark", Caste: "silver",
			Glow:      []LookGlow{{Surface: "glow_visor", Energy: 1.5, When: "always"}},
			Colourway: "B",
			Colourways: map[string]LookColourway{
				"A": {Zones: map[string]string{"plate": "#D9D9D5", "pad": "#1D1F23"}, Note: "light"},
				"B": {Zones: map[string]string{"plate": "#55606C", "pad": "#5B6672"}, Note: "dark"},
			},
			Materials:  map[string]LookMaterial{"plate": {Roughness: 0.42, Metallic: 0.15, Clearcoat: &coat}, "glass": {Roughness: 0.05}},
			Biomonitor: &LookBiomonitor{Surface: "glow_health"}}},
		PersonLooks: map[string]string{"char.guard": "look.guard"},
	}
	return root, a, biomonitorTable()
}

func TestLooksZonesMaterialsAndBiomonitorAccepted(t *testing.T) {
	root, a, monitor := zonedLookFixture(t)
	person := func(id string) bool { return id == "char.guard" }
	if err := checkLooks(root, a, person, nil, monitor); err != nil {
		t.Fatal(err)
	}
	looks, _ := clientLooks(a, nil)
	raw, err := json.Marshal(looks)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{`"colourway":"B"`, `"colourways":{"A":{"zones":{"pad":"#1D1F23","plate":"#D9D9D5"},"note":"light"}`,
		`"materials":{"glass":{"roughness":0.05,"metallic":0},"plate":{"roughness":0.42,"metallic":0.15,"clearcoat":0.3}}`, `"biomonitor":{"surface":"glow_health"}`} {
		if !strings.Contains(string(raw), part) {
			t.Fatalf("client look lacks %s:\n%s", part, raw)
		}
	}
	if got := biomonitorWearers(a); len(got) != 1 || got[0] != "char.guard" {
		t.Fatalf("wearers %v", got)
	}
}

func TestLooksZonesMaterialsAndBiomonitorRejected(t *testing.T) {
	person := func(id string) bool { return id == "char.guard" }
	edit := func(f func(l *Look)) func(string, *Art, **BiomonitorCatalog) {
		return func(_ string, a *Art, _ **BiomonitorCatalog) {
			l := a.Looks["look.guard"]
			f(&l)
			a.Looks["look.guard"] = l
		}
	}
	record := func(body string) func(string, *Art, **BiomonitorCatalog) {
		return func(root string, _ *Art, _ **BiomonitorCatalog) {
			_ = os.WriteFile(filepath.Join(root, "art-source/caste/v1/look.json"), []byte(`{"looks":{"mark":{"file":"art-source/caste/v1/model/mark.glb","sha256":"`+
				strings.Repeat("a", 64)+`","surfaces":{"plate":"a","glass":"g","glow_visor":"b","glow_health":"c"},"uv2_surfaces":["glow_health"]`+body+`}}}`), 0o644)
		}
	}
	bad := 1.2
	cases := map[string]func(string, *Art, **BiomonitorCatalog){
		"no_colourway":      edit(func(l *Look) { l.Colourway = "" }),
		"unknown_colourway": edit(func(l *Look) { l.Colourway = "C" }),
		"colourway_lacks":   edit(func(l *Look) { delete(l.Colourways["A"].Zones, "pad") }),
		"colourway_extra":   edit(func(l *Look) { l.Colourways["A"].Zones["visor"] = "#000000" }),
		"colourway_colour":  edit(func(l *Look) { l.Colourways["B"].Zones["plate"] = "grey" }),
		"colourway_no_note": edit(func(l *Look) {
			l.Colourways["C"] = LookColourway{Zones: map[string]string{"plate": "#000000", "pad": "#000000"}}
		}),
		"colourway_no_mask":  record(``),
		"mask_path":          record(`,"zones":{"mask":"../mark_zones.png","mask_sha256":"` + strings.Repeat("b", 64) + `","reference_srgb":{"plate":"#D9D9D5","pad":"#1D1F22"}}`),
		"mask_sha":           record(`,"zones":{"mask":"art-source/caste/v1/model/mark_zones.png","mask_sha256":"b","reference_srgb":{"plate":"#D9D9D5","pad":"#1D1F22"}}`),
		"mask_reference":     record(`,"zones":{"mask":"art-source/caste/v1/model/mark_zones.png","mask_sha256":"` + strings.Repeat("b", 64) + `","reference_srgb":{"plate":"white","pad":"#1D1F22"}}`),
		"material_glow":      edit(func(l *Look) { l.Materials["glow_visor"] = LookMaterial{Roughness: 0.4} }),
		"material_unknown":   edit(func(l *Look) { l.Materials["cloth"] = LookMaterial{Roughness: 0.4} }),
		"material_range":     edit(func(l *Look) { l.Materials["plate"] = LookMaterial{Roughness: 0.4, Clearcoat: &bad} }),
		"biomonitor_plain":   edit(func(l *Look) { l.Biomonitor = &LookBiomonitor{Surface: "plate"} }),
		"biomonitor_unknown": edit(func(l *Look) { l.Biomonitor = &LookBiomonitor{Surface: "glow_back"} }),
		"biomonitor_no_uv2":  edit(func(l *Look) { l.Biomonitor = &LookBiomonitor{Surface: "glow_visor"}; l.Glow = nil }),
		"biomonitor_glow":    edit(func(l *Look) { l.Glow = append(l.Glow, LookGlow{Surface: "glow_health", Energy: 1, When: "always"}) }),
		"biomonitor_table":   func(_ string, _ *Art, m **BiomonitorCatalog) { *m = nil },
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			root, a, monitor := zonedLookFixture(t)
			spoil(root, &a, &monitor)
			if err := checkLooks(root, a, person, nil, monitor); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

// The body height of a look (height_m, crown to sole in the rest pose): an adult's height reaches the client look row;
// without it the row has none (the model's own height as built).
func TestLooksHeightAcceptedAndProjected(t *testing.T) {
	person := func(id string) bool { return id == "char.guard" }
	for _, h := range []float64{1.45, 1.92, 2.05} {
		root, a, turrets := lookFixture(t)
		l := a.Looks["look.guard"]
		height := h
		l.HeightM = &height
		a.Looks["look.guard"] = l
		if err := checkLooks(root, a, person, turrets, nil); err != nil {
			t.Fatalf("height %g: %v", h, err)
		}
		looks, _ := clientLooks(a, turrets)
		raw, err := json.Marshal(looks)
		if err != nil {
			t.Fatal(err)
		}
		if want := fmt.Sprintf(`"height_m":%g`, h); !strings.Contains(string(raw), want) {
			t.Fatalf("client look lacks %s:\n%s", want, raw)
		}
	}
	_, a, turrets := lookFixture(t)
	looks, _ := clientLooks(a, turrets)
	if _, ok := looks[0]["height_m"]; ok {
		t.Fatal("a look without a height must not get height_m")
	}
}

func TestLooksHeightRejected(t *testing.T) {
	person := func(id string) bool { return id == "char.guard" }
	for name, h := range map[string]float64{"short": 1.44, "child": 1.1, "tall": 2.06, "zero": 0, "negative": -1.8, "nan": math.NaN(), "inf": math.Inf(1)} {
		t.Run(name, func(t *testing.T) {
			root, a, turrets := lookFixture(t)
			l := a.Looks["look.guard"]
			height := h
			l.HeightM = &height
			a.Looks["look.guard"] = l
			err := checkLooks(root, a, person, turrets, nil)
			if err == nil || !strings.Contains(err.Error(), "height_m") {
				t.Fatalf("height %g: %v", h, err)
			}
		})
	}
	// A height that is not a number never reaches the check: world.art does not decode (generate.go reads it so).
	for name, v := range map[string]string{"string": `"1.92"`, "bool": `true`, "list": `[1.92]`, "struct": `{m: 1.92}`} {
		t.Run("type_"+name, func(t *testing.T) {
			src := `{looks: "look.guard": {id: "look.guard", record: "art-source/caste/v1/look.json", model: "mark", caste: "silver", glow: [], height_m: ` + v + `}}`
			var a Art
			if err := cuecontext.New().CompileString(src).Decode(&a); err == nil {
				t.Fatalf("height_m %s decoded as %v", v, a.Looks["look.guard"].HeightM)
			}
		})
	}
	src := `{looks: "look.guard": {id: "look.guard", record: "art-source/caste/v1/look.json", model: "mark", caste: "silver", glow: [], height_m: 1.92}}`
	var a Art
	if err := cuecontext.New().CompileString(src).Decode(&a); err != nil || a.Looks["look.guard"].HeightM == nil || *a.Looks["look.guard"].HeightM != 1.92 {
		t.Fatalf("height_m 1.92 did not decode: %v", err)
	}
}

// The crowd (market people v1): a visitor kind's look bound to a 3D look and the colourways its wearers are drawn in.
func populationFixture(t *testing.T) (Art, *VisitorCatalog) {
	t.Helper()
	_, a, _ := zonedLookFixture(t)
	a.PopulationLooks = map[string]PopulationLook{"bronze_work": {Look: "look.guard", Colourways: map[string]int64{"B": 3, "A": 1},
		HeightM: &PopulationHeight{Min: 1.70, Max: 1.84}}}
	return a, &VisitorCatalog{Kinds: []VisitorKind{{ID: "bronze_fitter", Look: "bronze_work"}, {ID: "copper_worker", Look: "copper"}}}
}

func TestLooksIdleAcceptedAndProjected(t *testing.T) {
	person := func(id string) bool { return id == "char.guard" }
	root, a, turrets := lookFixture(t)
	l := a.Looks["look.guard"]
	idle := "post"
	l.Idle = &idle
	a.Looks["look.guard"] = l
	if err := checkLooks(root, a, person, turrets, nil); err != nil {
		t.Fatalf("idle post: %v", err)
	}
	looks, _ := clientLooks(a, turrets)
	raw, err := json.Marshal(looks)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"idle":"post"`) {
		t.Fatalf("client look lacks the idle:\n%s", raw)
	}
	_, a, turrets = lookFixture(t)
	looks, _ = clientLooks(a, turrets)
	if _, ok := looks[0]["idle"]; ok {
		t.Fatal("a look without an idle must not get one (the shared Idle)")
	}
}

func TestLooksIdleRejected(t *testing.T) {
	person := func(id string) bool { return id == "char.guard" }
	for name, v := range map[string]string{"empty": "", "unknown": "sentry", "case": "Post", "clip_name": "Idle", "spaced": "post ", "other_clip": "sit"} {
		t.Run(name, func(t *testing.T) {
			root, a, turrets := lookFixture(t)
			l := a.Looks["look.guard"]
			idle := v
			l.Idle = &idle
			a.Looks["look.guard"] = l
			err := checkLooks(root, a, person, turrets, nil)
			if err == nil || !strings.Contains(err.Error(), "idle") {
				t.Fatalf("idle %q: %v", v, err)
			}
		})
	}
	// An idle that is not a string never reaches the check: world.art does not decode.
	for name, v := range map[string]string{"number": `1`, "bool": `true`, "list": `["post"]`, "struct": `{stance: "post"}`, "null": `null`} {
		t.Run("type_"+name, func(t *testing.T) {
			src := `{looks: "look.guard": {id: "look.guard", record: "art-source/caste/v1/look.json", model: "mark", caste: "silver", glow: [], idle: ` + v + `}}`
			var a Art
			if err := cuecontext.New().CompileString(src).Decode(&a); err == nil && a.Looks["look.guard"].Idle != nil {
				t.Fatalf("idle %s decoded as %q", v, *a.Looks["look.guard"].Idle)
			}
		})
	}
	src := `{looks: "look.guard": {id: "look.guard", record: "art-source/caste/v1/look.json", model: "mark", caste: "silver", glow: [], idle: "post"}}`
	var a Art
	if err := cuecontext.New().CompileString(src).Decode(&a); err != nil || a.Looks["look.guard"].Idle == nil || *a.Looks["look.guard"].Idle != "post" {
		t.Fatalf("idle post did not decode: %v", err)
	}
}

func TestPopulationLooksAcceptedAndProjected(t *testing.T) {
	a, visitors := populationFixture(t)
	if err := checkPopulationLooks(a, visitors); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(clientPopulationLooks(a))
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"colourways":[{"id":"A","share":1},{"id":"B","share":3}],"height_m":{"max":1.84,"min":1.7},"id":"bronze_work","look":"look.guard"}]`
	if string(raw) != want {
		t.Fatalf("client population looks:\n got %s\nwant %s", raw, want)
	}
	if _, ok := clientArt(Compiled{Art: a})["population_looks"]; !ok {
		t.Fatal("a world that binds a crowd look must get art.population_looks")
	}
	if _, ok := clientArt(Compiled{})["population_looks"]; ok {
		t.Fatal("a world without crowd looks must not get art.population_looks")
	}
}

func TestPopulationLooksRejected(t *testing.T) {
	edit := func(f func(p *PopulationLook)) func(*Art, **VisitorCatalog) {
		return func(a *Art, _ **VisitorCatalog) {
			p := a.PopulationLooks["bronze_work"]
			f(&p)
			a.PopulationLooks["bronze_work"] = p
		}
	}
	cases := map[string]func(*Art, **VisitorCatalog){
		"name": func(a *Art, _ **VisitorCatalog) {
			a.PopulationLooks["Bronze Work"] = a.PopulationLooks["bronze_work"]
		},
		"not_worn":          func(a *Art, _ **VisitorCatalog) { a.PopulationLooks["bronze_home"] = a.PopulationLooks["bronze_work"] },
		"no_visitors":       func(_ *Art, v **VisitorCatalog) { *v = nil },
		"unknown_look":      edit(func(p *PopulationLook) { p.Look = "look.none" }),
		"no_colourways":     edit(func(p *PopulationLook) { p.Colourways = nil }),
		"unknown_colourway": edit(func(p *PopulationLook) { p.Colourways["C"] = 1 }),
		"share_zero":        edit(func(p *PopulationLook) { p.Colourways["A"] = 0 }),
		"share_big":         edit(func(p *PopulationLook) { p.Colourways["A"] = 101 }),
		"height_short":      edit(func(p *PopulationLook) { p.HeightM = &PopulationHeight{Min: 1.40, Max: 1.80} }),
		"height_tall":       edit(func(p *PopulationLook) { p.HeightM = &PopulationHeight{Min: 1.70, Max: 2.10} }),
		"height_order":      edit(func(p *PopulationLook) { p.HeightM = &PopulationHeight{Min: 1.84, Max: 1.70} }),
		"height_nan":        edit(func(p *PopulationLook) { p.HeightM = &PopulationHeight{Min: math.NaN(), Max: 1.80} }),
		"look_without_mask": func(a *Art, _ **VisitorCatalog) {
			l := a.Looks["look.guard"]
			l.Colourways, l.Colourway = nil, ""
			a.Looks["look.guard"] = l
		},
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			a, visitors := populationFixture(t)
			spoil(&a, &visitors)
			if err := checkPopulationLooks(a, visitors); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
