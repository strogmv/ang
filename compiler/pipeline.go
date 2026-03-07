package compiler

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/parser"
	planpkg "github.com/strogmv/ang/compiler/plan"
	"github.com/strogmv/ang/compiler/transformers"
)

const (
	Version       = "0.1.116"
	SchemaVersion = "1"
)

func ComputeProjectHash(path string) (string, error) {
	h := sha256.New()
	err := filepath.Walk(filepath.Join(path, "cue"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".cue") {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(h, f); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

type PipelineOptions struct {
	WarningSink       func(normalizer.Warning)
	ArchitectureMode  string
	AllowCrossService map[string]map[string]struct{}
}

type RunPhase string

const (
	PhaseAll   RunPhase = "all"
	PhasePlan  RunPhase = "plan"
	PhaseApply RunPhase = "apply"
)

type RunOptions struct {
	Phase       RunPhase
	PlanFile    string
	JSON        bool
	OutPlan     string
	WarningSink func(normalizer.Warning)
}

var LatestDiagnostics []normalizer.Warning

func RunWithOptions(basePath string, opts RunOptions) (*planpkg.BuildPlan, error) {
	phase := opts.Phase
	if phase == "" {
		phase = PhaseAll
	}

	var currentPlan *planpkg.BuildPlan
	var err error
	switch phase {
	case PhasePlan:
		currentPlan, err = BuildPlan(basePath, opts)
		if err != nil {
			return nil, err
		}
		if opts.OutPlan != "" {
			if err := planpkg.WritePlan(opts.OutPlan, currentPlan); err != nil {
				return nil, err
			}
		}
		return currentPlan, nil
	case PhaseApply:
		if opts.PlanFile == "" {
			return nil, fmt.Errorf("--plan-file is required for apply phase")
		}
		currentPlan, err = planpkg.ReadPlan(opts.PlanFile)
		if err != nil {
			return nil, err
		}
		if err := ApplyPlan(basePath, currentPlan); err != nil {
			return nil, err
		}
		return currentPlan, nil
	case PhaseAll:
		currentPlan, err = BuildPlan(basePath, opts)
		if err != nil {
			return nil, err
		}
		if opts.OutPlan != "" {
			if err := planpkg.WritePlan(opts.OutPlan, currentPlan); err != nil {
				return nil, err
			}
		}
		if err := ApplyPlan(basePath, currentPlan); err != nil {
			return nil, err
		}
		return currentPlan, nil
	default:
		return nil, fmt.Errorf("unknown run phase: %s", phase)
	}
}

func RunPipeline(basePath string) ([]normalizer.Entity, []normalizer.Service, []normalizer.Endpoint, []normalizer.Repository, []normalizer.EventDef, []normalizer.ErrorDef, []normalizer.ScheduleDef, []normalizer.ScenarioDef, []normalizer.ScopeDef, error) {
	return RunPipelineWithOptions(basePath, PipelineOptions{
		WarningSink: func(w normalizer.Warning) {
			LatestDiagnostics = append(LatestDiagnostics, w)
		},
	})
}

func RunPipelineWithOptions(basePath string, opts PipelineOptions) ([]normalizer.Entity, []normalizer.Service, []normalizer.Endpoint, []normalizer.Repository, []normalizer.EventDef, []normalizer.ErrorDef, []normalizer.ScheduleDef, []normalizer.ScenarioDef, []normalizer.ScopeDef, error) {
	normalized, err := RunSemanticPhasesWithOptions(basePath, opts)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}

	return normalized.Entities, normalized.Services, normalized.Endpoints, normalized.Repos, normalized.Events, normalized.Errors, normalized.Schedules, normalized.Scenarios, normalized.Scopes, nil
}

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
// two sets: broadcastOnly and planned.
func loadEventAnnotations(basePath string) (map[string]struct{}, map[string]struct{}, map[string]struct{}) {
	broadcastOnly := make(map[string]struct{})
	planned := make(map[string]struct{})
	compatAllowBreaking := make(map[string]struct{})

	path := filepath.Join(basePath, "cue", "events_meta", "annotations.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		return broadcastOnly, planned, compatAllowBreaking
	}

	// very small ad-hoc parser for map-like sections; avoid pulling CUE parser here
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
				Hint:     "Set owner in cue/events, e.g. owner: \"tender\"",
				File:     file,
				Line:     line,
			}, opts)
		} else if owner := resolveEventServiceName(ownerRaw, serviceSet); owner == "" {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "event-contract",
				Code:     "EVENT_OWNER_UNKNOWN_SERVICE",
				Severity: contractSeverity,
				Message:  fmt.Sprintf("Event %s owner '%s' does not match any service", e.Name, e.Owner),
				Hint:     "Use an existing service name in owner, e.g. \"tender\" (or set ANG_EVENT_CONTRACT_STRICT=1 to fail CI)",
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

func emitReadModelDiagnostics(entities []normalizer.Entity, events []normalizer.EventDef, opts PipelineOptions) {
	eventSet := make(map[string]struct{}, len(events))
	for _, evt := range events {
		eventSet[strings.TrimSpace(evt.Name)] = struct{}{}
	}

	normalizeContext := func(v string) string {
		v = strings.TrimSpace(strings.ToLower(v))
		if v == "" {
			return ""
		}
		for _, sep := range []string{"_", "-", "."} {
			if i := strings.Index(v, sep); i > 0 {
				return v[:i]
			}
		}
		return v
	}

	for _, ent := range entities {
		rm := ent.ReadModel
		if rm == nil {
			if b, ok := ent.Metadata["read_model"].(bool); !ok || !b {
				continue
			}
			rm = &normalizer.ReadModelDef{}
			if src, ok := ent.Metadata["read_model_source_context"].(string); ok {
				rm.SourceContext = strings.TrimSpace(strings.ToLower(src))
			}
			switch v := ent.Metadata["read_model_refresh_on"].(type) {
			case []string:
				rm.RefreshOn = append([]string(nil), v...)
			case []any:
				for _, item := range v {
					s := strings.TrimSpace(fmt.Sprint(item))
					if s != "" {
						rm.RefreshOn = append(rm.RefreshOn, s)
					}
				}
			}
		}

		file, line := parseSourcePos(ent.Source)
		srcCtx := normalizeContext(rm.SourceContext)
		entityCtx := normalizeContext(ent.BoundedContext)
		if entityCtx == "" {
			entityCtx = normalizeContext(ent.Owner)
		}

		if srcCtx == "" {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "read-model",
				Code:     "READ_MODEL_SOURCE_CONTEXT_MISSING",
				Severity: "error",
				Message:  fmt.Sprintf("Read model %s must declare read_model.source_context", ent.Name),
				Hint:     "Set read_model: { source_context: \"company\", refreshOn: [\"EventName\"] }",
				File:     file,
				Line:     line,
			}, opts)
		}
		if len(rm.RefreshOn) == 0 {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "read-model",
				Code:     "READ_MODEL_REFRESH_ON_MISSING",
				Severity: "error",
				Message:  fmt.Sprintf("Read model %s must declare read_model.refreshOn events", ent.Name),
				Hint:     "Add read_model.refreshOn with at least one domain event name",
				File:     file,
				Line:     line,
			}, opts)
			continue
		}
		for _, evt := range rm.RefreshOn {
			evt = strings.TrimSpace(evt)
			if evt == "" {
				continue
			}
			if _, ok := eventSet[evt]; ok {
				continue
			}
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "read-model",
				Code:     "READ_MODEL_REFRESH_EVENT_UNKNOWN",
				Severity: "error",
				Message:  fmt.Sprintf("Read model %s references unknown refresh event %s", ent.Name, evt),
				Hint:     "Define this event in cue/events or fix the refreshOn name",
				File:     file,
				Line:     line,
			}, opts)
		}
		if srcCtx != "" && entityCtx != "" && srcCtx == entityCtx {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "read-model",
				Code:     "READ_MODEL_SAME_CONTEXT",
				Severity: "warn",
				Message:  fmt.Sprintf("Read model %s source_context equals bounded context (%s)", ent.Name, srcCtx),
				Hint:     "Use read model for cross-context data; same-context projection is usually unnecessary",
				File:     file,
				Line:     line,
			}, opts)
		}
	}
}

