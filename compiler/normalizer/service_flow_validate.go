package normalizer

import (
	"fmt"
	"strings"
)

func validateFlowSteps(opName string, svcName string, steps []FlowStep, entities []Entity, svcUses []string, policies map[string]PolicyDef, architectureMode string, allowCrossService map[string]map[string]struct{}) []FlowWarning {
	var warnings []FlowWarning
	seenWarnings := make(map[string]struct{})
	declaredVars := make(map[string]bool)
	assignedFields := make(map[string]bool)
	newEntities := make(map[string]string)

	entityOwners := make(map[string]string)
	entityContexts := make(map[string]string)
	aggregateOwnedByContext := make(map[string]map[string]struct{})
	isDTO := make(map[string]bool)
	isSharedArch := make(map[string]bool)
	for _, e := range entities {
		entityOwners[e.Name] = e.Owner
		ctx := strings.TrimSpace(strings.ToLower(e.BoundedContext))
		if ctx == "" {
			ctx = inferBoundedContext(e.Owner)
		}
		entityContexts[e.Name] = ctx
		if dto, ok := e.Metadata["dto"].(bool); ok && dto {
			isDTO[e.Name] = true
		}
		if shared, ok := e.Metadata["shared_arch"].(bool); ok && shared {
			isSharedArch[e.Name] = true
		}
	}
	for _, e := range entities {
		if !e.AggregateRoot {
			continue
		}
		rootCtx := strings.TrimSpace(strings.ToLower(e.BoundedContext))
		if rootCtx == "" {
			rootCtx = entityContexts[e.Name]
		}
		if rootCtx == "" {
			rootCtx = inferBoundedContext(e.Owner)
		}
		if rootCtx == "" {
			continue
		}
		ownedSet := aggregateOwnedByContext[rootCtx]
		if ownedSet == nil {
			ownedSet = make(map[string]struct{})
			aggregateOwnedByContext[rootCtx] = ownedSet
		}
		ownedSet[e.Name] = struct{}{}
		for _, owned := range e.Owns {
			owned = strings.TrimSpace(owned)
			if owned == "" {
				continue
			}
			ownedSet[owned] = struct{}{}
		}
	}
	serviceContext := inferBoundedContext(svcName)
	isSchemaScaffoldFile := func(file string) bool {
		f := strings.TrimSpace(strings.ReplaceAll(file, "\\", "/"))
		if f == "" {
			return false
		}
		return strings.Contains(f, "/cue/schema/") || strings.HasPrefix(f, "cue/schema/")
	}

	var currentStep FlowStep
	appendWarn := func(w FlowWarning) {
		if strings.HasPrefix(strings.TrimSpace(opName), "#") {
			return
		}
		key := fmt.Sprintf("%s|%s|%s|%d|%d|%d", w.Code, w.Action, w.File, w.Line, w.Column, w.Step)
		if _, ok := seenWarnings[key]; ok {
			return
		}
		seenWarnings[key] = struct{}{}
		warnings = append(warnings, w)
	}
	addWarn := func(step int, action, code, message, hint string, file string, line int, column int, fixes ...Fix) {
		appendWarn(FlowWarning{
			Op:           opName,
			Step:         step,
			Action:       action,
			Message:      message,
			Code:         code,
			Severity:     "error",
			Hint:         hint,
			File:         file,
			Line:         line,
			Column:       column,
			CUEPath:      currentStep.CUEPath,
			SuggestedFix: fixes,
		})
	}

	addWarnWithSeverity := func(step int, action, code, severity, message, hint string, file string, line int, column int, fixes ...Fix) {
		sev := strings.TrimSpace(strings.ToLower(severity))
		if sev == "" {
			sev = "error"
		}
		appendWarn(FlowWarning{
			Op:           opName,
			Step:         step,
			Action:       action,
			Message:      message,
			Code:         code,
			Severity:     sev,
			Hint:         hint,
			File:         file,
			Line:         line,
			Column:       column,
			CUEPath:      currentStep.CUEPath,
			SuggestedFix: fixes,
		})
	}

	archSeverity := "error"
	if strings.EqualFold(strings.TrimSpace(architectureMode), "relaxed") {
		archSeverity = "warn"
	}

	allowedDeps := map[string]struct{}{}
	allowedDeps[strings.ToLower(normalizeServiceName(svcName))] = struct{}{}
	for _, dep := range svcUses {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		allowedDeps[strings.ToLower(normalizeServiceName(dep))] = struct{}{}
	}

	isBoundaryViolation := func(entityName string) (bool, string, string) {
		owner, ok := entityOwners[entityName]
		if !ok {
			return false, "", ""
		}
		if strings.EqualFold(svcName, "admin") || strings.EqualFold(svcName, "audit") {
			return false, "", ""
		}
		if isSharedArch[entityName] {
			return false, "", ""
		}
		if serviceContext != "" {
			if ownedSet, ok := aggregateOwnedByContext[serviceContext]; ok {
				if _, allowed := ownedSet[entityName]; allowed {
					return false, "", ""
				}
			}
		}

		entityCtx := entityContexts[entityName]
		if entityCtx != "" && serviceContext != "" {
			if strings.EqualFold(entityCtx, serviceContext) {
				return false, "", ""
			}
			return true, fmt.Sprintf("bounded_context='%s'", entityCtx), entityCtx
		}

		ownerMatch := strings.EqualFold(owner, svcName) ||
			strings.EqualFold(owner+"s", svcName) ||
			strings.EqualFold(svcName+"s", owner)
		ownerPrefixMatch := strings.HasPrefix(strings.ToLower(owner), strings.ToLower(svcName)+"_")
		if owner != "" && !ownerMatch && !ownerPrefixMatch {
			return true, fmt.Sprintf("owned by '%s'", owner), owner
		}
		return false, "", ""
	}

	var validate func(steps []FlowStep, inTx bool, depth int)
	validate = func(steps []FlowStep, inTx bool, depth int) {
		for i := range steps {
			step := &steps[i]
			currentStep = *step
			stepNum := i + 1
			if isSchemaScaffoldFile(step.File) {
				continue
			}

			if handleFlowDataAndCalls(stepNum, step, inTx, depth, svcName, serviceContext, archSeverity, entityOwners, isDTO, allowedDeps, allowCrossService, declaredVars, assignedFields, newEntities, isBoundaryViolation, addWarn, addWarnWithSeverity, validate) {
				continue
			}
			if handleFlowControlAndInfra(stepNum, step, inTx, depth, svcName, archSeverity, entityOwners, isDTO, policies, allowCrossService, declaredVars, addWarn, addWarnWithSeverity, isBoundaryViolation, validate) {
				continue
			}

			if isUnknownFlowAction(step.Action) {
				addWarn(stepNum, step.Action, "UNKNOWN_ACTION", fmt.Sprintf("unknown action '%s'", step.Action), "{action: \"repo.Find\" | \"mapping.Assign\" | \"flow.If\" ...}", step.File, step.Line, step.Column)
			}
		}
	}

	validate(steps, false, 0)
	return warnings
}
