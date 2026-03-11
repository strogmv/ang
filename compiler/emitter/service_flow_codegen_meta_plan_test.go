package emitter

import (
	"strings"
	"testing"
)

func TestRenderCueEmitProjectCode_NormalizesServiceContext(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"single_file"`, "projectFiles", ":=", "_files", "_err")
	if !strings.Contains(code, `_serviceName := _contextName(_usecases.ServiceName)`) {
		t.Fatalf("expected service context normalization in generated code, got:\n%s", code)
	}
	if !strings.Contains(code, `owner: \"%s\"\n", _serviceName`) {
		t.Fatalf("expected entities to use normalized service context, got:\n%s", code)
	}
	if !strings.Contains(code, `service:     \"%s\"\n", _serviceName`) {
		t.Fatalf("expected operations to use normalized service context, got:\n%s", code)
	}
}

func TestRenderCueEmitProjectCode_UsesCanonicalSplitLayout(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"split"`, "projectFiles", ":=", "_files", "_err")
	for _, snippet := range []string{
		`files["cue/domain/entities.cue"] = entitiesB.String()`,
		`files["cue/api/http.cue"] = httpB.String()`,
		`files["cue/repo/repositories.cue"] = repoB.String()`,
		`files["cue/infra/handlers.cue"] = infraB.String()`,
		`_opsFileStem := func(kind, entity string, capabilities []string) string`,
		`if strings.TrimSpace(entity) != "" {`,
		`for _, cap := range capabilities {`,
		`if strings.TrimSpace(kind) != "" {`,
		`path := "cue/api/operations_" + group + ".cue"`,
		`sort.Strings(opsOrder)`,
		`if _layout == "single_file" {`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected canonical split layout snippet %q, got:\n%s", snippet, code)
		}
	}
	for _, snippet := range []string{
		`conversationmessage`,
		`_notifyVerb := func`,
	} {
		if strings.Contains(code, snippet) {
			t.Fatalf("expected domain-specific emit snippet %q to be removed, got:\n%s", snippet, code)
		}
	}
}

func TestRenderPlanBuildMicroPlanCode_IsExplicitOnly(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`PrimaryOperationKind string ` + "`json:\"primary_operation_kind\"`",
		`Capabilities []string ` + "`json:\"capabilities\"`",
		`SideEffectsTyped []_metaPlanSideEffect ` + "`json:\"side_effects_typed\"`",
		`ManualRequired bool ` + "`json:\"manual_required\"`",
		`Planner *struct {`,
		`_normalizeSideEffects := func(effects []string) []_metaPlanSideEffect`,
		`_normalizeCapabilities := func(explicit []string, kind, entityName, method string, effects []_metaPlanSideEffect, name string, inputFields, outputFields, entityFields []struct{ Name, Type string }) []string`,
		`_primaryKindOf := func(name, entityName, method string, inputFields, outputFields, entityFields []struct{ Name, Type string }, isTransition bool, explicit string) string`,
		`return explicit`,
		`kind := _primaryKindOf(uc.Name, uc.PrimaryEntity, uc.Method, uc.InputFields, uc.OutputFields, entity.Fields, uc.IsStateTransition, uc.PrimaryOperationKind)`,
		`capabilities := _normalizeCapabilities(uc.Capabilities, kind, uc.PrimaryEntity, uc.Method, typedEffects, uc.Name, uc.InputFields, uc.OutputFields, entity.Fields)`,
		`manualReason = "notify_requires_explicit_flow"`,
		`"primary_operation_kind": kind`,
		`"capabilities": capabilities`,
		`"side_effects": typedEffects`,
		`"planner": planner`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected explicit-contract snippet %q, got:\n%s", snippet, code)
		}
	}
	for _, snippet := range []string{
		`_startsWithAny := func`,
		`_containsAny := func`,
		`_cleanFieldKey := func`,
		`_findFieldByNames := func`,
		`_findActorField := func`,
		`manualReason = "missing_email_recipient"`,
		`notifyStep := map[string]any{"p": "notify_email"`,
		`_isCanonicalAuthFlow := func(`,
		`_isCanonicalProfileUseCase := func(`,
		`_isCanonicalMessagingUseCase := func(`,
	} {
		if strings.Contains(code, snippet) {
			t.Fatalf("expected heuristic snippet %q to be removed, got:\n%s", snippet, code)
		}
	}
}

func TestRenderPlanBuildMicroPlanCode_PrefersExplicitPlannerBindings(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`planner := uc.Planner`,
		`explicitLoadMethod := ""`,
		`explicitListMethod := ""`,
		`explicitActorField := ""`,
		`explicitInputField := ""`,
		`explicitLoadMethod = strings.TrimSpace(planner.Repository.LoadMethod)`,
		`explicitListMethod = strings.TrimSpace(planner.Repository.ListMethod)`,
		`if explicitActorField != "" {`,
		`steps[len(steps)-1]["method"] = explicitLoadMethod`,
		`finder = explicitListMethod`,
		`finderInput = "req." + _pascal(explicitInputField)`,
		`"planner": planner`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected explicit planner snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderPlanBuildMicroPlanCode_RejectsNotifySideEffectsWithoutExplicitFlow(t *testing.T) {
	t.Parallel()

	code := renderPlanBuildMicroPlanCode(&flowRenderState{}, "", "usecasesDoc", "automataDoc", "microPlanDoc", ":=", "_micro", "_err")
	for _, snippet := range []string{
		`case "notify.email":`,
		`return "notify_side_effect_requires_explicit_flow"`,
		`if sideEffectReason := _appendNormalizedSideEffects(&steps, typedEffects, uc.PrimaryEntity, entity.Fields, entityVar, uc.InputFields, uc.OutputFields, uc.Name); sideEffectReason != "" {`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected explicit notify-side-effect rejection snippet %q, got:\n%s", snippet, code)
		}
	}
}

func TestRenderCueEmitProjectCode_UsesExplicitPlannerRouteAndGenericNotifyPath(t *testing.T) {
	t.Parallel()

	code := renderCueEmitProjectCode(&flowRenderState{}, "", "usecasesDoc", "microPlanDoc", `"single_file"`, "projectFiles", ":=", "_files", "_err")
	for _, snippet := range []string{
		`Planner *struct {`,
		`method = _plannerRouteMethod(op.Planner)`,
		`path := _plannerRoutePath(op.Planner)`,
		`if path == "" { path = _httpPath(op.Kind, op.Entity, op.Name) }`,
		`case "notify":`,
		`return base + "/{id}/notify"`,
		`case "notify_email":`,
		`"action: \"notify.Email\""`,
		`"to: " + _cueExprArg(step["to"])`,
		`"text: " + _cueExprArg(step["text"])`,
		`opsB.WriteString("    planner: {\n")`,
		`source_pack: \"%s\"\n", op.Planner.SourcePack`,
		`path: \"%s\"\n", op.Planner.Route.Path`,
	} {
		if !strings.Contains(code, snippet) {
			t.Fatalf("expected planner-aware emit snippet %q, got:\n%s", snippet, code)
		}
	}
	for _, snippet := range []string{
		`_notifyVerb := func`,
		`send-email`,
	} {
		if strings.Contains(code, snippet) {
			t.Fatalf("expected notify verb inference snippet %q to be removed, got:\n%s", snippet, code)
		}
	}
}
