package emitter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

// renderFlowDelay emits a context-aware timer pause.
func renderFlowDelay(st *flowRenderState, action flowir.FlowDelay, indent int, sfx string) string {
	duration := normalizeFlowExpr(action.Duration.Source)
	pad := strings.Repeat("\t", indent)
	if duration == "" {
		return renderInvalidFlowStepConfig(st, pad, "flow.Delay", "flow.Delay requires duration")
	}
	timerV := "_delayTimer" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := time.NewTimer(%s)\n", pad, timerV, duration))
	b.WriteString(fmt.Sprintf("%s\tdefer %s.Stop()\n", pad, timerV))
	b.WriteString(fmt.Sprintf("%s\tselect {\n", pad))
	b.WriteString(fmt.Sprintf("%s\tcase <-%s.C:\n", pad, timerV))
	b.WriteString(fmt.Sprintf("%s\tcase <-ctx.Done():\n", pad))
	b.WriteString(errReturn(st, pad+"\t\t", "ctx.Err()"))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

// renderFlowSchedule emits a blocking wait until an absolute time.Time arrives.
func renderFlowSchedule(st *flowRenderState, action flowir.FlowSchedule, indent int, sfx string) string {
	at := normalizeFlowExpr(action.At.Source)
	pad := strings.Repeat("\t", indent)
	if at == "" {
		return renderInvalidFlowStepConfig(st, pad, "flow.Schedule", "flow.Schedule requires at")
	}
	waitV := "_schedWait" + sfx
	timerV := "_schedTimer" + sfx
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))
	b.WriteString(fmt.Sprintf("%s\t%s := time.Until(%s)\n", pad, waitV, at))
	b.WriteString(fmt.Sprintf("%s\tif %s > 0 {\n", pad, waitV))
	b.WriteString(fmt.Sprintf("%s\t\t%s := time.NewTimer(%s)\n", pad, timerV, waitV))
	b.WriteString(fmt.Sprintf("%s\t\tdefer %s.Stop()\n", pad, timerV))
	b.WriteString(fmt.Sprintf("%s\t\tselect {\n", pad))
	b.WriteString(fmt.Sprintf("%s\t\tcase <-%s.C:\n", pad, timerV))
	b.WriteString(fmt.Sprintf("%s\t\tcase <-ctx.Done():\n", pad))
	b.WriteString(errReturn(st, pad+"\t\t\t", "ctx.Err()"))
	b.WriteString(fmt.Sprintf("%s\t\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}

// cronWindowSpec holds the parsed result of a window string.
type cronWindowSpec struct {
	weekdayMode    string // "range" | "list" | "single" | "any"
	wdStart        time.Weekday
	wdEnd          time.Weekday
	wdList         []time.Weekday
	hourMode       string // "range" | "any"
	hStart, mStart int
	hEnd, mEnd     int // exclusive end
}

var weekdayNames = map[string]time.Weekday{
	"Sun": time.Sunday,
	"Mon": time.Monday,
	"Tue": time.Tuesday,
	"Wed": time.Wednesday,
	"Thu": time.Thursday,
	"Fri": time.Friday,
	"Sat": time.Saturday,
}

var weekdayGoNames = map[time.Weekday]string{
	time.Sunday:    "time.Sunday",
	time.Monday:    "time.Monday",
	time.Tuesday:   "time.Tuesday",
	time.Wednesday: "time.Wednesday",
	time.Thursday:  "time.Thursday",
	time.Friday:    "time.Friday",
	time.Saturday:  "time.Saturday",
}

func parseTimeHHMM(s string) (h, m int, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time %q: expected HH:MM", s)
	}
	h, err = strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err = strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid minute in %q", s)
	}
	return h, m, nil
}

