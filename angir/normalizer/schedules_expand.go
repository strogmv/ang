package normalizer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ExpandSchedules turns each one-declaration schedule (no publish:) into the
// three parts the runtime needs: a trigger event <Name>Requested owned by the
// target service, the service's subscription of that event to the action, and
// the schedule publishing it. Schedules with publish: are the three-part form
// and are returned unchanged, so projects that declare the event and the
// subscription themselves generate exactly what they generated before.
//
//	Schedules: DailyDigest: {action: "SendDailyDigest", at: "06:00"}
//
// service: is optional (the service that has the action), at: alone means every
// day at that time, every: is an interval (at: + every: keeps its old meaning:
// first run at, then every), cron: is a five-field expression. payload: keys
// become fields of the event and must be input fields of the action.
func ExpandSchedules(services []Service, events []EventDef, schedules []ScheduleDef) ([]Service, []EventDef, []ScheduleDef, error) {
	if len(schedules) == 0 {
		return services, events, schedules, nil
	}
	eventByName := make(map[string]EventDef, len(events))
	for _, e := range events {
		eventByName[e.Name] = e
	}
	var errs []string
	fail := func(s ScheduleDef, format string, args ...any) {
		prefix := "schedule " + s.Name
		if s.Source != "" {
			prefix = s.Source + ": " + prefix
		}
		errs = append(errs, prefix+": "+fmt.Sprintf(format, args...))
	}

	out := make([]ScheduleDef, 0, len(schedules))
	for _, s := range schedules {
		if err := validateScheduleTiming(s); err != nil {
			fail(s, "%v", err)
			out = append(out, s)
			continue
		}
		if strings.TrimSpace(s.Publish) != "" {
			out = append(out, s)
			continue
		}
		action := strings.TrimSpace(s.Action)
		if action == "" {
			fail(s, "action is required (the operation the schedule runs)")
			out = append(out, s)
			continue
		}
		svcIdx, method, err := findScheduleAction(services, s.Service, action)
		if err != nil {
			fail(s, "%v", err)
			out = append(out, s)
			continue
		}
		svc := &services[svcIdx]

		eventName := s.Name
		if !strings.HasSuffix(eventName, "Requested") {
			eventName += "Requested"
		}
		if existing, ok := eventByName[eventName]; ok {
			where := ""
			if existing.Source != "" {
				where = " (" + existing.Source + ")"
			}
			fail(s, "event %s is already declared%s; rename the schedule, or name that event in publish: and subscribe the action to it", eventName, where)
			out = append(out, s)
			continue
		}
		if handler, ok := svc.Subscribes[eventName]; ok && handler != action {
			fail(s, "service %s already subscribes %s to %s", svc.Name, eventName, handler)
			out = append(out, s)
			continue
		}

		inputByName := make(map[string]Field, len(method.Input.Fields))
		for _, f := range method.Input.Fields {
			inputByName[strings.ToLower(f.Name)] = f
		}
		fields := make([]Field, 0, len(s.Payload))
		payloadOK := true
		for _, p := range s.Payload {
			in, ok := inputByName[strings.ToLower(p.Name)]
			if !ok {
				fail(s, "payload.%s is not an input field of %s; the job would never see it", p.Name, action)
				payloadOK = false
				continue
			}
			fields = append(fields, Field{Name: in.Name, Type: p.Type, Source: s.Source})
		}
		if !payloadOK {
			out = append(out, s)
			continue
		}

		ev := EventDef{
			Name:     eventName,
			Owner:    svc.Name,
			Fields:   fields,
			Source:   s.Source,
			Metadata: map[string]any{"schedule": s.Name},
		}
		events = append(events, ev)
		eventByName[eventName] = ev
		if svc.Subscribes == nil {
			svc.Subscribes = map[string]string{}
		}
		svc.Subscribes[eventName] = action
		svc.RequiresNats = true

		s.Service = svc.Name
		s.Action = action
		s.Publish = eventName
		s.Generated = true
		// at: alone is a time of day: every day at that time, in the
		// scheduler's timezone (the three-part form fires it once).
		if s.Cron == "" && s.Every == "" && s.At != "" {
			if spec, ok := dailyCron(s.At); ok {
				s.Cron = spec
				s.At = ""
			}
		}
		out = append(out, s)
	}
	if len(errs) > 0 {
		return services, events, out, fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return services, events, out, nil
}

func findScheduleAction(services []Service, serviceName, action string) (int, Method, error) {
	want := strings.ToLower(normalizeServiceName(serviceName))
	var found []int
	var method Method
	for i, svc := range services {
		if want != "" && strings.ToLower(svc.Name) != want {
			continue
		}
		for _, m := range svc.Methods {
			if m.Name == action {
				found = append(found, i)
				method = m
				break
			}
		}
	}
	switch {
	case len(found) == 1:
		return found[0], method, nil
	case len(found) > 1:
		names := make([]string, 0, len(found))
		for _, i := range found {
			names = append(names, services[i].Name)
		}
		return 0, Method{}, fmt.Errorf("action %s exists in services %s; set service:", action, strings.Join(names, ", "))
	case want != "":
		return 0, Method{}, fmt.Errorf("service %s has no operation %s", serviceName, action)
	default:
		return 0, Method{}, fmt.Errorf("no operation %s", action)
	}
}

var clockRe = regexp.MustCompile(`^([01]?[0-9]|2[0-3]):([0-5][0-9])$`)

// dailyCron turns "06:00" into "0 6 * * *".
func dailyCron(at string) (string, bool) {
	m := clockRe.FindStringSubmatch(strings.TrimSpace(at))
	if m == nil {
		return "", false
	}
	h, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	return fmt.Sprintf("%d %d * * *", min, h), true
}

// validateScheduleTiming checks the timing keys of one schedule. cron: excludes
// at: and every:; a one-declaration schedule needs one of them.
func validateScheduleTiming(s ScheduleDef) error {
	if s.Cron != "" {
		if s.At != "" || s.Every != "" {
			return fmt.Errorf("cron: excludes at: and every:")
		}
		if err := ValidateCron(s.Cron); err != nil {
			return fmt.Errorf("cron %q: %v", s.Cron, err)
		}
		return nil
	}
	if s.Every != "" {
		d, err := time.ParseDuration(s.Every)
		if err != nil || d <= 0 {
			return fmt.Errorf("every %q is not a positive duration such as \"15m\" or \"24h\"", s.Every)
		}
	}
	if s.At != "" {
		if _, ok := dailyCron(s.At); !ok {
			if _, err := time.Parse(time.RFC3339, s.At); err != nil {
				return fmt.Errorf("at %q is neither a time of day (\"06:00\") nor an RFC 3339 time", s.At)
			}
		}
	}
	if strings.TrimSpace(s.Publish) == "" && s.At == "" && s.Every == "" {
		return fmt.Errorf("needs at: (\"06:00\", every day), every: (\"15m\") or cron: (\"0 6 * * 1-5\")")
	}
	return nil
}

// cronFieldBounds are the ranges of minute, hour, day of month, month and day
// of week (0 and 7 are Sunday).
var cronFieldBounds = [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}

var cronFieldNames = [5]string{"minute", "hour", "day of month", "month", "day of week"}

// ValidateCron checks a five-field cron expression: each field is *, a number,
// a range a-b, a step */n or a-b/n, or a comma-separated list of those. The
// generated scheduler (internal/scheduler/cron.go) accepts the same syntax.
func ValidateCron(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return fmt.Errorf("want 5 fields (minute hour day-of-month month day-of-week), got %d", len(fields))
	}
	for i, field := range fields {
		lo, hi := cronFieldBounds[i][0], cronFieldBounds[i][1]
		for _, part := range strings.Split(field, ",") {
			if err := validateCronPart(part, lo, hi); err != nil {
				return fmt.Errorf("%s field %q: %v", cronFieldNames[i], field, err)
			}
		}
	}
	return nil
}

func validateCronPart(part string, lo, hi int) error {
	rng, step, hasStep := strings.Cut(part, "/")
	if hasStep {
		n, err := strconv.Atoi(step)
		if err != nil || n <= 0 {
			return fmt.Errorf("step %q is not a positive number", step)
		}
	}
	if rng == "*" {
		return nil
	}
	from, to, isRange := strings.Cut(rng, "-")
	a, err := strconv.Atoi(from)
	if err != nil {
		return fmt.Errorf("%q is not a number", from)
	}
	b := a
	if isRange {
		if b, err = strconv.Atoi(to); err != nil {
			return fmt.Errorf("%q is not a number", to)
		}
	} else if hasStep {
		b = hi
	}
	if a < lo || b > hi || a > b {
		return fmt.Errorf("%q is outside %d-%d", rng, lo, hi)
	}
	return nil
}
