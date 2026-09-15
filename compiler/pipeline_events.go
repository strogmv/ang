package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/angir/parser"
)

// emitEventUsageDiagnostics surfaces dead/unused events as warnings (non-fatal).
//   - dead event: defined but never published or consumed
//   - orphan publish: published but nothing consumes it
//   - missing publisher: subscribed but no publisher exists
//   - undeclared publish: a flow step publishes an event its operation does not
//     list in publishes:, so CUE contradicts itself
//
// An event is published when an operation declares it in publishes:, a
// schedule publishes it, or a flow step (event.Publish, event.Outbox,
// event.Broadcast) emits it. It is consumed by a subscriber, by websocket
// clients when a websocket endpoint lists it in messages:, by an
// event.Broadcast step, or when annotations mark it BroadcastOnly. Every
// warning carries a position: the event definition, the first publishing step
// or the subscribing operation.
func emitEventUsageDiagnostics(services []normalizer.Service, events []normalizer.EventDef, schedules []normalizer.ScheduleDef, endpoints []normalizer.Endpoint, broadcastOnly map[string]struct{}, planned map[string]struct{}, opts PipelineOptions) {
	defined := make(map[string]normalizer.EventDef)
	for _, e := range events {
		defined[e.Name] = e
	}
	definedNames := make(map[string]struct{}, len(defined))
	for name := range defined {
		definedNames[name] = struct{}{}
	}
	emitScheduleDiagnostics(services, definedNames, schedules, opts)

	type position struct {
		file         string
		line, column int
	}
	published := map[string]position{}
	consumed := map[string]struct{}{}
	subscribers := map[string]position{}
	markPublished := func(name string, at position) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := published[name]; !ok || published[name].file == "" {
			published[name] = at
		}
	}
	methodPosition := func(m normalizer.Method) position {
		file, line := parseSourcePos(m.Source)
		return position{file: file, line: line}
	}

	for _, s := range services {
		methodsByName := make(map[string]normalizer.Method, len(s.Methods))
		for _, m := range s.Methods {
			methodsByName[m.Name] = m
		}
		for _, evt := range s.Publishes {
			markPublished(evt, position{})
		}
		for evt, handler := range s.Subscribes {
			consumed[evt] = struct{}{}
			if _, ok := subscribers[evt]; !ok {
				subscribers[evt] = methodPosition(methodsByName[handler])
			}
		}
		for _, m := range s.Methods {
			declared := make(map[string]struct{}, len(m.Publishes))
			for _, evt := range m.Publishes {
				declared[evt] = struct{}{}
				markPublished(evt, methodPosition(m))
			}
			walkFlowSteps(m.Flow, func(step normalizer.FlowStep) {
				name, _ := step.Args["name"].(string)
				name = strings.TrimSpace(name)
				if name == "" {
					return
				}
				at := position{file: step.File, line: step.Line, column: step.Column}
				switch step.Action {
				case "event.Broadcast":
					markPublished(name, at)
					consumed[name] = struct{}{}
				case "event.Publish", "event.Outbox":
					markPublished(name, at)
					if _, ok := declared[name]; !ok {
						recordPipelineDiagnostic(normalizer.Warning{
							Kind:     "undeclared-publish",
							Code:     "EVENT_PUBLISH_UNDECLARED",
							Severity: "warn",
							Message:  fmt.Sprintf("%s.%s publishes %s but its operation does not list it in publishes:", s.Name, m.Name, name),
							Hint:     fmt.Sprintf(`Add "%s" to publishes: of operation %s.`, name, m.Name),
							File:     step.File,
							Line:     step.Line,
							Column:   step.Column,
							CUEPath:  step.CUEPath,
						}, opts)
					}
				}
			})
		}
	}
	for _, sch := range schedules {
		markPublished(sch.Publish, position{})
	}
	for _, ep := range endpoints {
		for _, msg := range ep.Messages {
			consumed[strings.TrimSpace(msg)] = struct{}{}
		}
	}
	for name := range broadcastOnly {
		consumed[name] = struct{}{}
	}
	definitionPosition := func(name string) position {
		file, line := parseSourcePos(defined[name].Source)
		return position{file: file, line: line}
	}
	firstKnown := func(candidates ...position) position {
		for _, p := range candidates {
			if p.file != "" {
				return p
			}
		}
		return position{}
	}

	for _, name := range sortedNames(definedNames) {
		_, isPublished := published[name]
		_, isConsumed := consumed[name]
		_, isPlanned := planned[name]
		if isPublished || isConsumed || isPlanned {
			continue
		}
		at := definitionPosition(name)
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "dead-event",
			Code:     "DEAD_EVENT",
			Severity: "warn",
			Message:  fmt.Sprintf("Event %s is defined but never published or consumed", name),
			File:     at.file,
			Line:     at.line,
		}, opts)
	}

	publishedNames := make(map[string]struct{}, len(published))
	for name := range published {
		publishedNames[name] = struct{}{}
	}
	for _, name := range sortedNames(publishedNames) {
		if _, ok := consumed[name]; ok {
			continue
		}
		at := firstKnown(published[name], definitionPosition(name))
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "orphan-publish",
			Code:     "ORPHAN_PUBLISH",
			Severity: "warn",
			Message:  fmt.Sprintf("Event %s is published but nothing consumes it (no subscriber, websocket message or broadcast)", name),
			File:     at.file,
			Line:     at.line,
			Column:   at.column,
		}, opts)
	}

	subscribedNames := make(map[string]struct{}, len(subscribers))
	for name := range subscribers {
		subscribedNames[name] = struct{}{}
	}
	for _, name := range sortedNames(subscribedNames) {
		if _, ok := published[name]; ok {
			continue
		}
		at := firstKnown(subscribers[name], definitionPosition(name))
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "missing-publisher",
			Code:     "MISSING_PUBLISH",
			Severity: "warn",
			Message:  fmt.Sprintf("Event %s is subscribed but never published", name),
			File:     at.file,
			Line:     at.line,
		}, opts)
	}
}

