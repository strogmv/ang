package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

func TestEmitEmailTemplatesFromIR_PreservesRequiredVars(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	em.GoModule = "example.com/project"

	schema := &ir.Schema{
		Templates: []ir.Template{
			{
				ID:           "password_reset",
				Kind:         "email",
				Channel:      "email",
				Subject:      "Reset",
				Text:         "Link {{.ResetURL}}",
				RequiredVars: []string{"ResetURL"},
				OptionalVars: []string{"Name"},
			},
		},
	}
	defs := []normalizer.EmailTemplateDef{
		{Name: "password_reset", Subject: "Reset", Text: "Link {{.ResetURL}}"},
	}

	if err := em.EmitEmailTemplatesFromIR(schema, defs); err != nil {
		t.Fatalf("EmitEmailTemplatesFromIR: %v", err)
	}

	path := filepath.Join(tmp, "internal", "pkg", "emailtemplates", "templates.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	src := string(data)
	for _, want := range []string{
		`RequiredVars: []string{"ResetURL"}`,
		`OptionalVars: []string{"Name"}`,
		`missing required template var`,
		`func hasTemplateVar`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("generated email templates missing %q\n%s", want, src)
		}
	}
}
