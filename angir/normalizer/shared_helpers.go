package normalizer

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/scanner"
	gotoken "go/token"
	"go/types"
	"sort"
	"strings"
	"unicode"

	"cuelang.org/go/cue"
)

// Helper kinds: the Go declaration a helper's code holds.
const (
	HelperFunc   = "func"   // func name(…) — callers write name(…)
	HelperMethod = "method" // func (s *Service) name(…) — callers write s.name(…)
	HelperType   = "type"
	HelperVar    = "var"
	HelperConst  = "const"
)

// HelperReceiverType is the receiver a method helper is written with; the
// emitter puts each service's implementation type in its place.
const HelperReceiverType = "Service"

// Helper is Go written once in CUE and called by name from any operation's
// Go (`Helpers: <name>: {code, imports}` in cue/api). Before helpers, a
// project kept such Go in a CUE definition and interpolated it, `\(#XFunc)`,
// into every operation that used it, and the generator emitted one copy per
// operation (item 37 of docs/tech-debt-backlog.md).
type Helper struct {
	Name        string
	Kind        string
	Code        string
	Imports     []string
	Description string
	// Source is where the helper is declared (file:line).
	Source string
	// CodeText is where the first line of the code is, when the code is one
	// multi-line literal (its lines map to CUE one to one).
	CodeText TextPos
	// Uses lists the helpers the code references, sorted.
	Uses []string
}

// HelperError lists every problem of the project's helpers, one per line.
type HelperError struct {
	Problems []string
}

func (e *HelperError) Error() string { return strings.Join(e.Problems, "\n") }

// ExtractHelpers reads `Helpers` of cue/api and checks each declaration on
// its own: the code is one top-level Go declaration named after the key, a
// method is written on (s *Service), and every name the code leaves
// unresolved is a predeclared name, a package (`name.Sel`) or another helper.
func (n *Normalizer) ExtractHelpers(val cue.Value) ([]Helper, error) {
	if !val.Exists() {
		return nil, nil
	}
	hv := val.LookupPath(cue.ParsePath("Helpers"))
	if !hv.Exists() {
		return nil, nil
	}
	iter, err := hv.Fields()
	if err != nil {
		return nil, fmt.Errorf("%s: Helpers: %w", formatPos(hv), err)
	}
	var helpers []Helper
	var problems []string
	for iter.Next() {
		name := strings.TrimSpace(iter.Selector().Unquoted())
		v := iter.Value()
		h := Helper{Name: name, Source: formatPos(v)}
		codeVal := v.LookupPath(cue.ParsePath("code"))
		code, cerr := codeVal.String()
		if cerr != nil || strings.TrimSpace(code) == "" {
			problems = append(problems, fmt.Sprintf("%s: helper %s: code must be a string of Go", h.Source, name))
			continue
		}
		h.Code = code
		if text, ok := multilineLiteralText(codeVal); ok {
			h.CodeText = text
		}
		h.Description = getString(v, "description")
		if iv := v.LookupPath(cue.ParsePath("imports")); iv.Exists() {
			list, lerr := iv.List()
			if lerr != nil {
				problems = append(problems, fmt.Sprintf("%s: helper %s: imports must be a list of strings", formatPos(iv), name))
				continue
			}
			for list.Next() {
				s, serr := list.Value().String()
				if serr != nil {
					problems = append(problems, fmt.Sprintf("%s: helper %s: imports must be a list of strings", formatPos(list.Value()), name))
					continue
				}
				h.Imports = append(h.Imports, s)
			}
		}
		helpers = append(helpers, h)
	}
	sort.Slice(helpers, func(i, j int) bool { return helpers[i].Name < helpers[j].Name })

	byName := map[string]*Helper{}
	byFile := map[string]string{}
	for i := range helpers {
		h := &helpers[i]
		if !isHelperName(h.Name) {
			problems = append(problems, fmt.Sprintf("%s: helper %s: the name must be a Go identifier starting with a lower-case letter (helpers are private to the service package)", h.Source, h.Name))
			continue
		}
		file := HelperFileName(h.Name)
		if other, ok := byFile[file]; ok {
			problems = append(problems, fmt.Sprintf("%s: helper %s: duplicates helper %s (both would be %s)", h.Source, h.Name, other, file))
			continue
		}
		byFile[file] = h.Name
		byName[h.Name] = h
	}
	// Parse every declaration first: whether a name is a method helper is
	// known only then.
	parsed := map[string]*parsedHelper{}
	for i := range helpers {
		h := &helpers[i]
		if byName[h.Name] != h {
			continue
		}
		ph, errs := parseHelper(h)
		problems = append(problems, errs...)
		if ph != nil {
			parsed[h.Name] = ph
		}
	}
	for i := range helpers {
		h := &helpers[i]
		if ph := parsed[h.Name]; ph != nil {
			problems = append(problems, checkHelperRefs(h, ph, byName)...)
		}
	}
	if len(problems) > 0 {
		return nil, &HelperError{Problems: problems}
	}
	return helpers, nil
}

