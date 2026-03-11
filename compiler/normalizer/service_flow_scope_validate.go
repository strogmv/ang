package normalizer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"unicode"
)

func validateFlowVariableScope(steps []FlowStep, emit func(stepNum int, step FlowStep, code, message, hint string)) {
	baseScope := cloneFlowScope(flowKnownRoots)
	isSchemaScaffoldFile := func(file string) bool {
		f := strings.TrimSpace(strings.ReplaceAll(file, "\\", "/"))
		if f == "" {
			return false
		}
		return strings.Contains(f, "/cue/schema/") || strings.HasPrefix(f, "cue/schema/")
	}

	var walk func(items []FlowStep, scope map[string]struct{})
	walk = func(items []FlowStep, scope map[string]struct{}) {
		for i := range items {
			step := items[i]
			stepNum := i + 1
			if isSchemaScaffoldFile(step.File) {
				continue
			}

			for _, expr := range flowStepReferenceExprs(step) {
				for _, root := range flowExprRoots(expr.Expr) {
					if isKnownFlowRoot(root) {
						continue
					}
					if _, ok := scope[root]; ok {
						continue
					}
					if _, ok := expr.LocalScope[root]; ok {
						continue
					}
					emit(
						stepNum,
						step,
						"UNDECLARED_FLOW_VAR",
						"undefined flow variable '"+root+"' in "+step.Action+" arg '"+expr.ArgName+"'",
						"Declare '"+root+"' in the same scope before usage, or move this step inside the branch where '"+root+"' is declared.",
					)
				}
			}

			switch step.Action {
			case "flow.If":
				if thenSteps, ok := step.Args["_then"].([]FlowStep); ok && len(thenSteps) > 0 {
					walk(thenSteps, cloneFlowScope(scope))
				}
				if elseSteps, ok := step.Args["_else"].([]FlowStep); ok && len(elseSteps) > 0 {
					walk(elseSteps, cloneFlowScope(scope))
				}
			case "flow.Switch":
				if cases, ok := step.Args["_cases"].(map[string][]FlowStep); ok && len(cases) > 0 {
					keys := make([]string, 0, len(cases))
					for k := range cases {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						walk(cases[k], cloneFlowScope(scope))
					}
				}
				if defaultSteps, ok := step.Args["_default"].([]FlowStep); ok && len(defaultSteps) > 0 {
					walk(defaultSteps, cloneFlowScope(scope))
				}
			case "flow.For":
				inner := cloneFlowScope(scope)
				if as, _ := step.Args["as"].(string); isSimpleIdent(strings.TrimSpace(as)) {
					inner[strings.TrimSpace(as)] = struct{}{}
				}
				if doSteps, ok := step.Args["_do"].([]FlowStep); ok && len(doSteps) > 0 {
					walk(doSteps, inner)
				}
			case "batch.Run", "tx.Block", "flow.Block", "flow.While", "flow.Try", "flow.Retry", "flow.Timeout", "flow.Catch", "flow.Fallback", "flow.Replay", "flow.Saga", "flow.Compensate", "flow.Defer", "event.Subscribe", "concurrency.Run", "bulkhead.Run", "circuit.Breaker", "trace.Span", "slo.Budget":
				for _, key := range []string{"_do", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
					if child, ok := step.Args[key].([]FlowStep); ok && len(child) > 0 {
						walk(child, cloneFlowScope(scope))
					}
				}
			default:
				if ifNew, ok := step.Args["_ifNew"].([]FlowStep); ok && len(ifNew) > 0 {
					walk(ifNew, cloneFlowScope(scope))
				}
				if ifExists, ok := step.Args["_ifExists"].([]FlowStep); ok && len(ifExists) > 0 {
					walk(ifExists, cloneFlowScope(scope))
				}
				if branches, ok := step.Args["_branches"].(map[string][]FlowStep); ok && len(branches) > 0 {
					keys := make([]string, 0, len(branches))
					for k := range branches {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						walk(branches[k], cloneFlowScope(scope))
					}
				}
			}

			for _, name := range flowStepDeclaredVars(step) {
				scope[name] = struct{}{}
			}
		}
	}

	walk(steps, baseScope)
}

type flowRefExpr struct {
	ArgName    string
	Expr       string
	LocalScope map[string]struct{}
}

