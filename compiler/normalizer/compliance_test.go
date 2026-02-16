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
