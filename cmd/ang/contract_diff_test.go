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

func TestDiffOpenAPIContractsRecoversMalformedGeneratedBaseline(t *testing.T) {
	previous := []byte("openapi: 3.0.0\ninfo:\n  description: invalid: scalar\n")
	current := []byte("openapi: 3.0.0\npaths:\n  /health:\n    get: {}\n")

	diff, recovered, err := diffOpenAPIContractsWithRecovery(previous, current)
	if err != nil {
		t.Fatal(err)
	}
	if !recovered {
		t.Fatal("expected malformed generated baseline recovery")
	}
	if len(diff.AddedOperations) != 1 || diff.AddedOperations[0] != "GET /health" {
		t.Fatalf("unexpected recovered diff: %#v", diff)
	}
}

func TestDiffOpenAPIContractsRecoveryStillRejectsMalformedCurrent(t *testing.T) {
	malformed := []byte("openapi: 3.0.0\ninfo:\n  description: invalid: scalar\n")
	if _, _, err := diffOpenAPIContractsWithRecovery(malformed, malformed); err == nil {
		t.Fatal("expected malformed current contract error")
	}
}
