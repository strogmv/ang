package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractTemplates(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
package infra

#Templates: {
  dir: "emails"
  items: [
    {
      id: "tender_created_email"
      kind: "email"
      channel: "email"
      locale: "en"
      subject: "Tender created"
      textFile: "tender_created.txt"
      htmlFile: "tender_created.html"
      requiredVars: ["temporary_token", "ttl_minutes"]
    },
  ]
}
`)
	if err := val.Err(); err != nil {
		t.Fatalf("compile cue: %v", err)
	}

	n := New()
	templates, err := n.ExtractTemplates(val)
	if err != nil {
		t.Fatalf("ExtractTemplates error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}
	got := templates[0]
	if got.ID != "tender_created_email" {
		t.Fatalf("unexpected id: %q", got.ID)
	}
	if got.TextFile != "emails/tender_created.txt" {
		t.Fatalf("unexpected textFile: %q", got.TextFile)
	}
	if got.HTMLFile != "emails/tender_created.html" {
		t.Fatalf("unexpected htmlFile: %q", got.HTMLFile)
	}
	if len(got.RequiredVars) != 2 || got.RequiredVars[0] != "temporary_token" || got.RequiredVars[1] != "ttl_minutes" {
		t.Fatalf("unexpected required vars: %#v", got.RequiredVars)
	}

	val2 := ctx.CompileString(`package infra
#Templates: [{id: "a", body: "{{.x}}", vars: {required: ["x"], optional:["y"]}}]`)
	tpls2, err := n.ExtractTemplates(val2)
	if err != nil || len(tpls2) != 1 {
		t.Fatalf("expected templates, err=%v templates=%#v", err, tpls2)
	}
	if len(tpls2[0].RequiredVars) != 1 || tpls2[0].RequiredVars[0] != "x" {
		t.Fatalf("expected required vars from vars.required, got %#v", tpls2[0].RequiredVars)
	}
	if len(tpls2[0].OptionalVars) != 1 || tpls2[0].OptionalVars[0] != "y" {
		t.Fatalf("expected optional vars from vars.optional, got %#v", tpls2[0].OptionalVars)
	}
}
