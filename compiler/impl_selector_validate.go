package compiler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/pkg/names"
)

func validateImplDTOSelectors(services []normalizer.Service) error {
	dtoCatalog := map[string]normalizer.Entity{}
	for _, service := range services {
		for _, method := range service.Methods {
			if method.Input.Name != "" {
				dtoCatalog[method.Input.Name] = method.Input
			}
			if method.Output.Name != "" {
				dtoCatalog[method.Output.Name] = method.Output
			}
		}
	}
	for _, service := range services {
		for _, method := range service.Methods {
			if err := validateMethodImplDTOSelectors(service.Name, method, dtoCatalog); err != nil {
				return err
			}
			if err := validateFlowLogicCallDTOSelectors(method, dtoCatalog); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFlowLogicCallDTOSelectors(method normalizer.Method, catalog map[string]normalizer.Entity) error {
	var visit func([]normalizer.FlowStep) error
	visit = func(steps []normalizer.FlowStep) error {
		for _, step := range steps {
			fn, _ := step.Args["func"].(string)
			_, hasRef := step.Args["funcRef"]
			fn = strings.TrimSpace(fn)
			bindings := map[string]dtoSelectorBinding{
				"req":  {name: method.Input.Name, fields: dtoGoFields(method.Input)},
				"resp": {name: method.Output.Name, fields: dtoGoFields(method.Output)},
				"out":  {name: method.Output.Name, fields: dtoGoFields(method.Output)},
			}
			if step.Action == "logic.Call" {
				for _, expression := range flowCallArgumentExpressions(step.Args["args"]) {
					wrapper := "package impl\nvar _ = " + expression + "\n"
					fset := token.NewFileSet()
					file, err := parser.ParseFile(fset, "logic_call_arg.go", wrapper, parser.AllErrors)
					if err == nil {
						if err := validateDTOSelectorAST(file, fset, bindings, step.File, step.Line, 1); err != nil {
							return err
						}
					}
				}
			}
			if step.Action == "logic.Call" && !hasRef && strings.HasPrefix(fn, "func") {
				wrapper := "package impl\nvar _ = " + fn + "\n"
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "logic_call.go", wrapper, parser.AllErrors)
				if err == nil {
					bindLocalDTOSelectors(file, bindings, catalog)
					if err := validateDTOSelectorAST(file, fset, bindings, step.File, step.Line, 1); err != nil {
						return err
					}
				}
			}
			for _, key := range []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
				if nested, ok := step.Args[key].([]normalizer.FlowStep); ok {
					if err := visit(nested); err != nil {
						return err
					}
				}
			}
			for _, key := range []string{"_cases", "_branches"} {
				if groups, ok := step.Args[key].(map[string][]normalizer.FlowStep); ok {
					for _, nested := range groups {
						if err := visit(nested); err != nil {
							return err
						}
					}
				}
			}
		}
		return nil
	}
	return visit(method.Flow)
}

func flowCallArgumentExpressions(raw any) []string {
	switch values := raw.(type) {
	case string:
		return []string{values}
	case []string:
		return values
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func validateDTOSelectorAST(file *ast.File, fset *token.FileSet, bindings map[string]dtoSelectorBinding, source string, baseLine, wrapperLines int) error {
	var validationErr error
	ast.Inspect(file, func(node ast.Node) bool {
		if validationErr != nil {
			return false
		}
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		root, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		binding, tracked := bindings[root.Name]
		if !tracked || len(binding.fields) == 0 {
			return true
		}
		if _, exists := binding.fields[sel.Sel.Name]; exists {
			return true
		}
		position := fset.Position(sel.Sel.Pos())
		line := baseLine + maxInt(position.Line-wrapperLines-1, 0)
		suggestion := nearestField(sel.Sel.Name, binding.fields)
		validationErr = &DTOSelectorError{File: source, Line: line, Field: sel.Sel.Name, DTO: binding.name, Suggestion: suggestion}
		return false
	})
	return validationErr
}

type dtoSelectorBinding struct {
	name   string
	fields map[string]string
}

type DTOSelectorError struct {
	File       string
	Line       int
	Field      string
	DTO        string
	Suggestion string
}

func (e *DTOSelectorError) Error() string {
	hint := ""
	if e.Suggestion != "" {
		hint = fmt.Sprintf("; did you mean %q?", e.Suggestion)
	}
	return fmt.Sprintf("%s:%d: field %q does not exist in %s%s", e.File, e.Line, e.Field, e.DTO, hint)
}

func validateMethodImplDTOSelectors(serviceName string, method normalizer.Method, catalog ...map[string]normalizer.Entity) error {
	if method.Impl == nil || strings.TrimSpace(method.Impl.Code) == "" {
		return nil
	}

	wrapper := "package impl\nfunc _() {\n" + method.Impl.Code + "\n}\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "impl.go", wrapper, parser.AllErrors)
	if err != nil {
		// Syntax diagnostics are already produced by the normalizer.
		return nil
	}

	types := map[string]normalizer.Entity{method.Input.Name: method.Input, method.Output.Name: method.Output}
	if len(catalog) > 0 {
		for name, entity := range catalog[0] {
			types[name] = entity
		}
	}
	fieldsByRoot := map[string]dtoSelectorBinding{
		"req":  {name: method.Input.Name, fields: dtoGoFields(method.Input)},
		"resp": {name: method.Output.Name, fields: dtoGoFields(method.Output)},
		"out":  {name: method.Output.Name, fields: dtoGoFields(method.Output)},
	}
	bindLocalDTOSelectors(file, fieldsByRoot, types)

	var validationErr error
	ast.Inspect(file, func(node ast.Node) bool {
		if validationErr != nil {
			return false
		}
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		root, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		binding, tracked := fieldsByRoot[root.Name]
		if !tracked || len(binding.fields) == 0 {
			return true
		}
		if _, exists := binding.fields[sel.Sel.Name]; exists {
			return true
		}

		position := fset.Position(sel.Sel.Pos())
		line := sourceLine(method.Source) + maxInt(position.Line-3, 0)
		fileName := sourceFile(method.Source)
		dtoName := binding.name
		suggestion := nearestField(sel.Sel.Name, binding.fields)
		validationErr = &DTOSelectorError{File: fileName, Line: line, Field: sel.Sel.Name, DTO: dtoName, Suggestion: suggestion}
		return false
	})
	return validationErr
}

func dtoGoFields(entity normalizer.Entity) map[string]string {
	fields := make(map[string]string, len(entity.Fields))
	for _, field := range entity.Fields {
		goName := names.ToGoName(field.Name)
		fields[goName] = goName
	}
	return fields
}

func bindLocalDTOSelectors(file *ast.File, bindings map[string]dtoSelectorBinding, types map[string]normalizer.Entity) {
	var bindingForType func(ast.Expr) (dtoSelectorBinding, bool)
	bindingForType = func(expr ast.Expr) (dtoSelectorBinding, bool) {
		name := ""
		switch value := expr.(type) {
		case *ast.Ident:
			name = value.Name
		case *ast.SelectorExpr:
			name = value.Sel.Name
		case *ast.StarExpr:
			return bindingForType(value.X)
		}
		entity, ok := types[name]
		return dtoSelectorBinding{name: name, fields: dtoGoFields(entity)}, ok
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE {
				for index, lhs := range n.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok || index >= len(n.Rhs) {
						continue
					}
					switch rhs := n.Rhs[index].(type) {
					case *ast.CompositeLit:
						if binding, ok := bindingForType(rhs.Type); ok {
							bindings[id.Name] = binding
						} else {
							delete(bindings, id.Name)
						}
					case *ast.Ident:
						if binding, ok := bindings[rhs.Name]; ok {
							bindings[id.Name] = binding
						} else {
							delete(bindings, id.Name)
						}
					}
				}
			}
		case *ast.ValueSpec:
			if binding, ok := bindingForType(n.Type); ok {
				for _, name := range n.Names {
					bindings[name.Name] = binding
				}
			} else {
				for _, name := range n.Names {
					delete(bindings, name.Name)
				}
			}
		}
		return true
	})
}

