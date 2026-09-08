package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/paymentprovider/integration"
)

func TestInitProjectRendersConsumerTemplates(t *testing.T) {
	root := t.TempDir()
	initDir := filepath.Join(root, ".ang", "init")
	if err := copyTree(filepath.Join("testdata", "init"), initDir); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "widget")
	result, err := integration.InitProject(integration.InitOptions{
		ProjectPath: dir,
		SID:         "mx6",
		Label:       "MX-6",
		PackageName: "mx6_centrobill",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 2 {
		t.Fatalf("created = %v skipped = %v", result.Created, result.Skipped)
	}
	got, err := os.ReadFile(filepath.Join(dir, "greeting.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"sid=mx6",
		"package=mx6_centrobill",
		"pascal=Mx6Centrobill",
		"upper=MX6",
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("greeting.txt missing %q:\n%s", want, got)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "nested", "item.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestInitProjectUsesExplicitInitDir(t *testing.T) {
	dir := t.TempDir()
	result, err := integration.InitProject(integration.InitOptions{
		ProjectPath: dir,
		InitDir:     filepath.Join("testdata", "init"),
		SID:         "ab",
		Label:       "AB",
		PackageName: "foo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) == 0 {
		t.Fatalf("created = %v", result.Created)
	}
}

func TestInitProjectRequiresConsumerTemplates(t *testing.T) {
	_, err := integration.InitProject(integration.InitOptions{
		ProjectPath: t.TempDir(),
		SID:         "ab",
		Label:       "AB",
	})
	if err == nil || !strings.Contains(err.Error(), ".ang/init") {
		t.Fatalf("err = %v", err)
	}
}

func TestInitProjectSkipExisting(t *testing.T) {
	dir := t.TempDir()
	opts := integration.InitOptions{
		ProjectPath: dir,
		InitDir:     filepath.Join("testdata", "init"),
		SID:         "ab",
		Label:       "AB",
		PackageName: "foo",
	}
	if _, err := integration.InitProject(opts); err != nil {
		t.Fatal(err)
	}
	result, err := integration.InitProject(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 0 || len(result.Skipped) == 0 {
		t.Fatalf("created = %v skipped = %v", result.Created, result.Skipped)
	}
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func TestLoadBriefFromExpert(t *testing.T) {
	expertRoot := filepath.Clean(filepath.Join("..", "..", "..", "..", "deal", "expert"))
	if _, err := os.Stat(filepath.Join(expertRoot, "knowledge", "data", "centrobill-mx6.json")); err != nil {
		t.Skip("deal/expert knowledge not available at", expertRoot)
	}
	brief, err := integration.LoadBriefFromExpert(expertRoot, "centrobill-mx6")
	if err != nil {
		t.Fatal(err)
	}
	if brief.Provider.Code != "mx6" || brief.Implementation != "investigation" {
		t.Fatalf("brief = %+v", brief)
	}
	if brief.References.ExpertKnowledge != "centrobill-mx6" {
		t.Fatalf("expert knowledge ref = %q", brief.References.ExpertKnowledge)
	}
}

func TestValidateExpertPack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pack.cue")
	if err := os.WriteFile(path, []byte(`
manifest: {
	id: "demo.research"
	version: "0.0.1"
	goals: ["payment_provider.audit"]
	reads: ["pp_operation"]
	writes: ["finding"]
}
rules: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := integration.ValidateExpertPack(path); err != nil {
		t.Fatal(err)
	}
}
