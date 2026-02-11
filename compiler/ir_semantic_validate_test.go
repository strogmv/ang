package compiler

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/ir"
)

func TestValidateIRSemantics_OK(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Entities: []ir.Entity{
			{
				Name: "User",
				Fields: []ir.Field{
					{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
					{Name: "email", Type: ir.TypeRef{Kind: ir.KindString}},
				},
			},
		},
		Services: []ir.Service{
			{
				Name: "Users",
				Methods: []ir.Method{
					{Name: "GetUser"},
				},
			},
		},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/users/{id}", Service: "Users", RPC: "GetUser"},
		},
		Repos: []ir.Repository{
			{
				Name:   "UserRepository",
				Entity: "User",
				Finders: []ir.Finder{
					{
						Name: "FindByEmail",
						Where: []ir.WhereClause{
							{Field: "email", Param: "email", ParamType: "string"},
						},
						Returns: "one",
					},
				},
			},
		},
	}

	if err := ValidateIRSemantics(schema); err != nil {
		t.Fatalf("expected valid schema, got error: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnUnknownRPC(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Services: []ir.Service{
			{Name: "Users", Methods: []ir.Method{{Name: "GetUser"}}},
		},
		Endpoints: []ir.Endpoint{
			{Method: "GET", Path: "/users/{id}", Service: "Users", RPC: "UnknownRPC"},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "unknown RPC") {
		t.Fatalf("expected unknown RPC error, got: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnUnknownFinderField(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Entities: []ir.Entity{
			{Name: "User", Fields: []ir.Field{{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}}}},
		},
		Repos: []ir.Repository{
			{
				Name:   "UserRepository",
				Entity: "User",
				Finders: []ir.Finder{
					{
						Name: "FindByEmail",
						Where: []ir.WhereClause{
							{Field: "email", Param: "email", ParamType: "string"},
						},
					},
				},
			},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "where field") {
		t.Fatalf("expected unknown finder field error, got: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnUnknownEntityTypeRef(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Entities: []ir.Entity{
			{
				Name: "Order",
				Fields: []ir.Field{
					{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
					{Name: "user", Type: ir.TypeRef{Kind: ir.KindEntity, Name: "User"}},
				},
			},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "unknown entity type") {
		t.Fatalf("expected unknown entity type error, got: %v", err)
	}
}

func TestValidateIRSemantics_FailsOnServiceCycle(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{
		Services: []ir.Service{
			{Name: "A", Uses: []string{"B"}},
			{Name: "B", Uses: []string{"A"}},
		},
	}
	err := ValidateIRSemantics(schema)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got: %v", err)
	}
}
