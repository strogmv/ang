package emitter

import (
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestExtractFrontendFieldEnums(t *testing.T) {
	entities := []normalizer.Entity{
		{
			Name: "Counterparty",
			Fields: []normalizer.Field{
				{
					Name:        "catalogShareStatus",
					Constraints: &normalizer.Constraints{Enum: []string{"active", "revoked", "expired"}},
				},
				{
					Name:        "quantityMode",
					Constraints: &normalizer.Constraints{Enum: []string{"catalog", "allocated", "hidden"}},
				},
			},
		},
	}

	got := extractFrontendFieldEnums(entities, nil)
	if len(got) != 2 {
		t.Fatalf("expected two field enums, got %#v", got)
	}
	if got[0].Name != "CounterpartyCatalogShareStatusValues" || got[1].Name != "CounterpartyQuantityModeValues" {
		t.Fatalf("unexpected generated names: %#v", got)
	}
	if got[0].Values[1] != "revoked" || got[1].Values[2] != "hidden" {
		t.Fatalf("enum values were not preserved: %#v", got)
	}
}

func TestExtractFrontendFieldEnumsSkipsNamedEnumCollision(t *testing.T) {
	entities := []normalizer.Entity{{
		Name: "Counterparty",
		Fields: []normalizer.Field{{
			Name:        "status",
			Constraints: &normalizer.Constraints{Enum: []string{"active", "blocked"}},
		}},
	}}
	named := []NamedEnum{{Name: "CounterpartyStatusValues", Values: []string{"already", "defined"}}}

	if got := extractFrontendFieldEnums(entities, named); len(got) != 0 {
		t.Fatalf("expected named enum collision to be skipped, got %#v", got)
	}
}