func flowStepReferenceExprs(step FlowStep) []flowRefExpr {
	var out []flowRefExpr
	seen := map[string]struct{}{}
	add := func(argName, expr string, local map[string]struct{}) {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			return
		}
		key := argName + "\x00" + expr
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, flowRefExpr{
			ArgName:    argName,
			Expr:       expr,
			LocalScope: local,
		})
	}
	addArg := func(argName string) {
		if v, _ := step.Args[argName].(string); v != "" {
			add(argName, v, nil)
		}
	}
	addArgs := func(argNames ...string) {
		for _, name := range argNames {
			addArg(name)
		}
	}
	addArgExprList := func(argName string) {
		for _, expr := range flowArgExpressions(step.Args[argName]) {
			add(argName, expr, nil)
		}
	}
	addArgMapValues := func(argName string) {
		for _, expr := range flowStringMap(step.Args[argName]) {
			add(argName, expr, nil)
		}
	}

	switch step.Action {
	case "repo.Find", "repo.Get", "repo.GetForUpdate", "repo.Save", "repo.Delete", "repo.List",
		"db.Get", "db.List", "db.Insert", "db.Update", "db.Delete", "db.Lock", "db.SelectForUpdate":
		addArg("input")
	case "repo.Query", "db.Query":
		addArg("input")
		addArgExprList("args")
	case "repo.Upsert", "db.Upsert":
		addArg("input")
	case "mapping.Assign":
		addArg("value")
	case "mapping.Map":
		addArgs("input", "from")
	case "logic.Check", "flow.If", "flow.While":
		addArg("condition")
	case "logic.Call", "service.Call":
		addArgExprList("args")
	case "flow.Call":
		switch raw := step.Args["args"].(type) {
		case map[string]string:
			for _, expr := range raw {
				add("args", expr, nil)
			}
		case map[string]any:
			for _, anyExpr := range raw {
				add("args", strings.TrimSpace(toStringValue(anyExpr)), nil)
			}
		}
	case "flow.Switch":
		addArg("value")
	case "flow.For":
		addArg("each")
	case "list.Append":
		addArg("item")
	case "list.Filter":
		addArg("from")
		local := map[string]struct{}{}
		if as, _ := step.Args["as"].(string); isSimpleIdent(strings.TrimSpace(as)) {
			local[strings.TrimSpace(as)] = struct{}{}
		}
		if v, _ := step.Args["condition"].(string); v != "" {
			add("condition", v, local)
		}
	case "list.Paginate":
		addArgs("input", "offset", "limit")
	case "list.Sort":
		addArg("items")
	case "list.Enrich":
		addArgs("items", "lookupInput")
	case "math.Expr":
		addArg("expr")
	case "time.Parse":
		if v, _ := step.Args["value"].(string); v != "" {
			add("value", v, nil)
		} else {
			addArg("input")
		}
	case "str.Normalize":
		addArg("input")
	case "enum.Validate":
		addArg("value")
	case "auth.RequireRole":
		addArgs("userID", "companyID")
	case "auth.CheckRole":
		addArg("user")
	case "rbac.CheckPermission":
		addArg("user")
	case "audit.Log":
		addArgs("actor", "company", "event")
	case "entity.PatchNonZero", "entity.PatchValidated":
		addArgs("target", "from")
	case "field.CopyNonEmpty":
		addArgs("from", "to")
	case "cache.Get", "cache.Set", "cache.Del":
		addArg("key")
		if step.Action == "cache.Set" {
			addArg("value")
		}
	case "storage.Upload":
		if v, _ := step.Args["data"].(string); v != "" {
			add("data", v, nil)
		} else {
			addArg("input")
		}
		addArg("key")
	case "storage.Download", "storage.GetURL", "storage.Delete":
		addArg("key")
	case "storage.List":
		addArg("prefix")
	case "jwt.Sign":
		addArgs("claims", "secret")
	case "jwt.Verify":
		addArgs("token", "secret")
	case "crypto.Hash":
		addArg("input")
	case "event.Wait":
		addArgs("timeout", "match")
	case "event.Match":
		addArgs("event", "match")
	case "event.Publish", "event.Outbox", "event.Broadcast":
		addArg("payload")
		addArgMapValues("payloadMap")
	case "notification.Dispatch", "notify.Dispatch":
		addArgs("userID", "entityID", "payload")
	case "notify.Send", "notify.Email":
		addArgs("to", "template", "text", "subject", "html", "data")
	case "approval.Request":
		addArgs("approvalKey", "title", "requestedBy", "approvers", "policy", "payload", "description", "deadline", "ttl")
	case "approval.Wait":
		addArgs("approvalId", "timeout")
	case "approval.Decide":
		addArgs("approvalId", "decision", "actor", "reason")
	case "policy.Check":
		addArgs("user", "companyID", "status", "code", "throw")
	case "policy.Evaluate", "policy.Require", "policy.Decide":
		addArgs("policyKey", "subject", "resource", "operation", "tenant", "attrs", "context", "status", "code", "throw")
	case "exec.Run", "exec.Stream":
		addArgs("cmd", "stdin", "timeout")
		addArgExprList("args")
	case "fs.WriteFile":
		addArgs("path", "data")
	case "fs.ReadFile", "fs.Remove", "archive.ZipDir":
		addArg("path")
	case "http.Call", "http.Request", "http.RetryPolicy":
		addArgs("url", "body", "timeout", "auth")
		addArgMapValues("headers")
		addArgMapValues("query")
	case "http.Paginate":
		addArgs("url", "body", "timeout", "cursor", "next")
		addArgMapValues("headers")
		addArgMapValues("query")
	case "queue.Enqueue", "dlq.Publish":
		addArgs("subject", "payload", "reason", "timeout")
	case "queue.Dequeue":
		addArgs("subject", "timeout")
	case "queue.Ack", "queue.Nack":
		addArgs("subject", "messageID", "reason")
	case "webhook.Send":
		addArgs("url", "payload", "event")
	case "webhook.VerifySignature":
		addArgs("payload", "signature", "secret")
	case "webhook.Ack":
		addArg("body")
	case "state.Get":
		addArgs("key", "default")
	case "state.Set":
		addArgs("key", "value", "ttl")
	case "state.Delete":
		addArg("key")
	case "idem.DeriveKey", "idempotency.DeriveKey":
		addArgExprList("from")
		addArg("prefix")
	case "idem.Check", "idempotency.Check", "idem.SaveResult", "idempotency.SaveResult":
		addArgs("key", "ttl")
	case "dedupe.Once":
		addArgs("key", "ttl")
	case "ratelimit.Check", "ratelimit.Limit":
		addArgs("key", "throw")
	case "quota.Check":
		addArgs("key", "throw") // window is a static enum literal (day/hour/month), not a runtime expr
	case "budget.Check":
		addArgs("key", "throw")
	case "budget.Consume":
		addArgs("key", "tokens", "ttl")
	case "profile.Require":
		addArgs("key", "tier", "throw")
	case "context.Trim":
		addArgs("input", "strategy")
	case "openai.Chat", "openai.Stream":
		addArgs("system", "system_context", "user_message", "history", "model")
	case "stream.Emit":
		addArg("data")
	case "concurrency.Limit", "concurrency.Run":
		addArgs("key", "throw")
	case "circuit.Check", "circuit.RecordSuccess", "circuit.RecordFailure", "circuit.Breaker":
		addArgs("name", "throw", "openTTL")
	case "bulkhead.Acquire", "bulkhead.Run":
		addArgs("name", "throw")
	case "str.Format":
		addArg("template")
		addArgExprList("args")
	case "str.Concat":
		addArgExprList("parts")
	case "str.StripMarkdown":
		addArg("input")
	case "time.Format":
		addArgs("input", "format")
	case "math.Op", "num.Add", "num.Sub", "num.Mul", "num.Div":
		addArgs("a", "b", "value")
	case "jsonpath.Get", "jsonpath.Set":
		addArgs("input", "path", "value")
	case "cast.ToString", "json.Parse", "json.Marshal", "regex.Match", "regex.Replace",
		"base64.Encode", "base64.Decode", "url.Parse", "url.Build", "query.Encode", "query.Decode",
		"hash.Sum", "hash.HMAC", "oauth2.Token", "oauth2.Refresh",
		"crypto.Encrypt", "crypto.Decrypt", "secret.Get", "config.Get", "model.Resolve":
		addArgs("input", "key", "path", "url", "value", "pattern", "replacement", "secret")
		addArg("name")
	case "map.Build":
		addArgs("from", "key", "value")
	default:
		// Fallback: catch undeclared vars for exotic/rare actions by scanning common expression args.
		addArgs("input", "from", "value", "to", "key", "data", "url", "path", "payload", "token", "claims",
			"subject", "messageID", "reason", "signature", "secret", "actor", "company", "user", "companyID",
			"target", "resource", "operation", "tenant", "attrs", "context", "timeout", "ttl", "deadline",
			"event", "body", "match", "id", "prefix", "a", "b", "name", "tier", "window", "tokens", "strategy")
		addArgExprList("args")
		addArgExprList("parts")
		addArgExprList("from")
		addArgMapValues("headers")
		addArgMapValues("query")
		addArgMapValues("payloadMap")
	}

	return out
}

