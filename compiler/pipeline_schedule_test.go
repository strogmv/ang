package compiler

import (
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestEmitScheduleDiagnosticsRejectsSilentNoOps(t *testing.T) {
	services := []normalizer.Service{{
		Name:       "tender",
		Methods:    []normalizer.Method{{Name: "RunJob"}},
		Subscribes: map[string]string{"RunJobRequested": "RunJob"},
	}}
	events := map[string]struct{}{"RunJobRequested": {}}
	schedules := []normalizer.ScheduleDef{
		{Name: "MissingTrigger", Service: "tender", Action: "RunJob"},
		{Name: "MissingAction", Service: "tender", Action: "Gone", Publish: "RunJobRequested"},
		{Name: "MissingEvent", Service: "tender", Action: "RunJob", Publish: "UnknownRequested"},
	}

	var got []normalizer.Warning
	emitScheduleDiagnostics(services, events, schedules, PipelineOptions{WarningSink: func(w normalizer.Warning) {
		got = append(got, w)
	}})

	want := map[string]bool{
		"SCHEDULE_NO_TRIGGER":      false,
		"SCHEDULE_ACTION_UNKNOWN":  false,
		"SCHEDULE_EVENT_UNDEFINED": false,
		"SCHEDULE_TRIGGER_UNBOUND": false,
	}
	for _, diag := range got {
		if _, ok := want[diag.Code]; ok {
			want[diag.Code] = true
			if diag.Severity != "error" {
				t.Fatalf("%s severity = %q, want error", diag.Code, diag.Severity)
			}
		}
	}
	for code, seen := range want {
		if !seen {
			t.Fatalf("missing diagnostic %s in %#v", code, got)
		}
	}
}

func TestEmitScheduleDiagnosticsAcceptsBoundTrigger(t *testing.T) {
	services := []normalizer.Service{{
		Name:       "tender",
		Methods:    []normalizer.Method{{Name: "RunJob"}},
		Subscribes: map[string]string{"RunJobRequested": "RunJob"},
	}}
	events := map[string]struct{}{"RunJobRequested": {}}
	schedules := []normalizer.ScheduleDef{{
		Name: "RunJobHourly", Service: "tender", Action: "RunJob", Publish: "RunJobRequested", Every: "1h",
	}}

	var got []normalizer.Warning
	emitScheduleDiagnostics(services, events, schedules, PipelineOptions{WarningSink: func(w normalizer.Warning) {
		got = append(got, w)
	}})
	if len(got) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", got)
	}
}
