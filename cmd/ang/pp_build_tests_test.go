package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindGoModuleRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "payment_providers", "demo")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := findGoModuleRoot(nested)
	if err != nil {
		t.Fatalf("findGoModuleRoot: %v", err)
	}
	if got != root {
		t.Fatalf("module root = %q, want %q", got, root)
	}
}

func TestRunPaymentProviderGoTestsSkippedWithoutModule(t *testing.T) {
	dir := t.TempDir()
	status, codes := runPaymentProviderGoTests(dir)
	if status != "skipped" {
		t.Fatalf("status = %q", status)
	}
	if len(codes) != 0 {
		t.Fatalf("codes = %v", codes)
	}
}