func loadEventsFromGitRef(basePath, ref string) ([]normalizer.EventDef, error) {
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

func emitSelectProjectionDiagnostics(
	entity string,
	finders []normalizer.RepositoryFinder,
	entityFieldMap map[string]map[string]string,
	opts PipelineOptions,
) {
	fields, ok := entityFieldMap[entity]
	if !ok || len(fields) == 0 {
		return
	}
	total := len(fields)

	for _, f := range finders {
		if len(f.Select) == 0 {
			continue
		}
		if !finderReturnsEntity(f, entity) {
			continue
		}

		selected := make(map[string]struct{}, len(f.Select))
		for _, col := range f.Select {
			key := canonicalFieldToken(col)
			if key != "" {
				selected[key] = struct{}{}
			}
		}

		var missing []string
		for fieldName := range fields {
			if _, ok := selected[canonicalFieldToken(fieldName)]; !ok {
				missing = append(missing, fieldName)
			}
		}
		if len(missing) == 0 {
			continue
		}

		file, line := parseSourcePos(f.Source)
		msg := fmt.Sprintf(
			"Finder '%s.%s' returns domain.%s but select has %d/%d fields; partial entity select is forbidden",
			entity, f.Name, entity, total-len(missing), total,
		)
		hint := "Use full select for entity return, or set return_type to a projection DTO/custom type"
		diag := normalizer.Warning{
			Kind:     "architecture",
			Code:     "ENTITY_PARTIAL_SELECT_ERROR",
			Severity: "error",
			Message:  msg,
			File:     file,
			Line:     line,
			Hint:     hint,
		}
		recordPipelineDiagnostic(diag, opts)
	}
}

func synthesizeImplicitProjections(
	entity string,
	finders []normalizer.RepositoryFinder,
	entityByName map[string]normalizer.Entity,
	projNameByKey map[string]string,
) ([]normalizer.RepositoryFinder, []normalizer.Entity) {
	src, ok := entityByName[entity]
	if !ok {
		return finders, nil
	}

	lookup := make(map[string]normalizer.Field)
	for _, f := range src.Fields {
		if f.SkipDomain {
			continue
		}
		lookup[canonicalFieldToken(f.Name)] = f
	}

	var projections []normalizer.Entity
	for i := range finders {
		f := &finders[i]
		if len(f.Select) == 0 || !finderReturnsEntity(*f, entity) {
			continue
		}
		if strings.TrimSpace(f.ReturnType) != "" {
			continue // Explicit return_type must remain explicit; validator will enforce compatibility.
		}

		fields, orderedCols, key, ok := projectionFieldsForSelect(entity, f.Select, src)
		if !ok {
			continue
		}
		if len(fields) == len(lookup) {
			continue // Full select is allowed for domain entity return.
		}

		projName, ok := projNameByKey[key]
		if !ok {
			projName = projectionName(entity, orderedCols)
			projNameByKey[key] = projName
			projections = append(projections, normalizer.Entity{
				Name:   projName,
				Owner:  src.Owner,
				Fields: fields,
				Metadata: map[string]any{
					"projection": true,
					"source":     entity,
				},
				Source: f.Source,
			})
		}

		f.Select = append([]string(nil), orderedCols...)
		if finderReturnsMany(*f, entity) {
			f.ReturnType = "[]domain." + projName
		} else {
			f.ReturnType = "*domain." + projName
		}
	}
	return finders, projections
}

func projectionFieldsForSelect(
	entity string,
	selectCols []string,
	src normalizer.Entity,
) ([]normalizer.Field, []string, string, bool) {
	keys := make([]string, 0, len(selectCols))
	seen := make(map[string]struct{}, len(selectCols))
	for _, col := range selectCols {
		k := canonicalFieldToken(col)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil, nil, "", false
	}
	keyTokens := append([]string(nil), keys...)
	sort.Strings(keyTokens)

	fields := make([]normalizer.Field, 0, len(keys))
	orderedCols := make([]string, 0, len(keys))
	for _, f := range src.Fields {
		if f.SkipDomain {
			continue
		}
		k := canonicalFieldToken(f.Name)
		if _, ok := seen[k]; !ok {
			continue
		}
		delete(seen, k)
		fields = append(fields, f)
		orderedCols = append(orderedCols, f.Name)
	}
	if len(seen) > 0 {
		return nil, nil, "", false
	}
	return fields, orderedCols, entity + "|" + strings.Join(keyTokens, ","), true
}

func projectionName(entity string, sortedCols []string) string {
	parts := make([]string, 0, len(sortedCols))
	for _, c := range sortedCols {
		s := strings.TrimSpace(c)
		if s == "" {
			continue
		}
		parts = append(parts, strings.ToLower(ToSnakeCase(s)))
	}
	if len(parts) == 0 {
		return entity + "_Proj"
	}
	return entity + "_" + strings.Join(parts, "_") + "_Proj"
}

func finderReturnsEntity(f normalizer.RepositoryFinder, entity string) bool {
	if strings.EqualFold(f.Returns, "count") || strings.EqualFold(f.Action, "delete") {
		return false
	}
	if f.ReturnType == "" {
		return true
	}

	rt := strings.TrimSpace(strings.ToLower(f.ReturnType))
	rt = strings.TrimPrefix(rt, "[]")
	rt = strings.TrimPrefix(rt, "*")
	rt = strings.TrimPrefix(rt, "domain.")
	return rt == strings.ToLower(entity)
}

func finderReturnsMany(f normalizer.RepositoryFinder, entity string) bool {
	if strings.EqualFold(f.Returns, "many") || strings.EqualFold(f.Returns, "[]"+entity) {
		return true
	}
	rt := strings.TrimSpace(strings.ToLower(f.ReturnType))
	return strings.HasPrefix(rt, "[]")
}

func canonicalFieldToken(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return ""
	}
	s = strings.Trim(s, "`\"")
	s = strings.SplitN(s, " ", 2)[0]
	if idx := strings.LastIndex(s, "."); idx >= 0 {
		s = s[idx+1:]
	}
	s = strings.ToLower(s)
	return strings.ReplaceAll(s, "_", "")
}

