package normalizer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"cuelang.org/go/cue"
)

func addPaginationFields(method *Method) {
	if method == nil || method.Pagination == nil {
		return
	}
	exists := func(name string) bool {
		for _, f := range method.Input.Fields {
			if f.Name == name {
				return true
			}
		}
		return false
	}
	switch method.Pagination.Type {
	case "offset":
		if !exists("limit") {
			method.Input.Fields = append(method.Input.Fields, Field{Name: "limit", Type: "int", IsOptional: true})
		}
		if !exists("offset") {
			method.Input.Fields = append(method.Input.Fields, Field{Name: "offset", Type: "int", IsOptional: true})
		}
	case "cursor":
		if !exists("cursor") {
			method.Input.Fields = append(method.Input.Fields, Field{Name: "cursor", Type: "string", IsOptional: true})
		}
		if !exists("limit") {
			method.Input.Fields = append(method.Input.Fields, Field{Name: "limit", Type: "int", IsOptional: true})
		}
	}
}

func validateImplAntiPatterns(serviceName, methodName string, method Method, codeVal cue.Value) []Warning {
	if method.Impl == nil || strings.TrimSpace(method.Impl.Code) == "" {
		return nil
	}

	file, baseLine, baseCol := cuePosition(codeVal)
	lines := strings.Split(method.Impl.Code, "\n")
	var out []Warning
	appendWarning := func(code, message, hint string, lineOffset int, col int) {
		line := baseLine
		if lineOffset > 0 {
			line += lineOffset
		}
		if line <= 0 {
			line = baseLine
		}
		if col <= 0 {
			col = baseCol
		}
		out = append(out, Warning{
			Kind:     "impl",
			Code:     code,
			Severity: "error",
			Message:  fmt.Sprintf("%s.%s: %s", serviceName, methodName, message),
			Hint:     hint,
			File:     file,
			Line:     line,
			Column:   col,
			CUEPath:  codeVal.Path().String(),
		})
	}

	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "http://"); idx >= 0 || strings.Contains(line, "https://") {
			if idx < 0 {
				idx = strings.Index(line, "https://")
			}
			appendWarning(
				"IMPL_HARDCODED_URL_LITERAL",
				"do not hardcode external URLs in impls.go.code",
				"Move URL to cue/infra config or template variables and inject via service logic.",
				i,
				baseCol+idx,
			)
		}
	}
	return out
}

func cuePosition(v cue.Value) (file string, line int, col int) {
	pos := v.Pos()
	if pos.IsValid() {
		return pos.Filename(), pos.Line(), pos.Column()
	}
	return "", 0, 0
}

