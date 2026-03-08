package compiler

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestEmitSharedArchDiagnostics_RequiresReason(t *testing.T) {
	t.Parallel()

	var got []normalizer.Warning
	opts := PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			got = append(got, w)
		},
	}

	entities := []normalizer.Entity{
		{
			Name:     "Company",
			Source:   "cue/domain/company.cue:10:1",
			Metadata: map[string]any{"shared_arch": true},
		},
	}
	services := []normalizer.Service{
		{
			Name: "Tender",
			Methods: []normalizer.Method{
				{
					Name: "GetCompany",
					Flow: []normalizer.FlowStep{
						{Action: "repo.Find", Args: map[string]any{"source": "Company", "input": "req.CompanyID", "output": "company", "error": "Not found"}},
					},
				},
			},
		},
	}

	emitSharedArchDiagnostics(entities, services, opts)

	hasMissingReason := false
	for _, w := range got {
		if w.Code == "SHARED_ARCH_REASON_REQUIRED" {
			hasMissingReason = true
			break
		}
	}
	if !hasMissingReason {
		t.Fatalf("expected SHARED_ARCH_REASON_REQUIRED, got: %#v", got)
	}
}

func TestEmitSharedArchDiagnostics_UnderusedWarns(t *testing.T) {
	t.Parallel()

	var got []normalizer.Warning
	opts := PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			got = append(got, w)
		},
	}

	entities := []normalizer.Entity{
		{
			Name:     "Company",
			Source:   "cue/domain/company.cue:10:1",
			Metadata: map[string]any{"shared_arch": true, "shared_arch_reason": "legacy migration"},
		},
	}
	services := []normalizer.Service{
		{
			Name: "Tender",
			Methods: []normalizer.Method{
				{
					Name: "GetCompany",
					Flow: []normalizer.FlowStep{
						{Action: "repo.Find", Args: map[string]any{"source": "Company", "input": "req.CompanyID", "output": "company", "error": "Not found"}},
					},
				},
			},
		},
	}

	emitSharedArchDiagnostics(entities, services, opts)

	hasUnderused := false
	for _, w := range got {
		if w.Code == "SHARED_ARCH_UNDERUSED" {
			hasUnderused = true
			break
		}
	}
	if !hasUnderused {
		t.Fatalf("expected SHARED_ARCH_UNDERUSED, got: %#v", got)
	}
}

func TestEmitSharedArchDiagnostics_MultiContextNoUnderused(t *testing.T) {
	t.Parallel()

	var got []normalizer.Warning
	opts := PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			got = append(got, w)
		},
	}

	entities := []normalizer.Entity{
		{
			Name:     "Company",
			Source:   "cue/domain/company.cue:10:1",
			Metadata: map[string]any{"shared_arch": true, "shared_arch_reason": "cross-context identity lookups"},
		},
	}
	services := []normalizer.Service{
		{
			Name: "Tender",
			Methods: []normalizer.Method{
				{
					Name: "GetCompany",
					Flow: []normalizer.FlowStep{
						{Action: "repo.Find", Args: map[string]any{"source": "Company", "input": "req.CompanyID", "output": "company", "error": "Not found"}},
					},
				},
			},
		},
		{
			Name: "Bids",
			Methods: []normalizer.Method{
				{
					Name: "CheckCompany",
					Flow: []normalizer.FlowStep{
						{Action: "repo.Get", Args: map[string]any{"source": "Company", "input": "req.CompanyID", "output": "company", "error": "Not found"}},
					},
				},
			},
		},
	}

	emitSharedArchDiagnostics(entities, services, opts)

	for _, w := range got {
		if w.Code == "SHARED_ARCH_UNDERUSED" {
			t.Fatalf("did not expect SHARED_ARCH_UNDERUSED for multi-context usage, got: %#v", got)
		}
	}
}
