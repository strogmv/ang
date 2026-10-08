// Package golit keeps generated Go packages small: a function literal that CUE
// pastes into many operations (a `\(#XFunc)` interpolation of a whole table, a
// shared helper) is emitted once as a package-level variable and every copy
// becomes that variable's name.
//
// Only literals that capture nothing are hoisted: every identifier a literal
// leaves unresolved must be a predeclared one (len, string, nil…) or a package
// its file imports. A literal that reads `s`, `req`, `ctx` or a local type of
// its operation stays where it is; the duplicates inside it are still hoisted.
// The result is deterministic (a variable is named by the hash of its tokens),
// so an unchanged project rebuilds to the same files.
package golit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// File is the package file that holds the hoisted literals.
const File = "zz_ang_literals.gen.go"

const (
	// minBytes: a literal shorter than this is cheaper to repeat than to name.
	minBytes = 1024
	// minCopies: hoist when the literal appears at least this many times.
	minCopies = 2
)

// Stats reports what one package lost.
type Stats struct {
	Hoisted  int // distinct literals now declared once
	Replaced int // copies replaced by a name
	Before   int // bytes of the package's generated files before
	After    int // and after (including File)
}

type span struct{ start, end int } // byte offsets, end exclusive

type source struct {
	path    string
	src     []byte
	lits    []span            // every `(func(` … `)` literal, outer before inner
	keys    []string          // token key per literal
	imports map[string]string // package name used in the file → import spec text
}

type candidate struct {
	key     string
	name    string
	from    *source
	at      span
	imports map[string]string
}

// HoistDir hoists the repeated literals of the Go package in dir. It reads
// every *.gen.go file of the package (tests excluded) and rewrites only those.
func HoistDir(dir string) (Stats, error) {
	var st Stats
	entries, err := os.ReadDir(dir)
	if err != nil {
		return st, err
	}
	// A package this pass already rewrote: put its literals back first, so
	// the result depends only on the operations (and a second pass is a no-op).
	prev, prevImports, err := readPrevious(filepath.Join(dir, File))
	if err != nil {
		return st, err
	}
	var files []*source
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".gen.go") || strings.HasSuffix(name, "_test.go") || name == File {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return st, err
		}
		if len(prev) > 0 {
			src = expand(src, prev)
		}
		s, err := scan(path, src)
		if err != nil {
			return st, err
		}
		for name, spec := range prevImports {
			if _, ok := s.imports[name]; !ok {
				s.imports[name] = spec
			}
		}
		st.Before += len(src)
		files = append(files, s)
	}
	_ = os.Remove(filepath.Join(dir, File)) // a stale one from the previous build
	if len(files) == 0 {
		return st, nil
	}

	// Count every literal by its tokens.
	count := map[string]int{}
	first := map[string]*candidate{}
	for _, f := range files {
		for i, k := range f.keys {
			if len(k) < minBytes {
				continue
			}
			count[k]++
			if _, ok := first[k]; !ok {
				first[k] = &candidate{key: k, from: f, at: f.lits[i]}
			}
		}
	}
	chosen := map[string]*candidate{}
	keys := make([]string, 0, len(first))
	for k := range first {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	used := map[string]string{} // import name → spec across the hoisted literals
	for _, k := range keys {
		if count[k] < minCopies {
			continue
		}
		c := first[k]
		imports, ok := closed(c.from.src[c.at.start:c.at.end], c.from.imports)
		if !ok {
			continue
		}
		clash := false
		for name, spec := range imports {
			if prev, ok := used[name]; ok && prev != spec {
				clash = true
			}
		}
		if clash {
			continue
		}
		for name, spec := range imports {
			used[name] = spec
		}
		sum := sha256.Sum256([]byte(k))
		c.name = "angLit" + hex.EncodeToString(sum[:])[:12]
		c.imports = imports
		chosen[k] = c
	}
	if len(chosen) == 0 {
		for _, f := range files {
			if len(prev) > 0 {
				out, err := tidy(f.path, f.src)
				if err != nil {
					return st, err
				}
				f.src = out
				if err := os.WriteFile(f.path, out, 0o644); err != nil {
					return st, err
				}
			}
			st.After += len(f.src)
		}
		return st, nil
	}

	// Rewrite the files: the outermost chosen literals become their names.
	for _, f := range files {
		out, n := replace(f.src, f.lits, f.keys, chosen, span{-1, -1})
		if n == 0 && len(prev) == 0 {
			st.After += len(f.src)
			continue
		}
		st.Replaced += n
		out, err = tidy(f.path, out)
		if err != nil {
			return st, err
		}
		if err := os.WriteFile(f.path, out, 0o644); err != nil {
			return st, err
		}
		st.After += len(out)
	}

	// Declare each chosen literal once; the literals inside it are names too.
	var body bytes.Buffer
	for _, k := range keys {
		c, ok := chosen[k]
		if !ok {
			continue
		}
		text, _ := replace(c.from.src, c.from.lits, c.from.keys, chosen, c.at)
		text = text[c.at.start : len(text)-(len(c.from.src)-c.at.end)]
		fmt.Fprintf(&body, "\n// %s is a literal %d operations repeated (from %s).\nvar %s = %s\n",
			c.name, count[k], filepath.Base(c.from.path), c.name, text)
		st.Hoisted++
	}
	pkg, err := packageName(files[0].src)
	if err != nil {
		return st, err
	}
	specs := make([]string, 0, len(used))
	for _, spec := range used {
		specs = append(specs, spec)
	}
	sort.Strings(specs)
	var file bytes.Buffer
	fmt.Fprintf(&file, "// Code generated by ANG: function literals repeated across operations, declared once. DO NOT EDIT.\n\npackage %s\n", pkg)
	if len(specs) > 0 {
		file.WriteString("\nimport (\n")
		for _, s := range specs {
			file.WriteString("\t" + s + "\n")
		}
		file.WriteString(")\n")
	}
	file.Write(body.Bytes())
	out, err := tidy(filepath.Join(dir, File), file.Bytes())
	if err != nil {
		return st, err
	}
	if err := os.WriteFile(filepath.Join(dir, File), out, 0o644); err != nil {
		return st, err
	}
	st.After += len(out)
	return st, nil
}

