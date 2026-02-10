package mcp

import (
	"strings"
	"testing"
)

func TestApplySetFieldPatch_AddField(t *testing.T) {
	src := []byte(`package domain

#User: {
	fields: {
		id: { type: "uuid" }
	}
}
`)
	out, changed, err := applySetFieldPatch(src, "User", "email", "string", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	s := string(out)
	if !strings.Contains(s, "email:") || !strings.Contains(s, `type: "string"`) {
		t.Fatalf("expected generated email field, got:\n%s", s)
	}
}

func TestApplySetFieldPatch_ExistingRequiresOverwrite(t *testing.T) {
	src := []byte(`package domain

#User: {
	fields: {
		email: { type: "string" }
	}
}
`)
	_, _, err := applySetFieldPatch(src, "User", "email", "int", false, false)
	if err == nil {
		t.Fatal("expected error for existing field without overwrite")
	}
	if !strings.Contains(err.Error(), "overwrite=true") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplySetFieldPatch_Overwrite(t *testing.T) {
	src := []byte(`package domain

#User: {
	fields: {
		email: { type: "string" }
	}
}
`)
	out, changed, err := applySetFieldPatch(src, "User", "email", "int", true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	s := string(out)
	if !strings.Contains(s, "email?:") {
		t.Fatalf("expected optional label, got:\n%s", s)
	}
	if !strings.Contains(s, `type: "int"`) {
		t.Fatalf("expected overwritten type, got:\n%s", s)
	}
}

func TestApplySetFieldPatch_EntityNotFound(t *testing.T) {
	src := []byte(`package domain

#User: { fields: {} }
`)
	_, _, err := applySetFieldPatch(src, "Order", "status", "string", false, false)
	if err == nil {
		t.Fatal("expected entity not found error")
	}
}

