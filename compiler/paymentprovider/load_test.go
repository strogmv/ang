package paymentprovider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_minimalFixture(t *testing.T) {
	root := filepath.Join("testdata", "minimal")
	spec, err := Load(root, ".cue", "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if spec.SID != "testpp" {
		t.Fatalf("sid: %q", spec.SID)
	}
	if spec.PayinRequest == nil || len(spec.PayinRequest.Fields) == 0 {
		t.Fatal("expected payin_request fields")
	}
}

func TestBuild_minimalFixture(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(wd, "testdata", "minimal")
	out := t.TempDir()
	err = Build(BuildOptions{
		ProjectPath:  root,
		CueRoot:      ".cue",
		TemplatesDir: filepath.Join(wd, "testdata", "templates"),
		OutputDir:    out,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, name := range []string{"datatypes.go", "creds.go", "sign.go", "testpp.go", "testpp_test.go"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(out, "testpp.go"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	if !contains(content, "tx.Id") {
		t.Fatalf("expected generated payin field mapping, got:\n%s", content)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
