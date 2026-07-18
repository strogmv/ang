package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/compiler/paymentprovider/integration"
)

func TestInitProjectCreatesProviderScaffold(t *testing.T) {
	dir := t.TempDir()
	result, err := integration.InitProject(integration.InitOptions{
		ProjectPath:   dir,
		SID:           "mx6",
		Label:         "MX-6",
		Name:          "CentroBill",
		PackageName:   "mx6_centrobill",
		KnowledgeID:   "centrobill-mx6",
		TicketSummary: "Integrate Apple Pay and Google Pay for Centrobill",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) < 3 {
		t.Fatalf("created = %v skipped = %v", result.Created, result.Skipped)
	}
	for _, rel := range []string{
		"ang.yaml",
		".cue/provider.cue",
		".cue/cue.mod/module.cue",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		"integration/brief.yaml",
		"integration/api-notes.md",
		"integration/expert.pack.cue",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			t.Fatalf("unexpected legacy artifact %s", rel)
		}
	}
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
