package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/doctor"
)

func TestBuildDoctorResponse_FSMAutoFixAndMetrics(t *testing.T) {
	tmp := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	log1 := "⚠️  ERROR [E_FSM_UNDEFINED_STATE]: Entity 'Order' FSM transition 'paid→shipped' references undefined state 'paid'\n   at cue/domain/order.cue:3:0\nBuild FAILED due to diagnostic errors."
	resp1 := buildDoctorResponse(log1)

	if got := intFromMap(resp1, "iteration"); got != 1 {
		t.Fatalf("iteration#1 = %d, want 1", got)
	}
	if got := intFromMap(resp1, "errors_remaining"); got != 1 {
		t.Fatalf("errors_remaining#1 = %d, want 1", got)
	}
	auto, ok := resp1["auto_fixable"].([]any)
	if !ok || len(auto) != 1 {
		t.Fatalf("expected one auto_fixable item, got %#v", resp1["auto_fixable"])
	}
	item, ok := auto[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected auto_fixable item type: %#v", auto[0])
	}
	if item["code"] != "E_FSM_UNDEFINED_STATE" {
		t.Fatalf("unexpected code: %v", item["code"])
	}
	patch, ok := item["patch"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected patch payload: %#v", item["patch"])
	}
	if patch["path"] != "cue/domain/order.cue" {
		t.Fatalf("unexpected patch path: %#v", patch["path"])
	}

	log2 := "Build SUCCESSFUL."
	resp2 := buildDoctorResponse(log2)
	if got := intFromMap(resp2, "iteration"); got != 2 {
		t.Fatalf("iteration#2 = %d, want 2", got)
	}
	if got := intFromMap(resp2, "errors_fixed"); got != 1 {
		t.Fatalf("errors_fixed#2 = %d, want 1", got)
	}
	if got := intFromMap(resp2, "errors_remaining"); got != 0 {
		t.Fatalf("errors_remaining#2 = %d, want 0", got)
	}

	if _, err := os.Stat(filepath.Join(".ang", "doctor_state.json")); err != nil {
		t.Fatalf("state file not written: %v", err)
	}

	if got := intFromMap(resp1, "catalog_total"); got != len(doctor.BuildSuggestionCatalog(log1)) {
		t.Fatalf("catalog_total mismatch: %d", got)
	}
}

func TestSuggestionForStableCode_ReturnsPatchTemplate(t *testing.T) {
	cat := doctor.BuildSuggestionCatalog("CUE_DOMAIN_LOAD_ERROR")
	var found *doctor.Suggestion
	for i := range cat {
		if cat[i].Code == "CUE_DOMAIN_LOAD_ERROR" {
			found = &cat[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected CUE_DOMAIN_LOAD_ERROR in catalog")
	}
	if found.Patch == nil {
		t.Fatal("expected patch template for stable code")
	}
	if found.Patch["path"] == "" {
		t.Fatal("expected patch path")
	}
}

func TestBuildSuggestionCatalog_CoversAllCodes(t *testing.T) {
	cat := doctor.BuildSuggestionCatalog("")
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

func intFromMap(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}