func sortedNames(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// emitScheduleDiagnostics rejects schedules that the generated runtime would
// otherwise accept but never execute. The scheduler is event-driven: action is
// descriptive metadata, while publish is the actual trigger delivered to a
// subscriber in the owning service.
func emitScheduleDiagnostics(services []normalizer.Service, definedEvents map[string]struct{}, schedules []normalizer.ScheduleDef, opts PipelineOptions) {
	serviceByName := make(map[string]normalizer.Service, len(services))
	for _, service := range services {
		serviceByName[strings.ToLower(strings.TrimSpace(service.Name))] = service
	}

	for _, schedule := range schedules {
		serviceName := strings.ToLower(strings.TrimSpace(schedule.Service))
		service, serviceExists := serviceByName[serviceName]
		if !serviceExists {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "schedule-contract",
				Code:     "SCHEDULE_SERVICE_UNKNOWN",
				Severity: "error",
				Message:  fmt.Sprintf("Schedule %s targets unknown service %s", schedule.Name, schedule.Service),
				Hint:     "Set service to an existing ANG service.",
			}, opts)
		}

		if serviceExists && strings.TrimSpace(schedule.Action) != "" {
			actionExists := false
			for _, method := range service.Methods {
				if method.Name == schedule.Action {
					actionExists = true
					break
				}
			}
			if !actionExists {
				recordPipelineDiagnostic(normalizer.Warning{
					Kind:     "schedule-contract",
					Code:     "SCHEDULE_ACTION_UNKNOWN",
					Severity: "error",
					Message:  fmt.Sprintf("Schedule %s targets unknown action %s.%s", schedule.Name, schedule.Service, schedule.Action),
					Hint:     "Use an operation exposed by the target service or remove the stale schedule.",
				}, opts)
			}
		}

		publish := strings.TrimSpace(schedule.Publish)
		if publish == "" {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "schedule-contract",
				Code:     "SCHEDULE_NO_TRIGGER",
				Severity: "error",
				Message:  fmt.Sprintf("Schedule %s has no publish event and would never execute", schedule.Name),
				Hint:     "Set publish to a scheduler trigger event and subscribe the target operation to it.",
			}, opts)
			continue
		}

		if _, ok := definedEvents[publish]; !ok {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "schedule-contract",
				Code:     "SCHEDULE_EVENT_UNDEFINED",
				Severity: "error",
				Message:  fmt.Sprintf("Schedule %s publishes undefined event %s", schedule.Name, publish),
				Hint:     "Declare the event in cue/events.",
			}, opts)
		}
		if serviceExists {
			if _, ok := service.Subscribes[publish]; !ok {
				recordPipelineDiagnostic(normalizer.Warning{
					Kind:     "schedule-contract",
					Code:     "SCHEDULE_TRIGGER_UNBOUND",
					Severity: "error",
					Message:  fmt.Sprintf("Schedule %s publishes %s but service %s does not subscribe to it", schedule.Name, publish, schedule.Service),
					Hint:     "Add the event to subscribes on the operation that executes this job.",
				}, opts)
			}
		}
	}
}

