package lsp

import (
	"strings"
	"testing"
)

func TestHoverExampleIsARealStep(t *testing.T) {
	entry, ok := catalogEntry("repo.Query")
	if !ok {
		t.Fatal("repo.Query is not in the catalog")
	}
	example := actionExample(entry)
	if strings.Contains(example, "<expr>") || !strings.Contains(example, `source: "`) {
		t.Fatalf("hover example = %s", example)
	}
}
