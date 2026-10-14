package emitter

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoPackageFilesKeepsOnePackageInNameOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("b.go", "package port\n\ntype B interface{}\n")
	write("a.go", "package port\n\n// A is documented.\ntype A interface{}\n")
	write("a_test.go", "package port\n")
	write("other.go", "package mock\n")
	write("notes.txt", "package port\n")
	if err := os.Mkdir(filepath.Join(dir, "sub.go"), 0o755); err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	files, err := parseGoPackageFiles(fset, dir, "port", parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range files {
		names = append(names, filepath.Base(fset.Position(file.Package).Filename))
	}
	if strings.Join(names, ",") != "a.go,b.go" {
		t.Fatalf("want a.go,b.go, got %v", names)
	}
	if len(files[0].Comments) != 1 {
		t.Fatalf("comments must be kept with ParseComments, got %d", len(files[0].Comments))
	}

	write("broken.go", "package port\nfunc (\n")
	if _, err := parseGoPackageFiles(token.NewFileSet(), dir, "port", 0); err == nil {
		t.Fatal("a file that does not parse must fail the call")
	}
}