func isFlowFirstCandidate(methodName string) bool {
	name := strings.ToLower(strings.TrimSpace(methodName))
	prefixes := []string{
		"create",
		"get",
		"list",
		"update",
		"patch",
		"delete",
		"remove",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func validateFlowFirstImplCode(serviceName, methodName string, method Method, codeVal cue.Value, bypassReasonVal cue.Value, bypass bool, bypassReason string) []Warning {
	if method.Impl == nil || strings.TrimSpace(method.Impl.Code) == "" {
		return nil
	}
	if len(method.Flow) > 0 {
		return nil
	}
	if !isFlowFirstCandidate(methodName) {
		return nil
	}
	if bypass {
		if strings.TrimSpace(bypassReason) != "" {
			return nil
		}

		pos := bypassReasonVal.Pos()
		warnFile := ""
		warnLine := 0
		warnCol := 0
		if pos.IsValid() {
			warnFile = pos.Filename()
			warnLine = pos.Line()
			warnCol = pos.Column()
		}
		path := bypassReasonVal.Path().String()
		if strings.TrimSpace(path) == "" {
			path = "impls.go.flowFirstBypassReason"
		}
		return []Warning{
			{
				Kind:     "flow",
				Code:     "FLOW_FIRST_BYPASS_REASON_REQUIRED",
				Severity: "error",
				Message:  fmt.Sprintf("%s.%s: flowFirstBypass=true requires non-empty flowFirstBypassReason", serviceName, methodName),
				Hint:     "Set impls.go.flowFirstBypassReason with concrete rationale (e.g. external SDK orchestration, complex branching not expressible in flow yet).",
				File:     warnFile,
				Line:     warnLine,
				Column:   warnCol,
				CUEPath:  path,
			},
		}
	}

	pos := codeVal.Pos()
	warnFile := ""
	warnLine := 0
	warnCol := 0
	if pos.IsValid() {
		warnFile = pos.Filename()
		warnLine = pos.Line()
		warnCol = pos.Column()
	}

	return []Warning{
		{
			Kind:     "flow",
			Code:     "FLOW_FIRST_IMPL_REQUIRED",
			Severity: "error",
			Message:  fmt.Sprintf("%s.%s: CRUD/listing methods must use flow DSL instead of impls.go.code", serviceName, methodName),
			Hint:     "Move method logic into 'flow'. For exceptional complex cases set impls.go.flowFirstBypass: true with clear rationale.",
			File:     warnFile,
			Line:     warnLine,
			Column:   warnCol,
			CUEPath:  codeVal.Path().String(),
		},
	}
}

func validateSafeConditionExpr(value string) string {
	if err := validateValueExpr(value); err != "" {
		return strings.Replace(err, "Invalid Go expression", "Invalid condition expression", 1)
	}
	expr, err := parser.ParseExpr(value)
	if err != nil {
		return fmt.Sprintf("Invalid condition expression %q: %v", value, err)
	}
	if isSafeConditionExpr(expr) {
		return ""
	}
	return fmt.Sprintf("Unsafe condition expression %q: only refs, literals, calls of functions and methods (no repository or transaction), arithmetic and boolean comparisons (==, !=, <, <=, >, >=, &&, ||, !) are allowed", value)
}

func validateValueExpr(value string) string {
	if value == "" || strings.Contains(value, "{{") {
		return ""
	}
	if _, err := parser.ParseExpr(value); err != nil {
		return fmt.Sprintf("Invalid Go expression %q: %v", value, err)
	}
	return ""
}

func validateMappingAssignSafeValue(value string) string {
	if value == "" || strings.Contains(value, "{{") {
		return ""
	}
	value = strings.TrimSpace(value)
	switch value {
	case "uuid.NewString()", "time.Now().UTC()", "time.Now().UTC().Format(time.RFC3339)":
		return ""
	}
	expr, err := parser.ParseExpr(value)
	if err != nil {
		return ""
	}
	if isSafeMappingAssignExpr(expr) {
		return ""
	}
	return fmt.Sprintf("Unsafe mapping.Assign value %q: only dot-path refs (req.UserID, item.Name), identifiers, literals, and approved safe calls are allowed", value)
}

func isSafeConditionExpr(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.ParenExpr:
		return isSafeConditionExpr(x.X)
	case *ast.BinaryExpr:
		switch x.Op {
		case token.LAND, token.LOR:
			return isSafeConditionExpr(x.X) && isSafeConditionExpr(x.Y)
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			return isSafeConditionValueExpr(x.X) && isSafeConditionValueExpr(x.Y)
		default:
			return false
		}
	case *ast.UnaryExpr:
		if x.Op != token.NOT {
			return false
		}
		return isSafeConditionExpr(x.X)
	default:
		return isSafeConditionValueExpr(expr)
	}
}

func isSafeConditionValueExpr(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.Ident:
		switch x.Name {
		case "true", "false", "nil":
			return true
		}
	case *ast.CallExpr:
		return isSafeConditionCall(x)
	case *ast.ParenExpr:
		return isSafeConditionExpr(x.X)
	case *ast.BinaryExpr:
		switch x.Op {
		case token.ADD, token.SUB, token.MUL, token.QUO, token.REM:
			return isSafeConditionValueExpr(x.X) && isSafeConditionValueExpr(x.Y)
		}
	}
	return isSafeMappingAssignExpr(expr)
}

// conditionBuiltinsWithEffects are the builtins a condition must not call.
var conditionBuiltinsWithEffects = map[string]bool{
	"append": true, "clear": true, "close": true, "copy": true, "delete": true,
	"panic": true, "print": true, "println": true, "recover": true,
}

// isSafeConditionCall accepts a call in a condition — a helper, a builtin
// such as len, pkg.Func, a method of a value (t.IsZero(), time.Now().After(x)),
// s.Method or s.OtherService.Method — whose arguments are safe condition
// expressions. What would write, a repository (s.XRepo…) or the transaction
// manager (s.Tx, s.txManager), and function literals stay out: a condition
// must not hide effects the flow's analysis does not see.
func isSafeConditionCall(call *ast.CallExpr) bool {
	if !isSafeConditionCallee(call.Fun) {
		return false
	}
	for _, arg := range call.Args {
		if !isSafeConditionExpr(arg) {
			return false
		}
	}
	return true
}

func isSafeConditionCallee(fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.ParenExpr:
		return isSafeConditionCallee(f.X)
	case *ast.Ident:
		return !conditionBuiltinsWithEffects[f.Name]
	case *ast.SelectorExpr:
		name := f.Sel.Name
		if strings.HasSuffix(name, "Repo") || name == "Tx" || name == "txManager" {
			return false
		}
		switch x := f.X.(type) {
		case *ast.Ident:
			return true
		case *ast.SelectorExpr, *ast.ParenExpr:
			return isSafeConditionCallee(x)
		case *ast.CallExpr:
			return isSafeConditionCall(x)
		case *ast.IndexExpr:
			return isSafeMappingAssignExpr(x)
		}
	}
	return false
}

func validateSafeCallArgExpr(value string) string {
	if value == "" || strings.Contains(value, "{{") {
		return ""
	}
	expr, err := parser.ParseExpr(value)
	if err != nil {
		return fmt.Sprintf("Invalid argument expression %q: %v", value, err)
	}
	if isSafeCallArgExpr(expr) {
		return ""
	}
	return fmt.Sprintf("Unsafe argument expression %q: function calls and imperative Go constructs are not allowed", value)
}

