package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
)

// A secret field must not be reachable from a JSON encoder. Handlers routinely
// answer with a whole entity — an admin lookup returning *domain.APIKey, a list
// of users — and every one of those would otherwise publish the key hash, the
// password hash or the TOTP secret. The rule belongs in the struct tag, where
// no call site can forget it.
func TestDomainSecretFieldsAreNotSerialized(t *testing.T) {
	tmp := t.TempDir()
	em := &Emitter{OutputDir: tmp, GoModule: "example.com/project"}

	entity := ir.Entity{
		Name: "APIKey",
		Fields: []ir.Field{
			{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
			{Name: "keyHash", Type: ir.TypeRef{Kind: ir.KindString}, IsSecret: true},
			{Name: "email", Type: ir.TypeRef{Kind: ir.KindString}, IsPII: true},
		},
	}

	if err := em.EmitDomain([]ir.Entity{entity}); err != nil {
		t.Fatalf("EmitDomain: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(tmp, "internal", "domain", "apikey.go"))
	if err != nil {
		t.Fatalf("read generated entity: %v", err)
	}
	// gofmt pads the tags into a column, so compare without the padding.
	got := strings.Join(strings.Fields(string(b)), " ")

	if !strings.Contains(got, "KeyHash string `json:\"-\"`") {
		t.Errorf("a secret field is still serialized:\n%s", got)
	}
	// PII is masked in logs, not withheld from the client: a name and an email
	// are what the client asked for.
	if !strings.Contains(got, "Email string `json:\"email\"`") {
		t.Errorf("a PII field should keep its JSON name:\n%s", got)
	}
	if !strings.Contains(got, "ID string `json:\"id\"`") {
		t.Errorf("an ordinary field should keep its JSON name:\n%s", got)
	}
}
