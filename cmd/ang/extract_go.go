package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// extractGoFacts walks Go source files and extracts entities, repos, services, and events.
func extractGoFacts(root string) (FactsEnvelope, error) {
	var env FactsEnvelope

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		// Skip vendor, testdata
		base := filepath.Base(path)
		if base == "vendor" || base == "testdata" || base == ".git" {
			return filepath.SkipDir
		}
		parseGoDir(path, &env)
		return nil
	})
	return env, err
}

func parseGoDir(dir string, env *FactsEnvelope) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		name := fi.Name()
		// skip test and generated files
		if strings.HasSuffix(name, "_test.go") {
			return false
		}
		if strings.HasSuffix(name, ".pb.go") {
			return false
		}
		if strings.HasSuffix(name, "_mock.go") || strings.HasSuffix(name, "_gen.go") {
			return false
		}
		return true
	}, parser.ParseComments)
	if err != nil {
		return
	}

	for _, pkg := range pkgs {
		isDomainPkg := isDomainPackage(pkg.Name)
		for fileName, file := range pkg.Files {
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				if gd.Tok != token.TYPE {
					continue
				}
				for _, spec := range gd.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					name := ts.Name.Name
					if !ast.IsExported(name) {
						continue
					}
					pos := fset.Position(ts.Pos())
					source := fmt.Sprintf("%s:%d", trimRootPrefix(fileName), pos.Line)

					switch t := ts.Type.(type) {
					case *ast.StructType:
						// Entity: domain package OR has db: tag
						fields := extractStructFields(t)
						hasDBTag := false
						for _, f := range fields {
							if f.DBTag != "" {
								hasDBTag = true
								break
							}
						}
						if isDomainPkg || hasDBTag {
							if isEventName(name) {
								env.Events = append(env.Events, FactEvent{
									Name:          name,
									Source:        source,
									PayloadFields: fields,
								})
							} else {
								env.Entities = append(env.Entities, FactEntity{
									Name:      name,
									TableHint: toSnakePlural(name),
									Source:    source,
									Fields:    fields,
								})
							}
						} else if isEventName(name) {
							// Also catch event structs from non-domain packages
							env.Events = append(env.Events, FactEvent{
								Name:          name,
								Source:        source,
								PayloadFields: extractStructFields(t),
							})
						}

					case *ast.InterfaceType:
						if isRepoName(name) {
							entityName := repoEntityName(name)
							methods := extractRepoMethods(t)
							env.Repositories = append(env.Repositories, FactRepo{
								Entity:  entityName,
								Source:  source,
								Methods: methods,
							})
						} else if isServiceName(name) {
							serviceHint := serviceEntityHint(name)
							ops := extractServiceOps(t, serviceHint, source)
							env.Operations = append(env.Operations, ops...)
						}
					}
				}
			}
		}
	}
}

func isDomainPackage(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "domain", "model", "models", "entity", "entities", "core":
		return true
	}
	return false
}

