package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestEmitDiagnosticsDeduplicatesSuppressesAndSummarizes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.cue")
	if err := os.WriteFile(path, []byte("//ang:nolint SUPPRESSED\noperation: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diagnostics := []normalizer.Warning{
		{Code: "VISIBLE", Severity: "warn", Message: "visible", File: path, Line: 2},
		{Code: "VISIBLE", Severity: "warn", Message: "visible", File: path, Line: 2},
		{Code: "SUPPRESSED", Severity: "warn", Message: "hidden", File: path, Line: 2},
	}
	var output bytes.Buffer
	if emitDiagnostics(&output, diagnostics) {
		t.Fatal("warnings must not be reported as errors")
	}
	text := output.String()
	if strings.Count(text, "[VISIBLE]") != 1 {
		t.Fatalf("visible diagnostic was not deduplicated:\n%s", text)
	}
	if strings.Contains(text, "hidden") || !strings.Contains(text, "1 warnings, 1 suppressed") {
		t.Fatalf("unexpected suppression output:\n%s", text)
	}
}
