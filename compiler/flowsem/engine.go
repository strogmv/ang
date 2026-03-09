package flowsem

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"
	"strings"
)

type Step struct {
	Action   string
	Args     map[string]any
	Children map[string][]Step
	File     string
	Line     int
	Column   int
	CUEPath  string
}

// EventField describes one field from normalized event schema.
type EventField struct {
	Name string
}

// EventDef describes event schema available for flow validation.
type EventDef struct {
	Name   string
	Fields []EventField
}

// ValidateOptions provides optional semantic context to flow validator.
type ValidateOptions struct {
	Events            []EventDef
	InStreamingMethod bool
}

type Issue struct {
	Step     int
	Action   string
	Code     string
	Message  string
	Hint     string
	File     string
	Line     int
	Column   int
	CUEPath  string
	Severity string
}

type Spec struct {
	RequiredArgs      []string
	RequiredChildren  []string
	DeclaresFromArgs  []string
	OptionalArgKinds  map[string]ArgKind
	RequiresTx        bool
	RequiresStreaming bool
	CustomConstraints func(step Step) *Issue
}

type ArgKind string

const (
	ArgKindString            ArgKind = "string"
	ArgKindInt               ArgKind = "int"
	ArgKindBool              ArgKind = "bool"
	ArgKindStringMap         ArgKind = "map[string]string"
	ArgKindFieldsRuleMap     ArgKind = "map[string]map[string]string"
	ArgKindStringOrStringArr ArgKind = "string|[]string"
	ArgKindStringList        ArgKind = "[]string"
)

func Validate(steps []Step) []Issue {
	return ValidateWithOptions(steps, ValidateOptions{})
}

func ValidateWithOptions(steps []Step, opts ValidateOptions) []Issue {
	var out []Issue
	eventsByName := indexEventsByName(opts.Events)
	isSchemaScaffoldFile := func(file string) bool {
		f := strings.TrimSpace(strings.ReplaceAll(file, "\\", "/"))
		if f == "" {
			return false
		}
		return strings.Contains(f, "/cue/schema/") || strings.HasPrefix(f, "cue/schema/")
	}
	var walk func(items []Step, inTx bool)
	walk = func(items []Step, inTx bool) {
		for i := range items {
			step := items[i]
			if isSchemaScaffoldFile(step.File) {
				continue
			}
			spec, ok := specs[step.Action]
			if !ok {
				if !isKnownPrefix(step.Action) {
					out = append(out, issue(step, i+1, "UNKNOWN_ACTION", "unknown action '"+step.Action+"'", "{action: \"repo.Find\" | \"mapping.Assign\" | \"flow.If\" ...}"))
				}
			} else {
				for _, arg := range spec.RequiredArgs {
					v, present := step.Args[arg]
					if !present {
						out = append(out, issue(step, i+1, "MISSING_"+strings.ToUpper(arg), step.Action+" missing '"+arg+"'", "See action contract in flow semantics"))
						continue
					}
					if _, ok := nonEmptyString(v); !ok {
						if _, isString := v.(string); isString {
							out = append(out, issue(step, i+1, "MISSING_"+strings.ToUpper(arg), step.Action+" missing '"+arg+"'", "See action contract in flow semantics"))
						} else {
							out = append(out, issue(step, i+1, "INVALID_"+strings.ToUpper(arg)+"_TYPE", step.Action+" arg '"+arg+"' must be string expression", "Pass a CUE string expression"))
						}
					}
				}
				for _, child := range spec.RequiredChildren {
					if len(step.Children[child]) == 0 {
						out = append(out, issue(step, i+1, "MISSING_"+strings.ToUpper(strings.TrimPrefix(child, "_")), step.Action+" missing '"+strings.TrimPrefix(child, "_")+"'", "See action contract in flow semantics"))
					}
				}
				for argName, kind := range spec.OptionalArgKinds {
					v, present := step.Args[argName]
					if !present {
						continue
					}
					if !argMatchesKind(v, kind) {
						out = append(out, issue(step, i+1, "INVALID_"+strings.ToUpper(argName)+"_TYPE", step.Action+" arg '"+argName+"' must be "+string(kind), "Fix arg type in CUE step"))
					}
				}
				if spec.RequiresTx && !inTx {
					out = append(out, issue(step, i+1, "TX_REQUIRED", step.Action+" outside tx.Block", "{action: \"tx.Block\", do: [ ... ]}"))
				}
				if spec.RequiresStreaming && !opts.InStreamingMethod {
					out = append(out, issue(step, i+1, "STREAMING_REQUIRED", step.Action+" requires operation stream: true", "Set stream: true on operation or replace action with non-streaming variant"))
				}
				if spec.CustomConstraints != nil {
					if extra := spec.CustomConstraints(step); extra != nil {
						extra.Step = i + 1
						extra.Action = step.Action
						extra.File = step.File
						extra.Line = step.Line
						extra.Column = step.Column
						extra.CUEPath = step.CUEPath
						if extra.Severity == "" {
							extra.Severity = "error"
						}
						out = append(out, *extra)
					}
				}
			}
			if eventIssues := validateEventStep(step, i+1, eventsByName); len(eventIssues) > 0 {
				out = append(out, eventIssues...)
			}
			// Transaction context propagates down the subtree; tx-only actions are validated
			// against this propagated state, not global method position.
			nextTx := inTx || step.Action == "tx.Block"
			for _, children := range step.Children {
				if len(children) > 0 {
					walk(children, nextTx)
				}
			}
		}
	}
	walk(steps, false)
	return out
}

