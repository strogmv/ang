package compiler

import (
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestEmitReadModelDiagnostics_RequiredFields(t *testing.T) {
	t.Parallel()

	var got []normalizer.Warning
	opts := PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			got = append(got, w)
		},
	}

	entities := []normalizer.Entity{
		{
			Name:      "SupplierRatingView",
			Owner:     "tender",
			Source:    "cue/projections/supplier_rating_view.cue:1:1",
			ReadModel: &normalizer.ReadModelDef{},
		},
	}
	emitReadModelDiagnostics(entities, nil, opts)

	hasSource := false
	hasRefresh := false
	for _, w := range got {
		if w.Code == "READ_MODEL_SOURCE_CONTEXT_MISSING" {
			hasSource = true
		}
		if w.Code == "READ_MODEL_REFRESH_ON_MISSING" {
			hasRefresh = true
		}
	}
	if !hasSource || !hasRefresh {
		t.Fatalf("expected both missing-source and missing-refresh diagnostics, got: %#v", got)
	}
}

func TestEmitReadModelDiagnostics_UnknownRefreshEvent(t *testing.T) {
	t.Parallel()

	var got []normalizer.Warning
	opts := PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			got = append(got, w)
		},
	}

	entities := []normalizer.Entity{
		{
			Name:           "SupplierRatingView",
			Owner:          "tender",
			BoundedContext: "tender",
			Source:         "cue/projections/supplier_rating_view.cue:1:1",
			ReadModel: &normalizer.ReadModelDef{
				SourceContext: "company",
				RefreshOn:     []string{"CompanyReviewCreated", "UnknownEvent"},
			},
		},
	}
	events := []normalizer.EventDef{{Name: "CompanyReviewCreated"}}
	emitReadModelDiagnostics(entities, events, opts)

	hasUnknown := false
	for _, w := range got {
		if w.Code == "READ_MODEL_REFRESH_EVENT_UNKNOWN" {
			hasUnknown = true
			break
		}
	}
	if !hasUnknown {
		t.Fatalf("expected unknown refresh event diagnostic, got: %#v", got)
	}
}
