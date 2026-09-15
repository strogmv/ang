package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestTautologyVerdict(t *testing.T) {
	for condition, want := range map[string]string{
		"reservationWebhookSeller || !reservationWebhookSeller": "always true",
		"!ok || ok":                           "always true",
		"(a.B) && !(a.B)":                     "always false",
		"a || !b":                             "",
		"x != \"\" || !ok":                    "",
		"reservationWebhookSeller":            "",
		"!(req.ID == \"\") || req.ID == \"\"": "always true",
		"not go {":                            "",
	} {
		if got := tautologyVerdict(condition); got != want {
			t.Errorf("tautologyVerdict(%q) = %q, want %q", condition, got, want)
		}
	}
}

func TestEmitTautologicalConditionDiagnosticsReportsStepPosition(t *testing.T) {
	var got []normalizer.Warning
	opts := PipelineOptions{WarningSink: func(w normalizer.Warning) { got = append(got, w) }}
	services := []normalizer.Service{{
		Name: "Company",
		Methods: []normalizer.Method{{
			Name: "CancelB2BQuoteRequest",
			Flow: []normalizer.FlowStep{{
				Action: "flow.If",
				Args: map[string]any{
					"condition": "req.Notify",
					"_then": []normalizer.FlowStep{{
						Action: "logic.Check", File: "cue/api/impl_b2b_offers.cue", Line: 1613, Column: 6,
						Args: map[string]any{"condition": "sent || !sent", "throw": "dispatch failed"},
					}},
				},
			}},
		}},
	}}
	emitTautologicalConditionDiagnostics(services, opts)
	if len(got) != 1 {
		t.Fatalf("warnings = %#v", got)
	}
	w := got[0]
	if w.Code != "TAUTOLOGICAL_CHECK" || w.File != "cue/api/impl_b2b_offers.cue" || w.Line != 1613 || !strings.Contains(w.Message, "always true") {
		t.Fatalf("warning = %#v", w)
	}
}

func TestCUELinesOutsideMultilineStrings(t *testing.T) {
	src := "package api\n\nA: {\n\tfunc: \"\"\"\n\t\tline one\n\t\tline two\n\t\tline three\n\t\t\"\"\"\n\tsql: #\"\"\"\n\t\tSELECT 1\n\t\t\"\"\"#\n\tname: \"x\"\n}\n"
	path := filepath.Join(t.TempDir(), "a.cue")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cueLines, total := cueLinesOutsideMultilineStrings(path, []byte(src))
	// 14 lines in total; 3 Go lines and 1 SQL line are strings.
	if total != 14 || cueLines != 10 {
		t.Fatalf("cueLines=%d total=%d, want 10 and 14", cueLines, total)
	}
}
