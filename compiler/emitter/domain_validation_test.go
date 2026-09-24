package emitter

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

// An optional enum left unset is valid, a value outside the enum is a
// FieldValidationError (which the HTTP layer answers with 400, not 500). An
// optional `incoterms?: "EXW" | ...` used to refuse every offer saved without
// incoterms, and the refusal surfaced as an internal error.
func TestDomainValidateOptionalEnumAndTypedError(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles the generated package")
	}
	tmp := t.TempDir()
	em := &Emitter{OutputDir: tmp, GoModule: "example.com/project"}
	minLen := 2
	entity := ir.Entity{
		Name: "Offer",
		Fields: []ir.Field{
			{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
			{Name: "status", Type: ir.TypeRef{Kind: ir.KindString}, Constraints: &ir.Constraints{Enum: []string{"draft", "sent"}}},
			{Name: "incoterms", Type: ir.TypeRef{Kind: ir.KindString}, Optional: true, Constraints: &ir.Constraints{Enum: []string{"EXW", "DAP"}}},
			{Name: "code", Type: ir.TypeRef{Kind: ir.KindString}, Optional: true, Constraints: &ir.Constraints{MinLen: &minLen}},
		},
	}
	if err := em.EmitDomain([]ir.Entity{entity}); err != nil {
		t.Fatalf("EmitDomain: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/project\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	check := `package domain

import (
	"errors"
	"testing"
)

func TestGenerated(t *testing.T) {
	if err := (&Offer{Status: "draft"}).Validate(); err != nil {
		t.Fatalf("unset optional enum and min length must pass: %v", err)
	}
	if err := (&Offer{Status: "draft", Incoterms: "DAP", Code: "AB"}).Validate(); err != nil {
		t.Fatalf("valid values must pass: %v", err)
	}
	var fe *FieldValidationError
	if err := (&Offer{Status: "draft", Incoterms: "XXX"}).Validate(); !errors.As(err, &fe) || fe.Field != "incoterms" {
		t.Fatalf("a value outside the enum must be a FieldValidationError on incoterms, got %v", err)
	}
	if err := (&Offer{Status: "draft", Code: "A"}).Validate(); !errors.As(err, &fe) || fe.Field != "code" {
		t.Fatalf("a given value shorter than the minimum must fail, got %v", err)
	}
	if err := (&Offer{}).Validate(); !errors.As(err, &fe) || fe.Field != "status" {
		t.Fatalf("a required enum stays required, got %v", err)
	}
}
`
	if err := os.WriteFile(filepath.Join(tmp, "internal", "domain", "generated_check_test.go"), []byte(check), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "./internal/domain/")
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated domain package: %v\n%s", err, out)
	}
}
