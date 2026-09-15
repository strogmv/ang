package compiler

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func eventWarnings(t *testing.T, services []normalizer.Service, events []normalizer.EventDef, endpoints []normalizer.Endpoint, broadcastOnly map[string]struct{}) []normalizer.Warning {
	t.Helper()
	got, opts := collectWarnings()
	emitEventUsageDiagnostics(services, events, nil, endpoints, broadcastOnly, map[string]struct{}{}, opts)
	return *got
}

func warningFor(warnings []normalizer.Warning, code, event string) *normalizer.Warning {
	for i := range warnings {
		if warnings[i].Code == code && strings.Contains(warnings[i].Message, event) {
			return &warnings[i]
		}
	}
	return nil
}

func publishStep(event string, line int) normalizer.FlowStep {
	return normalizer.FlowStep{Action: "event.Publish", File: "cue/api/impl_quotes.cue", Line: line, Column: 5, Args: map[string]any{"name": event}}
}

// dealingi-back: B2BQuoteRequestCreated was published by an event.Publish step
// and had a subscriber, yet MISSING_PUBLISH said nobody published it, because
// only publishes: declarations were counted.
func TestEventUsage_PublishStepCountsAndUndeclaredIsReported(t *testing.T) {
	services := []normalizer.Service{
		{Name: "Company", Methods: []normalizer.Method{{Name: "CreateB2BQuoteRequest", Source: "cue/api/quotes.cue:12", Flow: []normalizer.FlowStep{publishStep("B2BQuoteRequestCreated", 412)}}}},
		{Name: "Notifications", Subscribes: map[string]string{"B2BQuoteRequestCreated": "OnCreated"}, Methods: []normalizer.Method{{Name: "OnCreated", Source: "cue/api/notifications.cue:131"}}},
	}
	events := []normalizer.EventDef{{Name: "B2BQuoteRequestCreated", Source: "cue/events/b2b.cue:7"}}
	warnings := eventWarnings(t, services, events, nil, nil)

	if w := warningFor(warnings, "MISSING_PUBLISH", "B2BQuoteRequestCreated"); w != nil {
		t.Fatalf("the event is published by a flow step: %#v", *w)
	}
	w := warningFor(warnings, "EVENT_PUBLISH_UNDECLARED", "B2BQuoteRequestCreated")
	if w == nil || w.File != "cue/api/impl_quotes.cue" || w.Line != 412 {
		t.Fatalf("undeclared publish must point at the step, got %#v", warnings)
	}

	services[0].Methods[0].Publishes = []string{"B2BQuoteRequestCreated"}
	if w := warningFor(eventWarnings(t, services, events, nil, nil), "EVENT_PUBLISH_UNDECLARED", "B2BQuoteRequestCreated"); w != nil {
		t.Fatalf("declared publish must not be reported: %#v", *w)
	}
}

// A progress event delivered to browsers through a websocket endpoint's
// messages: list has a consumer.
func TestEventUsage_WebsocketMessagesAreConsumers(t *testing.T) {
	services := []normalizer.Service{{Name: "Company", Methods: []normalizer.Method{{
		Name: "RunBulkJob", Publishes: []string{"BulkProgressChanged", "StockReserved"},
		Flow: []normalizer.FlowStep{publishStep("BulkProgressChanged", 20), publishStep("StockReserved", 30)},
	}}}}
	endpoints := []normalizer.Endpoint{{Path: "/ws", Messages: []string{"BulkProgressChanged"}}}
	warnings := eventWarnings(t, services, []normalizer.EventDef{{Name: "BulkProgressChanged"}, {Name: "StockReserved"}}, endpoints, nil)

	if w := warningFor(warnings, "ORPHAN_PUBLISH", "BulkProgressChanged"); w != nil {
		t.Fatalf("websocket clients consume the event: %#v", *w)
	}
	w := warningFor(warnings, "ORPHAN_PUBLISH", "StockReserved")
	if w == nil || w.File != "cue/api/impl_quotes.cue" || w.Line != 30 {
		t.Fatalf("orphan publish must point at the publishing step, got %#v", warnings)
	}
}

func TestEventUsage_WarningsCarryPositions(t *testing.T) {
	services := []normalizer.Service{{
		Name:       "Search",
		Subscribes: map[string]string{"NeverPublished": "OnNever"},
		Methods:    []normalizer.Method{{Name: "OnNever", Source: "cue/api/search.cue:40"}},
	}}
	events := []normalizer.EventDef{
		{Name: "NeverPublished", Source: "cue/events/search.cue:3"},
		{Name: "Unused", Source: "cue/events/search.cue:9"},
	}
	warnings := eventWarnings(t, services, events, nil, nil)

	if w := warningFor(warnings, "MISSING_PUBLISH", "NeverPublished"); w == nil || w.File != "cue/api/search.cue" || w.Line != 40 {
		t.Fatalf("missing publisher must point at the subscriber, got %#v", warnings)
	}
	if w := warningFor(warnings, "DEAD_EVENT", "Unused"); w == nil || w.File != "cue/events/search.cue" || w.Line != 9 {
		t.Fatalf("dead event must point at its definition, got %#v", warnings)
	}
}
