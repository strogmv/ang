package compiler

import (
	"testing"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func TestAttachTemplates(t *testing.T) {
	schema := &ir.Schema{}
	AttachTemplates(schema, []normalizer.TemplateDef{
		{
			ID:           "tender_created_email",
			Kind:         "email",
			Channel:      "email",
			Locale:       "en",
			Version:      "v1",
			Engine:       "go_template",
			Subject:      "Tender {{.ID}}",
			Text:         "Body",
			RequiredVars: []string{"ID"},
			OptionalVars: []string{"CompanyName"},
		},
	})
	if len(schema.Templates) != 1 {
		t.Fatalf("expected 1 template in IR, got %d", len(schema.Templates))
	}
	if schema.Templates[0].ID != "tender_created_email" {
		t.Fatalf("unexpected template id: %q", schema.Templates[0].ID)
	}
	if schema.Templates[0].Channel != "email" {
		t.Fatalf("unexpected template channel: %q", schema.Templates[0].Channel)
	}
	if len(schema.Templates[0].RequiredVars) != 1 || schema.Templates[0].RequiredVars[0] != "ID" {
		t.Fatalf("unexpected required vars: %#v", schema.Templates[0].RequiredVars)
	}
	if len(schema.Templates[0].OptionalVars) != 1 || schema.Templates[0].OptionalVars[0] != "CompanyName" {
		t.Fatalf("unexpected optional vars: %#v", schema.Templates[0].OptionalVars)
	}
}
