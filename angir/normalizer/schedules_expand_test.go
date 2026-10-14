package normalizer

import (
	"reflect"
	"strings"
	"testing"
)

func scheduleTestServices() []Service {
	return []Service{
		{Name: "Report", Subscribes: map[string]string{}, Methods: []Method{
			{Name: "SendDigest", Input: Entity{Fields: []Field{{Name: "leadHours", Type: "int"}}}},
			{Name: "Purge"},
		}},
		{Name: "Audit", Subscribes: map[string]string{}, Methods: []Method{{Name: "Purge"}}},
	}
}

// Item 23 of docs/tech-debt-backlog.md: one declaration creates the trigger
// event and the subscription that used to be written by hand.
func TestExpandSchedulesCreatesEventAndSubscription(t *testing.T) {
	services, events, schedules, err := ExpandSchedules(scheduleTestServices(), nil, []ScheduleDef{
		{Name: "DailyDigest", Action: "SendDigest", At: "06:00", Payload: []SchedulePayloadField{{Name: "leadhours", Type: "int", Value: "24"}}},
		{Name: "AuditPurgeTick", Service: "audit", Action: "Purge", Every: "1h"},
		{Name: "MonthlyPurgeRequested", Service: "report", Action: "Purge", Cron: "0 3 1 * *"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := services[0].Subscribes; !reflect.DeepEqual(got, map[string]string{"DailyDigestRequested": "SendDigest", "MonthlyPurgeRequested": "Purge"}) {
		t.Fatalf("report subscribes = %v", got)
	}
	if got := services[1].Subscribes; !reflect.DeepEqual(got, map[string]string{"AuditPurgeTickRequested": "Purge"}) {
		t.Fatalf("audit subscribes = %v", got)
	}
	if !services[0].RequiresNats || !services[1].RequiresNats {
		t.Fatal("a subscribing service needs NATS")
	}
	if len(events) != 3 {
		t.Fatalf("events = %#v", events)
	}
	digest := events[0]
	if digest.Name != "DailyDigestRequested" || digest.Owner != "Report" || len(digest.Fields) != 1 || digest.Fields[0].Name != "leadHours" || digest.Fields[0].Type != "int" {
		t.Fatalf("digest event = %#v", digest)
	}
	want := []ScheduleDef{
		{Name: "DailyDigest", Service: "Report", Action: "SendDigest", Publish: "DailyDigestRequested", Cron: "0 6 * * *", Generated: true,
			Payload: []SchedulePayloadField{{Name: "leadhours", Type: "int", Value: "24"}}},
		{Name: "AuditPurgeTick", Service: "Audit", Action: "Purge", Every: "1h", Publish: "AuditPurgeTickRequested", Generated: true},
		{Name: "MonthlyPurgeRequested", Service: "Report", Action: "Purge", Cron: "0 3 1 * *", Publish: "MonthlyPurgeRequested", Generated: true},
	}
	if !reflect.DeepEqual(schedules, want) {
		t.Fatalf("schedules =\n%#v\nwant\n%#v", schedules, want)
	}
}

func TestExpandSchedulesLeavesThreePartFormAlone(t *testing.T) {
	in := []ScheduleDef{{Name: "Nightly", Service: "report", Action: "SendDigest", At: "02:00", Every: "24h", Publish: "NightlyRequested"}}
	events := []EventDef{{Name: "NightlyRequested", Owner: "report"}}
	services, gotEvents, schedules, err := ExpandSchedules(scheduleTestServices(), events, in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(schedules, in) || !reflect.DeepEqual(gotEvents, events) || len(services[0].Subscribes) != 0 {
		t.Fatalf("three-part schedule changed: %#v %#v %#v", schedules, gotEvents, services[0].Subscribes)
	}
}

func TestExpandSchedulesRejectsWhatWouldNeverRun(t *testing.T) {
	cases := map[string]struct {
		schedule ScheduleDef
		events   []EventDef
		want     string
	}{
		"unknown action":  {ScheduleDef{Name: "X", Action: "Gone", Every: "1h"}, nil, "no operation Gone"},
		"ambiguous":       {ScheduleDef{Name: "X", Action: "Purge", Every: "1h"}, nil, "set service:"},
		"wrong service":   {ScheduleDef{Name: "X", Service: "audit", Action: "SendDigest", Every: "1h"}, nil, "service audit has no operation SendDigest"},
		"no timing":       {ScheduleDef{Name: "X", Action: "SendDigest"}, nil, "needs at:"},
		"bad cron":        {ScheduleDef{Name: "X", Action: "SendDigest", Cron: "0 25 * * *"}, nil, "hour field"},
		"cron and every":  {ScheduleDef{Name: "X", Action: "SendDigest", Cron: "* * * * *", Every: "1h"}, nil, "excludes"},
		"bad every":       {ScheduleDef{Name: "X", Action: "SendDigest", Every: "1d"}, nil, "positive duration"},
		"bad at":          {ScheduleDef{Name: "X", Action: "SendDigest", At: "6am"}, nil, "time of day"},
		"event exists":    {ScheduleDef{Name: "X", Action: "SendDigest", Every: "1h"}, []EventDef{{Name: "XRequested", Source: "cue/events/x.cue:3"}}, "XRequested is already declared (cue/events/x.cue:3)"},
		"payload unknown": {ScheduleDef{Name: "X", Action: "SendDigest", Every: "1h", Payload: []SchedulePayloadField{{Name: "days", Type: "int", Value: "1"}}}, nil, "payload.days is not an input field of SendDigest"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.schedule.Source = "cue/api/schedules.cue:4"
			_, _, _, err := ExpandSchedules(scheduleTestServices(), tc.events, []ScheduleDef{tc.schedule})
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "cue/api/schedules.cue:4: schedule X") {
				t.Fatalf("err = %v, want %q with the position", err, tc.want)
			}
		})
	}
}

func TestValidateCron(t *testing.T) {
	for _, ok := range []string{"* * * * *", "0 6 * * 1-5", "*/15 * * * *", "0 0 1,15 * 0", "30 2 * 1-12/3 7", "5-55/10 8-18 * * *"} {
		if err := ValidateCron(ok); err != nil {
			t.Errorf("ValidateCron(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "* * * *", "60 * * * *", "* * 0 * *", "* * * 13 *", "* * * * 8", "*/0 * * * *", "5-1 * * * *", "a * * * *"} {
		if err := ValidateCron(bad); err == nil {
			t.Errorf("ValidateCron(%q) accepted", bad)
		}
	}
}
