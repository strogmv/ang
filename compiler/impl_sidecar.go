package compiler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang-ir/normalizer"
)

func resolveGoSidecarImpls(projectRoot string, api cue.Value, services []normalizer.Service) ([]normalizer.Service, error) {
	if !api.Exists() || api.IncompleteKind() == cue.BottomKind {
		return services, nil
	}
	operations, err := api.Fields(cue.All())
	if err != nil {
		return services, err
	}
	type operationRef struct {
		service string
		method  string
		value   cue.Value
	}
	var refs []operationRef
	for operations.Next() {
		value := operations.Value()
		service, _ := value.LookupPath(cue.ParsePath("service")).String()
		if strings.TrimSpace(service) == "" {
			continue
		}
		refs = append(refs, operationRef{
			service: normalizeSidecarName(service),
			method:  strings.Trim(operations.Selector().String(), "\""),
			value:   value,
		})
	}

	for si := range services {
		for mi := range services[si].Methods {
			method := &services[si].Methods[mi]
			for _, ref := range refs {
				if !strings.EqualFold(ref.service, services[si].Name) || !strings.EqualFold(ref.method, method.Name) {
					continue
				}
				impl := ref.value.LookupPath(cue.ParsePath("impls.go"))
				if !impl.Exists() {
					continue
				}
				funcRef, _ := impl.LookupPath(cue.ParsePath("funcRef")).String()
				if strings.TrimSpace(funcRef) == "" {
					continue
				}
				code, imports, resolveErr := loadSidecarFunction(projectRoot, ref.value.Pos().Filename(), funcRef)
				if resolveErr != nil {
					return services, fmt.Errorf("%s.%s funcRef %q: %w", services[si].Name, method.Name, funcRef, resolveErr)
				}
				requiresTx, _ := impl.LookupPath(cue.ParsePath("tx")).Bool()
				method.Impl = &normalizer.MethodImpl{Lang: "go", Code: code, Imports: imports, RequiresTx: requiresTx}
			}
		}
	}
	return services, nil
}

func loadSidecarFunction(projectRoot, cueFile, ref string) (string, []string, error) {
	fileRef, functionName, ok := strings.Cut(strings.TrimSpace(ref), "#")
	if !ok || strings.TrimSpace(fileRef) == "" || strings.TrimSpace(functionName) == "" {
		return "", nil, fmt.Errorf("expected path.go#functionName")
	}
	base := filepath.Dir(cueFile)
	if strings.TrimSpace(cueFile) == "" {
		base = projectRoot
	} else if !filepath.IsAbs(cueFile) {
		base = filepath.Dir(filepath.Join(projectRoot, cueFile))
	}
	path := fileRef
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", nil, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", nil, fmt.Errorf("sidecar must stay inside project root")
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.AllErrors)
	if err != nil {
		return "", nil, fmt.Errorf("parse sidecar: %w", err)
	}
	var target *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == functionName {
			target = fn
			break
		}
	}
	if target == nil || target.Body == nil {
		return "", nil, fmt.Errorf("top-level function %q not found", functionName)
	}
	start := fset.Position(target.Body.Lbrace).Offset + 1
	end := fset.Position(target.Body.Rbrace).Offset
	if start < 0 || end < start || end > len(source) {
		return "", nil, fmt.Errorf("invalid function body offsets")
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		pathValue, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			return "", nil, fmt.Errorf("invalid import %s", spec.Path.Value)
		}
		if spec.Name != nil && spec.Name.Name != "." {
			return "", nil, fmt.Errorf("named import %q is not supported in sidecars", spec.Name.Name)
		}
		imports = append(imports, pathValue)
	}
	return strings.TrimSpace(string(source[start:end])), imports, nil
}

func resolveLogicCallSidecars(projectRoot string, services []normalizer.Service) ([]normalizer.Service, error) {
	var resolveSteps func([]normalizer.FlowStep) error
	resolveSteps = func(steps []normalizer.FlowStep) error {
		for i := range steps {
			step := &steps[i]
			if step.Action == "logic.Call" {
				if ref, _ := step.Args["funcRef"].(string); strings.TrimSpace(ref) != "" {
					literal, imports, err := loadSidecarFunctionLiteral(projectRoot, step.File, ref)
					if err != nil {
						return fmt.Errorf("%s:%d logic.Call funcRef %q: %w", step.File, step.Line, ref, err)
					}
					step.Args["func"] = "(" + literal + ")"
					step.Args["_funcRefImports"] = imports
				}
			}
			for _, key := range []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
				if nested, ok := step.Args[key].([]normalizer.FlowStep); ok {
					if err := resolveSteps(nested); err != nil {
						return err
					}
					step.Args[key] = nested
				}
			}
			for _, key := range []string{"_cases", "_branches"} {
				groups, ok := step.Args[key].(map[string][]normalizer.FlowStep)
				if !ok {
					continue
				}
				for name, nested := range groups {
					if err := resolveSteps(nested); err != nil {
						return err
					}
					groups[name] = nested
				}
				step.Args[key] = groups
			}
		}
		return nil
	}
	for si := range services {
		for mi := range services[si].Methods {
			if err := resolveSteps(services[si].Methods[mi].Flow); err != nil {
				return services, fmt.Errorf("%s.%s: %w", services[si].Name, services[si].Methods[mi].Name, err)
			}
		}
	}
	return services, nil
}

func loadSidecarFunctionLiteral(projectRoot, cueFile, ref string) (string, []string, error) {
	fileRef, functionName, ok := strings.Cut(strings.TrimSpace(ref), "#")
	if !ok || fileRef == "" || functionName == "" {
		return "", nil, fmt.Errorf("expected path.go#functionName")
	}
	if cueFile != "" && !filepath.IsAbs(cueFile) {
		cueFile = filepath.Join(projectRoot, cueFile)
	}
	path := fileRef
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(cueFile), path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	root, _ := filepath.Abs(projectRoot)
	rel, relErr := filepath.Rel(root, path)
	if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", nil, fmt.Errorf("sidecar must stay inside project root")
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.AllErrors)
	if err != nil {
		return "", nil, err
	}
	var target *ast.FuncDecl
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == functionName {
			target = fn
			break
		}
	}
	if target == nil || target.Body == nil {
		return "", nil, fmt.Errorf("top-level function %q not found", functionName)
	}
	nameEnd := fset.Position(target.Name.End()).Offset
	bodyEnd := fset.Position(target.Body.Rbrace).Offset + 1
	if nameEnd < 0 || bodyEnd > len(source) || nameEnd >= bodyEnd {
		return "", nil, fmt.Errorf("invalid function offsets")
	}
	literal := "func" + string(source[nameEnd:bodyEnd])
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		if spec.Name != nil {
			return "", nil, fmt.Errorf("named imports are not supported in sidecars")
		}
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return "", nil, err
		}
		imports = append(imports, value)
	}
	return literal, imports, nil
}

func normalizeSidecarName(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for i := range parts {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}
