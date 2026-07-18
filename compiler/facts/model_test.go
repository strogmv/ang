package facts

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestEnvelopeV1JSONRoundTrip(t *testing.T) {
	original := Envelope{
		Schema:     SchemaV1,
		SourceType: "go",
		SourcePath: "./example",
		Entities: []Entity{{
			Name:   "User",
			Fields: []Field{{Name: "ID", GoType: "string", Required: true}},
		}},
		Operations: []Operation{{
			Name:        "CreateUser",
			ServiceHint: "users",
			InputFields: []Field{{Name: "email", CueTypeHint: "string"}},
		}},
		Repositories: []Repository{{
			Entity:  "User",
			Methods: []RepositoryMethod{{Name: "FindByID", Returns: "one"}},
		}},
		Events: []Event{{Name: "UserCreated", PayloadFields: []Field{{Name: "user_id", GoType: "string"}}}},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Envelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", decoded, original)
	}
}

func TestEnvelopeV1JSONFieldNames(t *testing.T) {
	env := Envelope{
		Schema:        SchemaV1,
		SourceType:    "openapi",
		SourcePath:    "openapi.yaml",
		SecurityRules: []SecurityRule{{Requirement: "authenticated"}},
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"schema":"ang/facts/v1","source_type":"openapi","source_path":"openapi.yaml","entities":null,"operations":null,"repositories":null,"events":null,"security_rules":[{"requirement":"authenticated"}]}`
	if string(data) != want {
		t.Fatalf("unexpected v1 JSON\n got: %s\nwant: %s", data, want)
	}
}

func TestValidateAcceptsWellFormedV1Envelope(t *testing.T) {
	env := Envelope{
		Schema:       SchemaV1,
		Entities:     []Entity{{Name: "User", Fields: []Field{{Name: "ID"}}}},
		Operations:   []Operation{{Name: "CreateUser", InputFields: []Field{{Name: "email"}}}},
		Repositories: []Repository{{Entity: "User", Methods: []RepositoryMethod{{Name: "Create"}}}},
		Events:       []Event{{Name: "UserCreated", PayloadFields: []Field{{Name: "user_id"}}}},
	}
	if err := Validate(env); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateReportsStablePaths(t *testing.T) {
	err := Validate(Envelope{
		Schema:   "ang/facts/v2",
		Entities: []Entity{{Fields: []Field{{}}}},
		Events:   []Event{{PayloadFields: []Field{{}}}},
	})
	validation, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("Validate() error = %T (%v), want *ValidationError", err, err)
	}
	got := make([]string, 0, len(validation.Problems))
	for _, problem := range validation.Problems {
		got = append(got, problem.Path)
	}
	want := []string{
		"entities[0].fields[0].name",
		"entities[0].name",
		"events[0].name",
		"events[0].payload_fields[0].name",
		"schema",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("problem paths = %#v, want %#v", got, want)
	}
}

func TestCanonicalJSONAndHashIgnoreCollectionOrder(t *testing.T) {
	first := Envelope{
		Schema: SchemaV1,
		Entities: []Entity{
			{Name: "User", Fields: []Field{{Name: "Email"}, {Name: "ID"}}},
			{Name: "Account", Fields: []Field{{Name: "ID"}}},
		},
		Operations: []Operation{
			{Name: "Update", ServiceHint: "accounts", InputFields: []Field{{Name: "name"}, {Name: "id"}}},
			{Name: "Create", ServiceHint: "accounts"},
		},
	}
	second := Envelope{
		Schema: SchemaV1,
		Entities: []Entity{
			{Name: "Account", Fields: []Field{{Name: "ID"}}},
			{Name: "User", Fields: []Field{{Name: "ID"}, {Name: "Email"}}},
		},
		Operations: []Operation{
			{Name: "Create", ServiceHint: "accounts"},
			{Name: "Update", ServiceHint: "accounts", InputFields: []Field{{Name: "id"}, {Name: "name"}}},
		},
	}

	firstJSON, err := CanonicalJSON(first)
	if err != nil {
		t.Fatalf("CanonicalJSON(first): %v", err)
	}
	secondJSON, err := CanonicalJSON(second)
	if err != nil {
		t.Fatalf("CanonicalJSON(second): %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("canonical JSON differs\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
	firstHash, err := Hash(first)
	if err != nil {
		t.Fatalf("Hash(first): %v", err)
	}
	secondHash, err := Hash(second)
	if err != nil {
		t.Fatalf("Hash(second): %v", err)
	}
	if firstHash != secondHash {
		t.Fatalf("hash mismatch: %s != %s", firstHash, secondHash)
	}
	if first.Entities[0].Name != "User" || first.Entities[0].Fields[0].Name != "Email" {
		t.Fatal("CanonicalJSON mutated its input")
	}
}
