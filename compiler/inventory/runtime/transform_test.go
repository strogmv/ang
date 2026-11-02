package inventory

import (
	"reflect"
	"testing"
)

func TestTransformKeepsGradeConservesUnitsAndIsAtomic(t *testing.T) {
	m := fixtureModel()
	m.Sources[1].Grade = Grade{Quality: 4, Provenance: "heirloom"}
	work := "work/hero"
	m.Stores[work] = Store{ID: work, Kind: "container", Anchor: Anchor{Kind: "actor", Actor: "hero"}, Owner: "hero", Locked: true, Capacity: Capacity{Mass: 1000, Volume: 1000, Stacks: 100}}
	s := mustState(t, m)
	ore := InstanceID("ore", 0, 0)
	if s.Items[ore].Grade.Quality != 4 || s.Items[ore].Grade.Provenance != "heirloom" {
		t.Fatal("source grade was not given to the instance")
	}
	// split 5 units into the locked work store: the grade travels with the units, the total stays 20
	if _, err := s.Transform(m, Transform{Relocate: []Relocate{{Item: ore, Quantity: 5, Split: "job.1:in:0", To: Location{Kind: "store", Store: work}, Owner: "hero"}}}); err != nil {
		t.Fatal(err)
	}
	held := s.Items["job.1:in:0"]
	if held.Quantity != 5 || held.Grade.Provenance != "heirloom" || s.Items[ore].Quantity != 15 || held.Owner != "hero" {
		t.Fatalf("split lost units or grade: %+v / %+v", held, s.Items[ore])
	}
	// spend 3, make a product, restore a condition and pay — all at once
	before := s.Clone()
	res, err := s.Transform(m, Transform{
		Spend:    []Spend{{Item: "job.1:in:0", Quantity: 3}},
		Make:     []Make{{ID: "job.1:out:0", Definition: "ore", Quantity: 1, Condition: 1000000, Owner: "hero", Origin: "craft:job.1", Location: Location{Kind: "store", Store: ActorStore("hero")}, Grade: Grade{Quality: 5, Provenance: "handmade"}}},
		Restore:  []Restore{{Item: ore, Condition: 900000}},
		Payments: []Payment{{From: "a", To: "b", Currency: "CRD", Amount: 50}},
	})
	if err != nil || len(res.Created) != 1 || s.Revision != before.Revision+1 {
		t.Fatal(err)
	}
	if s.Items["job.1:in:0"].Quantity != 2 || s.Items["job.1:out:0"].Grade.Quality != 5 || s.Items[ore].Condition != 900000 || s.Accounts["a"].Balance != 950 || s.Accounts["b"].Balance != 50 {
		t.Fatal("transform legs were not all applied")
	}
	// a failing leg (unknown item) changes nothing
	after := s.Clone()
	if _, err := s.Transform(m, Transform{Spend: []Spend{{Item: "job.1:in:0", Quantity: 1}}, Make: []Make{{ID: "x", Definition: "missing", Quantity: 1, Origin: "x", Location: Location{Kind: "store", Store: work}}}}); err == nil || !reflect.DeepEqual(after, s) {
		t.Fatal("partial transform committed")
	}
	// making an existing id, an invalid grade or more than a stack is refused
	for _, mk := range []Make{
		{ID: ore, Definition: "ore", Quantity: 1, Origin: "x", Location: Location{Kind: "store", Store: work}},
		{ID: "y", Definition: "ore", Quantity: 1, Origin: "x", Location: Location{Kind: "store", Store: work}, Grade: Grade{Quality: 6}},
		{ID: "z", Definition: "ore", Quantity: 101, Origin: "x", Location: Location{Kind: "store", Store: work}},
	} {
		if _, err := s.Transform(m, Transform{Make: []Make{mk}}); err == nil {
			t.Fatalf("invalid making accepted: %+v", mk)
		}
	}
	if !reflect.DeepEqual(after, s) {
		t.Fatal("refused making changed state")
	}
}

// An older save wrote the provenance "prewar"; loading renames it to "heirloom" (owner 02.11) and the state validates.
func TestLegacyPrewarProvenanceMigrates(t *testing.T) {
	m := fixtureModel()
	m.Sources[1].Grade = Grade{Quality: 4, Provenance: "heirloom"}
	s := mustState(t, m)
	ore := InstanceID("ore", 0, 0)
	i := s.Items[ore]
	i.Grade.Provenance = "prewar"
	s.Items[ore] = i
	if s.Validate(m) == nil {
		t.Fatal("a legacy provenance must not validate before the migration")
	}
	if n := s.MigrateProvenance(); n != 1 || s.Items[ore].Grade.Provenance != "heirloom" || s.Items[ore].Grade.Quality != 4 {
		t.Fatalf("migration changed %d instances, grade %+v", n, s.Items[ore].Grade)
	}
	if err := s.Validate(m); err != nil {
		t.Fatal(err)
	}
	if s.MigrateProvenance() != 0 || LegacyProvenance("system") != "system" || LegacyProvenance("") != "" {
		t.Fatal("migration is not idempotent or touched other provenances")
	}
}