func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}
	var out []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, '_')
		}
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		out = append(out, r)
	}
	return string(out)
}

func parseSourcePos(source string) (file string, line int) {
	file = source
	if source == "" {
		return "", 0
	}
	idx := strings.LastIndex(source, ":")
	if idx <= 0 || idx == len(source)-1 {
		return source, 0
	}
	n, err := strconv.Atoi(source[idx+1:])
	if err != nil {
		return source, 0
	}
	return source[:idx], n
}

func emitFSMIntegrityDiagnostics(entities []normalizer.Entity, opts PipelineOptions) {
	seen := map[string]struct{}{}
	for _, e := range entities {
		if e.FSM == nil {
			continue
		}

		stateSet := make(map[string]struct{}, len(e.FSM.States))
		for _, s := range e.FSM.States {
			key := strings.TrimSpace(s)
			if key == "" {
				continue
			}
			stateSet[key] = struct{}{}
		}
		if len(stateSet) == 0 {
			continue
		}

		for from, toStates := range e.FSM.Transitions {
			fromState := strings.TrimSpace(from)
			if fromState != "" {
				if _, ok := stateSet[fromState]; !ok {
					file, line := parseSourcePos(e.Source)
					toPreview := ""
					if len(toStates) > 0 {
						toPreview = strings.TrimSpace(toStates[0])
					}
					diag := normalizer.Warning{
						Kind:     "architecture",
						Code:     "E_FSM_UNDEFINED_STATE",
						Severity: "error",
						Message: fmt.Sprintf(
							"Entity '%s' FSM transition '%s→%s' references undefined state '%s'",
							e.Name, fromState, toPreview, fromState,
						),
						File: file,
						Line: line,
						Hint: fmt.Sprintf("Add '%s' to fsm.states or update transition source.", fromState),
					}
					key := diag.Code + "|" + diag.Message + "|" + diag.File + "|" + strconv.Itoa(diag.Line)
					if _, ok := seen[key]; !ok {
						seen[key] = struct{}{}
						recordPipelineDiagnostic(diag, opts)
					}
				}
			}
			for _, to := range toStates {
				state := strings.TrimSpace(to)
				if state == "" {
					continue
				}
				if _, ok := stateSet[state]; ok {
					continue
				}
				file, line := parseSourcePos(e.Source)
				diag := normalizer.Warning{
					Kind:     "architecture",
					Code:     "E_FSM_UNDEFINED_STATE",
					Severity: "error",
					Message: fmt.Sprintf(
						"Entity '%s' FSM transition '%s→%s' references undefined state '%s'",
						e.Name, from, state, state,
					),
					File: file,
					Line: line,
					Hint: fmt.Sprintf("Add '%s' to fsm.states or update transition target.", state),
				}
				key := diag.Code + "|" + diag.Message + "|" + diag.File + "|" + strconv.Itoa(diag.Line)
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					recordPipelineDiagnostic(diag, opts)
				}
			}
		}
	}
}