// scan finds the function literals of a file and its imports.
func scan(path string, src []byte) (*source, error) {
	fset := token.NewFileSet()
	file := fset.AddFile(path, -1, len(src))
	var sc scanner.Scanner
	var scanErr error
	sc.Init(file, src, func(pos token.Position, msg string) {
		if scanErr == nil {
			scanErr = fmt.Errorf("%s: %s", pos, msg)
		}
	}, 0)
	type tok struct {
		off int
		tok token.Token
		lit string
	}
	var toks []tok
	for {
		pos, t, lit := sc.Scan()
		if t == token.EOF {
			break
		}
		toks = append(toks, tok{off: file.Offset(pos), tok: t, lit: lit})
	}
	if scanErr != nil {
		return nil, scanErr
	}
	// Match the parentheses, then keep `( func (` … `)`.
	match := make([]int, len(toks))
	var stack []int
	for i, t := range toks {
		switch t.tok {
		case token.LPAREN:
			stack = append(stack, i)
		case token.RPAREN:
			if len(stack) > 0 {
				match[stack[len(stack)-1]] = i
				stack = stack[:len(stack)-1]
			}
		}
	}
	s := &source{path: path, src: src, imports: map[string]string{}}
	for i := 0; i+2 < len(toks); i++ {
		if toks[i].tok != token.LPAREN || toks[i+1].tok != token.FUNC || toks[i+2].tok != token.LPAREN || match[i] == 0 {
			continue
		}
		j := match[i]
		var key strings.Builder
		for _, t := range toks[i : j+1] {
			if t.lit != "" && t.tok != token.SEMICOLON {
				key.WriteString(t.lit)
			} else {
				key.WriteString(t.tok.String())
			}
			key.WriteByte(' ')
		}
		s.lits = append(s.lits, span{toks[i].off, toks[j].off + 1})
		s.keys = append(s.keys, key.String())
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		name := guessName(p)
		spec := imp.Path.Value
		if imp.Name != nil {
			name = imp.Name.Name
			spec = imp.Name.Name + " " + imp.Path.Value
		}
		if name == "_" || name == "." {
			continue
		}
		s.imports[name] = spec
	}
	return s, nil
}

var reMajor = regexp.MustCompile(`^v[0-9]+$`)

// guessName is the package name an import path usually declares. A wrong
// guess only makes a literal look like it captures something: it stays put.
func guessName(path string) string {
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	if reMajor.MatchString(name) && len(parts) > 1 {
		name = parts[len(parts)-2]
	}
	if i := strings.IndexAny(name, ".-"); i >= 0 {
		name = name[:i]
	}
	return name
}

// closed reports whether a literal captures nothing, and the imports it uses.
func closed(lit []byte, imports map[string]string) (map[string]string, bool) {
	wrapped := "package p\nvar _ = " + string(lit) + "\n"
	f, err := parser.ParseFile(token.NewFileSet(), "", wrapped, 0)
	if err != nil {
		return nil, false
	}
	vs, ok := f.Decls[0].(*ast.GenDecl)
	if !ok || len(vs.Specs) != 1 {
		return nil, false
	}
	vals := vs.Specs[0].(*ast.ValueSpec).Values
	if len(vals) != 1 {
		return nil, false
	}
	if p, ok := vals[0].(*ast.ParenExpr); !ok {
		return nil, false
	} else if _, ok := p.X.(*ast.FuncLit); !ok {
		return nil, false
	}
	used := map[string]string{}
	for _, id := range f.Unresolved {
		if spec, ok := imports[id.Name]; ok {
			used[id.Name] = spec
			continue
		}
		if types.Universe.Lookup(id.Name) != nil {
			continue
		}
		return nil, false
	}
	return used, true
}