// HelperFileName is the generated file of a helper in internal/service.
// An upper-case letter starts a word after a lower-case letter or a digit,
// and ends an acronym before a lower-case letter: eInvoiceXML → e_invoice_xml,
// parseHTTPBody → parse_http_body.
func HelperFileName(name string) string {
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return "helper__" + b.String() + ".gen.go"
}

func isHelperName(name string) bool {
	if name == "" || !gotoken.IsIdentifier(name) || gotoken.IsKeyword(name) {
		return false
	}
	first := rune(name[0])
	if !unicode.IsLower(first) {
		return false
	}
	return types.Universe.Lookup(name) == nil
}

// helperLine maps a line of the parsed helper source (one header line, then
// the code) to its place in CUE.
func (h *Helper) where(line int) string {
	if h.CodeText.Line > 0 && line > 0 {
		return fmt.Sprintf("%s:%d", h.CodeText.File, h.CodeText.Line+line-1)
	}
	return h.Source
}

type parsedHelper struct {
	fset *gotoken.FileSet
	file *ast.File
	recv *ast.Ident // the Service of func (s *Service), for a method helper
}

func (ph *parsedHelper) line(n ast.Node) int { return ph.fset.Position(n.Pos()).Line - 1 }

// parseHelper parses a helper's code and sets its kind. It returns nil when
// the code is not one declaration of the helper's name.
func parseHelper(h *Helper) (*parsedHelper, []string) {
	const header = "package helper\n"
	fset := gotoken.NewFileSet()
	file, err := goparser.ParseFile(fset, "helper.go", header+h.Code, goparser.ParseComments)
	if err != nil {
		if list, ok := err.(scanner.ErrorList); ok && len(list) > 0 {
			return nil, []string{fmt.Sprintf("%s: helper %s: %s", h.where(list[0].Pos.Line-1), h.Name, list[0].Msg)}
		}
		return nil, []string{fmt.Sprintf("%s: helper %s: %v", h.Source, h.Name, err)}
	}
	lineOf := func(n ast.Node) int { return fset.Position(n.Pos()).Line - 1 }
	if len(file.Imports) > 0 {
		return nil, []string{fmt.Sprintf("%s: helper %s: write packages in imports:, not as an import declaration in the code", h.where(lineOf(file.Imports[0])), h.Name)}
	}
	if len(file.Decls) != 1 {
		return nil, []string{fmt.Sprintf("%s: helper %s: the code must be one top-level declaration named %s, found %d", h.Source, h.Name, h.Name, len(file.Decls))}
	}
	var problems []string
	ph := &parsedHelper{fset: fset, file: file}
	declared := ""
	switch d := file.Decls[0].(type) {
	case *ast.FuncDecl:
		declared = d.Name.Name
		h.Kind = HelperFunc
		if d.Recv != nil {
			h.Kind = HelperMethod
			if isHelperReceiver(d.Recv) {
				ph.recv = d.Recv.List[0].Type.(*ast.StarExpr).X.(*ast.Ident)
			} else {
				problems = append(problems, fmt.Sprintf("%s: helper %s: a method helper is written func (s *%s) %s(…); the emitter puts each service's type in place of %s", h.where(lineOf(d)), h.Name, HelperReceiverType, h.Name, HelperReceiverType))
			}
		}
		if d.Body == nil {
			problems = append(problems, fmt.Sprintf("%s: helper %s: func has no body", h.where(lineOf(d)), h.Name))
		}
	case *ast.GenDecl:
		switch d.Tok {
		case gotoken.TYPE:
			h.Kind = HelperType
		case gotoken.VAR:
			h.Kind = HelperVar
		case gotoken.CONST:
			h.Kind = HelperConst
		}
		if len(d.Specs) != 1 {
			return nil, []string{fmt.Sprintf("%s: helper %s: the code must declare one name, %s", h.where(lineOf(d)), h.Name, h.Name)}
		}
		switch s := d.Specs[0].(type) {
		case *ast.TypeSpec:
			declared = s.Name.Name
		case *ast.ValueSpec:
			if len(s.Names) != 1 {
				return nil, []string{fmt.Sprintf("%s: helper %s: the code must declare one name, %s", h.where(lineOf(d)), h.Name, h.Name)}
			}
			declared = s.Names[0].Name
		}
	}
	if declared != h.Name {
		return nil, append(problems, fmt.Sprintf("%s: helper %s: the code declares %s; a helper's code declares the helper's own name", h.Source, h.Name, declared))
	}
	return ph, problems
}

