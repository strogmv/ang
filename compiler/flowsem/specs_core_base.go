package flowsem

import "strings"

var specsCoreBase = map[string]Spec{
	"repo.Find": {
		RequiredArgs: []string{"source", "input", "output"},
		OptionalArgKinds: map[string]ArgKind{
			"error":  ArgKindString,
			"method": ArgKindString,
		},
		DeclaresFromArgs: []string{"output"},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := nonEmptyString(step.Args["error"]); ok {
				return nil
			}
			return &Issue{
				Code:     "REPO_FIND_WITHOUT_ERROR",
				Severity: "warn",
				Message:  "repo.Find without 'error' returns nil when not found; add explicit nil-check",
				Hint:     `Either add error: "Not found" or guard with flow.If/logic.Check before dereference`,
			}
		},
	},
	"repo.Get": {
		RequiredArgs: []string{"source", "input", "output"},
		OptionalArgKinds: map[string]ArgKind{
			"error":  ArgKindString,
			"method": ArgKindString,
		},
		DeclaresFromArgs: []string{"output"},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := nonEmptyString(step.Args["error"]); ok {
				return nil
			}
			return &Issue{
				Code:    "REPO_GET_MISSING_ERROR",
				Message: "repo.Get with output requires 'error' to guard not-found nil result",
				Hint:    `Use {action: "repo.Get", ..., error: "Not found"}`,
			}
		},
	},
	"repo.List": {
		RequiredArgs: []string{"source", "output"},
		OptionalArgKinds: map[string]ArgKind{
			"input":  ArgKindString,
			"error":  ArgKindString,
			"method": ArgKindString,
		},
		DeclaresFromArgs: []string{"output"},
	},
	"repo.Save": {
		RequiredArgs: []string{"source", "input"},
	},
	"repo.Delete": {
		RequiredArgs: []string{"source", "input"},
	},
	"logic.Check": {
		RequiredArgs: []string{"condition", "throw"},
		CustomConstraints: func(step Step) *Issue {
			return validateGoExprArg(step, "condition")
		},
	},
	"logic.Call": {
		RequiredArgs: []string{"func"},
		OptionalArgKinds: map[string]ArgKind{
			"args":      ArgKindStringOrStringArr,
			"output":    ArgKindString,
			"ignoreErr": ArgKindBool,
		},
		DeclaresFromArgs: []string{"output"},
		CustomConstraints: func(step Step) *Issue {
			return validateGoExprArg(step, "func")
		},
	},
	"mapping.Assign": {
		RequiredArgs: []string{"to", "value"},
		CustomConstraints: func(step Step) *Issue {
			if issue := validateGoExprArg(step, "value"); issue != nil {
				return issue
			}
			return validateMappingAssignValue(step)
		},
	},
	"mapping.Map": {
		OptionalArgKinds: map[string]ArgKind{
			"input":  ArgKindString,
			"from":   ArgKindString,
			"output": ArgKindString,
			"to":     ArgKindString,
			"entity": ArgKindString,
		},
		CustomConstraints: func(step Step) *Issue {
			in, hasIn := nonEmptyString(step.Args["input"])
			from, hasFrom := nonEmptyString(step.Args["from"])
			out, hasOut := nonEmptyString(step.Args["output"])
			to, hasTo := nonEmptyString(step.Args["to"])
			_, hasEntity := nonEmptyString(step.Args["entity"])
			// Allow declaration-only form: {action:"mapping.Map", output:"x", entity:"Entity"}
			// In this form no input/from is required; fields are set via mapping.Assign afterwards.
			if !(hasIn || hasFrom) && !hasEntity {
				return &Issue{
					Code:    "MISSING_INPUT",
					Message: "mapping.Map requires 'input' or 'from'",
					Hint:    "{action: \"mapping.Map\", input: \"req\", output: \"dst\"}",
				}
			}
			if !(hasOut || hasTo) {
				return &Issue{
					Code:    "MISSING_OUTPUT",
					Message: "mapping.Map requires 'output' or 'to'",
					Hint:    "{action: \"mapping.Map\", input: \"req\", output: \"dst\"}",
				}
			}
			_ = in
			_ = from
			_ = out
			_ = to
			return nil
		},
		DeclaresFromArgs: []string{"output"},
	},
	"math.Expr": {
		RequiredArgs:     []string{"expr", "output"},
		DeclaresFromArgs: []string{"output"},
		CustomConstraints: func(step Step) *Issue {
			return validateGoExprArg(step, "expr")
		},
	},
	"crypto.Hash": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"algo": ArgKindString,
		},
	},
	"event.Publish": {
		RequiredArgs: []string{"name"},
		OptionalArgKinds: map[string]ArgKind{
			"payload":    ArgKindString,
			"payloadMap": ArgKindStringMap,
		},
	},
	"service.Call": {
		RequiredArgs:     []string{"service", "method"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"args":      ArgKindStringOrStringArr,
			"output":    ArgKindString,
			"ignoreErr": ArgKindBool,
		},
	},
	"flow.Call": {
		RequiredArgs:     []string{"op"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"args":      ArgKindStringMap,
			"output":    ArgKindString,
			"ignoreErr": ArgKindBool,
		},
		CustomConstraints: func(step Step) *Issue {
			raw, ok := nonEmptyString(step.Args["op"])
			if !ok {
				return &Issue{
					Code:    "MISSING_OP",
					Message: "flow.Call requires non-empty 'op'",
					Hint:    "{action: \"flow.Call\", op: \"Tender.ValidateTenderForBid\", args: {tenderID: \"req.TenderID\"}}",
				}
			}
			op := strings.TrimSpace(raw)
			if strings.Contains(op, "..") || strings.HasPrefix(op, ".") || strings.HasSuffix(op, ".") {
				return &Issue{
					Code:    "INVALID_OP",
					Message: "flow.Call 'op' must be MethodName or Service.Method",
					Hint:    "{action: \"flow.Call\", op: \"ValidateTenderForBid\"}",
				}
			}
			return nil
		},
	},
	"event.Broadcast": {
		RequiredArgs: []string{"name"},
	},
	"event.Wait": {
		RequiredArgs:     []string{"name"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"timeout": ArgKindString,
			"match":   ArgKindString,
		},
	},
	"event.Subscribe": {
		RequiredArgs:     []string{"name", "match"},
		RequiredChildren: []string{"_do"},
	},
	"event.Match": {
		RequiredArgs: []string{"event", "match"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
		},
	},
	"notification.Dispatch": {
		// "event" is the canonical arg; "message" accepted for backwards compat
		CustomConstraints: func(step Step) *Issue {
			_, hasEvent := step.Args["event"]
			_, hasMessage := step.Args["message"]
			if !hasEvent && !hasMessage {
				return &Issue{
					Code:    "MISSING_EVENT",
					Message: "notification.Dispatch requires 'event' (or 'message') arg",
					Hint:    "{action: \"notification.Dispatch\", event: \"user.registered\", userID: \"user.ID\"}",
				}
			}
			return nil
		},
	},
	"notify.Send": {
		RequiredArgs: []string{"channel", "to"},
		OptionalArgKinds: map[string]ArgKind{
			"template": ArgKindString,
			"text":     ArgKindString,
			"subject":  ArgKindString,
			"html":     ArgKindString,
			"data":     ArgKindString,
			"output":   ArgKindString,
		},
		DeclaresFromArgs: []string{"output"},
		CustomConstraints: func(step Step) *Issue {
			_, hasTemplate := nonEmptyString(step.Args["template"])
			_, hasText := nonEmptyString(step.Args["text"])
			if !hasTemplate && !hasText {
				return &Issue{
					Code:    "MISSING_CONTENT",
					Message: "notify.Send requires either 'template' or 'text'",
					Hint:    "{action: \"notify.Send\", channel: \"email\", to: \"req.Email\", text: \"Build completed\"}",
				}
			}
			return nil
		},
	},
	"notify.Email": {
		RequiredArgs: []string{"to"},
		OptionalArgKinds: map[string]ArgKind{
			"template": ArgKindString,
			"text":     ArgKindString,
			"subject":  ArgKindString,
			"html":     ArgKindString,
			"data":     ArgKindString,
			"output":   ArgKindString,
		},
		DeclaresFromArgs: []string{"output"},
		CustomConstraints: func(step Step) *Issue {
			_, hasTemplate := nonEmptyString(step.Args["template"])
			_, hasText := nonEmptyString(step.Args["text"])
			if !hasTemplate && !hasText {
				return &Issue{
					Code:    "MISSING_CONTENT",
					Message: "notify.Email requires either 'template' or 'text'",
					Hint:    "{action: \"notify.Email\", to: \"req.Email\", text: \"Build completed\"}",
				}
			}
			return nil
		},
	},
	"approval.Request": {
		RequiredArgs: []string{"approvalKey", "title", "requestedBy", "approvers", "policy", "payload"},
		OptionalArgKinds: map[string]ArgKind{
			"description": ArgKindString,
			"deadline":    ArgKindString,
			"ttl":         ArgKindString,
			"approvalId":  ArgKindString,
			"status":      ArgKindString,
		},
		DeclaresFromArgs: []string{"approvalId", "status"},
		CustomConstraints: func(step Step) *Issue {
			switch v := step.Args["approvers"].(type) {
			case string:
				if strings.TrimSpace(v) == "" {
					return &Issue{Code: "MISSING_APPROVERS", Message: "approval.Request requires non-empty approvers", Hint: "{approvers: [\"manager@company.com\"]}"}
				}
			case []string:
				if len(v) == 0 {
					return &Issue{Code: "MISSING_APPROVERS", Message: "approval.Request requires non-empty approvers", Hint: "{approvers: [\"manager@company.com\"]}"}
				}
			case []any:
				if len(v) == 0 {
					return &Issue{Code: "MISSING_APPROVERS", Message: "approval.Request requires non-empty approvers", Hint: "{approvers: [\"manager@company.com\"]}"}
				}
			default:
				return &Issue{Code: "INVALID_APPROVERS_TYPE", Message: "approval.Request approvers must be string or []string", Hint: "{approvers: [\"manager@company.com\"]}"}
			}
			policyRaw, _ := nonEmptyString(step.Args["policy"])
			policy, isStatic := staticWordLiteral(policyRaw)
			if isStatic {
				switch policy {
				case "any", "all", "quorum":
				default:
					return &Issue{Code: "INVALID_POLICY", Message: "approval.Request policy must be any|all|quorum when literal value is used", Hint: "{policy: \"any\"}"}
				}
			}
			return nil
		},
	},
	"approval.Wait": {
		RequiredArgs: []string{"approvalId"},
		OptionalArgKinds: map[string]ArgKind{
			"timeout":   ArgKindString,
			"onTimeout": ArgKindString,
			"decision":  ArgKindString,
			"status":    ArgKindString,
			"decidedBy": ArgKindString,
			"decidedAt": ArgKindString,
			"reason":    ArgKindString,
		},
		DeclaresFromArgs: []string{"decision", "status", "decidedBy", "decidedAt", "reason"},
		CustomConstraints: func(step Step) *Issue {
			raw, ok := nonEmptyString(step.Args["onTimeout"])
			if !ok {
				return nil
			}
			mode, isStatic := staticWordLiteral(raw)
			if !isStatic {
				return nil
			}
			if mode != "reject" && mode != "auto-approve" && mode != "fallback" {
				return &Issue{
					Code:    "INVALID_ON_TIMEOUT",
					Message: "approval.Wait onTimeout must be reject|auto-approve|fallback when literal value is used",
					Hint:    "{action: \"approval.Wait\", approvalId: \"approvalID\", onTimeout: \"fallback\", onTimeout: [ ... ]}",
				}
			}
			return nil
		},
	},
	"approval.Decide": {
		RequiredArgs: []string{"approvalId", "decision", "actor"},
		OptionalArgKinds: map[string]ArgKind{
			"reason": ArgKindString,
			"status": ArgKindString,
		},
		DeclaresFromArgs: []string{"status"},
		CustomConstraints: func(step Step) *Issue {
			raw, ok := nonEmptyString(step.Args["decision"])
			if !ok {
				return nil
			}
			decision, isStatic := staticWordLiteral(raw)
			if !isStatic {
				return nil
			}
			if decision != "approved" && decision != "rejected" && decision != "timed_out" {
				return &Issue{
					Code:    "INVALID_DECISION",
					Message: "approval.Decide decision must be approved|rejected|timed_out when literal value is used",
					Hint:    "{action: \"approval.Decide\", approvalId: \"id\", decision: \"approved\", actor: \"req.UserID\"}",
				}
			}
			return nil
		},
	},
	"policy.Evaluate": {
		RequiredArgs: []string{"policyKey"},
		OptionalArgKinds: map[string]ArgKind{
			"subject":   ArgKindString,
			"resource":  ArgKindString,
			"operation": ArgKindString,
			"tenant":    ArgKindString,
			"attrs":     ArgKindString,
			"context":   ArgKindString,
			"decision":  ArgKindString,
			"reason":    ArgKindString,
			"effects":   ArgKindString,
			"output":    ArgKindString,
		},
		DeclaresFromArgs: []string{"decision", "reason", "effects", "output"},
		CustomConstraints: func(step Step) *Issue {
			for _, key := range []string{"decision", "reason", "effects", "output"} {
				if v, ok := nonEmptyString(step.Args[key]); ok && strings.TrimSpace(v) != "" {
					return nil
				}
			}
			return &Issue{
				Code:    "MISSING_POLICY_OUTPUT",
				Message: "policy.Evaluate should declare at least one output: decision/reason/effects/output",
				Hint:    "{action: \"policy.Evaluate\", policyKey: \"project.create\", decision: \"decision\"}",
			}
		},
	},
	"policy.Check": {
		RequiredArgs: []string{"policy", "user"},
		OptionalArgKinds: map[string]ArgKind{
			"companyID": ArgKindString,
			"output":    ArgKindString,
			"throw":     ArgKindString,
			"code":      ArgKindString,
			"status":    ArgKindString,
		},
		DeclaresFromArgs: []string{"output"},
	},
	"policy.Require": {
		RequiredArgs: []string{"policyKey"},
		OptionalArgKinds: map[string]ArgKind{
			"subject":   ArgKindString,
			"resource":  ArgKindString,
			"operation": ArgKindString,
			"tenant":    ArgKindString,
			"attrs":     ArgKindString,
			"context":   ArgKindString,
			"throw":     ArgKindString,
			"code":      ArgKindString,
			"status":    ArgKindString,
			"decision":  ArgKindString,
			"reason":    ArgKindString,
			"effects":   ArgKindString,
			"output":    ArgKindString,
		},
		DeclaresFromArgs: []string{"decision", "reason", "effects", "output"},
	},
	"policy.Decide": {
		RequiredArgs: []string{"policyKey", "output"},
		OptionalArgKinds: map[string]ArgKind{
			"subject":   ArgKindString,
			"resource":  ArgKindString,
			"operation": ArgKindString,
			"tenant":    ArgKindString,
			"attrs":     ArgKindString,
			"context":   ArgKindString,
			"decision":  ArgKindString,
			"reason":    ArgKindString,
			"effects":   ArgKindString,
		},
		DeclaresFromArgs: []string{"decision", "reason", "effects", "output"},
	},
	"repo.GetForUpdate": {
		RequiresTx: true,
	},
	"repo.Query": {
		RequiredArgs: []string{"method"},
		OptionalArgKinds: map[string]ArgKind{
			"input": ArgKindString,
			"args":  ArgKindStringOrStringArr,
		},
	},
	"http.Request": {
		RequiredArgs:     []string{"method", "url"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"auth":      ArgKindString,
			"timeout":   ArgKindString,
			"into":      ArgKindString,
			"statusVar": ArgKindString,
			"output":    ArgKindString,
			"headers":   ArgKindStringMap,
			"query":     ArgKindStringMap,
		},
	},
	"http.RetryPolicy": {
		RequiredArgs:     []string{"method", "url"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"auth":      ArgKindString,
			"timeout":   ArgKindString,
			"statusVar": ArgKindString,
			"output":    ArgKindString,
			"headers":   ArgKindStringMap,
			"query":     ArgKindStringMap,
		},
	},
	"http.Paginate": {
		RequiredArgs:     []string{"url", "into", "as", "cursor_expr"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"auth":         ArgKindString,
			"method":       ArgKindString,
			"cursor_param": ArgKindString,
			"items_expr":   ArgKindString,
			"output_type":  ArgKindString,
			"headers":      ArgKindStringMap,
		},
	},
	"db.Get":  {},
	"db.List": {},
	"db.Query": {
		RequiredArgs: []string{"method"},
	},
	"db.Insert": {
		RequiredArgs: []string{"source", "input"},
	},
	"db.Update": {
		RequiredArgs: []string{"source", "input"},
	},
	"db.Upsert": {
		RequiredArgs: []string{"source", "input"},
	},
	"db.Delete": {},
	"db.Lock": {
		RequiresTx: true,
	},
	"db.SelectForUpdate": {
		RequiresTx: true,
	},
	"state.Get": {
		RequiredArgs:     []string{"key", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"default": ArgKindString,
		},
	},
	"state.Set": {
		RequiredArgs: []string{"key", "value"},
		OptionalArgKinds: map[string]ArgKind{
			"ttl": ArgKindString,
		},
	},
	"state.Delete": {
		RequiredArgs: []string{"key"},
	},
	// Idempotency & Deduplication
	"idem.DeriveKey": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"prefix": ArgKindString,
			"output": ArgKindString,
			"from":   ArgKindStringList,
		},
		CustomConstraints: func(step Step) *Issue {
			v, ok := step.Args["from"]
			if !ok {
				return &Issue{Code: "MISSING_FROM", Message: "idem.DeriveKey missing 'from'", Hint: "{action: \"idem.DeriveKey\", from: [\"req.UserID\", \"req.OrderID\"], output: \"idemKey\"}"}
			}
			switch arr := v.(type) {
			case []string:
				if len(arr) == 0 {
					return &Issue{Code: "EMPTY_FROM", Message: "idem.DeriveKey 'from' list is empty", Hint: "Provide at least one expression"}
				}
			case []any:
				if len(arr) == 0 {
					return &Issue{Code: "EMPTY_FROM", Message: "idem.DeriveKey 'from' list is empty", Hint: "Provide at least one expression"}
				}
			default:
				return &Issue{Code: "INVALID_FROM_TYPE", Message: "idem.DeriveKey 'from' must be a list of expressions", Hint: "{from: [\"req.UserID\", \"req.OrderID\"]}"}
			}
			return nil
		},
	},
	"idem.Check": {
		RequiredArgs: []string{"key"},
	},
	"idem.SaveResult": {
		RequiredArgs: []string{"key"},
		OptionalArgKinds: map[string]ArgKind{
			"ttl": ArgKindString,
		},
	},
	"idempotency.DeriveKey": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"prefix": ArgKindString,
			"output": ArgKindString,
			"from":   ArgKindStringList,
		},
		CustomConstraints: func(step Step) *Issue {
			v, ok := step.Args["from"]
			if !ok {
				return &Issue{Code: "MISSING_FROM", Message: "idempotency.DeriveKey missing 'from'", Hint: "{action: \"idempotency.DeriveKey\", from: [\"req.UserID\", \"req.OrderID\"], output: \"idemKey\"}"}
			}
			switch arr := v.(type) {
			case []string:
				if len(arr) == 0 {
					return &Issue{Code: "EMPTY_FROM", Message: "idempotency.DeriveKey 'from' list is empty", Hint: "Provide at least one expression"}
				}
			case []any:
				if len(arr) == 0 {
					return &Issue{Code: "EMPTY_FROM", Message: "idempotency.DeriveKey 'from' list is empty", Hint: "Provide at least one expression"}
				}
			default:
				return &Issue{Code: "INVALID_FROM_TYPE", Message: "idempotency.DeriveKey 'from' must be a list of expressions", Hint: "{from: [\"req.UserID\", \"req.OrderID\"]}"}
			}
			return nil
		},
	},
	"idempotency.Check": {
		RequiredArgs: []string{"key"},
	},
	"idempotency.SaveResult": {
		RequiredArgs: []string{"key"},
		OptionalArgKinds: map[string]ArgKind{
			"ttl": ArgKindString,
		},
	},
	"dedupe.Once": {
		RequiredArgs:     []string{"key"},
		RequiredChildren: []string{"_do"},
		OptionalArgKinds: map[string]ArgKind{
			"ttl": ArgKindString,
		},
	},
	// Rate limiting & Concurrency
	"ratelimit.Check": {
		RequiredArgs: []string{"key"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
			"rps":   ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["rps"]; !ok {
				return &Issue{Code: "MISSING_RPS", Message: "ratelimit.Check missing 'rps'", Hint: "{action: \"ratelimit.Check\", key: \"req.UserID\", rps: 10}"}
			}
			if !isIntLike(step.Args["rps"]) {
				return &Issue{Code: "INVALID_RPS_TYPE", Message: "ratelimit.Check 'rps' must be an integer", Hint: "{rps: 10}"}
			}
			return nil
		},
	},
	"ratelimit.Limit": {
		RequiredArgs: []string{"key"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
			"rps":   ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["rps"]; !ok {
				return &Issue{Code: "MISSING_RPS", Message: "ratelimit.Limit missing 'rps'", Hint: "{action: \"ratelimit.Limit\", key: \"req.UserID\", rps: 10}"}
			}
			if !isIntLike(step.Args["rps"]) {
				return &Issue{Code: "INVALID_RPS_TYPE", Message: "ratelimit.Limit 'rps' must be an integer", Hint: "{rps: 10}"}
			}
			return nil
		},
	},
	"quota.Check": {
		RequiredArgs: []string{"key"},
		OptionalArgKinds: map[string]ArgKind{
			"throw":  ArgKindString,
			"window": ArgKindString,
			"limit":  ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["limit"]; !ok {
				return &Issue{Code: "MISSING_LIMIT", Message: "quota.Check missing 'limit'", Hint: "{action: \"quota.Check\", key: \"req.UserID\", limit: 100, window: \"day\"}"}
			}
			if !isIntLike(step.Args["limit"]) {
				return &Issue{Code: "INVALID_LIMIT_TYPE", Message: "quota.Check 'limit' must be an integer", Hint: "{limit: 100}"}
			}
			window, ok := nonEmptyString(step.Args["window"])
			if !ok {
				return &Issue{Code: "MISSING_WINDOW", Message: "quota.Check missing 'window'", Hint: "{window: \"day\" | \"hour\" | \"month\"}"}
			}
			if literal, ok := staticWordLiteral(window); ok {
				switch literal {
				case "hour", "day", "month":
					return nil
				default:
					return &Issue{Code: "INVALID_WINDOW", Message: "quota.Check 'window' must be one of hour|day|month", Hint: "{window: \"day\"}"}
				}
			}
			return nil
		},
	},
	"budget.Check": {
		RequiredArgs: []string{"key"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
			"limit": ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["limit"]; !ok {
				return &Issue{Code: "MISSING_LIMIT", Message: "budget.Check missing 'limit'", Hint: "{action: \"budget.Check\", key: \"req.UserID\", limit: 100000}"}
			}
			if !isIntLike(step.Args["limit"]) {
				return &Issue{Code: "INVALID_LIMIT_TYPE", Message: "budget.Check 'limit' must be an integer", Hint: "{limit: 100000}"}
			}
			return nil
		},
	},
	"budget.Consume": {
		RequiredArgs: []string{"key", "tokens"},
		OptionalArgKinds: map[string]ArgKind{
			"ttl": ArgKindString,
		},
	},
	"context.Trim": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"max_bytes": ArgKindInt,
			"strategy":  ArgKindString,
		},
		CustomConstraints: func(step Step) *Issue {
			if raw, ok := nonEmptyString(step.Args["strategy"]); ok {
				if literal, ok := staticWordLiteral(raw); ok {
					switch literal {
					case "lines", "chars", "sentences":
						return nil
					default:
						return &Issue{Code: "INVALID_STRATEGY", Message: "context.Trim 'strategy' must be one of lines|chars|sentences", Hint: "{strategy: \"lines\"}"}
					}
				}
			}
			return nil
		},
	},
	"profile.Require": {
		RequiredArgs: []string{"key", "tier"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
		},
		CustomConstraints: func(step Step) *Issue {
			raw, ok := nonEmptyString(step.Args["tier"])
			if !ok {
				return &Issue{Code: "MISSING_TIER", Message: "profile.Require missing 'tier'", Hint: "{action: \"profile.Require\", key: \"req.UserID\", tier: \"ops\"}"}
			}
			if literal, ok := staticWordLiteral(raw); ok {
				switch literal {
				case "free", "ops", "enterprise":
					return nil
				default:
					return &Issue{Code: "INVALID_TIER", Message: "profile.Require 'tier' must be one of free|ops|enterprise", Hint: "{tier: \"ops\"}"}
				}
			}
			return nil
		},
	},
	"concurrency.Limit": {
		RequiredArgs: []string{"key"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
			"max":   ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["max"]; !ok {
				return &Issue{Code: "MISSING_MAX", Message: "concurrency.Limit missing 'max'", Hint: "{action: \"concurrency.Limit\", key: \"\\\"slow-op\\\"\", max: 5}"}
			}
			if !isIntLike(step.Args["max"]) {
				return &Issue{Code: "INVALID_MAX_TYPE", Message: "concurrency.Limit 'max' must be an integer", Hint: "{max: 5}"}
			}
			return nil
		},
	},
	"concurrency.Run": {
		RequiredArgs:     []string{"key"},
		RequiredChildren: []string{"_do"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
			"max":   ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["max"]; !ok {
				return &Issue{Code: "MISSING_MAX", Message: "concurrency.Run missing 'max'", Hint: "{action: \"concurrency.Run\", key: \"\\\"slow-op\\\"\", max: 5, do: [...]}"}
			}
			if !isIntLike(step.Args["max"]) {
				return &Issue{Code: "INVALID_MAX_TYPE", Message: "concurrency.Run 'max' must be an integer", Hint: "{max: 5}"}
			}
			return nil
		},
	},
	// Circuit breaker & Bulkhead
	"circuit.Check": {
		RequiredArgs: []string{"name"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
		},
	},
	"circuit.RecordSuccess": {
		RequiredArgs: []string{"name"},
	},
	"circuit.RecordFailure": {
		RequiredArgs: []string{"name"},
		OptionalArgKinds: map[string]ArgKind{
			"threshold": ArgKindInt,
			"openTTL":   ArgKindString,
		},
	},
	"circuit.Breaker": {
		RequiredArgs:     []string{"name"},
		RequiredChildren: []string{"_do"},
		OptionalArgKinds: map[string]ArgKind{
			"throw":     ArgKindString,
			"threshold": ArgKindInt,
			"openTTL":   ArgKindString,
		},
	},
	"bulkhead.Acquire": {
		RequiredArgs: []string{"name"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
			"max":   ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["max"]; !ok {
				return &Issue{Code: "MISSING_MAX", Message: "bulkhead.Acquire missing 'max'", Hint: "{action: \"bulkhead.Acquire\", name: \"\\\"db-pool\\\"\", max: 20}"}
			}
			if !isIntLike(step.Args["max"]) {
				return &Issue{Code: "INVALID_MAX_TYPE", Message: "bulkhead.Acquire 'max' must be an integer", Hint: "{max: 20}"}
			}
			return nil
		},
	},
	"bulkhead.Run": {
		RequiredArgs:     []string{"name"},
		RequiredChildren: []string{"_do"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
			"max":   ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["max"]; !ok {
				return &Issue{Code: "MISSING_MAX", Message: "bulkhead.Run missing 'max'", Hint: "{action: \"bulkhead.Run\", name: \"\\\"db-pool\\\"\", max: 20, do: [...]}"}
			}
			if !isIntLike(step.Args["max"]) {
				return &Issue{Code: "INVALID_MAX_TYPE", Message: "bulkhead.Run 'max' must be an integer", Hint: "{max: 20}"}
			}
			return nil
		},
	},
	"claude.Chat": {
		RequiredArgs:     []string{"user_message"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"system":         ArgKindString,
			"system_context": ArgKindString,
			"history":        ArgKindString,
			"output":         ArgKindString,
			"model":          ArgKindString,
			"max_tokens":     ArgKindInt,
		},
	},
	"openai.Chat": {
		RequiredArgs:     []string{"user_message"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"system":         ArgKindString,
			"system_context": ArgKindString,
			"history":        ArgKindString,
			"output":         ArgKindString,
			"model":          ArgKindString,
			"max_tokens":     ArgKindInt,
		},
	},
	"log.Emit": {
		RequiredArgs: []string{"message"},
		OptionalArgKinds: map[string]ArgKind{
			"level":  ArgKindString,
			"fields": ArgKindStringMap,
		},
	},
	"metric.Emit": {
		RequiredArgs: []string{"name"},
		OptionalArgKinds: map[string]ArgKind{
			"kind":   ArgKindString,
			"value":  ArgKindString,
			"labels": ArgKindStringMap,
		},
	},
	"trace.Span": {
		RequiredArgs:     []string{"name"},
		RequiredChildren: []string{"_do"},
		OptionalArgKinds: map[string]ArgKind{
			"attrs": ArgKindStringMap,
		},
	},
	"slo.Budget": {
		RequiredArgs:     []string{"duration"},
		RequiredChildren: []string{"_do"},
		OptionalArgKinds: map[string]ArgKind{
			"name": ArgKindString,
		},
	},
}