// loadEventAnnotations reads cue/events/annotations.cue (optional) and returns
// three sets: broadcastOnly, planned, and compatAllowBreaking.
func loadEventAnnotations(basePath string) (map[string]struct{}, map[string]struct{}, map[string]struct{}) {
	broadcastOnly := make(map[string]struct{})
	planned := make(map[string]struct{})
	compatAllowBreaking := make(map[string]struct{})

	path := filepath.Join(basePath, DefaultCueRoot, "events_meta", "annotations.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		return broadcastOnly, planned, compatAllowBreaking
	}

	// Very small ad-hoc parser for map-like sections; avoid pulling CUE parser here.
	var current string
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "//") || line == "" {
			continue
		}
		if strings.HasPrefix(line, "BroadcastOnly:") {
			current = "broadcast"
			continue
		}
		if strings.HasPrefix(line, "Planned:") {
			current = "planned"
			continue
		}
		if strings.HasPrefix(line, "CompatAllowBreaking:") {
			current = "compat_allow_breaking"
			continue
		}
		if strings.HasSuffix(line, "{") || strings.HasSuffix(line, "}") {
			continue
		}
		name := parseEventAnnotationName(line)
		if name == "" {
			continue
		}
		if current == "broadcast" {
			broadcastOnly[name] = struct{}{}
		} else if current == "planned" {
			planned[name] = struct{}{}
		} else if current == "compat_allow_breaking" {
			compatAllowBreaking[name] = struct{}{}
		}
	}
	return broadcastOnly, planned, compatAllowBreaking
}

func parseEventAnnotationName(line string) string {
	name := strings.TrimSpace(line)
	if i := strings.Index(name, ":"); i >= 0 {
		name = strings.TrimSpace(name[:i])
	}
	name = strings.Trim(name, "\"")
	return strings.TrimSpace(name)
}

func resolveEventServiceName(raw string, serviceSet map[string]struct{}) string {
	name := strings.TrimSpace(strings.ToLower(raw))
	if name == "" {
		return ""
	}
	if _, ok := serviceSet[name]; ok {
		return name
	}

	norm := strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(name)
	if _, ok := serviceSet[norm]; ok {
		return norm
	}

	candidates := make([]string, 0, 8)
	addCandidate := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		for _, c := range candidates {
			if c == v {
				return
			}
		}
		candidates = append(candidates, v)
	}

	addCandidate(norm)
	for _, suffix := range []string{
		"_ws_events",
		"_domain_events",
		"_nats_events",
		"_events",
		"_ws_event",
		"_event",
	} {
		if strings.HasSuffix(norm, suffix) {
			addCandidate(strings.TrimSuffix(norm, suffix))
		}
	}
	if strings.HasPrefix(norm, "events_") {
		addCandidate(strings.TrimPrefix(norm, "events_"))
	}
	if i := strings.Index(norm, "_"); i > 0 {
		addCandidate(norm[:i])
	}

	for _, c := range candidates {
		if _, ok := serviceSet[c]; ok {
			return c
		}
	}
	return ""
}

