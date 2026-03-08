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
	switch step.Action {
	case "tx.Block", "flow.Block":
		if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
			validate(subSteps, step.Action == "tx.Block", depth+1)
		}
		return true

	case "flow.If":
		if subSteps, ok := step.Args["_then"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		if subSteps, ok := step.Args["_else"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "flow.Switch":
		cases, ok := step.Args["_cases"].(map[string][]FlowStep)
		if ok {
			for _, subSteps := range cases {
				validate(subSteps, inTx, depth+1)
			}
		}
		if subSteps, ok := step.Args["_default"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "flow.For", "batch.Run":
		if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
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
		return true

	case "auth.RequireRole":
		if authOutput, _ := step.Args["output"].(string); authOutput != "" {
			declaredVars[authOutput] = true
		} else {
			declaredVars["currentUser"] = true
		}
		return true

	case "auth.CheckRole":
		return true

	case "rbac.CheckPermission":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "entity.PatchNonZero":
		return true

	case "entity.PatchValidated":
		fields, ok := step.Args["fields"].(map[string]map[string]string)
		if !ok || len(fields) == 0 {
			return true
		}
		for _, rules := range fields {
			if uniqueMethod := strings.TrimSpace(rules["unique"]); uniqueMethod != "" {
				if step.Args["source"] == nil || step.Args["source"] == "" {
					addWarn(stepNum, step.Action, "MISSING_SOURCE", "entity.PatchValidated with unique checks requires explicit 'source' repository entity", "{action: \"entity.PatchValidated\", source: \"Company\", ...}", step.File, step.Line, step.Column)
				}
			}
		}
		return true

	case "field.CopyNonEmpty", "list.Paginate", "str.Normalize", "enum.Validate", "list.Sort", "list.Filter", "time.CheckExpiry", "map.Build", "time.Now", "time.Format", "mail.Send", "queue.Enqueue", "queue.Ack", "queue.Nack", "dlq.Publish", "event.Outbox", "webhook.Ack", "webhook.Send", "state.Set", "state.Delete", "idem.Check", "idem.SaveResult", "idempotency.Check", "idempotency.SaveResult", "ratelimit.Check", "concurrency.Limit", "circuit.Check", "circuit.RecordSuccess", "circuit.RecordFailure", "bulkhead.Acquire", "ratelimit.Limit", "log.Emit", "metric.Emit":
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

	case "cache.Get", "cache.Set", "cache.Del", "storage.Upload", "storage.Download", "storage.GetURL", "storage.Delete", "storage.List":
		if output, _ := step.Args["output"].(string); output != "" {
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

	case "http.Call", "http.Request", "http.RetryPolicy":
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
		"jwt.Sign", "jwt.Verify",
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
