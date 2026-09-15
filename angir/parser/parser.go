package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	cueparser "cuelang.org/go/cue/parser"
)

// keep cue in scope for cue.Value return types
var _ = cue.Value{}

// Parser loads and performs initial validation of CUE models.
type Parser struct {
	ctx        *cue.Context
	moduleRoot string // cached cue.mod root; avoids repeated filesystem walks
}

func New() *Parser {
	return &Parser{
		ctx: cuecontext.New(),
	}
}

// findModuleRoot walks up from absPath to find the directory that contains
// cue.mod/module.cue and caches the result so subsequent calls are O(1).
func (p *Parser) findModuleRoot(absPath string) string {
	if p.moduleRoot != "" {
		return p.moduleRoot
	}
	dir := absPath
	for {
		if _, err := os.Stat(filepath.Join(dir, "cue.mod", "module.cue")); err == nil {
			p.moduleRoot = dir
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// FormatCUELocationError converts CUE error into human-readable advice with locations.
func FormatCUELocationError(err error) string {
	if err == nil {
		return ""
	}

	var msg strings.Builder
	errs := errors.Errors(err)

	for _, e := range errs {
		msg.WriteString(fmt.Sprintf("❌ CUE Error: %v\n", e))

		// Extract positions
		positions := errors.Positions(e)
		if len(positions) > 1 {
			msg.WriteString("   Conflict detected between these locations:\n")
			for i, p := range positions {
				msg.WriteString(fmt.Sprintf("   %d. %s\n", i+1, p.String()))
			}
			msg.WriteString("   💡 Suggestion: These values are incompatible. Check if one should be optional (?) or remove the duplicate definition.\n")
		}
	}

	if msg.Len() == 0 {
		return err.Error()
	}
	return msg.String()
}

// stubFile parses a CUE file and returns a new AST where every top-level
// declaration has its value replaced with _ (top/any).  This lets CUE
// resolve import references without evaluating complex type constraints.
// The result is:  #TypeName: _   and   FieldName: _
// Unifying `_ & {concrete: "value"}` = `{concrete: "value"}`, so all
// concrete values in the importing package remain readable.
func stubFile(name string, src []byte) (*ast.File, error) {
	f, err := cueparser.ParseFile(name, src, cueparser.ParseComments)
	if err != nil {
		// Return just the package declaration on parse error.
		pkg := ""
		if idx := strings.Index(string(src), "package "); idx >= 0 {
			line := string(src)[idx:]
			if nl := strings.IndexByte(line, '\n'); nl > 0 {
				pkg = strings.TrimSpace(line[:nl])
			}
		}
		if pkg == "" {
			pkg = "package stub"
		}
		return cueparser.ParseFile(name, pkg)
	}

	stub := &ast.File{Filename: f.Filename}
	// Copy preamble (package clause + imports — imports will be empty in stub)
	for _, d := range f.Preamble() {
		switch d.(type) {
		case *ast.Package:
			stub.Decls = append(stub.Decls, d)
		// skip import declarations — stubs have no deps
		}
	}

	// Re-declare every top-level identifier as _ so references resolve.
	for _, decl := range f.Decls {
		field, ok := decl.(*ast.Field)
		if !ok {
			continue
		}
		stub.Decls = append(stub.Decls, &ast.Field{
			Label: field.Label,
			Value: ast.NewIdent("_"),
		})
	}
	return stub, nil
}

// makeStubParseFile returns a load.Config.ParseFile callback that replaces
// every file that lives OUTSIDE targetDir with a stub (all exported names → _).
// Files inside targetDir are parsed normally.
func makeStubParseFile(targetDir string) func(string, interface{}, cueparser.Config) (*ast.File, error) {
	return func(name string, src interface{}, cfg cueparser.Config) (*ast.File, error) {
		// Files inside the target package are parsed as-is.
		if strings.HasPrefix(filepath.Clean(name), filepath.Clean(targetDir)) {
			return cueparser.ParseFile(name, src, cueparser.ParseComments)
		}

		// For dependency files (schema, domain, …): read content and stub.
		var content []byte
		switch v := src.(type) {
		case []byte:
			content = v
		case string:
			content = []byte(v)
		default:
			var err error
			content, err = os.ReadFile(name)
			if err != nil {
				// Fall back to normal parse if we can't read the file.
				return cueparser.ParseFile(name, src, cueparser.ParseComments)
			}
		}
		return stubFile(name, content)
	}
}


// LoadDomain loads definitions from a specific path.
//
// Dependency packages (imports from the same CUE module) are loaded as stubs
// where every top-level identifier is replaced with _ (unconstrained).  This
// prevents CUE from evaluating complex schema constraints that can spin all CPU
// cores on large projects (e.g. 100+ api files × 300 schema types).  The
// concrete field values defined in the target package are unaffected because
// `_ & {field: "value"}` = `{field: "value"}`.
func (p *Parser) LoadDomain(path string) (cue.Value, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return cue.Value{}, err
	}
	cfg := &load.Config{
		Dir:       absPath,
		ParseFile: makeStubParseFile(absPath),
	}
	// Setting ModuleRoot explicitly avoids CUE re-scanning the filesystem on
	// every call.  The first successful lookup is cached in p.moduleRoot.
	if root := p.findModuleRoot(absPath); root != "" {
		cfg.ModuleRoot = root
	}
	bis := load.Instances([]string{"."}, cfg)

	if len(bis) == 0 {
		return cue.Value{}, fmt.Errorf("no CUE files found in %s", path)
	}
	if bis[0].Err != nil {
		return cue.Value{}, bis[0].Err
	}

	// BuildInstance evaluates the CUE package. With stub dependencies the
	// constraint graph is trivially small so this completes quickly.
	// We do NOT call v.Validate(cue.All()) — that would re-trigger a full
	// recursive constraint walk. v.Err() is sufficient; semantic violations
	// are caught by the normalizer.
	v := p.ctx.BuildInstance(bis[0])
	if v.Err() != nil {
		return cue.Value{}, v.Err()
	}

	return v, nil
}
