package mcp

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
)

const ReductionThreshold = 0.7

// GetMergedContent returns the resulting CUE content after merging, but does NOT write to disk.
func GetMergedContent(path string, selector string, patchContent string) ([]byte, error) {
	origContent, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read original: %w", err)
	}

	var origAST *ast.File
	origLines := 0
	if os.IsNotExist(err) {
		origAST = &ast.File{}
	} else {
		origAST, err = parser.ParseFile(path, origContent, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse original: %w", err)
		}
		origLines = bytes.Count(origContent, []byte("\n"))
	}

	patchAST, err := parser.ParseFile("patch.cue", patchContent, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse patch: %w", err)
	}

	if selector != "" {
		if err := mergeAtSelector(origAST, selector, patchAST.Decls); err != nil {
			return nil, err
		}
	} else {
		mergeDecls(origAST, patchAST.Decls)
	}

	res, err := format.Node(origAST)
	if err != nil {
		return nil, fmt.Errorf("format result: %w", err)
	}

	newLines := bytes.Count(res, []byte("\n"))
	if origLines > 10 && float64(newLines) < float64(origLines)*ReductionThreshold {
		return nil, fmt.Errorf("CRITICAL_REDUCTION_DETECTED: new file size (%d lines) is significantly smaller than original (%d lines)", newLines, origLines)
	}

	return res, nil
}

// MergeCUEFiles remains for backward compatibility or simple use cases
func MergeCUEFiles(path string, selector string, patchContent string) error {
	content, err := GetMergedContent(path, selector, patchContent)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

func mergeAtSelector(orig *ast.File, selector string, patchDecls []ast.Decl) error {
	parts := strings.Split(selector, ".")
	var currentDecls *[]ast.Decl = &orig.Decls

	for i := 0; i < len(parts); i++ {
		part := parts[i]
		found := false
		for _, decl := range *currentDecls {
			if f, ok := decl.(*ast.Field); ok && fmt.Sprint(f.Label) == part {
				if i == len(parts)-1 {
					mergeField(f, &ast.Field{Value: &ast.StructLit{Elts: patchDecls}})
					return nil
				}
				if s, ok := f.Value.(*ast.StructLit); ok {
					currentDecls = &s.Elts
					found = true
					break
				}
				return fmt.Errorf("selector path %s is not a struct", strings.Join(parts[:i+1], "."))
			}
		}
		if !found {
			return fmt.Errorf("selector path %s not found", strings.Join(parts[:i+1], "."))
		}
	}
	return nil
}

func mergeDecls(orig *ast.File, patchDecls []ast.Decl) {
	for _, patchDecl := range patchDecls {
		found := false
		if pField, ok := patchDecl.(*ast.Field); ok {
			pLabel := fmt.Sprint(pField.Label)
			for _, oDecl := range orig.Decls {
				if oField, ok := oDecl.(*ast.Field); ok {
					if fmt.Sprint(oField.Label) == pLabel {
						mergeField(oField, pField)
						found = true
						break
					}
				}
			}
		}
		if !found {
			orig.Decls = append(orig.Decls, patchDecl)
		}
	}
}

func mergeField(orig, patch *ast.Field) {
	oStruct, oOk := orig.Value.(*ast.StructLit)
	pStruct, pOk := patch.Value.(*ast.StructLit)

	if oOk && pOk {
		mergeStruct(oStruct, pStruct)
	} else {
		orig.Value = patch.Value
	}
}

func mergeStruct(orig, patch *ast.StructLit) {
	for _, pDecl := range patch.Elts {
		found := false
		if pField, ok := pDecl.(*ast.Field); ok {
			pLabel := fmt.Sprint(pField.Label)
			for _, oDecl := range orig.Elts {
				if oField, ok := oDecl.(*ast.Field); ok {
					if fmt.Sprint(oField.Label) == pLabel {
						mergeField(oField, pField)
						found = true
						break
					}
				}
			}
		}
		if !found {
			orig.Elts = append(orig.Elts, pDecl)
		}
	}
}