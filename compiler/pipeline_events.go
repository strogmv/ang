package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang-ir/parser"
)

// emitEventUsageDiagnostics surfaces dead/unused events as warnings (non-fatal).
// - dead event: defined but never published or subscribed
// - orphan publish: published but nobody subscribes
// - missing publisher: subscribed but no publisher exists
func emitEventUsageDiagnostics(services []normalizer.Service, events []normalizer.EventDef, schedules []normalizer.ScheduleDef, broadcastOnly map[string]struct{}, planned map[string]struct{}, opts PipelineOptions) {
	defined := make(map[string]struct{})
	for _, e := range events {
		defined[e.Name] = struct{}{}
	}

	published := make(map[string]struct{})
	subscribed := make(map[string]struct{})

	for _, s := range services {
		for _, evt := range s.Publishes {
			published[evt] = struct{}{}
		}
		for evt := range s.Subscribes {
			subscribed[evt] = struct{}{}
		}
		for _, m := range s.Methods {
			for _, evt := range m.Publishes {
				published[evt] = struct{}{}
			}
		}
	}
	for _, sch := range schedules {
		if sch.Publish != "" {
			published[sch.Publish] = struct{}{}
		}
	}

	for name := range defined {
		if _, okPub := published[name]; okPub {
			continue
		}
		if _, okSub := subscribed[name]; okSub {
			continue
		}
		if _, ok := planned[name]; ok {
			continue
		}
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "dead-event",
			Code:     "DEAD_EVENT",
			Severity: "warn",
			Message:  fmt.Sprintf("Event %s is defined but never published or subscribed", name),
		}, opts)
	}

	for name := range published {
		if _, ok := subscribed[name]; ok {
			continue
		}
		if _, ok := broadcastOnly[name]; ok {
			continue
		}
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "orphan-publish",
			Code:     "ORPHAN_PUBLISH",
			Severity: "warn",
			Message:  fmt.Sprintf("Event %s is published but has no subscribers", name),
		}, opts)
	}

	for name := range subscribed {
		if _, ok := published[name]; ok {
			continue
		}
		recordPipelineDiagnostic(normalizer.Warning{
			Kind:     "missing-publisher",
			Code:     "MISSING_PUBLISH",
			Severity: "warn",
			Message:  fmt.Sprintf("Event %s is subscribed but never published", name),
		}, opts)
	}
}

// loadEventAnnotations reads cue/events/annotations.cue (optional) and returns
// three sets: broadcastOnly, planned, and compatAllowBreaking.
func loadEventAnnotations(basePath string) (map[string]struct{}, map[string]struct{}, map[string]struct{}) {
	broadcastOnly := make(map[string]struct{})
	planned := make(map[string]struct{})
	compatAllowBreaking := make(map[string]struct{})

	path := filepath.Join(basePath, "cue", "events_meta", "annotations.cue")
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

	cmd := exec.Command("git", "-C", basePath, "ls-tree", "-r", "--name-only", ref, "--", "cue", "cue.mod")
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
	valEvents, okEvents, err := LoadOptionalDomain(p, filepath.Join(tmpDir, "cue/events"))
	if err != nil {
		return nil, err
	}
	valArch, okArch, err := LoadOptionalDomain(p, filepath.Join(tmpDir, "cue/architecture"))
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
