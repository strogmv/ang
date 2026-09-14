package emitter

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"
)

// formatGoStrict formats generated Go source and fails fast on syntax issues.
func formatGoStrict(src []byte, unit string) ([]byte, error) {
	if hinted, ok := withPreviousImports(src, unit); ok {
		if out, err := imports.Process(unit, hinted, nil); err == nil {
			return out, nil
		}
	}
	out, err := imports.Process(unit, src, nil)
	if err != nil {
		return nil, fmt.Errorf("generated go is invalid (%s): %w", unit, err)
	}
	return out, nil
}

// Method bodies written in CUE use packages (strconv, crypto/sha256, project
// packages) that the templates do not import; goimports adds them. To find a
// missing import it parses every other .go file in the unit's directory, then
// asks the module resolver. It does that again for each file, so a service
// directory with hundreds of generated files costs O(n²) parses: on a 740-file
// internal/service this was ~34 s of a ~41 s dry run.
//
// The unit's previous output already names the imports goimports chose for it.
// withPreviousImports adds exactly those that cover the source's unresolved
// references, the way goimports applies its own fixes (unused imports removed,
// then additions sorted by path), so goimports finishes on its first pass
// without reading the directory. When the previous file is missing or does not
// cover every reference, ok is false and the caller takes the full goimports
// path, so new references are still resolved exactly as before.
//
// The previous file is read from unit itself, relative to the working
// directory — the same directory goimports reads siblings from.
func withPreviousImports(src []byte, unit string) ([]byte, bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, unit, src, parser.ParseComments)
	if err != nil {
		return nil, false
	}
	used := packageReferences(file)
	imported := map[string]bool{}
	for _, spec := range file.Imports {
		imported[importIdentifier(spec)] = true
	}
	missing := map[string]bool{}
	for name := range used {
		if !imported[name] {
			missing[name] = true
		}
	}
	if len(missing) == 0 {
		// goimports is already fast here: nothing to resolve.
		return nil, false
	}

	previous, err := os.ReadFile(unit)
	if err != nil {
		return nil, false
	}
	previousFile, err := parser.ParseFile(token.NewFileSet(), unit, previous, parser.ImportsOnly)
	if err != nil {
		return nil, false
	}
	type namedImport struct{ name, path string }
	var additions []namedImport
	for _, spec := range previousFile.Imports {
		id := importIdentifier(spec)
		if !missing[id] {
			continue
		}
		additions = append(additions, namedImport{name: importName(spec), path: importPath(spec)})
		delete(missing, id)
	}
	if len(missing) != 0 {
		return nil, false
	}

	for _, spec := range append([]*ast.ImportSpec(nil), file.Imports...) {
		if !used[importIdentifier(spec)] {
			astutil.DeleteNamedImport(fset, file, importName(spec), importPath(spec))
		}
	}
	sort.Slice(additions, func(i, j int) bool {
		if additions[i].path != additions[j].path {
			return additions[i].path < additions[j].path
		}
		return additions[i].name < additions[j].name
	})
	for _, add := range additions {
		astutil.AddNamedImport(fset, file, add.name, add.path)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// packageReferences mirrors goimports: X in X.Sel where X is not resolved in
// the file and Sel is exported.
func packageReferences(file *ast.File) map[string]bool {
	refs := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok && x.Obj == nil && ast.IsExported(sel.Sel.Name) {
			refs[x.Name] = true
		}
		return true
	})
	return refs
}

func importName(spec *ast.ImportSpec) string {
	if spec.Name == nil {
		return ""
	}
	return spec.Name.Name
}

func importPath(spec *ast.ImportSpec) string {
	p, _ := strconv.Unquote(spec.Path.Value)
	return p
}

func importIdentifier(spec *ast.ImportSpec) string {
	if name := importName(spec); name != "" {
		return name
	}
	return assumedPackageName(importPath(spec))
}

// assumedPackageName is goimports' ImportPathToAssumedName.
func assumedPackageName(importPath string) string {
	base := path.Base(importPath)
	if strings.HasPrefix(base, "v") {
		if _, err := strconv.Atoi(base[1:]); err == nil {
			if dir := path.Dir(importPath); dir != "." {
				base = path.Base(dir)
			}
		}
	}
	base = strings.TrimPrefix(base, "go-")
	if i := strings.IndexFunc(base, notIdentifierRune); i >= 0 {
		base = base[:i]
	}
	return base
}

func notIdentifierRune(ch rune) bool {
	return !('a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' ||
		'0' <= ch && ch <= '9' ||
		ch == '_' ||
		ch >= utf8.RuneSelf && (unicode.IsLetter(ch) || unicode.IsDigit(ch)))
}
