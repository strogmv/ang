package emitter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalizeLineDirectivesMapsMarkedLinesAndResets(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	src := strings.Join([]string{
		"package service",
		"",
		"func F() (int, error) {",
		"\t//ang:line cue/api/impl.cue:6 1",
		"\tif !(x != \"\") {",
		"\t\treturn 0, nil",
		"\t}",
		"\t//ang:line cue/api/impl.cue:13 3",
		"\t// explicit ignoreErr=true",
		"\ty, err := (func() (int, error) {",
		"\t\treturn 1, nil",
		"\t})()",
		"\tif err != nil {",
		"\t\treturn 0, err",
		"\t}",
		"\t//ang:line cue/api/impl.cue:30 1",
		"\t//ang:line cue/api/impl.cue:31 1",
		"\treturn y, nil",
		"}",
		"",
	}, "\n")
	got := string(finalizeLineDirectives([]byte(src), "internal/service/x.gen.go"))
	want := strings.Join([]string{
		"package service",
		"",
		"func F() (int, error) {",
		"//line ../../cue/api/impl.cue:6",
		"\tif !(x != \"\") {",
		"//line x.gen.go:7",
		"\t\treturn 0, nil",
		"\t}",
		"\t// explicit ignoreErr=true",
		"//line ../../cue/api/impl.cue:13",
		"\ty, err := (func() (int, error) {",
		"\t\treturn 1, nil",
		"\t})()",
		"//line x.gen.go:15",
		"\tif err != nil {",
		"\t\treturn 0, err",
		"\t}",
		"//line ../../cue/api/impl.cue:31",
		"\treturn y, nil",
		"//line x.gen.go:21",
		"}",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// The compiler must report a type error inside mapped code at the CUE line,
// and code after the step at its real place in the generated file.
func TestFinalizeLineDirectivesCompilerReportsCUELines(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	root := t.TempDir()
	t.Chdir(root)
	write := func(rel, content string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module probe\n\ngo 1.22\n")
	src := strings.Join([]string{
		"package service",
		"",
		"func F() int {",
		"\t//ang:line cue/api/impl.cue:40 3",
		"\tv := (func() int {",
		"\t\treturn missingInCUE",
		"\t})()",
		"\treturn v + missingInGo",
		"}",
		"",
	}, "\n")
	goPath := "internal/service/x.gen.go"
	write(goPath, string(finalizeLineDirectives([]byte(src), goPath)))
	cmd := exec.Command("go", "build", "./...")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("build must fail")
	}
	for _, want := range []string{"cue/api/impl.cue:41:", "internal/service/x.gen.go:9:"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("go build output lacks %q:\n%s", want, out)
		}
	}
}