// replace swaps the outermost chosen literals inside `within` (the whole file
// when within.start < 0, else everything strictly inside that literal) for
// their names. It returns the full source with the swaps and their number.
func replace(src []byte, lits []span, keys []string, chosen map[string]*candidate, within span) ([]byte, int) {
	var out bytes.Buffer
	pos, n := 0, 0
	for i, l := range lits {
		if l.start < pos {
			continue // inside a literal already replaced
		}
		if within.start >= 0 && (l.start <= within.start || l.end > within.end) {
			continue
		}
		c, ok := chosen[keys[i]]
		if !ok {
			continue
		}
		out.Write(src[pos:l.start])
		out.WriteString(c.name)
		pos = l.end
		n++
	}
	out.Write(src[pos:])
	return out.Bytes(), n
}

var reLine = regexp.MustCompile(`^//line ([^\s/]+\.go):[0-9]+$`)

// tidy drops imports a rewrite left unused, points the file's own //line
// directives at their real lines again (a directive names the line after it)
// and gofmts the result.
func tidy(path string, src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("golit: %s: %w", filepath.Base(path), err)
	}
	usedNames := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				usedNames[id.Name] = true
			}
		}
		return true
	})
	var drop []span
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		name := guessName(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "_" || name == "." || usedNames[name] {
			continue
		}
		start, end := fset.Position(imp.Pos()).Offset, fset.Position(imp.End()).Offset
		for start > 0 && src[start-1] != '\n' {
			start--
		}
		for end < len(src) && src[end] != '\n' {
			end++
		}
		drop = append(drop, span{start, end + 1})
	}
	if len(drop) > 0 {
		var b bytes.Buffer
		pos := 0
		for _, d := range drop {
			b.Write(src[pos:d.start])
			pos = d.end
		}
		b.Write(src[pos:])
		src = b.Bytes()
	}
	out, err := format.Source(src)
	if err != nil {
		return nil, fmt.Errorf("golit: format %s: %w", filepath.Base(path), err)
	}
	base := filepath.Base(path)
	lines := strings.Split(string(out), "\n")
	for i, l := range lines {
		m := reLine.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		// The file's own directives, and in File the ones copied from the
		// file a literal came from.
		if m[1] == base || base == File {
			lines[i] = fmt.Sprintf("//line %s:%d", base, i+2)
		}
	}
	return []byte(strings.Join(lines, "\n")), nil
}

var reName = regexp.MustCompile(`\bangLit[0-9a-f]{12}\b`)

// readPrevious reads the literals a previous pass declared in File.
func readPrevious(path string) (map[string]string, map[string]string, error) {
	src, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, fmt.Errorf("golit: %s: %w", File, err)
	}
	lits := map[string]string{}
	imports := map[string]string{}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		name, spec := guessName(p), imp.Path.Value
		if imp.Name != nil {
			name, spec = imp.Name.Name, imp.Name.Name+" "+imp.Path.Value
		}
		imports[name] = spec
	}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.VAR {
			continue
		}
		for _, sp := range g.Specs {
			vs := sp.(*ast.ValueSpec)
			if len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			v := vs.Values[0]
			lits[vs.Names[0].Name] = string(src[fset.Position(v.Pos()).Offset:fset.Position(v.End()).Offset])
		}
	}
	return lits, imports, nil
}

// expand puts the previous pass's literals back in place of their names.
func expand(src []byte, lits map[string]string) []byte {
	for i := 0; i < 16 && reName.Match(src); i++ {
		src = reName.ReplaceAllFunc(src, func(name []byte) []byte {
			if text, ok := lits[string(name)]; ok {
				return []byte(text)
			}
			return name
		})
	}
	return src
}

func packageName(src []byte) (string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.PackageClauseOnly)
	if err != nil {
		return "", err
	}
	return f.Name.Name, nil
}

// HoistTree runs HoistDir on every package under root that has generated
// files (vendor, node_modules and dot directories skipped).
func HoistTree(root string) (Stats, error) {
	var total Stats
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != root && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules") {
			return filepath.SkipDir
		}
		matches, _ := filepath.Glob(filepath.Join(path, "*.gen.go"))
		if len(matches) == 0 {
			return nil
		}
		st, err := HoistDir(path)
		if err != nil {
			return fmt.Errorf("golit %s: %w", path, err)
		}
		total.Hoisted += st.Hoisted
		total.Replaced += st.Replaced
		total.Before += st.Before
		total.After += st.After
		return nil
	})
	return total, err
}
