package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	cueparser "cuelang.org/go/cue/parser"
)

// dtoFixResult is what `ang fix dto` changed.
type dtoFixResult struct {
	FilesScanned int      `json:"files_scanned"`
	FilesChanged int      `json:"files_changed"`
	ChangedFiles []string `json:"changed_files,omitempty"`
	// Removed lists every `_dto: true` taken out, as file:line.
	Removed []string `json:"removed,omitempty"`
	// Marked lists the declarations that got @dto(only="true") because they
	// had only `_dto: true`, which ANG does not read: they become DTOs, so
	// their generated code changes (no table, no repository).
	Marked []string `json:"marked,omitempty"`
	// Left lists `_dto` fields this fix does not rewrite (not a plain
	// `_dto: true` in the struct of a top-level declaration).
	Left []string `json:"left,omitempty"`
}

// fixDTOMarkers rewrites the deprecated DTO marker (item 36 of
// docs/tech-debt-backlog.md). A top-level declaration whose struct has a
// `_dto: true` field loses the field and gets the attribute
// @dto(only="true") unless it has it already. The edit is textual: every
// other byte of the file stays as it is, except that a file kept in cue fmt
// layout is formatted again.
func fixDTOMarkers(root string, checkOnly bool) (dtoFixResult, error) {
	files, err := collectCueFiles(root)
	if err != nil {
		return dtoFixResult{}, err
	}
	res := dtoFixResult{FilesScanned: len(files)}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			return res, err
		}
		out, removed, marked, left, err := fixDTOMarkersInFile(file, src)
		if err != nil {
			return res, err
		}
		// A file kept in cue fmt layout stays in it: a removed `_dto` may
		// have been the longest label its neighbours were aligned to.
		if string(out) != string(src) {
			if formatted, fmtErr := cueFmtBuffer(src); fmtErr == nil && string(formatted) == string(src) {
				if out, err = cueFmtBuffer(out); err != nil {
					return res, fmt.Errorf("cue fmt %s: %w", file, err)
				}
			}
		}
		res.Removed = append(res.Removed, removed...)
		res.Marked = append(res.Marked, marked...)
		res.Left = append(res.Left, left...)
		if string(out) == string(src) {
			continue
		}
		res.FilesChanged++
		res.ChangedFiles = append(res.ChangedFiles, filepath.ToSlash(file))
		if !checkOnly {
			if err := os.WriteFile(file, out, 0o644); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

type textEdit struct {
	start, end int
	text       string
}

func fixDTOMarkersInFile(name string, src []byte) (out []byte, removed, marked, left []string, err error) {
	f, err := cueparser.ParseFile(name, src, cueparser.ParseComments)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("parse %s: %w", name, err)
	}
	where := func(n ast.Node) string {
		return fmt.Sprintf("%s:%d", filepath.ToSlash(name), n.Pos().Line())
	}
	var edits []textEdit
	topLevel := map[*ast.Field]bool{}
	for _, decl := range f.Decls {
		field, ok := decl.(*ast.Field)
		if !ok {
			continue
		}
		topLevel[field] = true
		var dtoFields []*ast.Field
		for _, st := range declStructs(field.Value) {
			for _, elt := range st.Elts {
				if inner, ok := elt.(*ast.Field); ok && isDTOTrueField(inner) {
					dtoFields = append(dtoFields, inner)
				}
			}
		}
		if len(dtoFields) == 0 {
			continue
		}
		for _, inner := range dtoFields {
			edits = append(edits, removeFieldEdit(src, inner))
			removed = append(removed, where(inner))
		}
		if !hasDTOAttribute(field) {
			end := field.Value.End().Offset()
			edits = append(edits, textEdit{start: end, end: end, text: ` @dto(only="true")`})
			marked = append(marked, where(field)+" "+labelText(field))
		}
	}
	// A `_dto` anywhere else stays, and is reported.
	ast.Walk(f, func(n ast.Node) bool {
		field, ok := n.(*ast.Field)
		if !ok || topLevel[field] {
			return true
		}
		if ident, ok := field.Label.(*ast.Ident); ok && ident.Name == "_dto" && !isRemoved(removed, where(field)) {
			left = append(left, where(field))
		}
		return true
	}, nil)
	if len(edits) == 0 {
		return src, removed, marked, left, nil
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out = append([]byte(nil), src...)
	for _, e := range edits {
		out = append(out[:e.start], append([]byte(e.text), out[e.end:]...)...)
	}
	if _, err := cueparser.ParseFile(name, out, cueparser.ParseComments); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s: the rewritten file does not parse: %w", name, err)
	}
	return out, removed, marked, left, nil
}

func isRemoved(removed []string, at string) bool {
	for _, r := range removed {
		if r == at {
			return true
		}
	}
	return false
}

// declStructs are the struct literals a top-level declaration's value is
// made of: the literal itself, or the operands of a & expression.
func declStructs(expr ast.Expr) []*ast.StructLit {
	switch v := expr.(type) {
	case *ast.StructLit:
		return []*ast.StructLit{v}
	case *ast.BinaryExpr:
		return append(declStructs(v.X), declStructs(v.Y)...)
	case *ast.ParenExpr:
		return declStructs(v.X)
	}
	return nil
}

func isDTOTrueField(field *ast.Field) bool {
	ident, ok := field.Label.(*ast.Ident)
	if !ok || ident.Name != "_dto" {
		return false
	}
	switch v := field.Value.(type) {
	case *ast.BasicLit:
		return v.Value == "true"
	case *ast.Ident:
		return v.Name == "true"
	}
	return false
}

func hasDTOAttribute(field *ast.Field) bool {
	for _, attr := range field.Attrs {
		text := strings.ReplaceAll(attr.Text, " ", "")
		if strings.HasPrefix(text, "@dto(") && strings.Contains(text, `only="true"`) {
			return true
		}
	}
	return false
}

func labelText(field *ast.Field) string {
	if name, _, err := ast.LabelName(field.Label); err == nil {
		return name
	}
	return fmt.Sprint(field.Label)
}

// removeFieldEdit deletes a field: its whole line when nothing else is on it,
// else the field and the comma after it.
func removeFieldEdit(src []byte, field *ast.Field) textEdit {
	start, end := field.Pos().Offset(), field.End().Offset()
	lineStart := start
	for lineStart > 0 && (src[lineStart-1] == ' ' || src[lineStart-1] == '\t') {
		lineStart--
	}
	lineEnd := end
	for lineEnd < len(src) && (src[lineEnd] == ' ' || src[lineEnd] == '\t' || src[lineEnd] == ',') {
		lineEnd++
	}
	if (lineStart == 0 || src[lineStart-1] == '\n') && (lineEnd == len(src) || src[lineEnd] == '\n') {
		if lineEnd < len(src) {
			lineEnd++
		}
		return textEdit{start: lineStart, end: lineEnd}
	}
	for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
		end++
	}
	if end < len(src) && src[end] == ',' {
		end++
		for end < len(src) && src[end] == ' ' {
			end++
		}
	}
	return textEdit{start: start, end: end}
}
