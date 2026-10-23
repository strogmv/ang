package clothing

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func fixture() Model {
	return Model{
		Materials:      map[string]Material{"cloth": {Name: "Cloth", Washability: Contamination{Dirt: 1000, Blood: 800, Sanitary: 900}}},
		Garments:       map[string]GarmentDefinition{"uniform": {Name: "Uniform", AppearanceStyle: "bronze", Layers: []LayerDefinition{{ID: "inner", Material: "cloth", Color: "#555555", Order: 0}, {ID: "outer", Material: "cloth", Color: "#666666", Order: 1}}}},
		Reactions:      []ReactionRule{{ID: "blood_concern", Metric: "blood", Threshold: 100, Signal: "check_wellbeing", Weight: 300}, {ID: "medic_aid", Role: "medic", Metric: "blood", Threshold: 100, Signal: "offer_help", Weight: 700}, {ID: "crime_witness", Role: "security", When: map[string]bool{"witnessed_attack": true}, Metric: "blood", Threshold: 100, Signal: "alert", Weight: 800}, {ID: "sanitary_avoid", Metric: "sanitary", Threshold: 100, Signal: "avoid_contact", Weight: 500}, {ID: "odor_avoid", Metric: "sanitary_odor", Threshold: 100, Signal: "avoid_contact", Weight: 300}, {ID: "known_not_work", When: map[string]bool{"working": false}, Metric: "dirt", Threshold: 100, Signal: "suggest_cleanup", Weight: 100}},
		WashFacilities: map[string]WashFacility{"city": {Name: "Washer", Kind: "station", WaterML: 1000, CleanerUnits: 1, DurationTicks: 50, MinWaterQuality: 800, Removal: Contamination{Dirt: 1000, Blood: 1000, Sanitary: 1000}, WetnessAfter: 100}, "field": {Name: "Rinse", Kind: "self_service", WaterML: 2000, DurationTicks: 100, Removal: Contamination{Dirt: 600, Blood: 400, Sanitary: 500}, DirtyWaterDeposit: Contamination{Dirt: 500, Sanitary: 600}, WetnessAfter: 900}},
	}
}
func state(t *testing.T, m Model) State {
	t.Helper()
	s, err := NewState(m, "inventory:existing:17", "uniform", "hidden_stolen_origin", 0)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func expose(t *testing.T, m Model, s State, layer string, e Exposure, now int64) State {
	t.Helper()
	out, err := Expose(m, s, layer, e, now)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func has(signals []Signal, id string) bool {
	for _, s := range signals {
		if s.ID == id {
			return true
		}
	}
	return false
}
func TestObserveSeparatesVisibleLayersOdorAndHiddenOrigin(t *testing.T) {
	m := fixture()
	s := state(t, m)
	s = expose(t, m, s, "inner", Exposure{Blood: 700, Sanitary: 600, Source: "hidden_attacker"}, 10)
	o, err := Observe(m, s, Evidence{VisibleLayers: []string{"outer"}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Visible || o.Blood != 0 || o.Sanitary != 0 || o.SanitaryOdor != 0 {
		t.Fatal("covered layer leaked", o)
	}
	o, err = Observe(m, s, Evidence{SmellLayers: []string{"inner"}, SmellReachable: true}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if o.Visible || o.ItemID != "" || o.AppearanceStyle != "" || o.Name != "" || o.Blood != 0 || o.SanitaryOdor != 600 {
		t.Fatal("smell revealed identity or lost odor", o)
	}
	data, _ := json.Marshal(o)
	if strings.Contains(string(data), "hidden_") {
		t.Fatal("provenance leaked")
	}
	o, err = Observe(m, s, Evidence{SmellLayers: []string{"inner"}}, 10)
	if err != nil || o.SanitaryOdor != 0 {
		t.Fatal("smell passed a wall", o, err)
	}
	if _, err := Observe(m, s, Evidence{VisibleLayers: []string{"missing"}}, 10); err == nil {
		t.Fatal("unknown layer accepted")
	}
}
func TestBloodDoesNotImplyCrimeAndRolesDiffer(t *testing.T) {
	m := fixture()
	s := expose(t, m, state(t, m), "outer", Exposure{Blood: 600}, 10)
	o, _ := Observe(m, s, Evidence{VisibleLayers: []string{"outer"}}, 10)
	for _, role := range []string{"worker", "medic", "security"} {
		signals, err := Assess(m, o, Context{Role: role})
		if err != nil {
			t.Fatal(err)
		}
		if !has(signals, "check_wellbeing") || has(signals, "alert") {
			t.Fatal(role, signals)
		}
		if has(signals, "offer_help") != (role == "medic") {
			t.Fatal("role ignored", role, signals)
		}
	}
	signals, _ := Assess(m, o, Context{Role: "security", Known: map[string]bool{"witnessed_attack": true}})
	if !has(signals, "alert") {
		t.Fatal("known witness context ignored")
	}
	o.Blood = 0
	o.Dirt = 400
	unknown, _ := Assess(m, o, Context{})
	known, _ := Assess(m, o, Context{Known: map[string]bool{"working": false}})
	if has(unknown, "suggest_cleanup") || !has(known, "suggest_cleanup") {
		t.Fatal("unknown fact treated as false")
	}
}
func TestContaminantsRemainIndependentAndDeterministic(t *testing.T) {
	m := fixture()
	s := state(t, m)
	dirty := expose(t, m, s, "outer", Exposure{Dirt: 1000, Wear: 240}, 10)
	if dirty.Layers["outer"].Blood != 0 || dirty.Layers["outer"].Sanitary != 0 || s.Layers["outer"].Dirt != 0 {
		t.Fatal("contaminants or input aliased")
	}
	dirty = expose(t, m, dirty, "outer", Exposure{Dirt: 500, Sanitary: 200}, 11)
	if dirty.Layers["outer"].Dirt != 1000 {
		t.Fatal("did not clamp")
	}
	observation, _ := Observe(m, dirty, Evidence{VisibleLayers: []string{"outer"}, SmellLayers: []string{"outer"}, SmellReachable: true}, 11)
	a, _ := Assess(m, observation, Context{})
	b, _ := Assess(m, observation, Context{})
	if !reflect.DeepEqual(a, b) || !has(a, "avoid_contact") {
		t.Fatal("assessment inconsistent", a, b)
	}
	if len(a) != 1 || a[0].Weight != 800 {
		t.Fatal("signal merge lost independent reasons", a)
	}
}
func TestWashResourcesTimeQualityAndIndependentRemoval(t *testing.T) {
	m := fixture()
	s := expose(t, m, state(t, m), "outer", Exposure{Dirt: 800, Blood: 600, Sanitary: 700, Wear: 230}, 10)
	req := WashRequest{Facility: "city", ElapsedTicks: 50, AvailableWaterML: 1000, AvailableCleanerUnits: 1, WaterQuality: 1000, NowTick: 100}
	result, err := Wash(m, s, req)
	if err != nil {
		t.Fatal(err)
	}
	got := result.State.Layers["outer"]
	if got.Dirt != 0 || got.Blood != 120 || got.Sanitary != 70 || got.Wear != 230 || got.Wetness != 100 {
		t.Fatal("wrong independent wash result", got)
	}
	if result.State.ItemID != s.ItemID || result.State.Provenance != s.Provenance || result.SpentWaterML != 1000 || result.SpentCleanerUnits != 1 || s.Layers["outer"].Dirt != 800 {
		t.Fatal("wash changed custody/identity or mutated source")
	}
	for _, field := range []string{"time", "water", "cleaner", "quality", "future", "clock"} {
		bad := req
		switch field {
		case "time":
			bad.ElapsedTicks = 49
		case "water":
			bad.AvailableWaterML = 999
		case "cleaner":
			bad.AvailableCleanerUnits = 0
		case "quality":
			bad.WaterQuality = 700
		case "future":
			bad.ElapsedTicks = 101
		case "clock":
			bad.NowTick = 9
		}
		if _, err := Wash(m, s, bad); err == nil {
			t.Fatalf("invalid %s accepted", field)
		}
	}
}
func TestDirtyWaterCanMakeCleanClothingWorse(t *testing.T) {
	m := fixture()
	s := state(t, m)
	result, err := Wash(m, s, WashRequest{Facility: "field", ElapsedTicks: 100, AvailableWaterML: 2000, WaterQuality: 200, NowTick: 100})
	if err != nil {
		t.Fatal(err)
	}
	got := result.State.Layers["outer"]
	if got.Dirt != 400 || got.Sanitary != 480 || got.Blood != 0 || got.Wetness != 900 {
		t.Fatal("dirty water incorrectly sanitized", got)
	}
}
func TestStateSaveCloneValidationAndClock(t *testing.T) {
	m := fixture()
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	s := expose(t, m, state(t, m), "outer", Exposure{Blood: 300, Source: "private"}, 10)
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if err := loaded.Validate(m, 10); err != nil || !reflect.DeepEqual(s, loaded) {
		t.Fatal("save changed state", err)
	}
	clone := s.Clone()
	clone.Layers["outer"] = Surface{}
	if s.Layers["outer"].Blood != 300 {
		t.Fatal("clone alias")
	}
	if _, err := Expose(m, s, "outer", Exposure{Dirt: 1}, 9); err == nil {
		t.Fatal("clock reversal accepted")
	}
	if _, err := Expose(m, s, "outer", Exposure{Blood: -1}, 11); err == nil {
		t.Fatal("negative exposure accepted")
	}
	if _, err := Expose(m, s, "missing", Exposure{Blood: 1}, 11); err == nil {
		t.Fatal("unknown layer accepted")
	}
	loaded.Layers["outer"] = Surface{Blood: 1001}
	if err := loaded.Validate(m, 10); err == nil {
		t.Fatal("bad saved range accepted")
	}
}
func TestCatalogInvalidReferencesAndRanges(t *testing.T) {
	for _, name := range []string{"material", "layer", "order", "color", "style", "metric", "threshold", "weight", "wash"} {
		t.Run(name, func(t *testing.T) {
			m := fixture()
			g := m.Garments["uniform"]
			switch name {
			case "material":
				g.Layers[0].Material = "missing"
			case "layer":
				g.Layers[0].ID = g.Layers[1].ID
			case "order":
				g.Layers[0].Order = g.Layers[1].Order
			case "color":
				g.Layers[0].Color = "bad"
			case "style":
				g.AppearanceStyle = ""
			case "metric":
				m.Reactions[0].Metric = "infection"
			case "threshold":
				m.Reactions[0].Threshold = 0
			case "weight":
				m.Reactions[0].Weight = 1001
			case "wash":
				f := m.WashFacilities["city"]
				f.Removal.Blood = 1001
				m.WashFacilities["city"] = f
			}
			m.Garments["uniform"] = g
			if err := m.Validate(); err == nil {
				t.Fatal("invalid catalog accepted")
			}
		})
	}
}

func TestAssessmentRejectsHiddenVisualMeasurements(t *testing.T) {
	m := fixture()
	if _, err := Assess(m, Observation{Blood: 500}, Context{Role: "security"}); err == nil {
		t.Fatal("invisible blood accepted")
	}
	if _, err := Assess(m, Observation{SanitaryOdor: 1001}, Context{}); err == nil {
		t.Fatal("invalid smell range accepted")
	}
}
func TestWashNewWaterFoulingRecordsPrivateSource(t *testing.T) {
	m := fixture()
	s := state(t, m)
	result, err := Wash(m, s, WashRequest{Facility: "field", ElapsedTicks: 100, AvailableWaterML: 2000, WaterQuality: 200, NowTick: 100})
	if err != nil {
		t.Fatal(err)
	}
	layer := result.State.Layers["outer"]
	if layer.LastStainTick != 100 || layer.StainSource != "wash:field" {
		t.Fatal("water contamination lost age/source")
	}
	observed, err := Observe(m, result.State, Evidence{VisibleLayers: []string{"outer"}}, 100)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(observed)
	if strings.Contains(string(b), "wash:field") {
		t.Fatal("wash provenance leaked")
	}
}
