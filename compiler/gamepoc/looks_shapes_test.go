package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shapesFixture: the parts fixture's model with two body shapes in its record, a crowd binding drawing its wearers thin,
// heavy or as built, and a person with his own look and his body shape.
func shapesFixture(t *testing.T) (string, Art, *VisitorCatalog, func(string) bool) {
	t.Helper()
	root, a, visitors, person := partsFixture(t)
	path := filepath.Join(root, "art-source/people/v1/look.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), `"uv2_surfaces"`, `"shapes": ["body_heavy", "body_thin"], "uv2_surfaces"`, 1))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	p := a.PopulationLooks["bronze_work"]
	p.Shapes = map[string]PopulationShape{"none": {Share: 4}, "body_thin": {Share: 2, Min: 0.3, Max: 1}, "body_heavy": {Share: 3, Min: 0.2, Max: 0.9}}
	a.PopulationLooks["bronze_work"] = p
	a.PersonLooks = map[string]string{"char.guard": "look.woman"}
	a.PersonShapes = map[string]map[string]float64{"char.guard": {"body_heavy": 0.4}}
	return root, a, visitors, person
}

func TestLookShapesAcceptedAndProjected(t *testing.T) {
	root, a, visitors, person := shapesFixture(t)
	if err := checkLooks(root, a, person, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := checkPopulationLooks(a, visitors); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(clientPopulationLooks(a))
	if err != nil {
		t.Fatal(err)
	}
	want := `"shapes":[{"id":"body_heavy","max":0.9,"min":0.2,"share":3},{"id":"body_thin","max":1,"min":0.3,"share":2},{"id":"none","max":0,"min":0,"share":4}]`
	if !strings.Contains(string(raw), want) {
		t.Fatalf("client population looks lack %s:\n%s", want, raw)
	}
	_, wearers := clientLooks(a, nil)
	raw, err = json.Marshal(wearers)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"shapes":{"body_heavy":0.4}`) {
		t.Fatalf("client wearers lack the person's shapes:\n%s", raw)
	}
}

func TestLookShapesRejected(t *testing.T) {
	cases := map[string]func(a *Art){
		"unknown crowd shape": func(a *Art) {
			p := a.PopulationLooks["bronze_work"]
			p.Shapes = map[string]PopulationShape{"body_tall": {Share: 1, Max: 1}}
			a.PopulationLooks["bronze_work"] = p
		},
		"share above 100": func(a *Art) {
			p := a.PopulationLooks["bronze_work"]
			p.Shapes = map[string]PopulationShape{"body_thin": {Share: 101, Max: 1}}
			a.PopulationLooks["bronze_work"] = p
		},
		"shares sum to 0": func(a *Art) {
			p := a.PopulationLooks["bronze_work"]
			p.Shapes = map[string]PopulationShape{"body_thin": {Share: 0, Max: 1}, "none": {Share: 0}}
			a.PopulationLooks["bronze_work"] = p
		},
		"min above max": func(a *Art) {
			p := a.PopulationLooks["bronze_work"]
			p.Shapes = map[string]PopulationShape{"body_thin": {Share: 1, Min: 0.8, Max: 0.2}}
			a.PopulationLooks["bronze_work"] = p
		},
		"amount above 1": func(a *Art) {
			p := a.PopulationLooks["bronze_work"]
			p.Shapes = map[string]PopulationShape{"body_heavy": {Share: 1, Max: 1.5}}
			a.PopulationLooks["bronze_work"] = p
		},
		"none with an amount": func(a *Art) {
			p := a.PopulationLooks["bronze_work"]
			p.Shapes = map[string]PopulationShape{"none": {Share: 1, Max: 0.5}}
			a.PopulationLooks["bronze_work"] = p
		},
		"person shape unknown":    func(a *Art) { a.PersonShapes["char.guard"] = map[string]float64{"body_tall": 0.5} },
		"person amount above 1":   func(a *Art) { a.PersonShapes["char.guard"] = map[string]float64{"body_thin": 1.2} },
		"person without own look": func(a *Art) { a.PersonShapes["char.trader_a"] = map[string]float64{"body_thin": 0.2} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			root, a, _, person := shapesFixture(t)
			mutate(&a)
			if err := checkLooks(root, a, person, nil, nil); err == nil {
				t.Fatalf("%s: accepted", name)
			}
		})
	}
	// a record whose shape names clash with the crowd option none
	t.Run("record shape named none", func(t *testing.T) {
		root, a, _, person := shapesFixture(t)
		path := filepath.Join(root, "art-source/people/v1/look.json")
		raw, _ := os.ReadFile(path)
		_ = os.WriteFile(path, []byte(strings.Replace(string(raw), `"body_thin"]`, `"none"]`, 1)), 0o644)
		a.PopulationLooks["bronze_work"] = PopulationLook{Look: "look.woman", Colourways: map[string]int64{"steel": 1}}
		a.PersonShapes = nil
		if err := checkLooks(root, a, person, nil, nil); err == nil {
			t.Fatal("accepted")
		}
	})
}

// The calm stance (characters v1 job 05): a look may name it as its idle.
func TestLooksIdleCalmAccepted(t *testing.T) {
	person := func(id string) bool { return id == "char.guard" }
	root, a, turrets := lookFixture(t)
	l := a.Looks["look.guard"]
	idle := "calm"
	l.Idle = &idle
	a.Looks["look.guard"] = l
	if err := checkLooks(root, a, person, turrets, nil); err != nil {
		t.Fatal(err)
	}
	looks, _ := clientLooks(a, turrets)
	raw, _ := json.Marshal(looks)
	if !strings.Contains(string(raw), `"idle":"calm"`) {
		t.Fatalf("the calm idle is not projected: %s", raw)
	}
}
