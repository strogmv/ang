package compiler

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

const (
	codePackAuthMissingSelfProfileRoute  = "W_PACK_AUTH_MISSING_SELF_PROFILE_ROUTE"
	codePackModerationMissingTransitions = "E_PACK_MODERATION_MISSING_TRANSITIONS"
	codePackNotifyMissingRecipientSource = "E_PACK_NOTIFY_MISSING_RECIPIENT_SOURCE"
	codePackMissingPlannerHints          = "W_PACK_MISSING_PLANNER_HINTS"
	codePackMissingPlannerRoutePath      = "W_PACK_MISSING_PLANNER_ROUTE_PATH"
	codeIRCanonicalPackMismatch          = "W_IR_CANONICAL_PACK_MISMATCH"
)

func emitCanonicalPackDiagnostics(entities []normalizer.Entity, services []normalizer.Service, endpoints []normalizer.Endpoint, opts PipelineOptions) {
	for _, diag := range collectCanonicalPackDiagnostics(entities, services, endpoints) {
		recordPipelineDiagnostic(diag, opts)
	}
}

func CollectCanonicalPackDiagnostics(entities []normalizer.Entity, services []normalizer.Service, endpoints []normalizer.Endpoint) []normalizer.Warning {
	return collectCanonicalPackDiagnostics(entities, services, endpoints)
}

