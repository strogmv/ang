package main

import (
	"strings"
	"testing"
)

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

// contractWith is a contract whose POST /quotes reads QuoteRequest and answers
// QuoteResponse (with a nested QuoteLine), and whose PUT /lines both reads and
// answers SharedLine; schemas is the components.schemas block.
func contractWith(schemas string) []byte {
	return []byte(`openapi: 3.0.0
paths:
  /quotes:
    post:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/QuoteRequest'
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/QuoteResponse'
  /lines:
    put:
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/SharedLine'
      responses:
        '200':
          description: OK
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/SharedLine'
components:
  schemas:
` + schemas)
}

const quoteSchemasBefore = `    QuoteRequest:
      required: [id]
      properties:
        id: {type: string}
        note: {type: string}
    QuoteResponse:
      required: [id, total]
      properties:
        id: {type: string}
        total: {type: number}
        lines: {type: array, items: {$ref: '#/components/schemas/QuoteLine'}}
    QuoteLine:
      required: [sku]
      properties:
        sku: {type: string}
    SharedLine:
      required: [sku]
      properties:
        sku: {type: string}
`

func TestDiffOpenAPIContractsJudgesFieldsByDirection(t *testing.T) {
	cases := []struct {
		name  string
		after string
		want  []string
	}{
		{"response gains required fields", `    QuoteRequest:
      required: [id]
      properties:
        id: {type: string}
        note: {type: string}
    QuoteResponse:
      required: [id, total, currency]
      properties:
        id: {type: string}
        total: {type: number}
        currency: {type: string}
        lines: {type: array, items: {$ref: '#/components/schemas/QuoteLine'}}
    QuoteLine:
      required: [sku, qty]
      properties:
        sku: {type: string}
        qty: {type: integer}
    SharedLine:
      required: [sku]
      properties:
        sku: {type: string}
`, nil},
		{"request needs a new field, an optional one becomes required", `    QuoteRequest:
      required: [id, note, buyer]
      properties:
        id: {type: string}
        note: {type: string}
        buyer: {type: string}
    QuoteResponse:
      required: [id, total]
      properties:
        id: {type: string}
        total: {type: number}
        lines: {type: array, items: {$ref: '#/components/schemas/QuoteLine'}}
    QuoteLine:
      required: [sku]
      properties:
        sku: {type: string}
    SharedLine:
      required: [sku]
      properties:
        sku: {type: string}
`, []string{"added required field QuoteRequest.buyer", "field became required QuoteRequest.note"}},
		{"response loses a field and a guarantee, a request field changes type", `    QuoteRequest:
      required: [id]
      properties:
        id: {type: integer}
        note: {type: string}
    QuoteResponse:
      required: [id]
      properties:
        id: {type: string}
        total: {type: number}
    QuoteLine:
      required: [sku]
      properties:
        sku: {type: string}
    SharedLine:
      required: [sku]
      properties:
        sku: {type: string}
`, []string{"changed field type QuoteRequest.id (string -> integer)", "removed field QuoteResponse.lines", "response field no longer required QuoteResponse.total"}},
		{"a schema both read and answered keeps both rules", `    QuoteRequest:
      required: [id]
      properties:
        id: {type: string}
        note: {type: string}
    QuoteResponse:
      required: [id, total]
      properties:
        id: {type: string}
        total: {type: number}
        lines: {type: array, items: {$ref: '#/components/schemas/QuoteLine'}}
    QuoteLine:
      required: [sku]
      properties:
        sku: {type: string}
    SharedLine:
      required: [sku, qty]
      properties:
        sku: {type: string}
        qty: {type: integer}
`, []string{"added required field SharedLine.qty"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diff, err := diffOpenAPIContracts(contractWith(quoteSchemasBefore), contractWith(tc.after))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(diff.BreakingChanges, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("breaking changes:\n%s\nwant:\n%s", strings.Join(diff.BreakingChanges, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}
