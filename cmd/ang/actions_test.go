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
