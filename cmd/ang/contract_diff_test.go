package main

import "testing"

func TestDiffOpenAPIContractsDetectsAddedAndBreakingOperations(t *testing.T) {
	previous := []byte(`openapi: 3.0.0
paths:
  /customers:
    get: {}
    post: {}
`)
	current := []byte(`openapi: 3.0.0
paths:
  /customers:
    get: {}
  /customers/{id}:
    get: {}
`)
	diff, err := diffOpenAPIContracts(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.AddedOperations) != 1 || diff.AddedOperations[0] != "GET /customers/{id}" {
		t.Fatalf("unexpected additions: %#v", diff.AddedOperations)
	}
	if len(diff.RemovedOperations) != 1 || diff.RemovedOperations[0] != "POST /customers" {
		t.Fatalf("unexpected removals: %#v", diff.RemovedOperations)
	}
}

func TestDiffOpenAPIContractsDetectsSchemaBreakingChanges(t *testing.T) {
	previous := []byte(`openapi: 3.0.0
components:
  schemas:
    Customer:
      properties:
        id: {type: string}
        name: {type: string}
`)
	current := []byte(`openapi: 3.0.0
components:
  schemas:
    Customer:
      required: [name, email]
      properties:
        name: {type: integer}
        email: {type: string}
`)
	diff, err := diffOpenAPIContracts(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.BreakingChanges) != 4 {
		t.Fatalf("breaking changes=%#v", diff.BreakingChanges)
	}
}