func collectCanonicalPackDiagnostics(entities []normalizer.Entity, services []normalizer.Service, endpoints []normalizer.Endpoint) []normalizer.Warning {
	var out []normalizer.Warning
	endpointByOp := make(map[string]normalizer.Endpoint, len(endpoints))
	for _, ep := range endpoints {
		endpointByOp[strings.ToLower(strings.TrimSpace(ep.ServiceName)+"."+strings.TrimSpace(ep.RPC))] = ep
	}

	hasAuthPack := false
	firstAuthMethod := normalizer.Method{}
	firstAuthService := ""
	for _, svc := range services {
		for _, method := range svc.Methods {
			if method.PrimaryOperationKind == normalizer.OperationKindAuth || hasCapability(method, normalizer.CapabilityAuth) {
				hasAuthPack = true
				if firstAuthMethod.Name == "" {
					firstAuthMethod = method
					firstAuthService = svc.Name
				}
			}
		}
	}
	if hasAuthPack && !hasSelfProfileRoute(endpoints) {
		file, line := parseSourcePos(firstAuthMethod.Source)
		out = append(out, normalizer.Warning{
			Kind:     "canonical-pack",
			Code:     codePackAuthMissingSelfProfileRoute,
			Severity: "warn",
			Message:  "auth capability is present but no GET self-profile route was found",
			Op:       strings.TrimSpace(firstAuthService + "." + firstAuthMethod.Name),
			File:     file,
			Line:     line,
			Hint:     "Add a self-profile endpoint such as GET /auth/profile or GET /me and map it to GetProfile/GetMyProfile. This stabilizes the canonical auth/profile pack.",
			SuggestedFix: []normalizer.Fix{{
				Op:        "insert",
				File:      file,
				CUEPath:   strings.TrimSpace(firstAuthService + "." + firstAuthMethod.Name),
				Value:     map[string]any{"method": "GET", "path": "/auth/profile", "rpc": "GetProfile"},
				After:     `GET /auth/profile -> GetProfile`,
				Rationale: "canonical auth pack should expose a stable self-profile route",
			}},
		})
	}

	for _, entity := range entities {
		if !isModerationEntity(entity) {
			continue
		}
		if entity.FSM == nil || len(entity.FSM.Transitions) == 0 || !hasModerationStates(entity) {
			file, line := parseSourcePos(entity.Source)
			out = append(out, normalizer.Warning{
				Kind:     "canonical-pack",
				Code:     codePackModerationMissingTransitions,
				Severity: "error",
				Message:  fmt.Sprintf("moderation entity %q is missing canonical moderation transitions", entity.Name),
				File:     file,
				Line:     line,
				Hint:     "Define fsm.states with pending/approved/rejected and add transitions for Approve/Reject flows. Example: pending -> approved|rejected.",
				SuggestedFix: []normalizer.Fix{{
					Op:      "merge",
					File:    file,
					CUEPath: strings.TrimSpace(entity.Name + ".fsm"),
					Value: map[string]any{
						"states":      []string{"pending", "approved", "rejected"},
						"transitions": map[string]any{"pending": []string{"approved", "rejected"}},
					},
					After:     `fsm: { states: ["pending", "approved", "rejected"], transitions: { pending: ["approved", "rejected"] } }`,
					Rationale: "canonical moderation pack requires explicit pending/approved/rejected transitions",
				}},
			})
		}
	}

	for _, svc := range services {
		for _, method := range svc.Methods {
			op := strings.TrimSpace(svc.Name + "." + method.Name)
			if needsPlannerHints(method) && !hasPlannerHints(method) {
				file, line := parseSourcePos(method.Source)
				out = append(out, normalizer.Warning{
					Kind:     "canonical-pack",
					Code:     codePackMissingPlannerHints,
					Severity: "warn",
					Message:  fmt.Sprintf("%s looks like a canonical pack operation but has no explicit planner hints", op),
					Op:       op,
					File:     file,
					Line:     line,
					Hint:     "Add planner.source_pack and planner.route / planner.repository so sandbox owns pack interpretation and ANG only renders explicit intent.",
					SuggestedFix: []normalizer.Fix{{
						Op:      "merge",
						File:    file,
						CUEPath: op,
						Value: map[string]any{
							"planner": map[string]any{
								"source_pack": "custom",
								"route": map[string]any{
									"method": "...",
								},
							},
						},
						After:     `planner: { source_pack: "custom", route: { method: "..." } }`,
						Rationale: "canonical pack operations should carry explicit planner hints instead of relying on compiler heuristics",
					}},
				})
			}
			if requiresPlannerRoutePath(method) && !hasPlannerRoutePath(method) {
				file, line := parseSourcePos(method.Source)
				out = append(out, normalizer.Warning{
					Kind:     "canonical-pack",
					Code:     codePackMissingPlannerRoutePath,
					Severity: "warn",
					Message:  fmt.Sprintf("%s carries planner hints but planner.route.path is empty", op),
					Op:       op,
					File:     file,
					Line:     line,
					Hint:     "Add planner.route.path so sandbox owns canonical routing and ANG only renders explicit intent.",
					SuggestedFix: []normalizer.Fix{{
						Op:      "merge",
						File:    file,
						CUEPath: op,
						Value: map[string]any{
							"planner": map[string]any{
								"route": map[string]any{
									"path": "/...",
								},
							},
						},
						After:     `planner: { route: { path: "/..." } }`,
						Rationale: "canonical pack operations should carry explicit route path in planner metadata",
					}},
				})
			}
			if needsNotifyRecipient(method) && !hasRecipientSource(method) && !flowHasRecipientSource(method.Flow) {
				file, line := parseSourcePos(method.Source)
				out = append(out, normalizer.Warning{
					Kind:     "canonical-pack",
					Code:     codePackNotifyMissingRecipientSource,
					Severity: "error",
					Message:  fmt.Sprintf("%s defines notify side effects but no recipient source can be inferred", op),
					Op:       op,
					File:     file,
					Line:     line,
					Hint:     "Add a recipient source such as req.Email, req.RecipientEmail, requester.Email, or explicit notify.Email/notify.Send to: ... in flow. The notify pack must know who receives the message.",
					SuggestedFix: []normalizer.Fix{{
						Op:      "merge",
						File:    file,
						CUEPath: strings.TrimSpace(op),
						Value: map[string]any{
							"side_effects": []map[string]any{{
								"kind":     "notify.email",
								"template": firstNonEmptyTemplate(method.SideEffects),
								"to":       "req.Email",
							}},
						},
						After:     `side_effects: [{kind: "notify.email", template: "...", to: "req.Email"}]`,
						Rationale: "notify pack needs explicit recipient source",
					}},
				})
			}
			out = append(out, collectIRMismatchDiagnostics(op, method, endpointByOp[strings.ToLower(op)])...)
		}
	}

	return dedupeWarnings(out)
}

