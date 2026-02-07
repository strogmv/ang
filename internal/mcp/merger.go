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

// GetMergedContent returns the resulting CUE content after merging.
// If force is true, it overwrites existing nodes instead of deep merging structs.
func GetMergedContent(path string, selector string, patchContent string, force bool) ([]byte, error) {
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
		if err := mergeAtSelector(origAST, selector, patchAST.Decls, force); err != nil {
			return nil, err
		}
	} else {
		mergeDecls(origAST, patchAST.Decls, force)
	}

	res, err := format.Node(origAST)
	if err != nil {
		return nil, fmt.Errorf("format result: %w", err)
	}

	newLines := bytes.Count(res, []byte("\n"))
	if origLines > 20 && float64(newLines) < float64(origLines)*ReductionThreshold {
		return nil, fmt.Errorf("CRITICAL_REDUCTION_DETECTED: file reduced from %d to %d lines", origLines, newLines)
	}

	return res, nil
}

func mergeAtSelector(orig *ast.File, selector string, patchDecls []ast.Decl, force bool) error {
	parts := strings.Split(selector, ".")
	var currentDecls *[]ast.Decl = &orig.Decls

	for i := 0; i < len(parts); i++ {
		part := parts[i]
		found := false
		for _, decl := range *currentDecls {
			if f, ok := decl.(*ast.Field); ok && fmt.Sprint(f.Label) == part {
				if i == len(parts)-1 {
					// Apply merge or overwrite at the target node
					if force {
						f.Value = &ast.StructLit{Elts: patchDecls}
					} else {
						mergeField(f, &ast.Field{Value: &ast.StructLit{Elts: patchDecls}}, false)
					}
					return nil
				}
				if s, ok := f.Value.(*ast.StructLit); ok {
					currentDecls = &s.Elts
					found = true
					break
				}
				// Auto-create struct if navigating deeper
				newStruct := &ast.StructLit{}
				f.Value = newStruct
				currentDecls = &newStruct.Elts
				found = true
				break
			}
		}
		if !found {
			newField := &ast.Field{Label: ast.NewIdent(part)}
			*currentDecls = append(*currentDecls, newField)
			if i == len(parts)-1 {
				newField.Value = &ast.StructLit{Elts: patchDecls}
				return nil
			}
			newStruct := &ast.StructLit{}
			newField.Value = newStruct
			currentDecls = &newStruct.Elts
		}
	}
	return nil
}

func mergeDecls(orig *ast.File, patchDecls []ast.Decl, force bool) {
	for _, patchDecl := range patchDecls {
		if pField, ok := patchDecl.(*ast.Field); ok {
			pLabel := fmt.Sprint(pField.Label)
			found := false
			for _, oDecl := range orig.Decls {
				if oField, ok := oDecl.(*ast.Field); ok {
					if fmt.Sprint(oField.Label) == pLabel {
						mergeField(oField, pField, force)
						found = true
						break
					}
				}
			}
			if !found {
				orig.Decls = append(orig.Decls, patchDecl)
			}
		} else {
			orig.Decls = append(orig.Decls, patchDecl)
		}
	}
}

func mergeField(orig, patch *ast.Field, force bool) {
	oStruct, oOk := orig.Value.(*ast.StructLit)
	pStruct, pOk := patch.Value.(*ast.StructLit)

	if !force && oOk && pOk {
		mergeStruct(oStruct, pStruct, force)
	} else {
		// Overwrite if force is true OR if it's a scalar value
		orig.Value = patch.Value
	}
}

func mergeStruct(orig, patch *ast.StructLit, force bool) {
	for _, pDecl := range patch.Elts {
		if pField, ok := pDecl.(*ast.Field); ok {
			pLabel := fmt.Sprint(pField.Label)
			found := false
			for _, oDecl := range orig.Elts {
				if oField, ok := oDecl.(*ast.Field); ok {
					if fmt.Sprint(oField.Label) == pLabel {
						mergeField(oField, pField, force)
						found = true
						break
					}
				}
			}
			if !found {
				orig.Elts = append(orig.Elts, pDecl)
			}
		} else {
			orig.Elts = append(orig.Elts, pDecl)
		}
	}
}