func isSafeCallArgExpr(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.ParenExpr:
		return isSafeCallArgExpr(x.X)
	case *ast.CallExpr:
		return false
	case *ast.CompositeLit:
		if x.Type == nil || !isSafeCallArgExpr(x.Type) {
			return false
		}
		for _, elt := range x.Elts {
			switch kv := elt.(type) {
			case *ast.KeyValueExpr:
				if !isSafeCallArgExpr(kv.Key) || !isSafeCallArgExpr(kv.Value) {
					return false
				}
			case ast.Expr:
				if !isSafeCallArgExpr(kv) {
					return false
				}
			default:
				return false
			}
		}
		return true
	case *ast.ArrayType:
		if x.Len != nil && !isSafeCallArgExpr(x.Len) {
			return false
		}
		return x.Elt != nil && isSafeCallArgExpr(x.Elt)
	case *ast.MapType:
		return x.Key != nil && x.Value != nil && isSafeCallArgExpr(x.Key) && isSafeCallArgExpr(x.Value)
	case *ast.StructType:
		if x.Fields == nil {
			return true
		}
		for _, f := range x.Fields.List {
			if f.Type == nil || !isSafeCallArgExpr(f.Type) {
				return false
			}
		}
		return true
	case *ast.UnaryExpr:
		switch x.Op {
		case token.AND, token.MUL, token.ADD, token.SUB, token.NOT:
			return isSafeCallArgExpr(x.X)
		default:
			return false
		}
	case *ast.BinaryExpr:
		return false
	default:
		return isSafeMappingAssignExpr(expr)
	}
}

func isSafeMappingAssignExpr(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.ParenExpr:
		return isSafeMappingAssignExpr(x.X)
	case *ast.Ident:
		return strings.TrimSpace(x.Name) != ""
	case *ast.BasicLit:
		switch x.Kind {
		case token.STRING, token.INT, token.FLOAT, token.CHAR:
			return true
		default:
			return false
		}
	case *ast.SelectorExpr:
		return isSafeMappingAssignRef(x.X) && x.Sel != nil && strings.TrimSpace(x.Sel.Name) != ""
	case *ast.IndexExpr:
		return isSafeMappingAssignRef(x.X) && isSafeMappingAssignIndex(x.Index)
	case *ast.IndexListExpr:
		if !isSafeMappingAssignRef(x.X) || len(x.Indices) == 0 {
			return false
		}
		for _, idx := range x.Indices {
			if !isSafeMappingAssignIndex(idx) {
				return false
			}
		}
		return true
	case *ast.UnaryExpr:
		return (x.Op == token.SUB || x.Op == token.ADD || x.Op == token.NOT) && isSafeMappingAssignExpr(x.X)
	case *ast.BinaryExpr:
		switch x.Op {
		case token.ADD, token.SUB, token.MUL, token.QUO, token.REM,
			token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ,
			token.LAND, token.LOR:
			return isSafeMappingAssignExpr(x.X) && isSafeMappingAssignExpr(x.Y)
		}
		return false
	case *ast.CallExpr:
		return false
	case *ast.CompositeLit:
		return true
	case *ast.FuncLit:
		return false
	default:
		return false
	}
}

func flowStringMap(v any) map[string]string {
	if v == nil {
		return nil
	}
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
			out[k] = fmt.Sprint(val)
		}
		return out
	default:
		return nil
	}
}

func isSafeMappingAssignRef(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.ParenExpr:
		return isSafeMappingAssignRef(x.X)
	case *ast.Ident:
		return strings.TrimSpace(x.Name) != ""
	case *ast.SelectorExpr:
		return isSafeMappingAssignRef(x.X) && x.Sel != nil && strings.TrimSpace(x.Sel.Name) != ""
	case *ast.IndexExpr:
		return isSafeMappingAssignRef(x.X) && isSafeMappingAssignIndex(x.Index)
	default:
		return false
	}
}

func isSafeMappingAssignIndex(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.Ident:
		return strings.TrimSpace(x.Name) != ""
	case *ast.BasicLit:
		return x.Kind == token.INT || x.Kind == token.STRING || x.Kind == token.CHAR
	case *ast.SelectorExpr:
		return isSafeMappingAssignRef(x)
	case *ast.IndexExpr:
		return isSafeMappingAssignRef(x)
	case *ast.ParenExpr:
		return isSafeMappingAssignIndex(x.X)
	default:
		return false
	}
}

func isCrossServiceAllowed(allow map[string]map[string]struct{}, serviceName, entityName string) bool {
	if len(allow) == 0 {
		return false
	}
	serviceKey := strings.ToLower(normalizeServiceName(strings.TrimSpace(serviceName)))
	entityKey := strings.ToLower(strings.TrimSpace(entityName))
	if serviceKey == "" || entityKey == "" {
		return false
	}
	entities, ok := allow[serviceKey]
	if !ok {
		return false
	}
	_, ok = entities[entityKey]
	return ok
}