func emitEventContractDiagnostics(basePath string, services []normalizer.Service, events []normalizer.EventDef, compatAllowBreaking map[string]struct{}, opts PipelineOptions) {
	ciMode := strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") || strings.TrimSpace(os.Getenv("CI")) == "1"
	strictContract := strings.EqualFold(strings.TrimSpace(os.Getenv("ANG_EVENT_CONTRACT_STRICT")), "true") || strings.TrimSpace(os.Getenv("ANG_EVENT_CONTRACT_STRICT")) == "1"
	contractSeverity := "warn"
	if strictContract {
		contractSeverity = "error"
	}

	serviceSet := make(map[string]struct{}, len(services))
	subscribersByEvent := make(map[string]map[string]struct{})
	for _, s := range services {
		svc := strings.TrimSpace(strings.ToLower(s.Name))
		if svc != "" {
			serviceSet[svc] = struct{}{}
		}
		for evt := range s.Subscribes {
			evt = strings.TrimSpace(evt)
			if evt == "" {
				continue
			}
			if subscribersByEvent[evt] == nil {
				subscribersByEvent[evt] = make(map[string]struct{})
			}
			subscribersByEvent[evt][svc] = struct{}{}
		}
	}

	for _, e := range events {
		file, line := parseSourcePos(e.Source)
		ownerRaw := strings.TrimSpace(e.Owner)
		if ownerRaw == "" {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "event-contract",
				Code:     "EVENT_OWNER_MISSING",
				Severity: "warn",
				Message:  fmt.Sprintf("Event %s has no owner declared", e.Name),
				Hint:     "Set owner in cue/events, e.g. owner: \"service_name\"",
				File:     file,
				Line:     line,
			}, opts)
		} else if owner := resolveEventServiceName(ownerRaw, serviceSet); owner == "" {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "event-contract",
				Code:     "EVENT_OWNER_UNKNOWN_SERVICE",
				Severity: contractSeverity,
				Message:  fmt.Sprintf("Event %s owner '%s' does not match any service", e.Name, e.Owner),
				Hint:     "Use an existing service name in owner, e.g. \"service_name\" (or set ANG_EVENT_CONTRACT_STRICT=1 to fail CI)",
				File:     file,
				Line:     line,
			}, opts)
		}

		for _, consumer := range e.Consumers {
			consumerRaw := strings.TrimSpace(consumer)
			if consumerRaw == "" {
				continue
			}
			consumerSvc := resolveEventServiceName(consumerRaw, serviceSet)
			if consumerSvc == "" {
				recordPipelineDiagnostic(normalizer.Warning{
					Kind:     "event-contract",
					Code:     "EVENT_CONSUMER_UNKNOWN_SERVICE",
					Severity: contractSeverity,
					Message:  fmt.Sprintf("Event %s consumer '%s' does not match any service", e.Name, consumerRaw),
					Hint:     "Use an existing service name in consumers list (or set ANG_EVENT_CONTRACT_STRICT=1 to fail CI)",
					File:     file,
					Line:     line,
				}, opts)
				continue
			}
			if subs := subscribersByEvent[e.Name]; subs != nil {
				if _, ok := subs[consumerSvc]; ok {
					continue
				}
			}
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "event-contract",
				Code:     "EVENT_CONSUMER_NOT_SUBSCRIBED",
				Severity: contractSeverity,
				Message:  fmt.Sprintf("Event %s declares consumer '%s' but no subscribes contract exists", e.Name, consumerRaw),
				Hint:     fmt.Sprintf("Add subscribes: {\"%s\": \"HandlerName\"} in service %s", e.Name, consumerSvc),
				File:     file,
				Line:     line,
			}, opts)
		}
	}

	baseRef := strings.TrimSpace(os.Getenv("ANG_EVENT_COMPAT_BASE_REF"))
	if baseRef == "" && ciMode {
		baseRef = "origin/main"
	}
	if baseRef == "" {
		return
	}

	baseEvents, err := loadEventsFromGitRef(basePath, baseRef)
	if err != nil {
		sev := "warn"
		if ciMode {
			sev = "error"
		}
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "event-contract",
			Code:     "EVENT_COMPAT_BASE_LOAD_FAILED",
			Severity: sev,
			Message:  fmt.Sprintf("Cannot load baseline events from git ref %q: %v", baseRef, err),
			Hint:     "Set ANG_EVENT_COMPAT_BASE_REF to a valid ref (e.g. origin/main) in CI",
		}, opts)
		return
	}

	currentByName := make(map[string]normalizer.EventDef, len(events))
	for _, e := range events {
		currentByName[e.Name] = e
	}
	baseByName := make(map[string]normalizer.EventDef, len(baseEvents))
	for _, e := range baseEvents {
		baseByName[e.Name] = e
	}

	for eventName, oldEvent := range baseByName {
		newEvent, ok := currentByName[eventName]
		if !ok {
			if _, allowed := compatAllowBreaking[eventName]; !allowed {
				recordPipelineDiagnostic(normalizer.Warning{
					Kind:     "event-contract",
					Code:     "EVENT_PAYLOAD_BREAKING",
					Severity: "error",
					Message:  fmt.Sprintf("Breaking event change: %s removed compared to %s", eventName, baseRef),
					Hint:     fmt.Sprintf("Add %s to CompatAllowBreaking in cue/events_meta/annotations.cue when migration is ready", eventName),
				}, opts)
			}
			continue
		}

		breaking := eventPayloadBreakingChanges(oldEvent, newEvent)
		if len(breaking) == 0 {
			continue
		}
		if _, allowed := compatAllowBreaking[eventName]; allowed {
			continue
		}
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "event-contract",
			Code:     "EVENT_PAYLOAD_BREAKING",
			Severity: "error",
			Message:  fmt.Sprintf("Breaking payload change in event %s vs %s: %s", eventName, baseRef, strings.Join(breaking, "; ")),
			Hint:     fmt.Sprintf("Add %s to CompatAllowBreaking in cue/events_meta/annotations.cue when migration is ready", eventName),
		}, opts)
	}
}

