package emitter

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

var reFlowSelectorID = regexp.MustCompile(`\.([A-Za-z][A-Za-z0-9]*)Id\b`)
var reFlowTimeIsZero = regexp.MustCompile(`(\w+At)\s*==\s*""`)
var rePayloadTimeAt = regexp.MustCompile(`(\w+At):\s*([\w.]+At)\b`)

func normalizeFlowExpr(s string) string {
	if s == "" {
		return s
	}
	s = reFlowSelectorID.ReplaceAllString(s, ".$1ID")
	// time.Time fields can't be compared with ""; convert to .IsZero()
	s = reFlowTimeIsZero.ReplaceAllString(s, "$1.IsZero()")
	return s
}

// normalizePayloadExpr transforms event payload struct literals: XxxAt fields that
// reference time.Time domain variables are converted to .Format(time.RFC3339) so they
// satisfy the string type of the domain event struct fields.
func normalizePayloadExpr(s string) string {
	return rePayloadTimeAt.ReplaceAllString(s, "$1: $2.Format(time.RFC3339)")
}

// flowRenderable reports whether all actions inside steps are supported by RenderFlow.
func flowRenderable(steps []normalizer.FlowStep) bool {
	typed, _ := flowir.DecodeSteps(steps)
	return typedFlowRenderable(typed)
}

func typedFlowRenderable(steps []flowir.TypedStep) bool {
	for _, step := range steps {
		if !flowActionSupported(step.Name) {
			return false
		}
		for _, child := range step.Children {
			if !typedFlowRenderable(child) {
				return false
			}
		}
		for _, branch := range step.Branches {
			if !typedFlowRenderable(branch) {
				return false
			}
		}
	}
	return true
}

func flowActionSupported(action string) bool {
	_, ok := flowir.Lookup(action)
	return ok
}

type flowRenderState struct {
	declared map[string]bool
	// pointers tracks whether a declared variable is a pointer type (from repo.Find/Get)
	// vs a value type (from mapping.Assign declare). Used to decide whether to pass &var to repo.Save.
	pointers          map[string]bool
	types             map[string]string // varName → Go type (e.g. "*domain.User", "[]domain.Order")
	goroutineMode     bool              // if true, errors set _pErr/_mu instead of returning
	stepN             *int              // shared monotonic counter; unique suffix for internal temp vars
	returnErrOnly     bool              // if true, errReturn emits `return err` for inner error closures
	deferMode         bool              // if true, errReturn emits `_ = err` (inside defer func(){}())
	concurrMode       string            // "parallel" | "join" | "race" for flow.Parallel/Join/Race goroutines
	concurrVarPfx     string            // e.g. "_fp_0" / "_fj_0" / "_fr_0"
	sagaCompVar       string            // variable name for compensation slice
	serviceName       string            // current service name for flow.Call self-call resolution
	opName            string            // Service.Method for diagnostics
	warningSink       func(normalizer.Warning)
	eventDefsByName   map[string]normalizer.EventDef
	entityDefsByName  map[string]normalizer.Entity
	serviceDefsByName map[string]normalizer.Service
	isStreaming       bool
	infraValues       map[string]any
	currentTyped      *flowir.TypedStep
}

func cloneFlowState(st *flowRenderState) *flowRenderState {
	cp := &flowRenderState{
		declared:         make(map[string]bool, len(st.declared)),
		pointers:         make(map[string]bool, len(st.pointers)),
		types:            make(map[string]string, len(st.types)),
		goroutineMode:    st.goroutineMode,
		stepN:            st.stepN, // share counter across all clones
		returnErrOnly:    st.returnErrOnly,
		deferMode:        st.deferMode,
		concurrMode:      st.concurrMode,
		concurrVarPfx:    st.concurrVarPfx,
		sagaCompVar:      st.sagaCompVar,
		serviceName:      st.serviceName,
		opName:           st.opName,
		warningSink:      st.warningSink,
		eventDefsByName:  st.eventDefsByName,
		entityDefsByName: st.entityDefsByName,
		isStreaming:      st.isStreaming,
		infraValues:      st.infraValues,
		currentTyped:     st.currentTyped,
	}
	for k, v := range st.declared {
		cp.declared[k] = v
	}
	for k, v := range st.pointers {
		cp.pointers[k] = v
	}
	for k, v := range st.types {
		cp.types[k] = v
	}
	return cp
}