// checkHelperRefs resolves what a helper's code names outside itself.
func checkHelperRefs(h *Helper, ph *parsedHelper, byName map[string]*Helper) []string {
	file := ph.file
	lineOf := ph.line
	var problems []string

	// What the declaration leaves unresolved comes from outside: predeclared
	// names, packages, other helpers. Anything else would capture a variable
	// of an operation, which a helper cannot see.
	selBase := map[*ast.Ident]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				selBase[id] = true
			}
		}
		return true
	})
	uses := map[string]bool{}
	reported := map[string]bool{}
	for _, id := range file.Unresolved {
		name := id.Name
		if id == ph.recv {
			continue // func (s *Service): the emitter names the type
		}
		if other, ok := byName[name]; ok && name != h.Name {
			if other.Kind == HelperMethod {
				if !reported[name] {
					reported[name] = true
					problems = append(problems, fmt.Sprintf("%s: helper %s: %s is a method helper; call it as s.%s from a method helper (func (s *%s) …)", h.where(lineOf(id)), h.Name, name, name, HelperReceiverType))
				}
				continue
			}
			uses[name] = true
			continue
		}
		if reported[name] {
			continue
		}
		if name == "s" {
			reported[name] = true
			problems = append(problems, fmt.Sprintf("%s: helper %s: uses s, the service; declare it as a method: func (s *%s) %s(…), and call it as s.%s(…)", h.where(lineOf(id)), h.Name, HelperReceiverType, h.Name, h.Name))
			continue
		}
		if name == "req" || name == "resp" {
			// Read as a package by its selectors, but these are an
			// operation's request and response.
			reported[name] = true
			problems = append(problems, fmt.Sprintf("%s: helper %s: %s is not declared: it is an operation's variable, which a helper cannot see; pass what the helper needs as parameters", h.where(lineOf(id)), h.Name, name))
			continue
		}
		if types.Universe.Lookup(name) != nil || selBase[id] || name == "_" {
			continue
		}
		reported[name] = true
		msg := fmt.Sprintf("%s: helper %s: %s is not declared: no helper, package or predeclared name (a helper cannot see an operation's variables; pass them as parameters)", h.where(lineOf(id)), h.Name, name)
		if near := nearestHelper(name, byName); near != "" {
			msg += fmt.Sprintf("; did you mean %s?", near)
		}
		problems = append(problems, msg)
	}
	if h.Kind == HelperMethod {
		// s.name on the receiver names a method helper, called or as a value
		// (Snapshot: s.attachmentSnapshot). Called with a lower-case name it
		// can only be one: the generated service fields are not called so.
		called := map[*ast.SelectorExpr]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					called[sel] = true
				}
			}
			return true
		})
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			recv, ok := sel.X.(*ast.Ident)
			if !ok || recv.Name != "s" || recv.Obj == nil || recv.Obj.Kind != ast.Var {
				return true
			}
			if _, isField := recv.Obj.Decl.(*ast.Field); !isField {
				return true // a local s, not the receiver
			}
			name := sel.Sel.Name
			if !unicode.IsLower(rune(name[0])) {
				return true
			}
			if other, ok := byName[name]; ok && other.Kind == HelperMethod {
				if name != h.Name {
					uses[name] = true
				}
				return true
			}
			if !called[sel] || reported["s."+name] {
				return true
			}
			reported["s."+name] = true
			msg := fmt.Sprintf("%s: helper %s: s.%s is no method helper", h.where(lineOf(sel)), h.Name, name)
			if near := nearestHelper(name, byName); near != "" {
				msg += fmt.Sprintf("; did you mean s.%s?", near)
			}
			problems = append(problems, msg)
			return true
		})
	}
	h.Uses = sortedKeys(uses)
	return problems
}

func isHelperReceiver(recv *ast.FieldList) bool {
	if recv == nil || len(recv.List) != 1 || len(recv.List[0].Names) != 1 || recv.List[0].Names[0].Name != "s" {
		return false
	}
	star, ok := recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	id, ok := star.X.(*ast.Ident)
	return ok && id.Name == HelperReceiverType
}

