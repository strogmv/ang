package goemitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceImplStepUsesCanonicalEmitterPath(t *testing.T) {
	registerPath := filepath.Join("register.go")
	src, err := os.ReadFile(registerPath)
	if err != nil {
		t.Fatalf("read %s: %v", registerPath, err)
	}
	text := string(src)

	if !strings.Contains(text, `Name: "Service Impls"`) {
		t.Fatal(`missing "Service Impls" step in go register`)
	}
	if !strings.Contains(text, `in.Em.EmitServiceImplFromIR(in.IRSchema, in.AuthDef)`) {
		t.Fatal(`"Service Impls" step is not wired to EmitServiceImplFromIR`)
	}
}