func renderFlow(steps []normalizer.FlowStep) string {
	return renderFlowForServiceWithSchemaAndSink("", "", steps, nil, nil, nil)
}

func renderFlowForService(serviceName string, steps []normalizer.FlowStep) string {
	return renderFlowForServiceWithSchemaAndSink(serviceName, "", steps, nil, nil, nil)
}

func renderFlowForServiceWithSchema(serviceName string, steps []normalizer.FlowStep, entities []normalizer.Entity, events []normalizer.EventDef) string {
	return renderFlowForServiceWithSchemaAndSink(serviceName, "", steps, entities, events, nil)
}

func renderFlowForServiceWithSchemaAndSink(serviceName, methodName string, steps []normalizer.FlowStep, entities []normalizer.Entity, events []normalizer.EventDef, warningSink func(normalizer.Warning)) string {
	return renderFlowForServiceWithSchemaAndSinkMode(serviceName, methodName, false, steps, entities, events, warningSink)
}

func renderFlowForServiceWithSchemaAndSinkMode(serviceName, methodName string, isStreaming bool, steps []normalizer.FlowStep, entities []normalizer.Entity, events []normalizer.EventDef, warningSink func(normalizer.Warning)) string {
	return renderFlowForServiceWithSchemaAndSinkModeWithInfra(serviceName, methodName, isStreaming, steps, entities, events, warningSink, nil)
}

func renderFlowForServiceWithSchemaAndSinkModeWithInfra(serviceName, methodName string, isStreaming bool, steps []normalizer.FlowStep, entities []normalizer.Entity, events []normalizer.EventDef, warningSink func(normalizer.Warning), infraValues map[string]any) string {
	typedSteps, _ := flowir.DecodeSteps(steps)
	n := 0
	svcName := strings.TrimSpace(serviceName)
	mName := strings.TrimSpace(methodName)
	opName := svcName
	if opName != "" && mName != "" {
		opName = opName + "." + mName
	}
	st := &flowRenderState{
		declared:          map[string]bool{"resp": true, "err": true},
		pointers:          map[string]bool{},
		types:             map[string]string{},
		stepN:             &n,
		serviceName:       svcName,
		opName:            opName,
		warningSink:       warningSink,
		eventDefsByName:   flowEventDefsByName(events),
		entityDefsByName:  flowEntityDefsByName(entities),
		serviceDefsByName: flowServiceDefsByName(infraValues),
		isStreaming:       isStreaming,
		infraValues:       infraValues,
	}
	var b strings.Builder
	if typedFlowHasAction(typedSteps, "flow.Checkpoint", "flow.Resume") {
		st.declared["_flowCheckpoints"] = true
		st.pointers["_flowCheckpoints"] = false
		st.types["_flowCheckpoints"] = "map[string]any"
		b.WriteString("var _flowCheckpoints map[string]any\n")
	}
	if typedFlowHasAction(typedSteps, "flow.Try", "flow.Catch", "flow.Retry", "flow.Fallback", "flow.Timeout", "flow.ExplainError") {
		st.declared["_flowLastError"] = true
		st.pointers["_flowLastError"] = false
		st.types["_flowLastError"] = "error"
		b.WriteString("var _flowLastError error\n")
	}
	if typedFlowHasAction(typedSteps, "flow.RecordEvent", "flow.Replay", "flow.History.Get") {
		st.declared["_flowHistory"] = true
		st.pointers["_flowHistory"] = false
		st.types["_flowHistory"] = "[]map[string]any"
		st.declared["_flowReplayMode"] = true
		st.pointers["_flowReplayMode"] = false
		st.types["_flowReplayMode"] = "bool"
		b.WriteString("var _flowHistory []map[string]any\n")
		b.WriteString("var _flowReplayMode bool\n")
	}
	b.WriteString(renderTypedFlowSteps(st, typedSteps, 0))
	return b.String()
}

func isSafetyCriticalNoCodegenAction(action string) bool {
	switch strings.TrimSpace(action) {
	case "auth.RequireRole", "logic.Check", "repo.Get", "repo.Find", "repo.Save":
		return true
	default:
		return false
	}
}