func parseCronWindow(window string) (cronWindowSpec, error) {
	var spec cronWindowSpec
	spec.weekdayMode = "any"
	spec.hourMode = "any"

	window = strings.TrimSpace(window)
	if window == "" {
		return spec, fmt.Errorf("empty window")
	}

	// Split on first space: optional weekday part + optional time part.
	// Tokens that contain ":" are time tokens; others are weekday tokens.
	parts := strings.Fields(window)
	var wdPart, timePart string
	for _, p := range parts {
		if strings.Contains(p, ":") {
			timePart = p
		} else {
			if wdPart != "" {
				wdPart += " " + p
			} else {
				wdPart = p
			}
		}
	}

	// Parse weekday part.
	if wdPart != "" {
		if strings.Contains(wdPart, "-") {
			// Range: "Mon-Fri"
			lr := strings.SplitN(wdPart, "-", 2)
			wd0, ok0 := weekdayNames[strings.TrimSpace(lr[0])]
			wd1, ok1 := weekdayNames[strings.TrimSpace(lr[1])]
			if !ok0 || !ok1 {
				return spec, fmt.Errorf("unknown weekday in range %q", wdPart)
			}
			spec.weekdayMode = "range"
			spec.wdStart = wd0
			spec.wdEnd = wd1
		} else if strings.Contains(wdPart, ",") {
			// List: "Mon,Wed,Fri"
			spec.weekdayMode = "list"
			for _, token := range strings.Split(wdPart, ",") {
				wd, ok := weekdayNames[strings.TrimSpace(token)]
				if !ok {
					return spec, fmt.Errorf("unknown weekday %q", token)
				}
				spec.wdList = append(spec.wdList, wd)
			}
		} else {
			// Single day: "Mon"
			wd, ok := weekdayNames[strings.TrimSpace(wdPart)]
			if !ok {
				return spec, fmt.Errorf("unknown weekday %q", wdPart)
			}
			spec.weekdayMode = "single"
			spec.wdStart = wd
		}
	}

	// Parse time part: "09:00-17:00"
	if timePart != "" {
		lr := strings.SplitN(timePart, "-", 2)
		if len(lr) != 2 {
			return spec, fmt.Errorf("invalid time range %q: expected HH:MM-HH:MM", timePart)
		}
		h0, m0, err := parseTimeHHMM(strings.TrimSpace(lr[0]))
		if err != nil {
			return spec, err
		}
		h1, m1, err := parseTimeHHMM(strings.TrimSpace(lr[1]))
		if err != nil {
			return spec, err
		}
		spec.hourMode = "range"
		spec.hStart, spec.mStart = h0, m0
		spec.hEnd, spec.mEnd = h1, m1
	}

	return spec, nil
}

func renderFlowCron(st *flowRenderState, action flowir.FlowCron, indent int, sfx string) string {
	window := action.Window
	if window == "" {
		return renderInvalidFlowStepConfig(st, strings.Repeat("\t", indent), "flow.Cron", "flow.Cron requires window")
	}

	spec, err := parseCronWindow(window)
	if err != nil {
		return renderInvalidFlowStepConfig(st, strings.Repeat("\t", indent), "flow.Cron", fmt.Sprintf("flow.Cron invalid window %q: %v", window, err))
	}

	tz := action.Timezone

	pad := strings.Repeat("\t", indent)
	nowV := "_cronNow" + sfx
	matchV := "_cronMatch" + sfx

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s{\n", pad))

	// Emit time.Now() call with timezone.
	if tz == "UTC" {
		b.WriteString(fmt.Sprintf("%s\t%s := time.Now().UTC()\n", pad, nowV))
	} else {
		locV := "_cronLoc" + sfx
		locErrV := "_cronLocErr" + sfx
		b.WriteString(fmt.Sprintf("%s\t%s, %s := time.LoadLocation(%q)\n", pad, locV, locErrV, tz))
		b.WriteString(fmt.Sprintf("%s\tif %s != nil { %s = time.UTC }\n", pad, locErrV, locV))
		b.WriteString(fmt.Sprintf("%s\t%s := time.Now().In(%s)\n", pad, nowV, locV))
	}

	// Build match condition.
	var conditions []string

	switch spec.weekdayMode {
	case "range":
		conditions = append(conditions,
			fmt.Sprintf("(%s.Weekday() >= %s && %s.Weekday() <= %s)",
				nowV, weekdayGoNames[spec.wdStart],
				nowV, weekdayGoNames[spec.wdEnd]))
	case "list":
		parts := make([]string, 0, len(spec.wdList))
		for _, wd := range spec.wdList {
			parts = append(parts, fmt.Sprintf("%s.Weekday() == %s", nowV, weekdayGoNames[wd]))
		}
		conditions = append(conditions, "("+strings.Join(parts, " || ")+")")
	case "single":
		conditions = append(conditions,
			fmt.Sprintf("%s.Weekday() == %s", nowV, weekdayGoNames[spec.wdStart]))
	}

	if spec.hourMode == "range" {
		minutesExpr := fmt.Sprintf("%s.Hour()*60+%s.Minute()", nowV, nowV)
		conditions = append(conditions,
			fmt.Sprintf("(%s >= %d*60+%d && %s < %d*60+%d)",
				minutesExpr, spec.hStart, spec.mStart,
				minutesExpr, spec.hEnd, spec.mEnd))
	}

	matchExpr := "true"
	if len(conditions) > 0 {
		matchExpr = strings.Join(conditions, " &&\n\t\t")
	}
	b.WriteString(fmt.Sprintf("%s\t%s := %s\n", pad, matchV, matchExpr))

	// Emit the mismatch gate.
	b.WriteString(fmt.Sprintf("%s\tif !%s {\n", pad, matchV))
	var onMismatch []normalizer.FlowStep
	if flowNestedStepCount(st, "_onMismatch", onMismatch) > 0 {
		inner := renderFlowNestedSteps(cloneFlowState(st), "_onMismatch", onMismatch, indent+2)
		b.WriteString(inner)
	} else {
		b.WriteString(errReturn(st, pad+"\t\t",
			`errors.New(http.StatusForbidden, "OUTSIDE_SCHEDULE", "outside allowed schedule window")`))
	}
	b.WriteString(fmt.Sprintf("%s\t}\n", pad))
	b.WriteString(fmt.Sprintf("%s}\n", pad))
	return b.String()
}
