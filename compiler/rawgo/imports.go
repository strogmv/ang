// Package rawgo reads the Go a project writes inside CUE (an operation's
// `impls: go: code` and the lambdas of `logic.Call`) the way the generated file
// will see it: which packages the code reaches and which the file imports.
package rawgo

//go:generate go run ./gen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"
)

// ServiceBaseImports are the imports every generated service method file
// starts with; raw Go may use them without listing them in `imports:`.
func ServiceBaseImports(goModule string) []string {
	imports := []string{
		"context",
		"encoding/json",
		"fmt",
		"log/slog",
		"net/http",
		"os",
		"sort",
		"strings",
		"sync",
		"time",
		"github.com/google/uuid",
		"golang.org/x/crypto/bcrypt",
	}
	for _, rel := range []string{
		"internal/config",
		"internal/domain",
		"internal/pkg/auth",
		"internal/pkg/errors",
		"internal/pkg/helpers",
		"internal/pkg/logger",
		"internal/pkg/presence",
		"internal/port",
	} {
		if goModule == "" {
			imports = append(imports, rel)
			continue
		}
		imports = append(imports, goModule+"/"+rel)
	}
	return imports
}

// NormalizeImport turns an `imports:` entry into an import path the way the
// service emitter does ("http" and "uuid" are shorthands), plus an alias when
// the entry is written `alias "path"`.
func NormalizeImport(entry string) (alias, path string) {
	entry = strings.TrimSpace(entry)
	if fields := strings.Fields(entry); len(fields) == 2 && strings.HasPrefix(fields[1], `"`) {
		alias, entry = fields[0], fields[1]
	}
	path = strings.Trim(entry, `"`)
	switch path {
	case "http":
		path = "net/http"
	case "uuid":
		path = "github.com/google/uuid"
	}
	return alias, path
}

var majorVersion = regexp.MustCompile(`^v[0-9]+$`)
var dotVersion = regexp.MustCompile(`\.v[0-9]+$`)

// PackageName is the name code uses for an import path: its last element
// without a /vN major-version suffix, a gopkg.in .vN suffix, or a go- prefix
// or -go suffix ("github.com/redis/go-redis/v9" -> redis).
func PackageName(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	name := parts[len(parts)-1]
	if majorVersion.MatchString(name) && len(parts) > 1 {
		name = parts[len(parts)-2]
	}
	name = dotVersion.ReplaceAllString(name, "")
	name = strings.TrimPrefix(name, "go-")
	name = strings.TrimSuffix(name, "-go")
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, ".", "")
	return name
}

var stdlibByName = func() map[string][]string {
	out := map[string][]string{}
	for _, path := range stdlibPaths {
		name := PackageName(path)
		out[name] = append(out[name], path)
	}
	for _, paths := range out {
		sort.Strings(paths)
	}
	return out
}()

// StdlibPaths returns the standard-library import paths whose package name is
// name: one for strconv, several for rand (crypto/rand, math/rand,
// math/rand/v2), none for a name that is not a standard package.
func StdlibPaths(name string) []string {
	return stdlibByName[name]
}

// Ref is a use of a package-like name: `name.Sel` where name is not declared
// inside the snippet.
type Ref struct {
	Name string
	Sel  string
	Line int // line within the snippet, 1-based
}

// FuncBodyRefs parses code as the body of a function and returns the
// selectors whose left side is not declared in it. ok is false when the code
// does not parse; the Go compiler reports that.
func FuncBodyRefs(code string) (refs []Ref, ok bool) {
	return refsOf("package rawgo\nfunc _() {\n"+code+"\n}\n", 2)
}

// FuncLiteralRefs parses a function literal (a logic.Call lambda) and returns
// the selectors whose left side is not declared in it.
func FuncLiteralRefs(literal string) (refs []Ref, ok bool) {
	return refsOf("package rawgo\nvar _ = "+literal+"\n", 1)
}

func refsOf(src string, headerLines int) ([]Ref, bool) {
	fset := token.NewFileSet()
	// Object resolution (the default) binds every identifier declared inside
	// the snippet; what stays unresolved comes from outside: a package, a
	// parameter of the generated method, or a flow variable.
	file, err := parser.ParseFile(fset, "rawgo.go", src, 0)
	if err != nil {
		return nil, false
	}
	var refs []Ref
	ast.Inspect(file, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Obj != nil {
			return true
		}
		refs = append(refs, Ref{
			Name: id.Name,
			Sel:  sel.Sel.Name,
			Line: fset.Position(id.Pos()).Line - headerLines,
		})
		return true
	})
	return refs, true
}

// GuessedStdlib returns the standard packages named name that export sel,
// one path per package family (math/rand and math/rand/v2 are one family),
// when there are two or more: goimports then has to pick one for `name.sel`,
// and its pick (math/rand for rand.Read) may not be what the code means
// (crypto/rand). It returns nil when the standard library leaves no choice.
func GuessedStdlib(name, sel string) []string {
	paths := stdlibByName[name]
	if len(paths) < 2 {
		return nil
	}
	families := map[string]string{}
	for _, path := range paths {
		if !exports(path, sel) {
			continue
		}
		family := path
		if parts := strings.Split(path, "/"); len(parts) > 1 && majorVersion.MatchString(parts[len(parts)-1]) {
			family = strings.Join(parts[:len(parts)-1], "/")
		}
		if cur, ok := families[family]; !ok || len(path) < len(cur) {
			families[family] = path
		}
	}
	if len(families) < 2 {
		return nil
	}
	out := make([]string, 0, len(families))
	for _, path := range families {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func exports(path, sel string) bool {
	for _, s := range stdlibExports[path] {
		if s == sel {
			return true
		}
	}
	return false
}
