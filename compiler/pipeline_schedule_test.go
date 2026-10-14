package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
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

// Item 23 of docs/tech-debt-backlog.md: Schedules.X: {action, at} alone used
// to fail with SCHEDULE_NO_TRIGGER; the trigger event and the subscription
// had to be declared by hand in cue/events and subscribes:.
func TestSemanticPhasesExpandOneDeclarationSchedule(t *testing.T) {
	base := copyMinimalProject(t)
	writeFixtureFile(t, filepath.Join(base, "cue", "api", "schedules.cue"), `package api

Schedules: {
	UsersMorning: {action: "ListUsers", at: "06:00", payload: {limit: 5}}
	UsersMonthly: {service: "user", action: "ListUsers", cron: "0 3 1 * *"}
}
`)
	var diags []normalizer.Warning
	out, err := RunSemanticPhasesWithOptions(base, PipelineOptions{WarningSink: func(w normalizer.Warning) {
		diags = append(diags, w)
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diags {
		if strings.HasPrefix(d.Code, "SCHEDULE_") || d.Code == "EVENT_UNUSED" {
			t.Errorf("unexpected diagnostic %s: %s", d.Code, d.Message)
		}
	}
	events := map[string]normalizer.EventDef{}
	for _, e := range out.Events {
		events[e.Name] = e
	}
	if e, ok := events["UsersMorningRequested"]; !ok || len(e.Fields) != 1 || e.Fields[0].Name != "limit" {
		t.Fatalf("UsersMorningRequested = %#v (events %v)", e, out.Events)
	}
	if _, ok := events["UsersMonthlyRequested"]; !ok {
		t.Fatalf("UsersMonthlyRequested missing: %v", out.Events)
	}
	var user normalizer.Service
	for _, s := range out.Services {
		if s.Name == "User" {
			user = s
		}
	}
	if user.Subscribes["UsersMorningRequested"] != "ListUsers" || user.Subscribes["UsersMonthlyRequested"] != "ListUsers" {
		t.Fatalf("User subscribes = %v", user.Subscribes)
	}
	crons := map[string]string{}
	for _, s := range out.Schedules {
		crons[s.Name] = s.Cron
	}
	if crons["UsersMorning"] != "0 6 * * *" || crons["UsersMonthly"] != "0 3 1 * *" {
		t.Fatalf("schedule crons = %v", crons)
	}
}

func TestSemanticPhasesRejectScheduleOfUnknownAction(t *testing.T) {
	base := copyMinimalProject(t)
	writeFixtureFile(t, filepath.Join(base, "cue", "api", "schedules.cue"), `package api

Schedules: Nightly: {action: "Gone", at: "06:00"}
`)
	_, err := RunSemanticPhasesWithOptions(base, PipelineOptions{})
	var ce *ContractError
	if err == nil || !asContractError(err, &ce) || ce.Code != ErrCodeCUEScheduleNormalize || !strings.Contains(err.Error(), "schedules.cue:3") || !strings.Contains(err.Error(), "no operation Gone") {
		t.Fatalf("err = %v", err)
	}
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
