package expert

import (
	"testing"

	"github.com/strogmv/ang/compiler/facts"
)

func TestAdaptFactsProducesDeterministicIDsWithoutMachinePaths(t *testing.T) {
	first := facts.Envelope{
		Schema: facts.SchemaV1, SourceType: "go", SourcePath: "/Users/alice/service",
		Entities:   []facts.Entity{{Name: "User", Source: "/Users/alice/service/user.go", Fields: []facts.Field{{Name: "ID"}}}},
		Operations: []facts.Operation{{Name: "Login", ServiceHint: "Auth", InputFields: []facts.Field{{Name: "email"}}}},
		Endpoints:  []facts.Endpoint{{Operation: "Auth.Login", AuthExpr: "actor != nil"}},
	}
	second := first
	second.SourcePath = "/build/agent/service"
	second.Entities = append([]facts.Entity(nil), first.Entities...)
	second.Entities[0].Source = "/build/agent/service/user.go"

	firstFacts, firstEvidence, err := AdaptFacts(first)
	if err != nil {
		t.Fatalf("AdaptFacts(first): %v", err)
	}
	secondFacts, _, err := AdaptFacts(second)
	if err != nil {
		t.Fatalf("AdaptFacts(second): %v", err)
	}
	if len(firstFacts) != len(secondFacts) || len(firstFacts) == 0 {
		t.Fatalf("unexpected adapted fact count: %d / %d", len(firstFacts), len(secondFacts))
	}
	for i := range firstFacts {
		if firstFacts[i].ID != secondFacts[i].ID {
			t.Fatalf("machine path changed fact ID: %s != %s", firstFacts[i].ID, secondFacts[i].ID)
		}
	}
	if len(firstEvidence) == 0 || firstEvidence[0].SourcePath == "" {
		t.Fatalf("expected source evidence, got %#v", firstEvidence)
	}
}

func TestAdaptFactsDoesNotInventMissingAuth(t *testing.T) {
	adapted, _, err := AdaptFacts(facts.Envelope{
		Schema: facts.SchemaV1, SourceType: "openapi", Endpoints: []facts.Endpoint{{Operation: "Auth.Login"}},
	})
	if err != nil {
		t.Fatalf("AdaptFacts: %v", err)
	}
	for _, fact := range adapted {
		if fact.Kind == "endpoint" && fact.Subject == "Auth.Login" && fact.Predicate == "auth_expr" {
			t.Fatalf("missing auth expression was invented as a fact: %#v", fact)
		}
	}
}
