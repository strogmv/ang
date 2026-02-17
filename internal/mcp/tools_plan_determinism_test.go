package mcp

import "testing"

func TestAssessDeterminismRiskFlags(t *testing.T) {
	patches := []map[string]any{
		{
			"path":     "cue/project/project.cue",
			"selector": "",
			"content":  "targets: { go: { framework: \"chi\" } }",
		},
	}
	risks := assessDeterminismRiskFlags(patches)
	expect := []string{"build_target_change", "root_level_merge", "unordered_map_semantics"}
	for _, want := range expect {
		found := false
		for _, got := range risks {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected risk flag %q in %v", want, risks)
		}
	}
}

func TestDiffHashReasons(t *testing.T) {
	base := []artifactHashRecord{
		{Path: "a.txt", Hash: "1"},
		{Path: "b.txt", Hash: "2"},
	}
	next := []artifactHashRecord{
		{Path: "b.txt", Hash: "3"},
		{Path: "c.txt", Hash: "4"},
	}
	reasons := diffHashReasons(base, next)
	if len(reasons) != 3 {
		t.Fatalf("expected three drift reasons, got %d: %v", len(reasons), reasons)
	}
}
