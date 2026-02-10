package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/compiler"
)

func TestAnalyze_FSMAutoFixAndMetrics(t *testing.T) {
	tmp := t.TempDir()
	analyzer := NewAnalyzer(tmp)

	log1 := "ERROR [E_FSM_UNDEFINED_STATE]: Entity 'Order' FSM transition 'paid→shipped' references undefined state 'paid'\n   at cue/domain/order.cue:3:0\nBuild FAILED due to diagnostic errors."
	resp1 := analyzer.Analyze(log1)

	if resp1.Iteration != 1 {
		t.Fatalf("iteration#1 = %d, want 1", resp1.Iteration)
	}
	if resp1.ErrorsRemaining != 1 {
		t.Fatalf("errors_remaining#1 = %d, want 1", resp1.ErrorsRemaining)
	}
	if len(resp1.AutoFixable) != 1 {
		t.Fatalf("expected one auto_fixable item, got %d", len(resp1.AutoFixable))
	}
	if resp1.AutoFixable[0].Code != "E_FSM_UNDEFINED_STATE" {
		t.Fatalf("unexpected code: %s", resp1.AutoFixable[0].Code)
	}
	if resp1.AutoFixable[0].Patch["path"] != "cue/domain/order.cue" {
		t.Fatalf("unexpected patch path: %#v", resp1.AutoFixable[0].Patch["path"])
	}

	log2 := "Build SUCCESSFUL."
	resp2 := analyzer.Analyze(log2)
	if resp2.Iteration != 2 {
		t.Fatalf("iteration#2 = %d, want 2", resp2.Iteration)
	}
	if resp2.ErrorsFixed != 1 {
		t.Fatalf("errors_fixed#2 = %d, want 1", resp2.ErrorsFixed)
	}
	if resp2.ErrorsRemaining != 0 {
		t.Fatalf("errors_remaining#2 = %d, want 0", resp2.ErrorsRemaining)
	}

	if _, err := os.Stat(filepath.Join(tmp, ".ang", "doctor_state.json")); err != nil {
		t.Fatalf("state file not written: %v", err)
	}

	if resp1.CatalogTotal != len(BuildSuggestionCatalog(log1)) {
		t.Fatalf("catalog_total mismatch: %d", resp1.CatalogTotal)
	}
}

func TestBuildSuggestionCatalog_CoversAllCodes(t *testing.T) {
	cat := BuildSuggestionCatalog("")
	want := len(compiler.StableErrorCodes) + 1 // + E_FSM_UNDEFINED_STATE
	if len(cat) != want {
		t.Fatalf("catalog size = %d, want %d", len(cat), want)
	}
	for _, s := range cat {
		if s.Code == "" || s.Fix == "" || s.Patch == nil {
			t.Fatalf("invalid catalog item: %#v", s)
		}
	}
}