func isRepoName(name string) bool {
	for _, suffix := range []string{"Repository", "Repo", "Store", "DAO", "Storage"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func isServiceName(name string) bool {
	for _, suffix := range []string{"Service", "UseCase", "Application"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func isEventName(name string) bool {
	for _, suffix := range []string{"Event", "Notification", "Payload"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// repoEntityName strips known repo suffixes.
func repoEntityName(name string) string {
	for _, suffix := range []string{"Repository", "Repo", "Store", "DAO", "Storage"} {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return name
}

// serviceEntityHint derives a service hint from the interface name.
func serviceEntityHint(name string) string {
	for _, suffix := range []string{"Service", "UseCase", "Application"} {
		if strings.HasSuffix(name, suffix) {
			base := strings.TrimSuffix(name, suffix)
			return strings.ToLower(base)
		}
	}
	return strings.ToLower(name)
}

func extractStructFields(st *ast.StructType) []FactField {
	if st.Fields == nil {
		return nil
	}
	var fields []FactField
	for _, field := range st.Fields.List {
		goType := goExprToString(field.Type)
		cueHint := goTypeToCUE(goType)
		jsonTag := ""
		dbTag := ""
		if field.Tag != nil {
			raw := strings.Trim(field.Tag.Value, "`")
			jsonTag = extractTag(raw, "json")
			dbTag = extractTag(raw, "db")
		}
		if len(field.Names) == 0 {
			// embedded field
			name := goType
			if idx := strings.LastIndex(name, "."); idx >= 0 {
				name = name[idx+1:]
			}
			name = strings.TrimPrefix(name, "*")
			fields = append(fields, FactField{
				Name:        name,
				GoType:      goType,
				CueTypeHint: cueHint,
				JSONTag:     jsonTag,
				DBTag:       dbTag,
			})
			continue
		}
		for _, ident := range field.Names {
			if !ast.IsExported(ident.Name) {
				continue
			}
			fields = append(fields, FactField{
				Name:        ident.Name,
				GoType:      goType,
				CueTypeHint: cueHint,
				JSONTag:     jsonTag,
				DBTag:       dbTag,
			})
		}
	}
	return fields
}

func extractRepoMethods(iface *ast.InterfaceType) []FactRepoMethod {
	if iface.Methods == nil {
		return nil
	}
	var methods []FactRepoMethod
	for _, m := range iface.Methods.List {
		for _, ident := range m.Names {
			if !ast.IsExported(ident.Name) {
				continue
			}
			ft, ok := m.Type.(*ast.FuncType)
			if !ok {
				continue
			}
			returns := inferRepoReturns(ft)
			methods = append(methods, FactRepoMethod{
				Name:    ident.Name,
				Returns: returns,
			})
		}
	}
	return methods
}

// inferRepoReturns inspects return types to classify the method.
func inferRepoReturns(ft *ast.FuncType) string {
	if ft.Results == nil || len(ft.Results.List) == 0 {
		return "none"
	}
	// Collect non-error return types
	var nonErr []ast.Expr
	for _, r := range ft.Results.List {
		t := goExprToString(r.Type)
		if t == "error" {
			continue
		}
		nonErr = append(nonErr, r.Type)
	}
	if len(nonErr) == 0 {
		return "none"
	}
	t := goExprToString(nonErr[0])
	switch {
	case strings.HasPrefix(t, "[]"):
		return "many"
	case strings.HasPrefix(t, "*"):
		return "one"
	case t == "int64" || t == "int" || t == "int32":
		return "count"
	default:
		return "one"
	}
}

func extractServiceOps(iface *ast.InterfaceType, serviceHint, srcPrefix string) []FactOp {
	if iface.Methods == nil {
		return nil
	}
	var ops []FactOp
	for _, m := range iface.Methods.List {
		for _, ident := range m.Names {
			if !ast.IsExported(ident.Name) {
				continue
			}
			ft, ok := m.Type.(*ast.FuncType)
			if !ok {
				continue
			}
			var inputFields []FactField
			var outputFields []FactField
			if ft.Params != nil {
				for _, p := range ft.Params.List {
					goType := goExprToString(p.Type)
					if goType == "context.Context" {
						continue
					}
					cueHint := goTypeToCUE(goType)
					if len(p.Names) == 0 {
						inputFields = append(inputFields, FactField{
							Name:        "_",
							GoType:      goType,
							CueTypeHint: cueHint,
						})
						continue
					}
					for _, n := range p.Names {
						inputFields = append(inputFields, FactField{
							Name:        n.Name,
							GoType:      goType,
							CueTypeHint: cueHint,
						})
					}
				}
			}
			if ft.Results != nil {
				for _, r := range ft.Results.List {
					goType := goExprToString(r.Type)
					if goType == "error" {
						continue
					}
					cueHint := goTypeToCUE(goType)
					if len(r.Names) == 0 {
						outputFields = append(outputFields, FactField{
							Name:        "_",
							GoType:      goType,
							CueTypeHint: cueHint,
						})
						continue
					}
					for _, n := range r.Names {
						outputFields = append(outputFields, FactField{
							Name:        n.Name,
							GoType:      goType,
							CueTypeHint: cueHint,
						})
					}
				}
			}
			ops = append(ops, FactOp{
				Name:         ident.Name,
				ServiceHint:  serviceHint,
				Source:       srcPrefix,
				InputFields:  inputFields,
				OutputFields: outputFields,
			})
		}
	}
	return ops
}

// goExprToString converts an AST expression to a Go type string.
func goExprToString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return goExprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + goExprToString(t.X)
	case *ast.ArrayType:
		return "[]" + goExprToString(t.Elt)
	case *ast.MapType:
		return "map[" + goExprToString(t.Key) + "]" + goExprToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + goExprToString(t.Elt)
	case *ast.ChanType:
		return "chan " + goExprToString(t.Value)
	case *ast.FuncType:
		return "func(...)"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// goTypeToCUE maps a Go type string to a CUE type hint.
func goTypeToCUE(goType string) string {
	// strip pointer
	base := strings.TrimPrefix(goType, "*")
	switch base {
	case "string":
		return "string"
	case "bool":
		return "bool"
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return "int"
	case "float32", "float64":
		return "float"
	case "time.Time":
		return "string"
	case "uuid.UUID", "github.com/google/uuid.UUID":
		return "string"
	case "interface{}":
		return "_"
	case "any":
		return "_"
	}
	if strings.HasPrefix(base, "[]") {
		inner := goTypeToCUE(base[2:])
		return "[..." + inner + "]"
	}
	if strings.HasPrefix(base, "map[") {
		return "{...}"
	}
	if strings.HasPrefix(base, "func(") {
		return "_"
	}
	// selector expr (package.Type)
	if idx := strings.LastIndex(base, "."); idx >= 0 {
		short := base[idx+1:]
		return goTypeToCUE(short)
	}
	// unknown struct / type
	return "string"
}

// extractTag parses a struct tag literal and returns the value for key.
func extractTag(tag, key string) string {
	// tag is the raw content between backticks
	search := key + `:"`
	idx := strings.Index(tag, search)
	if idx < 0 {
		return ""
	}
	rest := tag[idx+len(search):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	val := rest[:end]
	// strip omitempty and other options
	if comma := strings.Index(val, ","); comma >= 0 {
		val = val[:comma]
	}
	return val
}

// trimRootPrefix removes current working directory prefix from a path.
func trimRootPrefix(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return path
	}
	return rel
}
