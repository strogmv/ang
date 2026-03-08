package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractEntities_Compliance(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
		#Sensitive: {
			email: string @pii(classification="restricted")
			notes: string @redact()
			secret: string @encrypt(mode="deterministic")
		}
	`)

	n := New()
	entities, err := n.ExtractEntities(val)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(entities))
	}

	fields := entities[0].Fields
	for _, f := range fields {
		switch f.Name {
		case "email":
			if !f.IsPII {
				t.Error("email should be PII")
			}
			if f.Metadata["pii_classification"] != "restricted" {
				t.Errorf("expected restricted classification, got %v", f.Metadata["pii_classification"])
			}
		case "notes":
			if f.Metadata["redact"] != true {
				t.Error("notes should be redacted")
			}
		case "secret":
			if f.Metadata["encrypt"] != "deterministic" {
				t.Errorf("expected deterministic encryption, got %v", f.Metadata["encrypt"])
			}
		}
	}
}

func TestExtractServices_Audit(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
		Login: {
			service: "auth"
			@audit("user.login")
			input: email: string
			output: ok: bool
		}
	`)

	n := New()
	services, err := n.ExtractServices(val, nil)
	if err != nil {
		t.Fatalf("ExtractServices failed: %v", err)
	}

	if len(services) != 1 {
		t.Fatalf("Expected 1 service, got %d", len(services))
	}

	m := services[0].Methods[0]
	if m.Metadata["audit"] != true {
		t.Error("method should have audit enabled")
	}
	if m.Metadata["audit_event"] != "user.login" {
		t.Errorf("expected audit event user.login, got %v", m.Metadata["audit_event"])
	}
}

func TestExtractEntities_BoundedContextAttribute(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
		#TenderCategory: {
			id: string
		} @bounded_context("tender")
	`)

	n := New()
	entities, err := n.ExtractEntities(val)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(entities))
	}
	if entities[0].BoundedContext != "tender" {
		t.Fatalf("expected bounded context 'tender', got %q", entities[0].BoundedContext)
	}
}

func TestExtractEntities_AggregateOwnership(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
		#Tender: {
			id: string
			root: true
			owns: ["TenderCategory", "TenderInvite"]
		} @bounded_context("tender")
	`)

	n := New()
	entities, err := n.ExtractEntities(val)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(entities))
	}
	if !entities[0].AggregateRoot {
		t.Fatalf("expected AggregateRoot=true")
	}
	if len(entities[0].Owns) != 2 {
		t.Fatalf("expected 2 owned entities, got %d", len(entities[0].Owns))
	}
	if entities[0].Owns[0] != "TenderCategory" || entities[0].Owns[1] != "TenderInvite" {
		t.Fatalf("unexpected owns list: %#v", entities[0].Owns)
	}
}

func TestExtractEntities_SharedArchMetadata(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
		#Company: {
			id: string
		} @shared_arch(reason="cross-context lookups", ticket="ARCH-123")
	`)

	n := New()
	entities, err := n.ExtractEntities(val)
	if err != nil {
		t.Fatalf("ExtractEntities failed: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(entities))
	}
	if shared, _ := entities[0].Metadata["shared_arch"].(bool); !shared {
		t.Fatalf("expected shared_arch=true in metadata")
	}
	if got := entities[0].Metadata["shared_arch_reason"]; got != "cross-context lookups" {
		t.Fatalf("expected shared_arch_reason, got %#v", got)
	}
	if got := entities[0].Metadata["shared_arch_ticket"]; got != "ARCH-123" {
		t.Fatalf("expected shared_arch_ticket, got %#v", got)
	}
}
