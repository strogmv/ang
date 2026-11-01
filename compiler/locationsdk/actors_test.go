package locationsdk

import (
	"encoding/json"
	"strings"
	"testing"
)

// demo-hub v3-people: the actor presentation is data of the layout, presentation only.
func TestActorsPresentationValidates(t *testing.T) {
	base := Layout{ID: "people"}
	if a, err := validateActors(base); err != nil || a != nil {
		t.Fatalf("omitted actors must keep sprites: %v %v", a, err)
	}
	for _, good := range []LayoutActors{{Presentation: "sprite"}, {Presentation: "humanoid", Cast: "cast.people.mannequin_ual.v1"}} {
		l := base
		g := good
		l.Actors = &g
		a, err := validateActors(l)
		if err != nil || a == nil || *a != good {
			t.Fatalf("%+v rejected: %v", good, err)
		}
		g.Cast = "changed"
		if a.Cast == "changed" {
			t.Fatal("validated actors alias the authored record")
		}
	}
	for name, bad := range map[string]LayoutActors{
		"unknown presentation":  {Presentation: "voxel", Cast: "cast.a"},
		"humanoid without cast": {Presentation: "humanoid"},
		"humanoid bad cast":     {Presentation: "humanoid", Cast: " cast"},
		"sprite with cast":      {Presentation: "sprite", Cast: "cast.a"},
		"empty":                 {},
	} {
		l := base
		b := bad
		l.Actors = &b
		if _, err := validateActors(l); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// The presentation reaches the runtime level with its presence bit and never changes the structural hash.
func TestActorsPresentationReachesRuntimeWithoutStructuralChange(t *testing.T) {
	layouts := testLayouts()
	input := Input{Catalog: testCatalog(), Layouts: layouts}
	plain, err := CompileRuntime(input, RuntimeOptions{TickHz: 10, MoveEveryTicks: 3})
	if err != nil {
		t.Fatal(err)
	}
	people := append([]Layout(nil), layouts...)
	people[0].Actors = &LayoutActors{Presentation: "humanoid", Cast: "cast.people.mannequin_ual.v1"}
	drawn, err := CompileRuntime(Input{Catalog: testCatalog(), Layouts: people}, RuntimeOptions{TickHz: 10, MoveEveryTicks: 3})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for i, level := range drawn.Levels {
		if level.ID != people[0].ID {
			if level.ActorsAuthored || level.Actors != nil {
				t.Fatalf("%s got actors it did not author", level.ID)
			}
			continue
		}
		found = true
		if !level.ActorsAuthored || level.Actors == nil || level.Actors.Presentation != "humanoid" || level.Actors.Cast != "cast.people.mannequin_ual.v1" {
			t.Fatalf("runtime actors: %+v %v", level.Actors, level.ActorsAuthored)
		}
		if level.StructuralHash != plain.Levels[i].StructuralHash {
			t.Fatal("actor presentation changed the structural hash")
		}
		raw, _ := json.Marshal(plain.Levels[i])
		if strings.Contains(string(raw), "actors") {
			t.Fatal("a layout without actors serializes an actors field")
		}
	}
	if !found {
		t.Fatal("layout missing")
	}
	people[0].Actors = &LayoutActors{Presentation: "humanoid"}
	if _, err := CompileRuntime(Input{Catalog: testCatalog(), Layouts: people}, RuntimeOptions{TickHz: 10, MoveEveryTicks: 3}); err == nil || !strings.Contains(err.Error(), "actors_presentation") {
		t.Fatalf("invalid actors accepted by CompileRuntime: %v", err)
	}
}
