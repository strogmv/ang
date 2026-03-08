package normalizer

import (
	"fmt"
	"strconv"
	"strings"
)

func handleFlowControlAndInfra(
	stepNum int,
	step *FlowStep,
	inTx bool,
	depth int,
	svcName string,
	archSeverity string,
	entityOwners map[string]string,
	isDTO map[string]bool,
	policies map[string]PolicyDef,
	allowCrossService map[string]map[string]struct{},
	declaredVars map[string]bool,
	addWarn flowAddWarnFn,
	addWarnWithSeverity flowAddWarnWithSeverityFn,
	isBoundaryViolation flowBoundaryViolationFn,
	validate flowValidateNestedFn,
) bool {
	hasNonEmptyListArg := func(v any) bool {
		switch vv := v.(type) {
		case []string:
			return len(vv) > 0
		case []any:
			return len(vv) > 0
		case string:
			return strings.TrimSpace(vv) != ""
		default:
			return false
		}
	}

	getStringArg := func(key string) string {
		s, _ := step.Args[key].(string)
		return strings.TrimSpace(s)
	}

	switch step.Action {
	case "tx.Block", "flow.Block":
		subSteps, ok := step.Args["_do"].([]FlowStep)
		if !ok || len(subSteps) == 0 {
			addWarn(stepNum, step.Action, "MISSING_DO", step.Action+" requires non-empty 'do' block", "{action: \""+step.Action+"\", do: [ ... ]}", step.File, step.Line, step.Column)
			return true
		}
		validate(subSteps, step.Action == "tx.Block", depth+1)
		return true

	case "flow.If":
		cond, _ := step.Args["condition"].(string)
		if strings.TrimSpace(cond) == "" {
			addWarn(stepNum, step.Action, "MISSING_CONDITION", "flow.If missing 'condition'", "{action: \"flow.If\", condition: \"req.Enabled\", then: [ ... ]}", step.File, step.Line, step.Column)
		}
		if subSteps, ok := step.Args["_then"].([]FlowStep); ok && len(subSteps) > 0 {
			validate(subSteps, inTx, depth+1)
		} else {
			addWarn(stepNum, step.Action, "MISSING_THEN", "flow.If requires non-empty 'then' block", "{action: \"flow.If\", condition: \"req.Enabled\", then: [ ... ]}", step.File, step.Line, step.Column)
		}
		if subSteps, ok := step.Args["_else"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "flow.Switch":
		value, _ := step.Args["value"].(string)
		if strings.TrimSpace(value) == "" {
			addWarn(stepNum, step.Action, "MISSING_VALUE", "flow.Switch missing 'value'", "{action: \"flow.Switch\", value: \"req.Status\", cases: {active: [ ... ]}}", step.File, step.Line, step.Column)
		}
		cases, ok := step.Args["_cases"].(map[string][]FlowStep)
		if ok && len(cases) > 0 {
			for _, subSteps := range cases {
				validate(subSteps, inTx, depth+1)
			}
		} else {
			addWarn(stepNum, step.Action, "MISSING_CASES", "flow.Switch requires at least one case", "{action: \"flow.Switch\", value: \"req.Status\", cases: {active: [ ... ]}}", step.File, step.Line, step.Column)
		}
		if subSteps, ok := step.Args["_default"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "flow.For", "batch.Run":
		if step.Action == "flow.For" {
			each, _ := step.Args["each"].(string)
			if strings.TrimSpace(each) == "" {
				addWarn(stepNum, step.Action, "MISSING_EACH", "flow.For missing 'each'", "{action: \"flow.For\", each: \"items\", as: \"item\", do: [ ... ]}", step.File, step.Line, step.Column)
			}
			as, _ := step.Args["as"].(string)
			if strings.TrimSpace(as) == "" {
				addWarn(stepNum, step.Action, "MISSING_AS", "flow.For missing 'as'", "{action: \"flow.For\", each: \"items\", as: \"item\", do: [ ... ]}", step.File, step.Line, step.Column)
			}
		}
		subSteps, ok := step.Args["_do"].([]FlowStep)
		if !ok || len(subSteps) == 0 {
			addWarn(stepNum, step.Action, "MISSING_DO", step.Action+" requires non-empty 'do' block", "{action: \""+step.Action+"\", do: [ ... ]}", step.File, step.Line, step.Column)
			return true
		}
		validate(subSteps, inTx, depth+1)
		return true

	case "flow.While":
		cond, _ := step.Args["condition"].(string)
		if strings.TrimSpace(cond) == "" {
			addWarn(stepNum, step.Action, "MISSING_CONDITION", "flow.While missing 'condition'", "{action: \"flow.While\", condition: \"i < 10\", do: [ ... ]}", step.File, step.Line, step.Column)
		}
		subSteps, ok := step.Args["_do"].([]FlowStep)
		if !ok || len(subSteps) == 0 {
			addWarn(stepNum, step.Action, "MISSING_DO", "flow.While requires non-empty 'do' block", "{action: \"flow.While\", condition: \"i < 10\", do: [ ... ]}", step.File, step.Line, step.Column)
			return true
		}
		validate(subSteps, inTx, depth+1)
		return true

	case "flow.Try", "flow.Retry", "flow.Timeout":
		if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		if subSteps, ok := step.Args["_catch"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		if subSteps, ok := step.Args["_onTimeout"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "flow.Catch":
		if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "flow.Fallback":
		if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		if subSteps, ok := step.Args["_fallback"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "flow.Resume":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		if subSteps, ok := step.Args["_onMissing"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "flow.Replay":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		if subSteps, ok := step.Args["_onMismatch"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "flow.Saga", "flow.Compensate":
		if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "flow.Rollback", "flow.Tag":
		return true

	case "flow.SuggestNext", "flow.ExplainError", "flow.RecordEvent", "flow.History.Get":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "approval.Request":
		if approvalID, _ := step.Args["approvalId"].(string); approvalID != "" {
			declaredVars[approvalID] = true
		}
		if status, _ := step.Args["status"].(string); status != "" {
			declaredVars[status] = true
		}
		return true

	case "approval.Wait":
		for _, key := range []string{"decision", "status", "decidedBy", "decidedAt", "reason"} {
			if out, _ := step.Args[key].(string); out != "" {
				declaredVars[out] = true
			}
		}
		if subSteps, ok := step.Args["_onTimeout"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "approval.Decide":
		if status, _ := step.Args["status"].(string); status != "" {
			declaredVars[status] = true
		}
		return true

	case "policy.Check":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		policyName, _ := step.Args["policy"].(string)
		policyName = strings.TrimSpace(policyName)
		if u, err := strconv.Unquote(policyName); err == nil {
			policyName = strings.TrimSpace(u)
		}
		if policyName == "" {
			addWarn(stepNum, step.Action, "MISSING_POLICY", "policy.Check missing 'policy'", "{action: \"policy.Check\", policy: \"CompanyAdminOnly\", user: \"currentUser\", companyID: \"req.CompanyID\"}", step.File, step.Line, step.Column)
			return true
		}
		if len(policies) == 0 {
			addWarn(stepNum, step.Action, "POLICY_REGISTRY_EMPTY", "policy.Check used but #Policies registry is empty (expected cue/policy/*.cue)", "Define #Policies map with typed policies and reload build", step.File, step.Line, step.Column)
			return true
		}
		p, ok := policies[policyName]
		if !ok {
			addWarn(stepNum, step.Action, "UNKNOWN_POLICY", fmt.Sprintf("policy '%s' is not defined in #Policies", policyName), "Allowed: "+strings.Join(sortedPolicyNames(policies), ", "), step.File, step.Line, step.Column)
			return true
		}
		if p.SameCompany {
			companyExpr, _ := step.Args["companyID"].(string)
			if strings.TrimSpace(companyExpr) == "" {
				addWarn(stepNum, step.Action, "MISSING_COMPANY_ID", fmt.Sprintf("policy '%s' requires companyID (sameCompany=true)", policyName), "{action: \"policy.Check\", policy: \""+policyName+"\", user: \"currentUser\", companyID: \"req.CompanyID\"}", step.File, step.Line, step.Column)
			}
		}
		return true

	case "policy.Evaluate", "policy.Require", "policy.Decide":
		for _, key := range []string{"decision", "reason", "effects", "output"} {
			if out, _ := step.Args[key].(string); out != "" {
				declaredVars[out] = true
			}
		}
		return true

	case "audit.Log":
		if getStringArg("actor") == "" {
			addWarn(stepNum, step.Action, "MISSING_ACTOR", "audit.Log missing 'actor'", "{action: \"audit.Log\", actor: \"req.UserID\", company: \"req.CompanyID\", event: \"tender.created\"}", step.File, step.Line, step.Column)
		}
		if getStringArg("company") == "" {
			addWarn(stepNum, step.Action, "MISSING_COMPANY", "audit.Log missing 'company'", "{action: \"audit.Log\", actor: \"req.UserID\", company: \"req.CompanyID\", event: \"tender.created\"}", step.File, step.Line, step.Column)
		}
		if getStringArg("event") == "" {
			addWarn(stepNum, step.Action, "MISSING_EVENT", "audit.Log missing 'event'", "{action: \"audit.Log\", actor: \"req.UserID\", company: \"req.CompanyID\", event: \"tender.created\"}", step.File, step.Line, step.Column)
		}
		return true

	case "auth.RequireRole":
		if getStringArg("userID") == "" {
			addWarn(stepNum, step.Action, "MISSING_USER_ID", "auth.RequireRole missing 'userID'", "{action: \"auth.RequireRole\", userID: \"req.UserID\", companyID: \"req.CompanyID\", roles: [\"owner\",\"admin\"]}", step.File, step.Line, step.Column)
		}
		if getStringArg("companyID") == "" {
			addWarn(stepNum, step.Action, "MISSING_COMPANY_ID", "auth.RequireRole missing 'companyID'", "{action: \"auth.RequireRole\", userID: \"req.UserID\", companyID: \"req.CompanyID\", roles: [\"owner\",\"admin\"]}", step.File, step.Line, step.Column)
		}
		if !hasNonEmptyListArg(step.Args["roles"]) {
			addWarn(stepNum, step.Action, "MISSING_ROLES", "auth.RequireRole missing 'roles'", "{action: \"auth.RequireRole\", userID: \"req.UserID\", companyID: \"req.CompanyID\", roles: [\"owner\",\"admin\"]}", step.File, step.Line, step.Column)
		}
		if authOutput, _ := step.Args["output"].(string); authOutput != "" {
			declaredVars[authOutput] = true
		} else {
			declaredVars["currentUser"] = true
		}
		return true

	case "auth.CheckRole":
		if getStringArg("user") == "" {
			addWarn(stepNum, step.Action, "MISSING_USER", "auth.CheckRole missing 'user'", "{action: \"auth.CheckRole\", user: \"currentUser\", roles: [\"owner\",\"admin\"]}", step.File, step.Line, step.Column)
		}
		if !hasNonEmptyListArg(step.Args["roles"]) {
			addWarn(stepNum, step.Action, "MISSING_ROLES", "auth.CheckRole missing 'roles'", "{action: \"auth.CheckRole\", user: \"currentUser\", roles: [\"owner\",\"admin\"]}", step.File, step.Line, step.Column)
		}
		return true

	case "rbac.CheckPermission":
		if getStringArg("user") == "" {
			addWarn(stepNum, step.Action, "MISSING_USER", "rbac.CheckPermission missing 'user'", "{action: \"rbac.CheckPermission\", user: \"currentUser\", permission: \"tender:create\", output: \"allowed\"}", step.File, step.Line, step.Column)
		}
		if getStringArg("permission") == "" {
			addWarn(stepNum, step.Action, "MISSING_PERMISSION", "rbac.CheckPermission missing 'permission'", "{action: \"rbac.CheckPermission\", user: \"currentUser\", permission: \"tender:create\", output: \"allowed\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "entity.PatchNonZero":
		if getStringArg("target") == "" {
			addWarn(stepNum, step.Action, "MISSING_TARGET", "entity.PatchNonZero missing 'target'", "{action: \"entity.PatchNonZero\", target: \"item\", from: \"req\", fields: [\"Title\",\"Description\"]}", step.File, step.Line, step.Column)
		}
		if getStringArg("from") == "" {
			addWarn(stepNum, step.Action, "MISSING_FROM", "entity.PatchNonZero missing 'from'", "{action: \"entity.PatchNonZero\", target: \"item\", from: \"req\", fields: [\"Title\",\"Description\"]}", step.File, step.Line, step.Column)
		}
		if !hasNonEmptyListArg(step.Args["fields"]) {
			addWarn(stepNum, step.Action, "MISSING_FIELDS", "entity.PatchNonZero missing 'fields'", "{action: \"entity.PatchNonZero\", target: \"item\", from: \"req\", fields: [\"Title\",\"Description\"]}", step.File, step.Line, step.Column)
		}
		return true

	case "entity.PatchValidated":
		if getStringArg("target") == "" {
			addWarn(stepNum, step.Action, "MISSING_TARGET", "entity.PatchValidated missing 'target'", "{action: \"entity.PatchValidated\", target: \"item\", from: \"req\", fields: {Title: {required: \"true\"}}}", step.File, step.Line, step.Column)
		}
		if getStringArg("from") == "" {
			addWarn(stepNum, step.Action, "MISSING_FROM", "entity.PatchValidated missing 'from'", "{action: \"entity.PatchValidated\", target: \"item\", from: \"req\", fields: {Title: {required: \"true\"}}}", step.File, step.Line, step.Column)
		}
		if step.Args["fields"] == nil {
			addWarn(stepNum, step.Action, "MISSING_FIELDS", "entity.PatchValidated missing 'fields'", "{action: \"entity.PatchValidated\", target: \"item\", from: \"req\", fields: {Title: {required: \"true\"}}}", step.File, step.Line, step.Column)
			return true
		}
		fields, ok := step.Args["fields"].(map[string]map[string]string)
		if !ok || len(fields) == 0 {
			addWarn(stepNum, step.Action, "MISSING_FIELDS", "entity.PatchValidated requires non-empty 'fields' map", "{action: \"entity.PatchValidated\", target: \"item\", from: \"req\", fields: {Title: {required: \"true\"}}}", step.File, step.Line, step.Column)
			return true
		}
		for fieldName, rules := range fields {
			if strings.TrimSpace(fieldName) == "" || len(rules) == 0 {
				addWarn(stepNum, step.Action, "INVALID_FIELDS", "entity.PatchValidated has invalid field rules in 'fields'", "Provide map[fieldName]map[rule]value with non-empty field names and rule sets", step.File, step.Line, step.Column)
				continue
			}
			if uniqueMethod := strings.TrimSpace(rules["unique"]); uniqueMethod != "" {
				if step.Args["source"] == nil || step.Args["source"] == "" {
					addWarn(stepNum, step.Action, "MISSING_SOURCE", "entity.PatchValidated with unique checks requires explicit 'source' repository entity", "{action: \"entity.PatchValidated\", source: \"Company\", ...}", step.File, step.Line, step.Column)
				}
			}
		}
		return true

	case "enum.Validate":
		if getStringArg("value") == "" {
			addWarn(stepNum, step.Action, "MISSING_VALUE", "enum.Validate missing 'value'", "{action: \"enum.Validate\", value: \"req.Status\", allowed: [\"draft\",\"active\"], throw: \"invalid status\"}", step.File, step.Line, step.Column)
		}
		if !hasNonEmptyListArg(step.Args["allowed"]) {
			addWarn(stepNum, step.Action, "MISSING_ALLOWED", "enum.Validate missing 'allowed'", "{action: \"enum.Validate\", value: \"req.Status\", allowed: [\"draft\",\"active\"], throw: \"invalid status\"}", step.File, step.Line, step.Column)
		}
		if getStringArg("throw") == "" {
			addWarn(stepNum, step.Action, "MISSING_THROW", "enum.Validate missing 'throw'", "{action: \"enum.Validate\", value: \"req.Status\", allowed: [\"draft\",\"active\"], throw: \"invalid status\"}", step.File, step.Line, step.Column)
		}
		return true

	case "field.CopyNonEmpty", "str.Normalize", "time.CheckExpiry", "map.Build", "time.Now", "time.Format", "mail.Send", "queue.Enqueue", "queue.Ack", "queue.Nack", "dlq.Publish", "event.Outbox", "webhook.Ack", "webhook.Send", "state.Set", "state.Delete", "idem.Check", "idem.SaveResult", "idempotency.Check", "idempotency.SaveResult", "ratelimit.Check", "concurrency.Limit", "circuit.Check", "circuit.RecordSuccess", "circuit.RecordFailure", "bulkhead.Acquire", "ratelimit.Limit", "log.Emit", "metric.Emit":
		return true

	case "list.Filter":
		if step.Args["from"] == nil || step.Args["from"] == "" {
			addWarn(stepNum, step.Action, "MISSING_FROM", "list.Filter missing 'from'", "{action: \"list.Filter\", from: \"items\", condition: \"item.Active\", output: \"filtered\"}", step.File, step.Line, step.Column)
		}
		if step.Args["condition"] == nil || step.Args["condition"] == "" {
			addWarn(stepNum, step.Action, "MISSING_CONDITION", "list.Filter missing 'condition'", "{action: \"list.Filter\", from: \"items\", condition: \"item.Active\", output: \"filtered\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); strings.TrimSpace(output) == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", "list.Filter missing 'output'", "{action: \"list.Filter\", from: \"items\", condition: \"item.Active\", output: \"filtered\"}", step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true

	case "list.Sort":
		if step.Args["items"] == nil || step.Args["items"] == "" {
			addWarn(stepNum, step.Action, "MISSING_ITEMS", "list.Sort missing 'items'", "{action: \"list.Sort\", items: \"items\", by: \"CreatedAt\", order: \"desc\"}", step.File, step.Line, step.Column)
		}
		if step.Args["by"] == nil || step.Args["by"] == "" {
			addWarn(stepNum, step.Action, "MISSING_BY", "list.Sort missing 'by'", "{action: \"list.Sort\", items: \"items\", by: \"CreatedAt\", order: \"desc\"}", step.File, step.Line, step.Column)
		}
		return true

	case "list.Paginate":
		if step.Args["input"] == nil || step.Args["input"] == "" {
			addWarn(stepNum, step.Action, "MISSING_INPUT", "list.Paginate missing 'input'", "{action: \"list.Paginate\", input: \"items\", offset: \"req.Offset\", limit: \"req.Limit\", output: \"page\"}", step.File, step.Line, step.Column)
		}
		if step.Args["offset"] == nil || step.Args["offset"] == "" {
			addWarn(stepNum, step.Action, "MISSING_OFFSET", "list.Paginate missing 'offset'", "{action: \"list.Paginate\", input: \"items\", offset: \"req.Offset\", limit: \"req.Limit\", output: \"page\"}", step.File, step.Line, step.Column)
		}
		if step.Args["limit"] == nil || step.Args["limit"] == "" {
			addWarn(stepNum, step.Action, "MISSING_LIMIT", "list.Paginate missing 'limit'", "{action: \"list.Paginate\", input: \"items\", offset: \"req.Offset\", limit: \"req.Limit\", output: \"page\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); strings.TrimSpace(output) == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", "list.Paginate missing 'output'", "{action: \"list.Paginate\", input: \"items\", offset: \"req.Offset\", limit: \"req.Limit\", output: \"page\"}", step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true

	case "time.Parse":
		value, _ := step.Args["value"].(string)
		input, _ := step.Args["input"].(string)
		if strings.TrimSpace(value) == "" && strings.TrimSpace(input) == "" {
			addWarn(stepNum, step.Action, "MISSING_VALUE", "time.Parse missing 'value' (or legacy 'input')", "{action: \"time.Parse\", value: \"req.StartsAt\", output: \"parsed\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); strings.TrimSpace(output) == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", "time.Parse missing 'output'", "{action: \"time.Parse\", value: \"req.StartsAt\", output: \"parsed\"}", step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true

	case "list.Len", "list.New", "map.New", "list.Map", "list.Reduce", "list.GroupBy", "list.Distinct", "list.Chunk":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "list.Enrich":
		if step.Args["items"] == nil || step.Args["items"] == "" {
			addWarn(stepNum, step.Action, "MISSING_ITEMS", "list.Enrich missing 'items'", "{action: \"list.Enrich\", items: \"items\", lookupSource: \"Company\", lookupInput: \"item.CompanyID\", set: \"Name=Name\"}", step.File, step.Line, step.Column)
		}
		if step.Args["lookupSource"] == nil || step.Args["lookupSource"] == "" {
			addWarn(stepNum, step.Action, "MISSING_LOOKUP_SOURCE", "list.Enrich missing 'lookupSource'", "{action: \"list.Enrich\", items: \"items\", lookupSource: \"Company\", lookupInput: \"item.CompanyID\", set: \"Name=Name\"}", step.File, step.Line, step.Column)
		}
		if step.Args["lookupInput"] == nil || step.Args["lookupInput"] == "" {
			addWarn(stepNum, step.Action, "MISSING_LOOKUP_INPUT", "list.Enrich missing 'lookupInput'", "{action: \"list.Enrich\", items: \"items\", lookupSource: \"Company\", lookupInput: \"item.CompanyID\", set: \"Name=Name\"}", step.File, step.Line, step.Column)
		}
		if step.Args["set"] == nil || step.Args["set"] == "" {
			addWarn(stepNum, step.Action, "MISSING_SET", "list.Enrich missing 'set'", "{action: \"list.Enrich\", items: \"items\", lookupSource: \"Company\", lookupInput: \"item.CompanyID\", set: \"Name=Name\"}", step.File, step.Line, step.Column)
		}
		lookupSource, _ := step.Args["lookupSource"].(string)
		if lookupSource != "" {
			_, ok := entityOwners[lookupSource]
			if !ok {
				addWarn(stepNum, step.Action, "UNKNOWN_ENTITY", fmt.Sprintf("Entity '%s' is not defined in any domain CUE file", lookupSource), "Define the entity in cue/domain/ or check spelling", step.File, step.Line, step.Column)
			} else if isDTO[lookupSource] {
				addWarn(stepNum, step.Action, "DTO_AS_REPO", fmt.Sprintf("Entity '%s' is a DTO-only entity and cannot be accessed via repository", lookupSource), "Remove @dto(only=true) or use a real domain entity", step.File, step.Line, step.Column)
			}
			if violation, reason, targetService := isBoundaryViolation(lookupSource); ok && violation {
				if !isCrossServiceAllowed(allowCrossService, svcName, lookupSource) {
					hintTarget := strings.TrimSpace(targetService)
					if hintTarget == "" {
						hintTarget = "target"
					}
					addWarnWithSeverity(stepNum, step.Action, "ARCHITECTURE_VIOLATION", archSeverity, fmt.Sprintf("Service '%s' is not allowed to directly access entity '%s' (%s)", svcName, lookupSource, reason), fmt.Sprintf("Use events or call %sService", strings.Title(hintTarget)), step.File, step.Line, step.Column)
				}
			}
		}
		return true

	case "exec.Run":
		if step.Args["cmd"] == nil || step.Args["cmd"] == "" {
			addWarn(stepNum, step.Action, "MISSING_CMD", "exec.Run missing 'cmd'", "{action: \"exec.Run\", cmd: \"/usr/bin/ang\", args: [\"build\"], output: \"result\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "fs.TempDir":
		if output, _ := step.Args["output"].(string); output == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", "fs.TempDir missing 'output'", "{action: \"fs.TempDir\", output: \"workDir\", pattern: \"sendbox-*\"}", step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true

	case "fs.WriteFile":
		if step.Args["path"] == nil || step.Args["path"] == "" {
			addWarn(stepNum, step.Action, "MISSING_PATH", "fs.WriteFile missing 'path'", "{action: \"fs.WriteFile\", path: \"filePath\", data: \"req.Content\"}", step.File, step.Line, step.Column)
		}
		if step.Args["data"] == nil || step.Args["data"] == "" {
			addWarn(stepNum, step.Action, "MISSING_DATA", "fs.WriteFile missing 'data'", "{action: \"fs.WriteFile\", path: \"filePath\", data: \"req.Content\"}", step.File, step.Line, step.Column)
		}
		return true

	case "fs.ReadFile":
		if step.Args["path"] == nil || step.Args["path"] == "" {
			addWarn(stepNum, step.Action, "MISSING_PATH", "fs.ReadFile missing 'path'", "{action: \"fs.ReadFile\", path: \"filePath\", output: \"contents\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); output == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", "fs.ReadFile missing 'output'", "{action: \"fs.ReadFile\", path: \"filePath\", output: \"contents\"}", step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true

	case "fs.Remove":
		if step.Args["path"] == nil || step.Args["path"] == "" {
			addWarn(stepNum, step.Action, "MISSING_PATH", "fs.Remove missing 'path'", "{action: \"fs.Remove\", path: \"workDir\"}", step.File, step.Line, step.Column)
		}
		return true

	case "cache.Get":
		if step.Args["key"] == nil || step.Args["key"] == "" {
			addWarn(stepNum, step.Action, "MISSING_KEY", "cache.Get missing 'key'", "{action: \"cache.Get\", key: \"cacheKey\", output: \"cached\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); strings.TrimSpace(output) == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", "cache.Get missing 'output'", "{action: \"cache.Get\", key: \"cacheKey\", output: \"cached\"}", step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true

	case "cache.Set":
		if step.Args["key"] == nil || step.Args["key"] == "" {
			addWarn(stepNum, step.Action, "MISSING_KEY", "cache.Set missing 'key'", "{action: \"cache.Set\", key: \"cacheKey\", value: \"payload\", ttl: \"5m\"}", step.File, step.Line, step.Column)
		}
		if step.Args["value"] == nil || step.Args["value"] == "" {
			addWarn(stepNum, step.Action, "MISSING_VALUE", "cache.Set missing 'value'", "{action: \"cache.Set\", key: \"cacheKey\", value: \"payload\", ttl: \"5m\"}", step.File, step.Line, step.Column)
		}
		return true

	case "cache.Del", "storage.Download", "storage.GetURL", "storage.Delete", "storage.List":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "storage.Upload":
		if step.Args["key"] == nil || step.Args["key"] == "" {
			addWarn(stepNum, step.Action, "MISSING_KEY", "storage.Upload missing 'key'", "{action: \"storage.Upload\", key: \"attachments/file.bin\", data: \"req.Content\", output: \"url\"}", step.File, step.Line, step.Column)
		}
		dataArg, _ := step.Args["data"].(string)
		inputArg, _ := step.Args["input"].(string)
		if strings.TrimSpace(dataArg) == "" && strings.TrimSpace(inputArg) == "" {
			addWarn(stepNum, step.Action, "MISSING_DATA", "storage.Upload missing 'data' (or legacy 'input')", "{action: \"storage.Upload\", key: \"attachments/file.bin\", data: \"req.Content\", output: \"url\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); strings.TrimSpace(output) != "" {
			declaredVars[output] = true
		}
		return true

	case "queue.Dequeue":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		if ackToken, _ := step.Args["ackToken"].(string); ackToken != "" {
			declaredVars[ackToken] = true
		}
		return true

	case "webhook.VerifySignature":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "http.Call":
		if step.Args["method"] == nil || step.Args["method"] == "" {
			addWarn(stepNum, step.Action, "MISSING_METHOD", "http.Call missing 'method'", "{action: \"http.Call\", method: \"GET\", url: \"https://api.example.com\", output: \"body\"}", step.File, step.Line, step.Column)
		}
		if step.Args["url"] == nil || step.Args["url"] == "" {
			addWarn(stepNum, step.Action, "MISSING_URL", "http.Call missing 'url'", "{action: \"http.Call\", method: \"GET\", url: \"https://api.example.com\", output: \"body\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		if statusVar, _ := step.Args["statusVar"].(string); statusVar != "" {
			declaredVars[statusVar] = true
		}
		return true

	case "http.Request", "http.RetryPolicy":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		if statusVar, _ := step.Args["statusVar"].(string); statusVar != "" {
			declaredVars[statusVar] = true
		}
		return true

	case "http.Paginate":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "jwt.Sign":
		if step.Args["claims"] == nil || step.Args["claims"] == "" {
			addWarn(stepNum, step.Action, "MISSING_CLAIMS", "jwt.Sign missing 'claims'", "{action: \"jwt.Sign\", claims: \"map[string]any{\\\"sub\\\": req.UserID}\", output: \"token\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); strings.TrimSpace(output) == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", "jwt.Sign missing 'output'", "{action: \"jwt.Sign\", claims: \"map[string]any{\\\"sub\\\": req.UserID}\", output: \"token\"}", step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true

	case "jwt.Verify":
		if step.Args["token"] == nil || step.Args["token"] == "" {
			addWarn(stepNum, step.Action, "MISSING_TOKEN", "jwt.Verify missing 'token'", "{action: \"jwt.Verify\", token: \"req.Token\", output: \"claims\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); strings.TrimSpace(output) == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", "jwt.Verify missing 'output'", "{action: \"jwt.Verify\", token: \"req.Token\", output: \"claims\"}", step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true

	case "crypto.Hash":
		if step.Args["input"] == nil || step.Args["input"] == "" {
			addWarn(stepNum, step.Action, "MISSING_INPUT", "crypto.Hash missing 'input'", "{action: \"crypto.Hash\", input: \"req.Password\", output: \"hash\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); strings.TrimSpace(output) == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", "crypto.Hash missing 'output'", "{action: \"crypto.Hash\", input: \"req.Password\", output: \"hash\"}", step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true

	case "rand.Code", "rand.Token",
		"uuid.New", "ulid.New",
		"regex.Match", "regex.Replace",
		"base64.Encode", "base64.Decode",
		"url.Parse", "url.Build",
		"query.Encode", "query.Decode",
		"hash.Sum", "hash.HMAC",
		"str.Format", "str.Concat",
		"cast.ToString",
		"json.Parse", "json.Marshal",
		"math.Op",
		"num.Add", "num.Sub", "num.Mul", "num.Div",
		"jsonpath.Get", "jsonpath.Set",
		"oauth2.Token", "oauth2.Refresh",
		"crypto.Encrypt", "crypto.Decrypt":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "parallel.Run":
		if branches, ok := step.Args["_branches"].(map[string][]FlowStep); ok {
			for _, branchSteps := range branches {
				validate(branchSteps, inTx, depth+1)
			}
		}
		return true

	case "state.Get":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "idem.DeriveKey", "idempotency.DeriveKey":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "dedupe.Once", "concurrency.Run", "bulkhead.Run", "circuit.Breaker", "trace.Span", "slo.Budget":
		if doSteps, ok := step.Args["_do"].([]FlowStep); ok {
			validate(doSteps, inTx, depth+1)
		}
		return true

	case "secret.Get", "config.Get":
		if step.Args["key"] == nil || step.Args["key"] == "" {
			addWarn(stepNum, step.Action, "MISSING_KEY", fmt.Sprintf("%s missing 'key'", step.Action), fmt.Sprintf("{action: \"%s\", key: \"KEY_NAME\", output: \"val\"}", step.Action), step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); output == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", fmt.Sprintf("%s missing 'output'", step.Action), fmt.Sprintf("{action: \"%s\", key: \"KEY_NAME\", output: \"val\"}", step.Action), step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true
	}

	return false
}

func isUnknownFlowAction(action string) bool {
	if action == "" {
		return false
	}
	if strings.HasPrefix(action, "repo.") ||
		strings.HasPrefix(action, "mapping.") ||
		strings.HasPrefix(action, "logic.") ||
		strings.HasPrefix(action, "event.") ||
		strings.HasPrefix(action, "fsm.") ||
		strings.HasPrefix(action, "flow.") ||
		strings.HasPrefix(action, "tx.") ||
		strings.HasPrefix(action, "list.") ||
		strings.HasPrefix(action, "notification.") ||
		strings.HasPrefix(action, "notify.") ||
		strings.HasPrefix(action, "approval.") ||
		strings.HasPrefix(action, "policy.") ||
		strings.HasPrefix(action, "audit.") ||
		strings.HasPrefix(action, "auth.") ||
		strings.HasPrefix(action, "entity.") ||
		strings.HasPrefix(action, "field.") ||
		strings.HasPrefix(action, "str.") ||
		strings.HasPrefix(action, "enum.") ||
		strings.HasPrefix(action, "time.") ||
		strings.HasPrefix(action, "map.") ||
		strings.HasPrefix(action, "exec.") ||
		strings.HasPrefix(action, "fs.") ||
		strings.HasPrefix(action, "cache.") ||
		strings.HasPrefix(action, "mail.") ||
		strings.HasPrefix(action, "storage.") ||
		strings.HasPrefix(action, "http.") ||
		strings.HasPrefix(action, "webhook.") ||
		strings.HasPrefix(action, "queue.") ||
		strings.HasPrefix(action, "rand.") ||
		strings.HasPrefix(action, "json.") ||
		strings.HasPrefix(action, "regex.") ||
		strings.HasPrefix(action, "base64.") ||
		strings.HasPrefix(action, "url.") ||
		strings.HasPrefix(action, "query.") ||
		strings.HasPrefix(action, "hash.") ||
		strings.HasPrefix(action, "uuid.") ||
		strings.HasPrefix(action, "ulid.") ||
		strings.HasPrefix(action, "math.") ||
		strings.HasPrefix(action, "jsonpath.") ||
		strings.HasPrefix(action, "jwt.") ||
		strings.HasPrefix(action, "oauth2.") ||
		strings.HasPrefix(action, "crypto.") ||
		strings.HasPrefix(action, "rbac.") ||
		strings.HasPrefix(action, "batch.") ||
		strings.HasPrefix(action, "parallel.") ||
		strings.HasPrefix(action, "archive.") ||
		strings.HasPrefix(action, "session.") ||
		strings.HasPrefix(action, "idem.") ||
		strings.HasPrefix(action, "idempotency.") ||
		strings.HasPrefix(action, "dedupe.") ||
		strings.HasPrefix(action, "ratelimit.") ||
		strings.HasPrefix(action, "concurrency.") ||
		strings.HasPrefix(action, "circuit.") ||
		strings.HasPrefix(action, "bulkhead.") ||
		strings.HasPrefix(action, "log.") ||
		strings.HasPrefix(action, "metric.") ||
		strings.HasPrefix(action, "trace.") ||
		strings.HasPrefix(action, "slo.") ||
		strings.HasPrefix(action, "state.") ||
		strings.HasPrefix(action, "dlq.") ||
		strings.HasPrefix(action, "db.") {
		return false
	}
	return true
}
