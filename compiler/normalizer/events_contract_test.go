package normalizer

import (
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestExtractEvents_ParsesOwnerAndConsumers(t *testing.T) {
	ctx := cuecontext.New()
	val := ctx.CompileString(`
		#TenderReportReady: {
			owner: "tender"
			consumers: ["notifications", "analytics"]
			tenderId: string
		}
	`)

	n := New()
	events, err := n.ExtractEvents(val)
	if err != nil {
		t.Fatalf("ExtractEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Owner != "tender" {
		t.Fatalf("expected owner=tender, got %q", events[0].Owner)
	}
	if len(events[0].Consumers) != 2 {
		t.Fatalf("expected 2 consumers, got %d", len(events[0].Consumers))
	}
	if events[0].Consumers[0] != "notifications" || events[0].Consumers[1] != "analytics" {
		t.Fatalf("unexpected consumers: %#v", events[0].Consumers)
	}
}
