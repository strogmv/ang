package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// repositoryMethods maps a generated repository interface (port.XRepository)
// to the names of its methods.
type repositoryMethods map[string]map[string]bool

// collectRepositoryMethods reads the repository interfaces of a generated Go
// backend from <backend>/internal/port. A missing directory is not an error
// (first build, or a target that is not Go): it returns nil.
func collectRepositoryMethods(backendDir string) (repositoryMethods, error) {
	portDir := filepath.Join(backendDir, "internal", "port")
	entries, err := os.ReadDir(portDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(portDir, name))
		if err != nil {
			return nil, err
		}
		files[name] = data
	}
	return parseRepositoryMethods(portDir, files)
}

// parseRepositoryMethods reads the *Repository interfaces of the port files
// (by base name; dir only names them in errors).
func parseRepositoryMethods(dir string, files map[string][]byte) (repositoryMethods, error) {
	out := repositoryMethods{}
	fset := token.NewFileSet()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), files[name], parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !strings.HasSuffix(ts.Name.Name, "Repository") {
					continue
				}
				iface, ok := ts.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				methods := out[ts.Name.Name]
				if methods == nil {
					methods = map[string]bool{}
					out[ts.Name.Name] = methods
				}
				for _, field := range iface.Methods.List {
					if _, isFunc := field.Type.(*ast.FuncType); !isFunc {
						continue
					}
					for _, ident := range field.Names {
						methods[ident.Name] = true
					}
				}
			}
		}
	}
	return out, nil
}

// diffRepositoryRemovals lists what the new generation no longer has: whole
// repository interfaces and methods of the interfaces that remain. Sorted.
func diffRepositoryRemovals(before, after repositoryMethods) []string {
	var removed []string
	for iface, methods := range before {
		now, exists := after[iface]
		if !exists {
			removed = append(removed, "port."+iface+" (the whole interface)")
			continue
		}
		for method := range methods {
			if !now[method] {
				removed = append(removed, "port."+iface+"."+method)
			}
		}
	}
	sort.Strings(removed)
	return removed
}

// repositoryRemovalError explains a refused build. The list is capped so a
// mass removal (the usual sign of a cue/repo problem) stays readable.
func repositoryRemovalError(backendDir string, removed []string) error {
	const shown = 20
	list := removed
	more := ""
	if len(list) > shown {
		more = fmt.Sprintf("\n  … and %d more", len(list)-shown)
		list = list[:shown]
	}
	return fmt.Errorf(
		"this build removes %d repository method(s) from %s that the previous generation had:\n  %s%s\nIf the finders were deleted on purpose, rerun with --accept-removals; otherwise check cue/repo",
		len(removed), filepath.ToSlash(filepath.Join(backendDir, "internal", "port")), strings.Join(list, "\n  "), more,
	)
}