func flowArgExpressions(raw any) []string {
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		return []string{s}
	case []string:
		out := make([]string, 0, len(v))
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s := strings.TrimSpace(toStringValue(item))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func flowStepDeclaredVars(step FlowStep) []string {
	var out []string
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if !isSimpleIdent(raw) {
			return
		}
		out = append(out, raw)
	}

	for _, key := range []string{"output", "approvalId", "status", "decision", "decidedBy", "decidedAt", "reason", "effects", "ackToken", "statusVar", "exitCodeVar"} {
		if v, _ := step.Args[key].(string); v != "" {
			add(v)
		}
	}

	// flow.Parallel / flow.Join / flow.Race: branch outputs are available in outer scope
	if branches, ok := step.Args["_branches"].(map[string][]FlowStep); ok {
		for _, branchSteps := range branches {
			for _, bs := range branchSteps {
				for _, v := range flowStepDeclaredVars(bs) {
					add(v)
				}
			}
		}
	}

	// tx.Block, flow.Block, etc.: inner _do vars are visible after the block
	for _, childKey := range []string{"_do", "_catch", "_fallback", "_onTimeout", "_onMissing"} {
		if childSteps, ok := step.Args[childKey].([]FlowStep); ok {
			for _, cs := range childSteps {
				for _, v := range flowStepDeclaredVars(cs) {
					add(v)
				}
			}
		}
	}

	switch step.Action {
	case "mapping.Map":
		if v, _ := step.Args["output"].(string); v != "" {
			add(v)
		} else if v, _ := step.Args["to"].(string); v != "" {
			add(v)
		}
	case "mapping.Assign":
		if isDeclareArg(step.Args["declare"]) {
			if v, _ := step.Args["to"].(string); v != "" {
				add(v)
			}
		}
	case "auth.RequireRole":
		add("currentUser")
	}

	return out
}