func recordPipelineDiagnostic(diag normalizer.Warning, opts PipelineOptions) {
	if opts.WarningSink != nil {
		opts.WarningSink(diag)
		return
	}
	LatestDiagnostics = append(LatestDiagnostics, diag)
}

func emitFileSizeDiagnostics(path string, opts PipelineOptions) {
	const lineLimit = 300
	matches, _ := filepath.Glob(filepath.Join(path, "*.cue"))
	for _, filePath := range matches {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		lineCount := strings.Count(string(content), "\n") + 1
		if lineCount > lineLimit {
			diag := normalizer.Warning{
				Kind:     "file-size",
				Code:     "LARGE_CUE_FILE",
				Severity: "warn",
				Message:  fmt.Sprintf("CUE file has %d lines (recommended limit: %d)", lineCount, lineLimit),
				File:     filePath,
				Line:     1,
				Hint:     "Split into multiple files in the same directory (CUE merges files with same package automatically)",
			}
			recordPipelineDiagnostic(diag, opts)
		}
	}
}

func LoadOptionalDomain(p *parser.Parser, path string) (cue.Value, bool, error) {
	matches, _ := filepath.Glob(filepath.Join(path, "*.cue"))
	if len(matches) == 0 {
		return cue.Value{}, false, nil
	}
	val, err := p.LoadDomain(path)
	if err != nil {
		return cue.Value{}, false, err
	}
	return val, true, nil
}

