package paymentprovider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildWithResult_manifestHashStable(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(wd, "testdata", "minimal")
	out := t.TempDir()
	result, err := BuildWithResult(BuildOptions{
		ProjectPath:  root,
		CueRoot:      ".cue",
		TemplatesDir: filepath.Join(wd, "testdata", "templates"),
		OutputDir:    out,
	})
	if err != nil {
		t.Fatalf("BuildWithResult: %v", err)
	}
	if len(result.Files) == 0 {
		t.Fatal("expected generated files")
	}
	first, err := result.ManifestHash()
	if err != nil {
		t.Fatalf("ManifestHash: %v", err)
	}
	second, err := result.ManifestHash()
	if err != nil {
		t.Fatalf("ManifestHash: %v", err)
	}
	if first != second {
		t.Fatalf("manifest hash not stable: %q vs %q", first, second)
	}
}

func TestBuildWithResult_matchesBuild(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(wd, "testdata", "minimal")
	out := t.TempDir()
	if err := Build(BuildOptions{
		ProjectPath:  root,
		CueRoot:      ".cue",
		TemplatesDir: filepath.Join(wd, "testdata", "templates"),
		OutputDir:    out,
	}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	result, err := BuildWithResult(BuildOptions{
		ProjectPath:  root,
		CueRoot:      ".cue",
		TemplatesDir: filepath.Join(wd, "testdata", "templates"),
		OutputDir:    out,
	})
	if err != nil {
		t.Fatalf("BuildWithResult: %v", err)
	}
	if len(result.Files) == 0 {
		t.Fatal("expected manifest entries")
	}
}