func flowEventDefsByName(events []normalizer.EventDef) map[string]normalizer.EventDef {
	if len(events) == 0 {
		return nil
	}
	out := make(map[string]normalizer.EventDef, len(events))
	for _, evt := range events {
		name := strings.TrimSpace(evt.Name)
		if name == "" {
			continue
		}
		out[name] = evt
	}
	return out
}

func flowEntityDefsByName(entities []normalizer.Entity) map[string]normalizer.Entity {
	if len(entities) == 0 {
		return nil
	}
	out := make(map[string]normalizer.Entity, len(entities))
	for _, ent := range entities {
		name := strings.TrimSpace(ent.Name)
		if name == "" {
			continue
		}
		out[name] = ent
	}
	return out
}

const flowInfraKeyServicesCatalog = "emitter.services_catalog"

func flowServiceDefsByName(infraValues map[string]any) map[string]normalizer.Service {
	if infraValues == nil {
		return nil
	}
	raw, ok := infraValues[flowInfraKeyServicesCatalog]
	if !ok {
		return nil
	}
	services, ok := raw.([]normalizer.Service)
	if !ok || len(services) == 0 {
		return nil
	}
	out := make(map[string]normalizer.Service, len(services))
	for _, svc := range services {
		name := strings.TrimSpace(svc.Name)
		if name == "" {
			continue
		}
		out[name] = svc
	}
	return out
}

func typedFlowHasAction(steps []flowir.TypedStep, actions ...string) bool {
	need := make(map[string]struct{}, len(actions))
	for _, a := range actions {
		need[a] = struct{}{}
	}
	var walk func([]flowir.TypedStep) bool
	walk = func(items []flowir.TypedStep) bool {
		for _, s := range items {
			if _, ok := need[s.Name]; ok {
				return true
			}
			for _, child := range s.Children {
				if walk(child) {
					return true
				}
			}
			for _, branch := range s.Branches {
				if walk(branch) {
					return true
				}
			}
		}
		return false
	}
	return walk(steps)
}

func renderFlowSteps(st *flowRenderState, steps []normalizer.FlowStep, indent int) string {
	typed, _ := flowir.DecodeSteps(steps)
	return renderTypedFlowSteps(st, typed, indent)
}

func renderFlowChildSteps(st *flowRenderState, child func(string) []normalizer.FlowStep, key string, indent int) string {
	if st != nil && st.currentTyped != nil {
		return renderTypedFlowSteps(st, st.currentTyped.Children[key], indent)
	}
	return renderFlowSteps(st, child(key), indent)
}

func flowChildStepCount(st *flowRenderState, child func(string) []normalizer.FlowStep, key string) int {
	if st != nil && st.currentTyped != nil {
		return len(st.currentTyped.Children[key])
	}
	return len(child(key))
}

func renderFlowNestedSteps(st *flowRenderState, key string, fallback []normalizer.FlowStep, indent int) string {
	if st != nil && st.currentTyped != nil {
		return renderTypedFlowSteps(st, st.currentTyped.Children[key], indent)
	}
	return renderFlowSteps(st, fallback, indent)
}

func flowNestedStepCount(st *flowRenderState, key string, fallback []normalizer.FlowStep) int {
	if st != nil && st.currentTyped != nil {
		return len(st.currentTyped.Children[key])
	}
	return len(fallback)
}

func renderFlowBranchSteps(st *flowRenderState, name string, fallback []normalizer.FlowStep, indent int) string {
	if st != nil && st.currentTyped != nil {
		return renderTypedFlowSteps(st, st.currentTyped.Branches[name], indent)
	}
	return renderFlowSteps(st, fallback, indent)
}

