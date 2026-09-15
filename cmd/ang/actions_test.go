package main

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/flowir"
)

func TestDocumentedActionCatalogIncludesRendererGroup(t *testing.T) {
	t.Parallel()

	entries := documentedActionCatalog(mergedActionCatalog())
	if len(entries) == 0 {
		t.Fatal("expected documented action catalogue")
	}
	for _, entry := range entries {
		spec, ok := flowir.Lookup(entry.Name)
		if !ok {
			t.Fatalf("catalog action %q is absent from Typed Flow IR", entry.Name)
		}
		if entry.RendererGroup != spec.RendererGroup {
			t.Fatalf("catalog action %q renderer group = %q, want %q", entry.Name, entry.RendererGroup, spec.RendererGroup)
		}
	}
}

func TestRenderActionCatalogCUEIncludesRendererGroup(t *testing.T) {
	t.Parallel()

	output := renderActionCatalogCUE(documentedActionCatalog(mergedActionCatalog()))
	if !strings.Contains(output, "renderer_group:") {
		t.Fatalf("CUE action catalogue does not expose renderer_group:\n%s", output)
	}
}

// The catalog shows real steps, not "<expr>" placeholders.
func TestActionCatalogExamplesAreRealSteps(t *testing.T) {
	t.Parallel()

	for _, entry := range mergedActionCatalog() {
		if strings.Contains(entry.Example, "<expr>") {
			t.Errorf("%s example is a placeholder: %s", entry.Name, entry.Example)
		}
		if entry.Name == "repo.Query" && !strings.Contains(entry.Example, `source: "`) {
			t.Errorf("repo.Query example = %s", entry.Example)
		}
	}
}

func TestExplainActionShowsCatalogEntry(t *testing.T) {
	t.Parallel()

	entry, ok := explainAction("repo.Query")
	if !ok {
		t.Fatal("repo.Query is not in the catalog")
	}
	text := renderActionExplanation(entry)
	if !strings.Contains(text, "repo.Query") || !strings.Contains(text, "Example:") || !strings.Contains(text, `source: "`) {
		t.Fatalf("explanation:\n%s", text)
	}
	if _, ok := explainAction("no.SuchAction"); ok {
		t.Fatal("unknown action must not be found")
	}
}
