package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/imports"
)

// chdirTemp runs the test inside a fresh directory holding module example.com/app;
// formatGoStrict resolves units relative to the working directory.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	writeGoFormatTestFile(t, "go.mod", "module example.com/app\n\ngo 1.22\n")
	return dir
}

func writeGoFormatTestFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// rendered imports "fmt" and "strings", uses fmt, sha256 and base64 — the way
// a service template renders a method whose CUE body needs extra packages.
const renderedMethod = `package service

import (
	"fmt"
	"strings"
)

func Digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprint(base64.StdEncoding.EncodeToString(sum[:]))
}
`

func TestFormatGoStrictReusesPreviousImportsWithSameOutput(t *testing.T) {
	chdirTemp(t)
	unit := "internal/service/digest.gen.go"
	want, err := imports.Process(unit, []byte(renderedMethod), nil)
	if err != nil {
		t.Fatal(err)
	}
	writeGoFormatTestFile(t, unit, string(want))

	hinted, ok := withPreviousImports([]byte(renderedMethod), unit)
	if !ok {
		t.Fatal("the previous output covers every reference; the hint must apply")
	}
	if !strings.Contains(string(hinted), `"crypto/sha256"`) || !strings.Contains(string(hinted), `"encoding/base64"`) {
		t.Fatalf("hint lacks the previous imports:\n%s", hinted)
	}
	got, err := formatGoStrict([]byte(renderedMethod), unit)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("output differs from goimports:\n--- got\n%s\n--- want\n%s", got, want)
	}
}

func TestFormatGoStrictWithoutPreviousFileUsesGoimports(t *testing.T) {
	chdirTemp(t)
	unit := "internal/service/digest.gen.go"
	if _, ok := withPreviousImports([]byte(renderedMethod), unit); ok {
		t.Fatal("no previous file: the hint must not apply")
	}
	got, err := formatGoStrict([]byte(renderedMethod), unit)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := imports.Process(unit, []byte(renderedMethod), nil)
	if string(got) != string(want) || !strings.Contains(string(got), `"crypto/sha256"`) {
		t.Fatalf("got:\n%s", got)
	}
}

func TestFormatGoStrictNewReferenceFallsBackToGoimports(t *testing.T) {
	chdirTemp(t)
	unit := "internal/service/digest.gen.go"
	// The previous output predates base64: it cannot cover the new reference.
	writeGoFormatTestFile(t, unit, "package service\n\nimport \"crypto/sha256\"\n\nvar _ = sha256.Size\n")
	if _, ok := withPreviousImports([]byte(renderedMethod), unit); ok {
		t.Fatal("base64 is not in the previous file: the hint must not apply")
	}
	got, err := formatGoStrict([]byte(renderedMethod), unit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"encoding/base64"`) {
		t.Fatalf("goimports did not resolve the new reference:\n%s", got)
	}
}

func TestFormatGoStrictKeepsTemplateImportOverPreviousOne(t *testing.T) {
	chdirTemp(t)
	unit := "internal/service/rand.gen.go"
	writeGoFormatTestFile(t, unit, "package service\n\nimport (\n\t\"crypto/sha256\"\n\trand \"example.com/app/internal/pkg/oldrand\"\n)\n")
	src := "package service\n\nimport rand \"math/rand/v2\"\n\nfunc Pick() (int, [32]byte) { return rand.IntN(3), sha256.Sum256(nil) }\n"
	got, err := formatGoStrict([]byte(src), unit)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "oldrand") || !strings.Contains(string(got), `"math/rand/v2"`) || !strings.Contains(string(got), `"crypto/sha256"`) {
		t.Fatalf("template import must win and only the missing one be added:\n%s", got)
	}
}

func TestFormatGoStrictReportsInvalidSource(t *testing.T) {
	chdirTemp(t)
	if _, err := formatGoStrict([]byte("package service\n\nfunc {"), "internal/service/broken.gen.go"); err == nil || !strings.Contains(err.Error(), "generated go is invalid") {
		t.Fatalf("err = %v", err)
	}
}

func TestAssumedPackageName(t *testing.T) {
	for path, want := range map[string]string{
		"crypto/sha256":                           "sha256",
		"github.com/jackc/pgx/v5":                 "pgx",
		"github.com/aws/aws-sdk-go-v2/service/s3": "s3",
		"github.com/go-chi/chi/v5":                "chi",
		"gopkg.in/yaml.v3":                        "yaml",
		"math/rand/v2":                            "rand",
	} {
		if got := assumedPackageName(path); got != want {
			t.Errorf("assumedPackageName(%q) = %q, want %q", path, got, want)
		}
	}
}