func nearestHelper(name string, byName map[string]*Helper) string {
	best, bestDist := "", 4
	lower := strings.ToLower(name)
	for other := range byName {
		d := levenshtein(lower, strings.ToLower(other))
		if d < bestDist || (d == bestDist && other < best) {
			best, bestDist = other, d
		}
	}
	return best
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// HelperReferences returns the helpers Go code references directly: a func,
// type, var or const helper by its name (not after a dot), a method helper as
// s.name. calls lists the method helpers the code calls without s. — a
// mistake when the name is not a local of the code.
func HelperReferences(code string, byName map[string]*Helper) (refs []string, bareMethodCalls []string) {
	if len(byName) == 0 || strings.TrimSpace(code) == "" {
		return nil, nil
	}
	fset := gotoken.NewFileSet()
	src := []byte(code)
	file := fset.AddFile("", fset.Base(), len(src))
	var sc scanner.Scanner
	sc.Init(file, src, nil, 0)
	type tok struct {
		t   gotoken.Token
		lit string
	}
	var prev1, prev2 tok // the token before, and the one before that
	found := map[string]bool{}
	bare := map[string]bool{}
	var pending string // a method helper named without s., waiting for "("
	for {
		_, t, lit := sc.Scan()
		if t == gotoken.EOF {
			break
		}
		if pending != "" {
			if t == gotoken.LPAREN {
				bare[pending] = true
			}
			pending = ""
		}
		if t == gotoken.IDENT {
			if h, ok := byName[lit]; ok {
				afterDot := prev1.t == gotoken.PERIOD
				switch {
				case h.Kind == HelperMethod && afterDot && prev2.t == gotoken.IDENT && prev2.lit == "s":
					found[lit] = true
				case h.Kind == HelperMethod && !afterDot:
					pending = lit
				case h.Kind != HelperMethod && !afterDot:
					found[lit] = true
				}
			}
		}
		prev2, prev1 = prev1, tok{t, lit}
	}
	return sortedKeys(found), sortedKeys(bare)
}

// ResolveHelpers gives every operation the helpers its Go reaches, directly
// or through other helpers (Method.Helpers), and their code (Method.HelperCode):
// the analyses that read an operation's Go — repositories it reaches, s.Tx,
// typed objects — see a helper's Go as they saw it when it was pasted in. It
// fails on an operation that calls a method helper without s., and returns
// the helpers no operation reaches.
func ResolveHelpers(helpers []Helper, services []Service) (unused []Helper, err error) {
	if len(helpers) == 0 {
		return nil, nil
	}
	byName := make(map[string]*Helper, len(helpers))
	for i := range helpers {
		byName[helpers[i].Name] = &helpers[i]
	}
	reached := map[string]bool{}
	var problems []string
	for si := range services {
		for mi := range services[si].Methods {
			m := &services[si].Methods[mi]
			m.Helpers, m.HelperCode = nil, ""
			direct, bare := HelperReferences(MethodCodeText(*m), byName)
			for _, name := range bare {
				problems = append(problems, fmt.Sprintf("%s: operation %s.%s calls %s(…), a method helper; write s.%s(…)", m.Source, services[si].Name, m.Name, name, name))
			}
			closure := map[string]bool{}
			var visit func(string)
			visit = func(name string) {
				if closure[name] {
					return
				}
				closure[name] = true
				for _, dep := range byName[name].Uses {
					visit(dep)
				}
			}
			for _, name := range direct {
				visit(name)
			}
			if len(closure) == 0 {
				continue
			}
			m.Helpers = sortedKeys(closure)
			var b strings.Builder
			for _, name := range m.Helpers {
				reached[name] = true
				b.WriteString(byName[name].Code)
				b.WriteByte('\n')
			}
			m.HelperCode = b.String()
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, &HelperError{Problems: problems}
	}
	for _, h := range helpers {
		if !reached[h.Name] {
			unused = append(unused, h)
		}
	}
	return unused, nil
}

// ServiceMethodHelpers returns the method helpers the operations of a service
// reach, sorted: the service gets one copy of each.
func ServiceMethodHelpers(svc Service, kinds map[string]string) []string {
	set := map[string]bool{}
	for _, m := range svc.Methods {
		for _, name := range m.Helpers {
			if kinds[name] == HelperMethod {
				set[name] = true
			}
		}
	}
	return sortedKeys(set)
}