func indexEventsByName(events []EventDef) map[string]EventDef {
	if len(events) == 0 {
		return nil
	}
	idx := make(map[string]EventDef, len(events))
	for _, evt := range events {
		name := strings.TrimSpace(evt.Name)
		if name == "" {
			continue
		}
		idx[name] = evt
	}
	return idx
}

func validateEventStep(step Step, idx int, eventsByName map[string]EventDef) []Issue {
	if len(eventsByName) == 0 {
		return nil
	}
	switch step.Action {
	case "event.Publish", "event.Outbox", "event.Broadcast":
	default:
		return nil
	}

	eventName, ok := staticEventName(step.Args["name"])
	if !ok {
		// dynamic event name expression cannot be validated statically
		return nil
	}

	def, exists := eventsByName[eventName]
	if !exists {
		return []Issue{issue(step, idx, "UNKNOWN_EVENT", "event "+strconv.Quote(eventName)+" not defined in cue/events/", "Define event in cue/events or fix event.Publish name")}
	}

	payloadMap := stepPayloadMap(step.Args["payloadMap"])
	if len(payloadMap) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(def.Fields))
	fieldNames := make([]string, 0, len(def.Fields))
	for _, f := range def.Fields {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			continue
		}
		allowed[canonicalFieldName(name)] = struct{}{}
		fieldNames = append(fieldNames, name)
	}
	if len(allowed) == 0 {
		return []Issue{issue(step, idx, "PAYLOAD_FIELD_NOT_IN_EVENT", "event "+strconv.Quote(eventName)+" has no fields, but payloadMap is provided", "Remove payloadMap or define fields in event schema")}
	}

	var out []Issue
	for field := range payloadMap {
		key := strings.TrimSpace(field)
		if key == "" {
			continue
		}
		if _, ok := allowed[canonicalFieldName(key)]; ok {
			continue
		}
		hint := "Available fields: " + strings.Join(fieldNames, ", ")
		out = append(out, issue(step, idx, "PAYLOAD_FIELD_NOT_IN_EVENT", "event "+strconv.Quote(eventName)+": field "+strconv.Quote(key)+" does not exist in event schema", hint))
	}
	return out
}

func canonicalFieldName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return strings.ToLower(s)
}

func staticEventName(v any) (string, bool) {
	raw, ok := nonEmptyString(v)
	if !ok {
		return "", false
	}
	if s, err := strconv.Unquote(raw); err == nil {
		raw = strings.TrimSpace(s)
	}
	if raw == "" {
		return "", false
	}
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' {
			continue
		}
		return "", false
	}
	return raw, true
}

func stepPayloadMap(v any) map[string]string {
	switch raw := v.(type) {
	case map[string]string:
		out := make(map[string]string, len(raw))
		for k, val := range raw {
			out[k] = val
		}
		return out
	case map[string]any:
		out := make(map[string]string, len(raw))
		for k, val := range raw {
			if s, ok := val.(string); ok {
				out[k] = s
				continue
			}
			out[k] = ""
		}
		return out
	default:
		return nil
	}
}

func issue(step Step, idx int, code, message, hint string) Issue {
	return Issue{
		Step:     idx,
		Action:   step.Action,
		Code:     code,
		Message:  message,
		Hint:     hint,
		File:     step.File,
		Line:     step.Line,
		Column:   step.Column,
		CUEPath:  step.CUEPath,
		Severity: "error",
	}
}