func ConvertAndTransform(
	entities []normalizer.Entity, services []normalizer.Service, events []normalizer.EventDef,
	errors []normalizer.ErrorDef, endpoints []normalizer.Endpoint, scopes []normalizer.ScopeDef, repos []normalizer.Repository,
	config normalizer.ConfigDef, auth *normalizer.AuthDef, rbac *normalizer.RBACDef,
	schedules []normalizer.ScheduleDef, views []normalizer.ViewDef, project normalizer.ProjectDef,
) (*ir.Schema, error) {
	schema := ir.ConvertFromNormalizer(entities, services, events, errors, endpoints, scopes, repos, config, auth, rbac, schedules, views, project)
	if err := ir.MigrateToCurrent(schema); err != nil {
		return nil, WrapContractError(StageIR, ErrCodeIRVersionMigration, "migrate ir schema", err)
	}

	registry := transformers.DefaultRegistry()
	if err := registry.Apply(schema); err != nil {
		return nil, WrapContractError(StageTransformers, ErrCodeTransformerApply, "apply transformers", err)
	}

	hooks := transformers.DefaultHookRegistry()
	if err := hooks.Process(schema); err != nil {
		return nil, WrapContractError(StageTransformers, ErrCodeHookProcess, "process hooks", err)
	}
	if err := ir.ValidateABIV2(schema); err != nil {
		return nil, WrapContractError(StageIR, ErrCodeIRVersionMigration, "validate ir abi v2", err)
	}

	return schema, nil
}

func validateFlowIntegrity(services []normalizer.Service) error {
	for _, svc := range services {
		for _, m := range svc.Methods {
			if len(m.Flow) == 0 {
				continue
			}
			declared := make(map[string]string)
			used := make(map[string]bool)
			for _, s := range m.Flow {
				for _, arg := range []string{"input", "value", "condition", "payload", "actor", "company"} {
					if val, ok := s.Args[arg].(string); ok {
						for name := range declared {
							if strings.Contains(val, name) {
								used[name] = true
							}
						}
					}
				}
				if out, ok := s.Args["output"].(string); ok && out != "" && out != "resp" {
					declared[out] = fmt.Sprintf("%s:%d", s.File, s.Line)
				}
			}
			for name, loc := range declared {
				if !used[name] {
					return fmt.Errorf("Logic Error: variable %s declared at %s is never used in method %s.%s", name, loc, svc.Name, m.Name)
				}
			}
		}
	}
	return nil
}