func hasPlannerHints(method normalizer.Method) bool {
	if method.Planner == nil {
		return false
	}
	if strings.TrimSpace(method.Planner.SourcePack) != "" {
		return true
	}
	if method.Planner.Route != nil && (strings.TrimSpace(method.Planner.Route.Method) != "" || strings.TrimSpace(method.Planner.Route.Path) != "") {
		return true
	}
	if method.Planner.Repository != nil && (strings.TrimSpace(method.Planner.Repository.LoadMethod) != "" || strings.TrimSpace(method.Planner.Repository.ListMethod) != "" || strings.TrimSpace(method.Planner.Repository.ActorField) != "" || strings.TrimSpace(method.Planner.Repository.InputField) != "") {
		return true
	}
	return false
}

func hasPlannerRoutePath(method normalizer.Method) bool {
	return method.Planner != nil && method.Planner.Route != nil && strings.TrimSpace(method.Planner.Route.Path) != ""
}

func requiresPlannerRoutePath(method normalizer.Method) bool {
	return needsPlannerHints(method) && hasPlannerHints(method)
}

func needsPlannerHints(method normalizer.Method) bool {
	if method.PrimaryOperationKind == normalizer.OperationKindAuth || method.PrimaryOperationKind == normalizer.OperationKindMessage || hasCapability(method, normalizer.CapabilityProfile) || hasCapability(method, normalizer.CapabilityMessaging) {
		return true
	}
	for _, cap := range method.Capabilities {
		switch strings.ToLower(strings.TrimSpace(string(cap))) {
		case "commerce", "catalog", "payment":
			return true
		}
	}
	return false
}

func collectIRMismatchDiagnostics(op string, method normalizer.Method, endpoint normalizer.Endpoint) []normalizer.Warning {
	var out []normalizer.Warning
	file, line := parseSourcePos(method.Source)
	if method.PrimaryOperationKind == normalizer.OperationKindAuth && !hasCapability(method, normalizer.CapabilityAuth) {
		out = append(out, normalizer.Warning{
			Kind:     "canonical-pack",
			Code:     codeIRCanonicalPackMismatch,
			Severity: "warn",
			Message:  fmt.Sprintf("%s is marked as auth operation but is missing auth capability metadata", op),
			Op:       op,
			File:     file,
			Line:     line,
			Hint:     "Add capabilities: [\"auth\"] or correct primary_operation_kind if this is not an auth operation.",
			SuggestedFix: []normalizer.Fix{{
				Op:      "merge",
				File:    file,
				CUEPath: op,
				Value:   map[string]any{"capabilities": []string{"auth"}},
				After:   `capabilities: ["auth"]`,
			}},
		})
	}
	if method.PrimaryOperationKind == normalizer.OperationKindNotify && !hasCapability(method, normalizer.CapabilityNotify) && !needsNotifyRecipient(method) {
		out = append(out, normalizer.Warning{
			Kind:     "canonical-pack",
			Code:     codeIRCanonicalPackMismatch,
			Severity: "warn",
			Message:  fmt.Sprintf("%s is marked as notify operation but notify metadata is incomplete", op),
			Op:       op,
			File:     file,
			Line:     line,
			Hint:     "Add capabilities: [\"notify\"] and a canonical side_effect such as {kind: \"notify.email\", template: \"...\"}, or change primary_operation_kind.",
			SuggestedFix: []normalizer.Fix{{
				Op:      "merge",
				File:    file,
				CUEPath: op,
				Value: map[string]any{
					"capabilities": []string{"notify"},
					"side_effects": []map[string]any{{"kind": "notify.email", "template": "generic_email"}},
				},
				After: `capabilities: ["notify"]`,
			}},
		})
	}
	if hasSideEffect(method, "create_review") && !hasCapability(method, normalizer.CapabilityModeration) {
		out = append(out, normalizer.Warning{
			Kind:     "canonical-pack",
			Code:     codeIRCanonicalPackMismatch,
			Severity: "warn",
			Message:  fmt.Sprintf("%s uses moderation side effects but moderation capability is missing", op),
			Op:       op,
			File:     file,
			Line:     line,
			Hint:     "Add capabilities: [\"moderation\"] so canonical moderation lowering and validation stay aligned.",
			SuggestedFix: []normalizer.Fix{{
				Op:      "merge",
				File:    file,
				CUEPath: op,
				Value:   map[string]any{"capabilities": []string{"moderation"}},
				After:   `capabilities: ["moderation"]`,
			}},
		})
	}
	return out
}

