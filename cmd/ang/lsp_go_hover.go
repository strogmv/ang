package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Hover inside Go blocks written in CUE: s.<Name>Repo.<Method>, port.<Type> and
// domain.<Type> show the signature or declaration from the generated code, so
// rules like "a delete finder returns (int64, error)" are visible where the call
// is written.

var (
	goHoverRepoCall = regexp.MustCompile(`\bs\.([A-Z]\w*)Repo\.([A-Z]\w*)`)
	goHoverTypeRef  = regexp.MustCompile(`\b(port|domain)\.([A-Z]\w*)`)
)

const goHoverMaxLines = 40

type goDeclIndex struct {
	stamp   time.Time
	types   map[string]string            // "port.UserRepository" → declaration
	methods map[string]map[string]string // "UserRepository" → method → signature
}

type goHoverCache struct {
	mu      sync.Mutex
	root    string
	indexes map[string]*goDeclIndex // "port", "domain"
}

var lspGoHover = &goHoverCache{}

// goHoverAt returns hover markdown and the column span for the Go symbol under
// the cursor on line, or ok=false.
func goHoverAt(workspaceRoot, line string, character int) (value string, start, end int, ok bool) {
	for _, m := range goHoverRepoCall.FindAllStringSubmatchIndex(line, -1) {
		if character < m[0] || character > m[1] {
			continue
		}
		repo, method := line[m[2]:m[3]]+"Repository", line[m[4]:m[5]]
		index := lspGoHover.index(workspaceRoot, "port")
		if signature, found := index.methods[repo][method]; found {
			return "```go\n" + signature + "\n```\n\n`port." + repo + "`", m[0], m[1], true
		}
		return "", 0, 0, false
	}
	for _, m := range goHoverTypeRef.FindAllStringSubmatchIndex(line, -1) {
		if character < m[0] || character > m[1] {
			continue
		}
		pkg, name := line[m[2]:m[3]], line[m[4]:m[5]]
		if decl, found := lspGoHover.index(workspaceRoot, pkg).types[pkg+"."+name]; found {
			return "```go\n" + decl + "\n```", m[0], m[1], true
		}
		return "", 0, 0, false
	}
	return "", 0, 0, false
}

// index parses internal/<pkg> of the workspace once and again only when a file
// in it changes; a syntax check is enough, no type checking is needed.
func (c *goHoverCache) index(workspaceRoot, pkg string) *goDeclIndex {
	dir := filepath.Join(workspaceRoot, "internal", pkg)
	stamp := latestGoFileTime(dir)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.root != workspaceRoot || c.indexes == nil {
		c.root = workspaceRoot
		c.indexes = map[string]*goDeclIndex{}
	}
	if cached, ok := c.indexes[pkg]; ok && cached.stamp.Equal(stamp) {
		return cached
	}
	index := buildGoDeclIndex(dir, pkg)
	index.stamp = stamp
	c.indexes[pkg] = index
	return index
}

func latestGoFileTime(dir string) time.Time {
	var latest time.Time
	entries, err := os.ReadDir(dir)
	if err != nil {
		return latest
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if info, err := entry.Info(); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

func buildGoDeclIndex(dir, pkg string) *goDeclIndex {
	index := &goDeclIndex{types: map[string]string{}, methods: map[string]map[string]string{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return index
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !typeSpec.Name.IsExported() {
					continue
				}
				index.types[pkg+"."+typeSpec.Name.Name] = limitLines(nodeText(fset, &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{typeSpec}}), goHoverMaxLines)
				iface, ok := typeSpec.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				methods := map[string]string{}
				for _, field := range iface.Methods.List {
					fn, ok := field.Type.(*ast.FuncType)
					if !ok || len(field.Names) == 0 {
						continue
					}
					signature := strings.TrimPrefix(nodeText(fset, fn), "func")
					for _, method := range field.Names {
						methods[method.Name] = method.Name + signature
					}
				}
				index.methods[typeSpec.Name.Name] = methods
			}
		}
	}
	return index
}

// nodeText prints a declaration the way gofmt would, aligned fields included.
func nodeText(fset *token.FileSet, node any) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
}

func limitLines(text string, max int) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= max {
		return text
	}
	return strings.Join(lines[:max], "\n") + "\n\t// …"
}