func nearestField(unknown string, fields map[string]string) string {
	best := ""
	bestDistance := len([]rune(unknown)) + 1
	for candidate := range fields {
		distance := levenshteinDistance(unknown, candidate)
		if distance < bestDistance || distance == bestDistance && candidate < best {
			best, bestDistance = candidate, distance
		}
	}
	if bestDistance > 3 {
		return ""
	}
	return best
}

func levenshteinDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	previous := make([]int, len(br)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, ac := range ar {
		current := make([]int, len(br)+1)
		current[0] = i + 1
		for j, bc := range br {
			cost := 0
			if ac != bc {
				cost = 1
			}
			current[j+1] = minInt(current[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous = current
	}
	return previous[len(br)]
}

func sourceFile(source string) string {
	if i := strings.LastIndex(source, ":"); i >= 0 {
		source = source[:i]
		if j := strings.LastIndex(source, ":"); j >= 0 {
			return source[:j]
		}
	}
	return source
}

func sourceLine(source string) int {
	last := strings.LastIndex(source, ":")
	if last < 0 {
		return 0
	}
	value := source[last+1:]
	prefix := source[:last]
	if previous := strings.LastIndex(prefix, ":"); previous >= 0 {
		if _, err := strconv.Atoi(value); err == nil {
			value = prefix[previous+1:]
		}
	}
	line, _ := strconv.Atoi(value)
	return line
}

func minInt(values ...int) int {
	best := values[0]
	for _, value := range values[1:] {
		if value < best {
			best = value
		}
	}
	return best
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