func flowBranchNames(st *flowRenderState, fallback map[string][]normalizer.FlowStep) []string {
	keys := make([]string, 0)
	if st != nil && st.currentTyped != nil {
		keys = make([]string, 0, len(st.currentTyped.Branches))
		for key := range st.currentTyped.Branches {
			keys = append(keys, key)
		}
	} else {
		keys = make([]string, 0, len(fallback))
		for key := range fallback {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func renderTypedFlowSteps(st *flowRenderState, steps []flowir.TypedStep, indent int) string {
	var b strings.Builder
	for i, typedStep := range steps {
		if trace := flowStepTraceComment(typedStep.Source, *st.stepN+1, indent); trace != "" {
			b.WriteString(trace)
		}
		code := ""
		if typedStep.DecodeError == nil {
			code = renderOneTypedFlowStep(st, typedStep, indent)
		}
		if code == "" && flowActionSupported(typedStep.Name) {
			severity := "warn"
			codeName := "FLOW_STEP_NO_CODEGEN"
			if isSafetyCriticalNoCodegenAction(typedStep.Name) {
				severity = "error"
				codeName = "FLOW_STEP_NO_CODEGEN_CRITICAL"
			}
			if st.warningSink != nil {
				st.warningSink(normalizer.Warning{
					Kind:     "flow",
					Code:     codeName,
					Severity: severity,
					Message:  fmt.Sprintf("step %d (%s) produced no code; check required fields", i+1, typedStep.Name),
					Op:       st.opName,
					Step:     i + 1,
					Action:   typedStep.Name,
					File:     typedStep.Source.File,
					Line:     typedStep.Source.Line,
					Column:   typedStep.Source.Column,
					CUEPath:  typedStep.Source.CUEPath,
					Hint:     "Verify required step fields in cue/schema/types.cue and flow docs",
				})
			}
			if st.warningSink == nil {
				slog.Warn("flow.step.no_codegen",
					"step", i+1,
					"action", typedStep.Name,
					"file", typedStep.Source.File,
					"line", typedStep.Source.Line,
					"severity", severity,
					"hint", "missing required flow fields",
				)
			}
			pad := strings.Repeat("\t", indent)
			b.WriteString(fmt.Sprintf("%s// WARNING: step %d (%s) produced no code; check required fields\n", pad, i+1, typedStep.Name))
			b.WriteString(renderInvalidFlowStepConfig(st, pad, typedStep.Name, "step produced no code; check required fields"))
		}
		b.WriteString(code)
	}
	return b.String()
}

func renderOneTypedFlowStep(st *flowRenderState, step flowir.TypedStep, indent int) string {
	previous := st.currentTyped
	st.currentTyped = &step
	defer func() { st.currentTyped = previous }()
	return renderOneFlowStepTyped(st, step, indent)
}

func renderOneFlowStepTyped(st *flowRenderState, typedStep flowir.TypedStep, indent int) string {
	return renderTypedStepDispatch(st, typedStep, indent)
}

// renderTypedStepDispatch is the sole production entry into action emission.
func renderTypedStepDispatch(st *flowRenderState, typedStep flowir.TypedStep, indent int) string {
	spec, registered := flowir.Lookup(typedStep.Name)
	if !registered {
		return ""
	}
	switch spec.RendererGroup {
	case flowir.RendererGroupState:
		if out, ok := renderTypedStepState(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupCache:
		if out, ok := renderTypedStepCache(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupStorageSimple:
		if out, ok := renderTypedStepStorageSimple(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupStorageData:
		if out, ok := renderTypedStepStorageData(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupMail:
		if out, ok := renderTypedStepMail(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupSecretConfig:
		if out, ok := renderTypedStepSecretConfig(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupCore:
		if out, ok := renderTypedStepCore(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupCall:
		if out, ok := renderTypedStepCall(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupRepositoryBasic:
		if out, ok := renderTypedStepRepositoryBasic(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupRepositoryAdvanced:
		if out, ok := renderTypedStepRepositoryAdvanced(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupDB:
		if out, ok := renderTypedStepDB(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupMapping:
		if out, ok := renderTypedStepMapping(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupJSON:
		if out, ok := renderTypedStepJSON(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupHTTPCall:
		if out, ok := renderTypedStepHTTPCall(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupHTTPAdvanced:
		if out, ok := renderTypedStepHTTPAdvanced(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupString:
		if out, ok := renderTypedStepString(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupDataTransform:
		if out, ok := renderTypedStepDataTransform(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupSecurity:
		if out, ok := renderTypedStepSecurity(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupConcurrencyDelivery:
		if out, ok := renderTypedStepConcurrencyAndDelivery(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupEventOrchestration:
		if out, ok := renderTypedStepEventOrchestration(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupPolicy:
		if out, ok := renderTypedStepPolicy(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupReliability:
		if out, ok := renderTypedStepReliability(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupOAuthGoogle:
		if out, ok := renderTypedStepOAuthGoogle(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupControlResilience:
		if out, ok := renderTypedStepControlResilience(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupControlFlowBasic:
		if out, ok := renderTypedStepControlFlowBasic(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupControlFlowStateful:
		if out, ok := renderTypedStepControlFlowStateful(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupControlParallel:
		if out, ok := renderTypedStepControlParallel(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupSaga:
		if out, ok := renderTypedStepSaga(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupControlScheduling:
		if out, ok := renderTypedStepControlScheduling(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupCollections:
		if out, ok := renderTypedStepCollections(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupDomainErrors:
		if out, ok := renderTypedStepDomainErrors(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupDomainAuth:
		if out, ok := renderTypedStepDomainAuth(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupDomainPrimitives:
		if out, ok := renderTypedStepDomainPrimitives(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupDomainTime:
		if out, ok := renderTypedStepDomainTime(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupDomainValidation:
		if out, ok := renderTypedStepDomainValidation(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupListEnrich:
		if out, ok := renderTypedStepListEnrich(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupDomainSpecial:
		if out, ok := renderTypedStepDomainSpecial(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupRBAC:
		if out, ok := renderTypedStepRBAC(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupLogicCheck:
		if out, ok := renderTypedStepLogicCheck(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupDomainComputed:
		if out, ok := renderTypedStepDomainComputed(st, typedStep, indent); ok {
			return out
		}
	case flowir.RendererGroupExecFS:
		if out, ok := renderTypedStepExecFS(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	case flowir.RendererGroupInfrastructure:
		if out, ok := renderTypedStepInfrastructure(st, typedStep, indent, nextFlowStepSuffix(st)); ok {
			return out
		}
	}
	pad := strings.Repeat("\t", indent)
	return renderInvalidFlowStepConfig(st, pad, typedStep.Name, fmt.Sprintf("renderer group %q has no typed emitter route", spec.RendererGroup))
}

func nextFlowStepSuffix(st *flowRenderState) string {
	sfx := fmt.Sprintf("_%d", *st.stepN)
	*st.stepN++
	return sfx
}

func typedActionAs[T flowir.Action](step flowir.TypedStep) (T, error) {
	var zero T
	if step.DecodeError != nil {
		return zero, step.DecodeError
	}
	typed, ok := step.Action.(T)
	if !ok {
		return zero, fmt.Errorf("action %q decoded as %T", step.Name, step.Action)
	}
	return typed, nil
}

func decodeCurrentActionAs[T flowir.Action](st *flowRenderState, raw normalizer.FlowStep) (T, error) {
	var zero T
	if st == nil || st.currentTyped == nil {
		return zero, fmt.Errorf("action %q has no typed Flow IR context", raw.Action)
	}
	if st.currentTyped.Name != raw.Action {
		return zero, fmt.Errorf("typed Flow IR action %q does not match renderer action %q", st.currentTyped.Name, raw.Action)
	}
	if st.currentTyped.DecodeError != nil {
		return zero, st.currentTyped.DecodeError
	}
	if typed, ok := st.currentTyped.Action.(T); ok {
		return typed, nil
	}
	return zero, fmt.Errorf("action %q decoded as %T", raw.Action, st.currentTyped.Action)
}

func flowStepTraceComment(source flowir.Source, stepIdx int, indent int) string {
	file := strings.TrimSpace(source.File)
	if file == "" && strings.TrimSpace(source.CUEPath) != "" {
		file = strings.TrimSpace(source.CUEPath)
	}
	if file == "" {
		return ""
	}
	ref := filepath.ToSlash(filepath.Clean(file))
	if source.Line > 0 {
		ref = fmt.Sprintf("%s:%d", ref, source.Line)
	}
	pad := strings.Repeat("\t", indent)
	return fmt.Sprintf("%s// Generated from: %s (flow step %d)\n", pad, ref, stepIdx)
}

func returnSuccess(st *flowRenderState, pad string) string {
	if st.isStreaming {
		return fmt.Sprintf("%sreturn nil\n", pad)
	}
	return fmt.Sprintf("%sreturn resp, nil\n", pad)
}

// errReturn generates the appropriate error-return code depending on context.
// In goroutine mode, errors are captured via mutex rather than returning from the outer func.
func errReturn(st *flowRenderState, pad, errExpr string) string {
	switch st.concurrMode {
	case "parallel":
		pfx := st.concurrVarPfx
		return fmt.Sprintf("%s%sMu.Lock()\nif %sErr == nil { %sErr = %s; %sCancel() }\n%s%sMu.Unlock()\n%sreturn\n",
			pad, pfx, pfx, pfx, errExpr, pfx, pad, pfx, pad)
	case "join":
		pfx := st.concurrVarPfx
		return fmt.Sprintf("%s%sMu.Lock()\n%s%sErrs = append(%sErrs, %s)\n%s%sMu.Unlock()\n%sreturn\n",
			pad, pfx, pad, pfx, pfx, errExpr, pad, pfx, pad)
	case "race":
		return fmt.Sprintf("%sreturn\n", pad)
	}
	if st.goroutineMode {
		return fmt.Sprintf("%s_mu.Lock()\nif _pErr == nil { _pErr = %s; _pCancel() }\n%s_mu.Unlock()\n%sreturn\n", pad, errExpr, pad, pad)
	}
	if st.deferMode {
		return fmt.Sprintf("%s_ = %s\n", pad, errExpr)
	}
	if st.returnErrOnly {
		return fmt.Sprintf("%sreturn %s\n", pad, errExpr)
	}
	if st.isStreaming {
		return fmt.Sprintf("%sreturn %s\n", pad, errExpr)
	}
	return fmt.Sprintf("%sreturn resp, %s\n", pad, errExpr)
}

func renderInvalidFlowStepConfig(st *flowRenderState, pad, action, msg string) string {
	msg = flowInvalidConfigMessage(action, msg)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s// invalid %s configuration\n", pad, action))
	b.WriteString(errReturn(st, pad, fmt.Sprintf("fmt.Errorf(%q)", action+": "+msg)))
	return b.String()
}

func flowInvalidConfigMessage(action, fallback string) string {
	required := map[string]string{
		"auth.RequireRole":      "auth.RequireRole requires userID, companyID, and roles",
		"circuit.RecordSuccess": "circuit.RecordSuccess requires name",
		"config.Get":            "config.Get requires key and output",
		"dedupe.Once":           "dedupe.Once requires key",
		"flow.Call":             "flow.Call requires op",
		"flow.Cron":             "flow.Cron requires window",
		"flow.Tag":              "flow.Tag requires name",
		"http.Paginate":         "http.Paginate requires url, into, as, and cursor_expr",
		"http.Request":          "http.Request requires method and url",
		"http.RetryPolicy":      "http.RetryPolicy requires method and url",
		"http.SOAP":             "http.SOAP requires url, namespace, and operation",
		"list.Sum":              "list.Sum requires input and output",
		"notify.Email":          "notify.Email requires to",
	}
	if message, ok := required[action]; ok && strings.Contains(strings.ToLower(fallback), "required") {
		return message
	}
	return fallback
}

func resolveFlowDynamicOutputType(st *flowRenderState, output, into string) string {
	outputType := strings.TrimSpace(into)
	if outputType == "" && output != "" && st.declared[output] {
		outputType = strings.TrimSpace(st.types[output])
	}
	if outputType == "" {
		outputType = "any"
	}
	return outputType
}

func emitFlowWarning(st *flowRenderState, code, severity, message, hint string) {
	meta := flowir.StepMeta{}
	if st.currentTyped != nil {
		meta = st.currentTyped.Meta()
	}
	if st.warningSink != nil {
		st.warningSink(normalizer.Warning{
			Kind:     "flow",
			Code:     code,
			Severity: severity,
			Message:  message,
			Op:       st.opName,
			Action:   meta.Name,
			File:     meta.Source.File,
			Line:     meta.Source.Line,
			Column:   meta.Source.Column,
			CUEPath:  meta.Source.CUEPath,
			Hint:     hint,
		})
		return
	}
	slog.Warn("flow.warning",
		"code", code,
		"severity", severity,
		"action", meta.Name,
		"file", meta.Source.File,
		"line", meta.Source.Line,
		"message", message,
		"hint", hint,
	)
}

func renderOneFlowStep(st *flowRenderState, step normalizer.FlowStep, indent int) string {
	typed, _ := flowir.DecodeSteps([]normalizer.FlowStep{step})
	if len(typed) == 1 {
		return renderOneTypedFlowStep(st, typed[0], indent)
	}
	return ""
}
