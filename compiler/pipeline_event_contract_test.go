package compiler

import (
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestEventPayloadBreakingChanges(t *testing.T) {
	oldEvent := normalizer.EventDef{
		Name: "TenderReportReady",
		Fields: []normalizer.Field{
			{Name: "tenderId", Type: "string", IsOptional: false},
			{Name: "generatedAt", Type: "time.Time", IsOptional: true},
		},
	}
	newEvent := normalizer.EventDef{
		Name: "TenderReportReady",
		Fields: []normalizer.Field{
			{Name: "tenderId", Type: "int", IsOptional: false},          // type changed
			{Name: "generatedAt", Type: "time.Time", IsOptional: false}, // optional -> required
			{Name: "companyId", Type: "string", IsOptional: false},      // new required
		},
	}

	breaking := eventPayloadBreakingChanges(oldEvent, newEvent)
	if len(breaking) != 3 {
		t.Fatalf("expected 3 breaking changes, got %d: %#v", len(breaking), breaking)
	}
}
