package normalizer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
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

func validateNamedReturnImplCode(serviceName, methodName string, method Method, codeVal cue.Value) []Warning {
	if method.Impl == nil || strings.TrimSpace(method.Impl.Code) == "" {
		return nil
	}

	wrapped := "package lint\nfunc _() (resp any, err error) {\n" + method.Impl.Code + "\n}\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", wrapped, parser.AllErrors)
	if err != nil || f == nil || len(f.Decls) == 0 {
		return nil
	}

	decl, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok || decl.Body == nil {
		return nil
	}

	warnFile, warnLine, warnCol := cuePosition(codeVal)

	type violation struct {
		code string
		msg  string
		hint string
		pos  token.Pos
	}

	var out []Warning
	emit := func(v violation) {
		line := warnLine
		col := warnCol
		if nodeLine, nodeCol, ok := wrappedNodePosition(fset, v.pos); ok {
			line = wrappedToCueLine(warnLine, nodeLine)
			col = nodeCol
		}
		out = append(out, Warning{
			Kind:     "impl",
			Code:     v.code,
			Severity: "error",
			Message:  fmt.Sprintf("%s.%s: %s", serviceName, methodName, v.msg),
			Hint:     v.hint,
			File:     warnFile,
			Line:     line,
			Column:   col,
			CUEPath:  codeVal.Path().String(),
		})
	}

	ast.Inspect(decl.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.GenDecl:
			if n.Tok != token.VAR {
				return true
			}
			for _, spec := range n.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					switch name.Name {
					case "resp":
						if method.Output.Name != "" {
							emit(violation{
								code: "IMPL_NAMED_RETURN_RESP_VAR",
								msg:  "do not redeclare 'resp' in impls.go.code when method uses named return",
								hint: "Use assignment 'resp = ...' instead of 'var resp ...'",
								pos:  name.Pos(),
							})
						}
					case "err":
						if ident, ok := vs.Type.(*ast.Ident); ok && ident.Name == "error" {
							emit(violation{
								code: "IMPL_NAMED_RETURN_ERR_VAR",
								msg:  "do not redeclare 'err' as local variable in impls.go.code when method uses named return",
								hint: "Use assignment 'err = ...' instead of 'var err error'",
								pos:  name.Pos(),
							})
						}
					}
				}
			}
		case *ast.AssignStmt:
			if n.Tok != token.DEFINE {
				return true
			}
			// Count how many LHS identifiers are named-return shadows vs truly new.
			// In Go, `x, err := foo()` is valid when x is new — `:=` is required.
			// Only flag when `err` (or `resp`) is the SOLE LHS variable,
			// meaning the developer should use `=` instead of `:=`.
			hasErr := false
			hasResp := false
			hasOther := false
			var errIdent, respIdent *ast.Ident
			for _, lhs := range n.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok {
					hasOther = true
					continue
				}
				switch id.Name {
				case "err":
					hasErr = true
					errIdent = id
				case "resp":
					hasResp = true
					respIdent = id
				case "_":
					// blank identifier doesn't count as a new variable
				default:
					hasOther = true
				}
			}
			// Only flag if no other new variables require `:=`
			if hasErr && !hasOther && errIdent != nil {
				emit(violation{
					code: "IMPL_NAMED_RETURN_ERR_SHORT_DECL",
					msg:  "do not use 'err :=' in impls.go.code when method uses named return",
					hint: "Use assignment 'err = ...' instead of short declaration",
					pos:  errIdent.Pos(),
				})
			}
			if hasResp && !hasOther && method.Output.Name != "" && respIdent != nil {
				emit(violation{
					code: "IMPL_NAMED_RETURN_RESP_SHORT_DECL",
					msg:  "do not use 'resp :=' in impls.go.code when method uses named return",
					hint: "Use assignment 'resp = ...' instead of short declaration",
					pos:  respIdent.Pos(),
				})
			}
		}
		return true
	})

	return out
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

	legacyLoggerRE := regexp.MustCompile(`\bl\.[A-Za-z_][A-Za-z0-9_]*`)
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "http://"); idx >= 0 || strings.Index(line, "https://") >= 0 {
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
		if loc := legacyLoggerRE.FindStringIndex(line); len(loc) == 2 {
			appendWarning(
				"IMPL_LEGACY_LOGGER_ALIAS",
				"legacy logger alias 'l.' is not allowed in impls.go.code",
				"Use slog.* directly (or injected logger variable) instead of legacy alias 'l'.",
				i,
				baseCol+loc[0],
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

func wrappedNodePosition(fset *token.FileSet, pos token.Pos) (line int, col int, ok bool) {
	if !pos.IsValid() {
		return 0, 0, false
	}
	p := fset.Position(pos)
	if p.Line <= 0 {
		return 0, 0, false
	}
	return p.Line, p.Column, true
}

func wrappedToCueLine(cueBaseLine int, wrappedLine int) int {
	// Wrapped body is:
	// 1: package lint
	// 2: func _() ...
	// 3+: original impl code
	// so wrapped line 3 maps to cueBaseLine.
	if wrappedLine <= 3 {
		return cueBaseLine
	}
	return cueBaseLine + (wrappedLine - 3)
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
	return fmt.Sprintf("Unsafe condition expression %q: only refs/literals and boolean comparisons (==, !=, <, <=, >, >=, &&, ||, !) are allowed", value)
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
	if id, ok := expr.(*ast.Ident); ok {
		switch id.Name {
		case "true", "false", "nil":
			return true
		}
	}
	return isSafeMappingAssignExpr(expr)
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

func validateGoSnippet(code string, file string, line int, col int) string {
	if code == "" || strings.Contains(code, "{{") {
		return "" // Skip templates for now
	}
	// Wrap code in a function block
	wrapped := fmt.Sprintf("package dummy\nfunc _() { _ = %s }", code)
	if strings.Contains(code, ";") || strings.Contains(code, "for ") || strings.Contains(code, "if ") {
		wrapped = fmt.Sprintf("package dummy\nfunc _() {\n%s\n}", code)
	}

	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "", wrapped, 0)
	if err != nil {
		return fmt.Sprintf("Invalid Go syntax: %v", err)
	}
	return ""
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

func isSharedArchitectureEntity(entityName string) bool {
	switch {
	case strings.EqualFold(entityName, "Company"),
		strings.EqualFold(entityName, "APIKey"),
		strings.EqualFold(entityName, "Application"),
		strings.EqualFold(entityName, "TenderTemplate"),
		strings.EqualFold(entityName, "TenderTemplateCategory"),
		strings.EqualFold(entityName, "CompanyCategoryScore"),
		strings.EqualFold(entityName, "Counterparty"),
		strings.EqualFold(entityName, "SearchDocument"),
		strings.EqualFold(entityName, "User"),
		strings.EqualFold(entityName, "AuditLog"),
		strings.EqualFold(entityName, "CategoryValue"),
		strings.EqualFold(entityName, "Notification"):
		return true
	default:
		return false
	}
}