func nonEmptyString(v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func argMatchesKind(v any, kind ArgKind) bool {
	switch kind {
	case ArgKindString:
		_, ok := nonEmptyString(v)
		return ok
	case ArgKindInt:
		return isIntLike(v)
	case ArgKindBool:
		_, ok := v.(bool)
		return ok
	case ArgKindStringMap:
		m, ok := v.(map[string]string)
		return ok && m != nil
	case ArgKindFieldsRuleMap:
		m, ok := v.(map[string]map[string]string)
		return ok && m != nil
	case ArgKindStringOrStringArr:
		if _, ok := nonEmptyString(v); ok {
			return true
		}
		arr, ok := v.([]string)
		return ok && len(arr) > 0
	case ArgKindStringList:
		switch arr := v.(type) {
		case []string:
			return len(arr) > 0
		case []any:
			return len(arr) > 0
		default:
			return false
		}
	default:
		return true
	}
}

func isIntLike(v any) bool {
	switch n := v.(type) {
	case int, int64:
		return true
	case float64:
		return n == math.Trunc(n)
	default:
		return false
	}
}

func intArg(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != math.Trunc(n) {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}

func staticWordLiteral(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	if unquoted, err := strconv.Unquote(s); err == nil {
		s = strings.TrimSpace(unquoted)
	}
	if !isWordToken(s) {
		return "", false
	}
	return strings.ToLower(s), true
}

func isWordToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') &&
			(r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') &&
			r != '_' &&
			r != '-' {
			return false
		}
	}
	return true
}

func isKnownPrefix(action string) bool {
	if action == "" {
		return true
	}
	// Prefix allow-list keeps diagnostics useful: unknown actions with known families
	// are handled by emitter-specific validation, while truly foreign actions fail here.
	prefixes := []string{
		"repo.", "mapping.", "logic.", "service.", "event.", "fsm.", "flow.", "tx.",
		"list.", "notification.", "notify.", "approval.", "policy.", "audit.", "auth.", "entity.", "field.",
		"rbac.",
		"str.", "enum.", "time.", "map.",
		"exec.", "fs.",
		"cache.", "mail.", "storage.",
		"webhook.", "queue.", "dlq.",
		"http.", "rand.", "json.", "regex.", "base64.", "url.", "query.", "hash.", "uuid.", "ulid.", "math.", "jsonpath.", "batch.", "parallel.",
		"jwt.", "oauth2.", "crypto.",
		"idem.", "idempotency.", "dedupe.", "ratelimit.", "concurrency.", "circuit.", "bulkhead.",
		"budget.", "quota.", "context.", "profile.",
		"log.", "metric.", "trace.", "slo.",
		"pdf.",
		"claude.",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(action, p) {
			return true
		}
	}
	return false
}

func validateGoExprArg(step Step, argName string) *Issue {
	expr, ok := nonEmptyString(step.Args[argName])
	if !ok {
		return nil
	}
	if _, err := parser.ParseExpr(expr); err != nil {
		return &Issue{
			Code:    "INVALID_GO_EXPR",
			Message: step.Action + " arg '" + argName + "' has invalid Go expression " + strconv.Quote(expr) + ": " + err.Error(),
			Hint:    "Fix Go syntax in this flow expression",
		}
	}
	return nil
}

func validateMappingAssignValue(step Step) *Issue {
	value, ok := nonEmptyString(step.Args["value"])
	if !ok {
		return nil
	}
	if isSafeMappingAssignValue(value) {
		return nil
	}
	return &Issue{
		Code:     "RAW_GO_EXPR_IN_ASSIGN",
		Severity: "warn",
		Message:  "mapping.Assign value " + strconv.Quote(value) + " contains raw Go code",
		Hint:     "Use dot-paths/literals or typed actions (uuid.New, time.Now, rand.Token) for complex expressions",
	}
}

func isSafeMappingAssignValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	switch value {
	case "uuid.NewString()", "time.Now().UTC()", "time.Now().UTC().Format(time.RFC3339)":
		return true
	}
	expr, err := parser.ParseExpr(value)
	if err != nil {
		return false
	}
	if isDotPathOrIdentExpr(expr) || isLiteralExpr(expr) {
		return true
	}
	return false
}

func isDotPathOrIdentExpr(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.ParenExpr:
		return isDotPathOrIdentExpr(x.X)
	case *ast.Ident:
		return strings.TrimSpace(x.Name) != ""
	case *ast.SelectorExpr:
		return isDotPathOrIdentExpr(x.X) && x.Sel != nil && strings.TrimSpace(x.Sel.Name) != ""
	case *ast.IndexExpr:
		return isDotPathOrIdentExpr(x.X) && isSafeDotPathIndex(x.Index)
	case *ast.IndexListExpr:
		if !isDotPathOrIdentExpr(x.X) || len(x.Indices) == 0 {
			return false
		}
		for _, idx := range x.Indices {
			if !isSafeDotPathIndex(idx) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func isSafeDotPathIndex(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.ParenExpr:
		return isSafeDotPathIndex(x.X)
	case *ast.BasicLit:
		return x.Kind == token.INT || x.Kind == token.STRING || x.Kind == token.CHAR
	case *ast.Ident:
		return strings.TrimSpace(x.Name) != ""
	case *ast.SelectorExpr:
		return isDotPathOrIdentExpr(x)
	default:
		return false
	}
}

func isLiteralExpr(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.ParenExpr:
		return isLiteralExpr(x.X)
	case *ast.BasicLit:
		switch x.Kind {
		case token.STRING, token.INT, token.FLOAT, token.CHAR:
			return true
		default:
			return false
		}
	case *ast.Ident:
		switch strings.TrimSpace(x.Name) {
		case "true", "false", "nil":
			return true
		default:
			return false
		}
	default:
		return false
	}
}