func buildFlowKnownRoots() map[string]struct{} {
	base := []string{
		"req", "resp", "ctx", "s", "tx", "err",
		"domain", "port", "errors", "http",
		"time", "uuid", "fmt", "strings", "strconv", "math",
		"json", "rand", "crypto", "hex", "base64",
		"sql", "os", "url", "regexp", "sort", "slices", "maps", "bytes", "io", "filepath",
		"true", "false", "nil",
		"len", "cap", "append", "copy", "delete", "new", "make", "clear",
		"complex", "real", "imag", "close", "panic", "recover", "min", "max",
		"string", "bool", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "byte", "rune", "error", "any",
	}
	out := make(map[string]struct{}, len(base))
	for _, v := range base {
		out[v] = struct{}{}
	}
	return out
}

var flowKnownRoots = buildFlowKnownRoots()

func cloneFlowScope(src map[string]struct{}) map[string]struct{} {
	dst := make(map[string]struct{}, len(src))
	for k := range src {
		dst[k] = struct{}{}
	}
	return dst
}

func flowExprRoots(expr string) []string {
	parsed, err := parser.ParseExpr(strings.TrimSpace(expr))
	if err != nil {
		return nil
	}

	roots := make(map[string]struct{})
	var stack []ast.Node
	ast.Inspect(parsed, func(n ast.Node) bool {
		if n == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return true
		}
		var parent ast.Node
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, n)

		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		name := strings.TrimSpace(id.Name)
		if name == "" || name == "_" {
			return true
		}
		if parent != nil {
			switch p := parent.(type) {
			case *ast.SelectorExpr:
				if p.Sel == id {
					return true
				}
			case *ast.KeyValueExpr:
				if p.Key == id {
					return true
				}
			case *ast.Field:
				if p.Names != nil {
					return true
				}
			case *ast.ImportSpec:
				return true
			}
		}
		roots[name] = struct{}{}
		return true
	})

	out := make([]string, 0, len(roots))
	for name := range roots {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func isKnownFlowRoot(name string) bool {
	if name == "" {
		return true
	}
	r, _ := utf8FirstRune(name)
	if unicode.IsUpper(r) {
		return true
	}
	_, ok := flowKnownRoots[name]
	return ok
}

func isSimpleIdent(s string) bool {
	if s == "" || strings.Contains(s, ".") {
		return false
	}
	if token.Lookup(s).IsKeyword() {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !(r == '_' || unicode.IsLetter(r)) {
				return false
			}
			continue
		}
		if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func isDeclareArg(v any) bool {
	switch raw := v.(type) {
	case bool:
		return raw
	case string:
		s := strings.TrimSpace(strings.ToLower(raw))
		return s == "true" || s == "1" || s == "yes"
	default:
		return false
	}
}

func utf8FirstRune(s string) (rune, int) {
	for _, r := range s {
		return r, len(string(r))
	}
	return rune(0), 0
}

func toStringValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprint(v)
	}
}