func loadEventsFromGitRef(basePath, ref string) ([]normalizer.EventDef, error) {
	// Check if we are in a git repository
	checkCmd := exec.Command("git", "-C", basePath, "rev-parse", "--is-inside-work-tree")
	if err := checkCmd.Run(); err != nil {
		return nil, nil // Not a git repo, skip compatibility check
	}

	cmd := exec.Command("git", "-C", basePath, "ls-tree", "-r", "--name-only", ref, "--", DefaultCueRoot, "cue.mod")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git ls-tree failed: %s", strings.TrimSpace(string(out)))
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	if len(files) == 0 {
		return nil, nil
	}

	tmpDir, err := os.MkdirTemp("", "ang-events-baseline-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	for _, rel := range files {
		showCmd := exec.Command("git", "-C", basePath, "show", fmt.Sprintf("%s:%s", ref, rel))
		data, err := showCmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git show %s:%s failed: %w", ref, rel, err)
		}
		dst := filepath.Join(tmpDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return nil, err
		}
	}

	p := parser.New()
	valEvents, okEvents, err := LoadOptionalDomain(p, filepath.Join(tmpDir, DefaultCueRoot, "events"))
	if err != nil {
		return nil, err
	}
	valArch, okArch, err := LoadOptionalDomain(p, filepath.Join(tmpDir, DefaultCueRoot, "architecture"))
	if err != nil {
		return nil, err
	}

	n := normalizer.New()
	var events []normalizer.EventDef
	if okEvents {
		events, _ = n.ExtractEvents(valEvents)
	}
	if len(events) == 0 && okArch {
		archEvents, _ := n.ExtractEventsFromArch(valArch)
		events = append(events, archEvents...)
	}
	return events, nil
}

func eventPayloadBreakingChanges(oldEvent, newEvent normalizer.EventDef) []string {
	type fieldSig struct {
		Type     string
		Optional bool
	}
	oldFields := make(map[string]fieldSig, len(oldEvent.Fields))
	newFields := make(map[string]fieldSig, len(newEvent.Fields))
	for _, f := range oldEvent.Fields {
		oldFields[f.Name] = fieldSig{Type: strings.TrimSpace(f.Type), Optional: f.IsOptional}
	}
	for _, f := range newEvent.Fields {
		newFields[f.Name] = fieldSig{Type: strings.TrimSpace(f.Type), Optional: f.IsOptional}
	}

	var breaking []string
	for name, oldSig := range oldFields {
		newSig, ok := newFields[name]
		if !ok {
			breaking = append(breaking, fmt.Sprintf("removed field %s", name))
			continue
		}
		if oldSig.Type != newSig.Type {
			breaking = append(breaking, fmt.Sprintf("type changed for %s (%s -> %s)", name, oldSig.Type, newSig.Type))
		}
		if oldSig.Optional && !newSig.Optional {
			breaking = append(breaking, fmt.Sprintf("field %s became required", name))
		}
	}
	for name, newSig := range newFields {
		if _, ok := oldFields[name]; ok {
			continue
		}
		if !newSig.Optional {
			breaking = append(breaking, fmt.Sprintf("added required field %s", name))
		}
	}
	sort.Strings(breaking)
	return breaking
}
