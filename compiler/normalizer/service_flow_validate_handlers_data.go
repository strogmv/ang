package normalizer

import (
	"fmt"
	"path/filepath"
	"strings"
)

type flowBoundaryViolationFn func(entityName string) (bool, string, string)
type flowAddWarnFn func(step int, action, code, message, hint string, file string, line int, column int, fixes ...Fix)
type flowAddWarnWithSeverityFn func(step int, action, code, severity, message, hint string, file string, line int, column int, fixes ...Fix)
type flowValidateNestedFn func(steps []FlowStep, inTx bool, depth int)

func handleFlowDataAndCalls(
	stepNum int,
	step *FlowStep,
	inTx bool,
	depth int,
	svcName string,
	serviceContext string,
	archSeverity string,
	entityOwners map[string]string,
	isDTO map[string]bool,
	allowedDeps map[string]struct{},
	allowCrossService map[string]map[string]struct{},
	declaredVars map[string]bool,
	assignedFields map[string]bool,
	newEntities map[string]string,
	isBoundaryViolation flowBoundaryViolationFn,
	addWarn flowAddWarnFn,
	addWarnWithSeverity flowAddWarnWithSeverityFn,
	validate flowValidateNestedFn,
) bool {
	switch step.Action {
	case "repo.Find", "repo.Get", "repo.GetForUpdate", "repo.Save", "repo.Delete", "repo.List", "repo.Query", "repo.Upsert",
		"db.Get", "db.List", "db.Query", "db.Insert", "db.Update", "db.Upsert", "db.Delete", "db.Lock", "db.SelectForUpdate":
		if strings.HasPrefix(step.Action, "repo.") {
			if step.Args["source"] == nil || step.Args["source"] == "" {
				addWarn(stepNum, step.Action, "MISSING_SOURCE", step.Action+" missing 'source'", "{action: \""+step.Action+"\", source: \"Entity\", ...}", step.File, step.Line, step.Column)
			}
		}
		switch step.Action {
		case "repo.Find", "repo.Get", "repo.GetForUpdate":
			if step.Args["input"] == nil || step.Args["input"] == "" {
				addWarn(stepNum, step.Action, "MISSING_INPUT", step.Action+" missing 'input'", "{action: \""+step.Action+"\", source: \"Entity\", input: \"req.ID\", output: \"item\"}", step.File, step.Line, step.Column)
			}
			if step.Args["output"] == nil || step.Args["output"] == "" {
				addWarn(stepNum, step.Action, "MISSING_OUTPUT", step.Action+" missing 'output'", "{action: \""+step.Action+"\", source: \"Entity\", input: \"req.ID\", output: \"item\"}", step.File, step.Line, step.Column)
			}
		case "repo.Save", "repo.Delete":
			if step.Args["input"] == nil || step.Args["input"] == "" {
				addWarn(stepNum, step.Action, "MISSING_INPUT", step.Action+" missing 'input'", "{action: \""+step.Action+"\", source: \"Entity\", input: \"item\"}", step.File, step.Line, step.Column)
			}
		case "repo.List":
			if step.Args["output"] == nil || step.Args["output"] == "" {
				addWarn(stepNum, step.Action, "MISSING_OUTPUT", step.Action+" missing 'output'", "{action: \"repo.List\", source: \"Entity\", output: \"items\"}", step.File, step.Line, step.Column)
			}
		case "repo.Query":
			if step.Args["method"] == nil || step.Args["method"] == "" {
				addWarn(stepNum, step.Action, "MISSING_METHOD", step.Action+" missing 'method'", "{action: \"repo.Query\", source: \"Entity\", method: \"ListBy...\", output: \"items\"}", step.File, step.Line, step.Column)
			}
			if step.Args["output"] == nil || step.Args["output"] == "" {
				addWarn(stepNum, step.Action, "MISSING_OUTPUT", step.Action+" missing 'output'", "{action: \"repo.Query\", source: \"Entity\", method: \"ListBy...\", output: \"items\"}", step.File, step.Line, step.Column)
			}
		}
		source, _ := step.Args["source"].(string)
		if source != "" {
			_, ok := entityOwners[source]

			if !ok {
				addWarn(stepNum, step.Action, "UNKNOWN_ENTITY",
					fmt.Sprintf("Entity '%s' is not defined in any domain CUE file", source),
					"Define the entity in cue/domain/ or check spelling", step.File, step.Line, step.Column)
			} else if isDTO[source] {
				addWarn(stepNum, step.Action, "DTO_AS_REPO",
					fmt.Sprintf("Entity '%s' is a DTO-only entity and cannot be accessed via repository", source),
					"Remove @dto(only=true) or use a real domain entity", step.File, step.Line, step.Column)
			}

			if violation, reason, targetService := isBoundaryViolation(source); ok && violation {
				filePath := filepath.ToSlash(strings.TrimSpace(step.File))
				if strings.Contains(filePath, "/cue/schema/") || strings.HasPrefix(filePath, "cue/schema/") {
					return true
				}
				if !isCrossServiceAllowed(allowCrossService, svcName, source) {
					hintTarget := strings.TrimSpace(targetService)
					if hintTarget == "" {
						hintTarget = "target"
					}
					targetCtx := strings.TrimSpace(serviceContext)
					if targetCtx == "" {
						targetCtx = strings.TrimSpace(strings.ToLower(normalizeServiceName(svcName)))
					}
					addWarnWithSeverity(stepNum, step.Action, "ARCHITECTURE_VIOLATION", archSeverity,
						fmt.Sprintf("Service '%s' is not allowed to directly access entity '%s' (%s)", svcName, source, reason),
						fmt.Sprintf("Define a #ReadModel in '%s' bounded context with read_model.source_context='%s' and read_model.refreshOn=[...events], then read that model", targetCtx, hintTarget), step.File, step.Line, step.Column)
				}
			}
		}

		if strings.HasPrefix(step.Action, "repo.Find") || strings.HasPrefix(step.Action, "repo.Get") || step.Action == "repo.Upsert" ||
			step.Action == "db.Get" || step.Action == "db.Lock" || step.Action == "db.SelectForUpdate" || step.Action == "db.Upsert" {
			output, _ := step.Args["output"].(string)
			if output != "" {
				declaredVars[output] = true
			}
		}
		if step.Action == "repo.Get" {
			output, _ := step.Args["output"].(string)
			errMsg, _ := step.Args["error"].(string)
			if strings.TrimSpace(output) != "" && strings.TrimSpace(errMsg) == "" {
				addWarn(stepNum, step.Action, "REPO_GET_MISSING_ERROR", "repo.Get with output requires 'error' to guard not-found nil result", `{action: "repo.Get", source: "Entity", input: "req.ID", output: "item", error: "Not found"}`, step.File, step.Line, step.Column)
			}
		}
		if step.Action == "repo.Find" {
			output, _ := step.Args["output"].(string)
			errMsg, _ := step.Args["error"].(string)
			if strings.TrimSpace(output) != "" && strings.TrimSpace(errMsg) == "" {
				addWarnWithSeverity(stepNum, step.Action, "REPO_FIND_WITHOUT_ERROR", "warn", "repo.Find without 'error' returns nil when not found; add explicit nil-check", `{action: "flow.If", condition: "item != nil", then: [ ... ]}`, step.File, step.Line, step.Column)
			}
		}
		if step.Action == "repo.List" || step.Action == "repo.Query" || step.Action == "db.List" || step.Action == "db.Query" {
			output, _ := step.Args["output"].(string)
			if output != "" {
				declaredVars[output] = true
			}
		}
		if step.Action == "repo.Query" || step.Action == "db.Query" {
			if input, _ := step.Args["input"].(string); strings.TrimSpace(input) != "" {
				if errStr := validateSafeCallArgExpr(input); errStr != "" {
					addWarn(stepNum, step.Action, "UNSAFE_QUERY_ARG_EXPR", step.Action+" input: "+errStr, "Use only refs/literals (or safe struct literals without calls) in query inputs.", step.File, step.Line, step.Column)
				}
			}
			if args, ok := step.Args["args"]; ok {
				switch v := args.(type) {
				case string:
					step.Args["args"] = []string{v}
				case []any:
					var ss []string
					for _, x := range v {
						ss = append(ss, fmt.Sprint(x))
					}
					step.Args["args"] = ss
				}
			}
			if args, ok := step.Args["args"].([]string); ok {
				for idx, arg := range args {
					if errStr := validateSafeCallArgExpr(arg); errStr != "" {
						addWarn(stepNum, step.Action, "UNSAFE_QUERY_ARG_EXPR", fmt.Sprintf("%s args[%d]: %s", step.Action, idx, errStr), "Use only refs/literals (or safe struct literals without calls) in query args.", step.File, step.Line, step.Column)
					}
				}
			}
		}
		if (step.Action == "repo.GetForUpdate" || step.Action == "db.Lock" || step.Action == "db.SelectForUpdate") && !inTx {
			addWarn(stepNum, step.Action, "TX_REQUIRED", step.Action+" outside tx.Block", "{action: \"tx.Block\", do: [ ... ]}", step.File, step.Line, step.Column)
		}
		if step.Action == "repo.Upsert" {
			if step.Args["source"] == nil || step.Args["source"] == "" {
				addWarn(stepNum, step.Action, "MISSING_SOURCE", "repo.Upsert missing 'source'", "{action: \"repo.Upsert\", source: \"Entity\", find: \"FindBy...\", input: \"...\", output: \"item\"}", step.File, step.Line, step.Column)
			}
			if step.Args["find"] == nil || step.Args["find"] == "" {
				addWarn(stepNum, step.Action, "MISSING_FIND", "repo.Upsert missing 'find'", "{action: \"repo.Upsert\", source: \"Entity\", find: \"FindBy...\", input: \"...\", output: \"item\"}", step.File, step.Line, step.Column)
			}
			if step.Args["input"] == nil || step.Args["input"] == "" {
				addWarn(stepNum, step.Action, "MISSING_INPUT", "repo.Upsert missing 'input'", "{action: \"repo.Upsert\", source: \"Entity\", find: \"FindBy...\", input: \"...\", output: \"item\"}", step.File, step.Line, step.Column)
			}
			if step.Args["output"] == nil || step.Args["output"] == "" {
				addWarn(stepNum, step.Action, "MISSING_OUTPUT", "repo.Upsert missing 'output'", "{action: \"repo.Upsert\", source: \"Entity\", find: \"FindBy...\", input: \"...\", output: \"item\"}", step.File, step.Line, step.Column)
			}
			_, hasIfNew := step.Args["_ifNew"].([]FlowStep)
			_, hasIfExists := step.Args["_ifExists"].([]FlowStep)
			if !hasIfNew && !hasIfExists {
				addWarn(stepNum, step.Action, "MISSING_BRANCHES", "repo.Upsert requires at least one branch: ifNew or ifExists", "{action: \"repo.Upsert\", ..., ifNew: [ ... ]}", step.File, step.Line, step.Column)
			}
			if subSteps, ok := step.Args["_ifNew"].([]FlowStep); ok {
				validate(subSteps, inTx, depth+1)
			}
			if subSteps, ok := step.Args["_ifExists"].([]FlowStep); ok {
				validate(subSteps, inTx, depth+1)
			}
		}
		return true

	case "mapping.Map":
		output, _ := step.Args["output"].(string)
		if output == "" {
			output, _ = step.Args["to"].(string)
		}
		entity, _ := step.Args["entity"].(string)
		if output != "" && strings.HasPrefix(strings.ToLower(output), "new") && entity == "" {
			addWarn(stepNum, step.Action, "MISSING_ENTITY", fmt.Sprintf("mapping.Map '%s' missing 'entity'", output), "{action: \"mapping.Map\", output: \""+output+"\", entity: \"Entity\"}", step.File, step.Line, step.Column)
		}
		if output != "" && entity != "" {
			declaredVars[output] = true
			newEntities[output] = entity
		}
		return true

	case "mapping.Assign":
		to, _ := step.Args["to"].(string)
		value, _ := step.Args["value"].(string)
		if to == "" {
			addWarn(stepNum, step.Action, "MISSING_TO", "mapping.Assign missing 'to'", "{action: \"mapping.Assign\", to: \"x.Field\", value: \"...\"}", step.File, step.Line, step.Column)
		}
		if value == "" {
			addWarn(stepNum, step.Action, "MISSING_VALUE", "mapping.Assign missing 'value'", "{action: \"mapping.Assign\", to: \"x.Field\", value: \"...\"}", step.File, step.Line, step.Column)
		}
		assignedFields[to] = true

		if errStr := validateValueExpr(value); errStr != "" {
			addWarn(stepNum, step.Action, "GO_SYNTAX_ERROR", errStr, "Check your Go expression syntax inside the CUE string.", step.File, step.Line, step.Column)
		}
		if errStr := validateMappingAssignSafeValue(value); errStr != "" {
			addWarn(stepNum, step.Action, "MAPPING_ASSIGN_UNSAFE_EXPR", errStr, "Use safe references/literals in mapping.Assign, and move function calls to typed actions (uuid.New, time.Now, rand.Token, ...).", step.File, step.Line, step.Column)
		}

		statusValues := map[string]bool{"draft": true, "active": true, "pending": true, "published": true, "closed": true, "approved": true, "rejected": true, "cancelled": true}
		if value != "" && !strings.Contains(value, "\"") && !strings.Contains(value, ".") && !strings.Contains(value, "(") {
			if statusValues[strings.ToLower(value)] {
				addWarn(stepNum, step.Action, "NEEDS_QUOTES", fmt.Sprintf("mapping.Assign '%s' needs quotes: \"\\\"%s\\\"\"", value, value), "{action: \"mapping.Assign\", to: \"x.Status\", value: \"\\\""+value+"\\\"\"}", step.File, step.Line, step.Column, Fix{
					Kind: "replace",
					Text: "\"" + value + "\"",
				})
			}
		}
		return true

	case "event.Publish":
		name, _ := step.Args["name"].(string)
		payload, _ := step.Args["payload"].(string)
		payloadMap := flowStringMap(step.Args["payloadMap"])
		if payload != "" {
			addWarnWithSeverity(stepNum, step.Action, "EVENT_PUBLISH_RAW_PAYLOAD_FORBIDDEN", "warn", "event.Publish raw 'payload' is not allowed; use payloadMap", "{action: \"event.Publish\", name: \""+name+"\", payloadMap: { TenderID: \"req.TenderID\" }}", step.File, step.Line, step.Column)
		}
		for field, expr := range payloadMap {
			field = strings.TrimSpace(field)
			if field == "" {
				addWarn(stepNum, step.Action, "INVALID_PAYLOAD_FIELD", "event.Publish payloadMap contains empty field name", "{action: \"event.Publish\", payloadMap: { Field: \"req.Value\" }}", step.File, step.Line, step.Column)
				continue
			}
			if errStr := validateSafeCallArgExpr(expr); errStr != "" {
				addWarn(stepNum, step.Action, "UNSAFE_EVENT_PAYLOAD_EXPR", fmt.Sprintf("event.Publish payloadMap[%q]: %s", field, errStr), "Use only refs/literals (or safe struct literals without calls) in payloadMap values.", step.File, step.Line, step.Column)
			}
		}
		return true

	case "event.Broadcast":
		return true

	case "event.Wait":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "event.Subscribe":
		if subSteps, ok := step.Args["_do"].([]FlowStep); ok {
			validate(subSteps, inTx, depth+1)
		}
		return true

	case "event.Match", "notification.Dispatch":
		return true

	case "notify.Send":
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "logic.Check":
		cond, _ := step.Args["condition"].(string)
		if strings.TrimSpace(cond) == "" {
			addWarn(stepNum, step.Action, "MISSING_CONDITION", "logic.Check missing 'condition'", "{action: \"logic.Check\", condition: \"req.ID != \\\"\\\"\", throw: \"invalid input\"}", step.File, step.Line, step.Column)
		}
		throwMsg, _ := step.Args["throw"].(string)
		if strings.TrimSpace(throwMsg) == "" {
			addWarn(stepNum, step.Action, "MISSING_THROW", "logic.Check missing 'throw'", "{action: \"logic.Check\", condition: \"ok\", throw: \"validation failed\"}", step.File, step.Line, step.Column)
		}
		if cond != "" {
			if errStr := validateSafeConditionExpr(cond); errStr != "" {
				addWarn(stepNum, step.Action, "UNSAFE_CONDITION_EXPR", "logic.Check condition: "+errStr, "Use safe boolean refs/comparisons only; avoid calls and Go-specific constructs.", step.File, step.Line, step.Column)
			}
		}
		return true

	case "logic.Call":
		if step.Args["func"] == nil || step.Args["func"] == "" {
			addWarn(stepNum, step.Action, "MISSING_FUNC", "logic.Call missing 'func'", "{action: \"logic.Call\", func: \"DoThing\", args: [\"a\", \"b\"]}", step.File, step.Line, step.Column)
		}
		funcExpr := strings.TrimSpace(fmt.Sprint(step.Args["func"]))
		if strings.Contains(funcExpr, "fmt.Errorf(") {
			addWarnWithSeverity(
				stepNum,
				step.Action,
				"LAMBDA_BARE_ERROR",
				"warn",
				"logic.Call uses fmt.Errorf; this propagates as HTTP 500 by default",
				"Use errors.New(http.StatusXxx, \"Code\", \"Message\") for business errors.",
				step.File,
				step.Line,
				step.Column,
			)
		}
		// Detect one-liner inline lambda: has func( but no real newlines (triple-quote produces \n).
		// Semicolons are used to join statements in one-liner style.
		isInlineLambda := strings.Contains(funcExpr, "func(")
		isOneLiner := !strings.Contains(funcExpr, "\n")
		semicolons := strings.Count(funcExpr, ";")
		if isInlineLambda && isOneLiner && (semicolons >= 3 || len(funcExpr) > 200) {
			addWarnWithSeverity(
				stepNum,
				step.Action,
				"LAMBDA_INLINE_ESCAPE",
				"warn",
				fmt.Sprintf("logic.Call.func is a one-liner inline lambda (%d chars, %d semicolons) — hard to read and error-prone", len(funcExpr), semicolons),
				"Rewrite using CUE triple-quoted string for readability:\n  func: \"\"\"\n    (func(ctx context.Context, req port.XRequest) (RetType, error) {\n        // write normal Go here — quotes don't need escaping\n    })\n    \"\"\"",
				step.File,
				step.Line,
				step.Column,
			)
		}
		if args, ok := step.Args["args"]; ok {
			switch v := args.(type) {
			case string:
				step.Args["args"] = []string{v}
			case []any:
				var ss []string
				for _, x := range v {
					ss = append(ss, fmt.Sprint(x))
				}
				step.Args["args"] = ss
			}
		} else {
			step.Args["args"] = []string{}
		}
		if args, ok := step.Args["args"].([]string); ok {
			for idx, arg := range args {
				if errStr := validateSafeCallArgExpr(arg); errStr != "" {
					addWarn(stepNum, step.Action, "UNSAFE_CALL_ARG_EXPR", fmt.Sprintf("logic.Call args[%d]: %s", idx, errStr), "Use only refs/literals (or safe struct literals without calls) in call args.", step.File, step.Line, step.Column)
				}
			}
		}
		return true

	case "service.Call":
		serviceTarget, _ := step.Args["service"].(string)
		methodTarget, _ := step.Args["method"].(string)
		if strings.TrimSpace(serviceTarget) == "" {
			addWarn(stepNum, step.Action, "MISSING_SERVICE", "service.Call missing 'service'", "{action: \"service.Call\", service: \"Tender\", method: \"GetTender\", args: [\"ctx\", \"req\"]}", step.File, step.Line, step.Column)
		}
		if strings.TrimSpace(methodTarget) == "" {
			addWarn(stepNum, step.Action, "MISSING_METHOD", "service.Call missing 'method'", "{action: \"service.Call\", service: \"Tender\", method: \"GetTender\", args: [\"ctx\", \"req\"]}", step.File, step.Line, step.Column)
		}
		dep := strings.ToLower(normalizeServiceName(serviceTarget))
		if dep != "" {
			if _, ok := allowedDeps[dep]; !ok {
				addWarn(stepNum, step.Action, "MISSING_SERVICE_DEP", fmt.Sprintf("service.Call targets '%s' but service '%s' does not declare it in uses", serviceTarget, svcName), "Add service name to operation uses: uses: [\""+serviceTarget+"\"]", step.File, step.Line, step.Column)
			}
		}
		if args, ok := step.Args["args"]; ok {
			switch v := args.(type) {
			case string:
				step.Args["args"] = []string{v}
			case []any:
				var ss []string
				for _, x := range v {
					ss = append(ss, fmt.Sprint(x))
				}
				step.Args["args"] = ss
			}
		} else {
			step.Args["args"] = []string{}
		}
		if args, ok := step.Args["args"].([]string); ok {
			for idx, arg := range args {
				if errStr := validateSafeCallArgExpr(arg); errStr != "" {
					addWarn(stepNum, step.Action, "UNSAFE_CALL_ARG_EXPR", fmt.Sprintf("service.Call args[%d]: %s", idx, errStr), "Use only refs/literals (or safe struct literals without calls) in call args.", step.File, step.Line, step.Column)
				}
			}
		}
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "flow.Call":
		opRaw, _ := step.Args["op"].(string)
		opRaw = strings.TrimSpace(opRaw)
		if opRaw == "" {
			addWarn(stepNum, step.Action, "MISSING_OP", "flow.Call missing 'op'", "{action: \"flow.Call\", op: \"ValidateTenderForBid\", args: {tenderID: \"req.TenderID\"}}", step.File, step.Line, step.Column)
			return true
		}

		targetService := ""
		targetMethod := opRaw
		if strings.Contains(opRaw, ".") {
			parts := strings.SplitN(opRaw, ".", 2)
			targetService = strings.TrimSpace(parts[0])
			targetMethod = strings.TrimSpace(parts[1])
		}
		if targetMethod == "" {
			addWarn(stepNum, step.Action, "INVALID_OP", "flow.Call op must be MethodName or ServiceName.MethodName", "{action: \"flow.Call\", op: \"Tender.ValidateTenderForBid\"}", step.File, step.Line, step.Column)
		}
		if targetService != "" {
			dep := strings.ToLower(normalizeServiceName(targetService))
			if _, ok := allowedDeps[dep]; !ok {
				addWarn(stepNum, step.Action, "MISSING_SERVICE_DEP", fmt.Sprintf("flow.Call targets '%s' but service '%s' does not declare it in uses", targetService, svcName), "Add service name to operation uses: uses: [\""+targetService+"\"]", step.File, step.Line, step.Column)
			}
		}

		if argsRaw, ok := step.Args["args"]; ok {
			switch v := argsRaw.(type) {
			case map[string]string:
				for key, expr := range v {
					if errStr := validateSafeCallArgExpr(expr); errStr != "" {
						addWarn(stepNum, step.Action, "UNSAFE_CALL_ARG_EXPR", fmt.Sprintf("flow.Call args.%s: %s", key, errStr), "Use only refs/literals (or safe struct literals without calls) in flow.Call args.", step.File, step.Line, step.Column)
					}
				}
			case map[string]any:
				for key, raw := range v {
					expr := fmt.Sprint(raw)
					if errStr := validateSafeCallArgExpr(expr); errStr != "" {
						addWarn(stepNum, step.Action, "UNSAFE_CALL_ARG_EXPR", fmt.Sprintf("flow.Call args.%s: %s", key, errStr), "Use only refs/literals (or safe struct literals without calls) in flow.Call args.", step.File, step.Line, step.Column)
					}
				}
			default:
				addWarn(stepNum, step.Action, "INVALID_ARGS_TYPE", "flow.Call args must be map[string]string", "{action: \"flow.Call\", op: \"ValidateTenderForBid\", args: {tenderID: \"req.TenderID\"}}", step.File, step.Line, step.Column)
			}
		}
		if output, _ := step.Args["output"].(string); output != "" {
			declaredVars[output] = true
		}
		return true

	case "list.Append":
		if step.Args["to"] == nil || step.Args["to"] == "" {
			addWarn(stepNum, step.Action, "MISSING_TO", "list.Append missing 'to'", "{action: \"list.Append\", to: \"resp.Items\", item: \"item\"}", step.File, step.Line, step.Column)
		}
		if step.Args["item"] == nil || step.Args["item"] == "" {
			addWarn(stepNum, step.Action, "MISSING_ITEM", "list.Append missing 'item'", "{action: \"list.Append\", to: \"resp.Items\", item: \"item\"}", step.File, step.Line, step.Column)
		}
		return true

	case "fsm.Transition":
		if step.Args["entity"] == nil || step.Args["entity"] == "" {
			addWarn(stepNum, step.Action, "MISSING_ENTITY", "fsm.Transition missing 'entity'", "{action: \"fsm.Transition\", entity: \"award\", to: \"approved\"}", step.File, step.Line, step.Column)
		}
		if step.Args["to"] == nil || step.Args["to"] == "" {
			addWarn(stepNum, step.Action, "MISSING_TO", "fsm.Transition missing 'to'", "{action: \"fsm.Transition\", entity: \"award\", to: \"approved\"}", step.File, step.Line, step.Column)
		}
		return true

	case "math.Expr":
		if step.Args["expr"] == nil || step.Args["expr"] == "" {
			addWarn(stepNum, step.Action, "MISSING_EXPR", "math.Expr missing 'expr'", "{action: \"math.Expr\", expr: \"req.Price*0.9\", output: \"discounted\"}", step.File, step.Line, step.Column)
		}
		if output, _ := step.Args["output"].(string); strings.TrimSpace(output) == "" {
			addWarn(stepNum, step.Action, "MISSING_OUTPUT", "math.Expr missing 'output'", "{action: \"math.Expr\", expr: \"req.Price*0.9\", output: \"discounted\"}", step.File, step.Line, step.Column)
		} else {
			declaredVars[output] = true
		}
		return true
	}

	return false
}