func hasAuthPackSelfProfileEndpoint(ep normalizer.Endpoint) bool {
	if !strings.EqualFold(strings.TrimSpace(ep.Method), "GET") {
		return false
	}
	path := strings.TrimSpace(strings.ToLower(ep.Path))
	if path == "/auth/profile" || path == "/me" || strings.HasSuffix(path, "/me") {
		return true
	}
	if strings.HasSuffix(path, "/profile") && strings.Contains(path, "/auth") {
		return true
	}
	return false
}

func hasSelfProfileRoute(endpoints []normalizer.Endpoint) bool {
	for _, ep := range endpoints {
		if hasAuthPackSelfProfileEndpoint(ep) {
			return true
		}
	}
	return false
}

func isModerationEntity(entity normalizer.Entity) bool {
	name := strings.ToLower(strings.TrimSpace(entity.Name))
	return name == "moderationreview" || name == "report"
}

func hasModerationStates(entity normalizer.Entity) bool {
	if entity.FSM == nil {
		return false
	}
	states := map[string]bool{}
	for _, s := range entity.FSM.States {
		states[strings.ToLower(strings.TrimSpace(s))] = true
	}
	return states["pending"] && states["approved"] && states["rejected"]
}

func needsNotifyRecipient(method normalizer.Method) bool {
	if method.PrimaryOperationKind == normalizer.OperationKindNotify {
		return true
	}
	for _, se := range method.SideEffects {
		kind := strings.TrimSpace(strings.ToLower(se.Kind))
		if kind == "notify.email" || kind == "notify.sms" || kind == "notify_user" {
			return true
		}
	}
	return false
}

func hasRecipientSource(method normalizer.Method) bool {
	for _, f := range method.Input.Fields {
		if isRecipientField(f.Name, method.SideEffects) {
			return true
		}
	}
	for _, f := range method.Output.Fields {
		if isRecipientField(f.Name, method.SideEffects) {
			return true
		}
	}
	for _, src := range method.Sources {
		for _, v := range src.By {
			if isRecipientField(v, method.SideEffects) {
				return true
			}
		}
		for _, v := range src.Filter {
			if isRecipientField(v, method.SideEffects) {
				return true
			}
		}
	}
	return false
}

func flowHasRecipientSource(flow []normalizer.FlowStep) bool {
	for _, step := range flow {
		if (step.Action == "notify.Email" || step.Action == "notify.Send") && step.Args != nil {
			if raw, ok := step.Args["to"]; ok && strings.TrimSpace(fmt.Sprint(raw)) != "" {
				return true
			}
		}
	}
	return false
}

func isRecipientField(name string, sideEffects []normalizer.SideEffect) bool {
	key := strings.ToLower(strings.TrimSpace(name))
	key = strings.TrimPrefix(key, "req.")
	key = strings.TrimPrefix(key, "input.")
	for _, se := range sideEffects {
		if strings.EqualFold(se.Kind, "notify.email") {
			switch key {
			case "email", "recipientemail", "useremail", "requesteremail", "to", "recipient", "owneremail":
				return true
			}
		}
	}
	switch key {
	case "recipient", "recipientid", "userid", "memberid", "requesterid", "ownerid":
		return true
	}
	return false
}

func hasCapability(method normalizer.Method, capability normalizer.CapabilityKind) bool {
	for _, cap := range method.Capabilities {
		if strings.EqualFold(strings.TrimSpace(string(cap)), strings.TrimSpace(string(capability))) {
			return true
		}
	}
	return false
}

func hasSideEffect(method normalizer.Method, kind string) bool {
	for _, se := range method.SideEffects {
		if strings.EqualFold(strings.TrimSpace(se.Kind), strings.TrimSpace(kind)) {
			return true
		}
	}
	return false
}

func dedupeWarnings(in []normalizer.Warning) []normalizer.Warning {
	seen := map[string]struct{}{}
	out := make([]normalizer.Warning, 0, len(in))
	for _, w := range in {
		key := strings.Join([]string{w.Code, w.Message, w.File, fmt.Sprint(w.Line), w.Op}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, w)
	}
	return out
}

func firstNonEmptyTemplate(effects []normalizer.SideEffect) string {
	for _, se := range effects {
		if strings.TrimSpace(se.Template) != "" {
			return strings.TrimSpace(se.Template)
		}
	}
	return "generic_email"
}
