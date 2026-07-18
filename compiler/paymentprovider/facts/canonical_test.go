package ppfacts_test

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/paymentprovider/facts"
)

func TestCanonicalFactsDeterministic(t *testing.T) {
	left := ppfacts.Envelope{
		Schema:     ppfacts.SchemaV1,
		ScopeID:    "payment-provider:test",
		ProviderID: "test",
		Facts: []ppfacts.Fact{
			{ID: "fact:b", Predicate: "pp_provider", Terms: []ppfacts.Term{{Sort: "provider", Value: "test"}, {Sort: "provider_id", Value: "test"}}},
			{ID: "fact:a", Predicate: "pp_capability", Terms: []ppfacts.Term{{Sort: "provider", Value: "test"}, {Sort: "capability", Value: "payout"}, {Sort: "enabled", Value: "true"}}},
		},
		Evidence: []ppfacts.Evidence{{ID: "evidence:1", Extractor: "cue_provider_spec", ContentHash: strings.Repeat("a", 64)}},
	}
	right := left
	right.Facts = []ppfacts.Fact{left.Facts[1], left.Facts[0]}

	leftJSON, err := ppfacts.CanonicalJSON(left)
	if err != nil {
		t.Fatalf("CanonicalJSON left: %v", err)
	}
	rightJSON, err := ppfacts.CanonicalJSON(right)
	if err != nil {
		t.Fatalf("CanonicalJSON right: %v", err)
	}
	if string(leftJSON) != string(rightJSON) {
		t.Fatalf("canonical JSON differs")
	}
}

func TestValidateRejectsUnknownPredicate(t *testing.T) {
	env := ppfacts.Envelope{
		Schema: ppfacts.SchemaV1, ScopeID: "payment-provider:test", ProviderID: "test",
		Facts: []ppfacts.Fact{{ID: "fact:1", Predicate: "pp_unknown", Terms: []ppfacts.Term{{Sort: "provider", Value: "test"}}}},
	}
	if err := ppfacts.Validate(env); err == nil {
		t.Fatal("expected unknown predicate error")
	}
}

func TestValidateRejectsDuplicateFactIDs(t *testing.T) {
	terms := []ppfacts.Term{{Sort: "provider", Value: "test"}, {Sort: "provider_id", Value: "test"}}
	env := ppfacts.Envelope{
		Schema: ppfacts.SchemaV1, ScopeID: "payment-provider:test", ProviderID: "test",
		Facts: []ppfacts.Fact{
			{ID: "fact:dup", Predicate: "pp_provider", Terms: terms},
			{ID: "fact:dup", Predicate: "pp_provider", Terms: terms},
		},
	}
	if err := ppfacts.Validate(env); err == nil {
		t.Fatal("expected duplicate fact id error")
	}
}

func TestValidateRejectsTermWithNewline(t *testing.T) {
	env := ppfacts.Envelope{
		Schema: ppfacts.SchemaV1, ScopeID: "payment-provider:test", ProviderID: "test",
		Facts: []ppfacts.Fact{{
			ID: "fact:1", Predicate: "pp_behavior", Terms: []ppfacts.Term{
				{Sort: "provider", Value: "test"},
				{Sort: "behavior", Value: "bad\nvalue"},
			},
		}},
	}
	if err := ppfacts.Validate(env); err == nil {
		t.Fatal("expected newline rejection")
	}
}
