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
	
	// Data Loss Guard: Fix. We compare FINAL state with ORIGINAL state.
	// If original was large, and final became significantly smaller - block.
	if origLines > 20 && float64(newLines) < float64(origLines)*ReductionThreshold {
		return nil, fmt.Errorf("CRITICAL_REDUCTION_DETECTED: file reduced from %d to %d lines. Patch aborted for safety", origLines, newLines)
	}

	return res, nil
}

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
					// STRATEGY: Smart Merge into field
					mergeField(f, &ast.Field{Value: &ast.StructLit{Elts: patchDecls}})
					return nil
				}
				if s, ok := f.Value.(*ast.StructLit); ok {
					currentDecls = &s.Elts
					found = true
					break
				}
				// If it's not a struct but we need to go deeper, we overwrite it with a struct
				newStruct := &ast.StructLit{}
				f.Value = newStruct
				currentDecls = &newStruct.Elts
				found = true
				break
			}
		}
		if !found {
			// Path not found - create it! (Targeted creation)
			newField := &ast.Field{Label: ast.NewIdent(part)}
			*currentDecls = append(*currentDecls, newField)
			if i == len(parts)-1 {
				mergeField(newField, &ast.Field{Value: &ast.StructLit{Elts: patchDecls}})
				return nil
			}
			newStruct := &ast.StructLit{}
			newField.Value = newStruct
			currentDecls = &newStruct.Elts
		}
	}
	return nil
}

func mergeDecls(orig *ast.File, patchDecls []ast.Decl) {
	for _, patchDecl := range patchDecls {
		if pField, ok := patchDecl.(*ast.Field); ok {
			pLabel := fmt.Sprint(pField.Label)
			found := false
			for _, oDecl := range orig.Decls {
				if oField, ok := oDecl.(*ast.Field); ok {
					if fmt.Sprint(oField.Label) == pLabel {
						mergeField(oField, pField)
						found = true
						break
					}
				}
			}
			if !found {
				orig.Decls = append(orig.Decls, patchDecl)
			}
		} else {
			// e.g. Imports or Comments
			orig.Decls = append(orig.Decls, patchDecl)
		}
	}
}

func mergeField(orig, patch *ast.Field) {
	oStruct, oOk := orig.Value.(*ast.StructLit)
	pStruct, pOk := patch.Value.(*ast.StructLit)

	if oOk && pOk {
		// Recursive merge for structs
		mergeStruct(oStruct, pStruct)
	} else {
		// STRATEGY: OVERRIDE for scalars
		// This prevents "conflicting values" in CUE, because we physically replace the AST node.
		orig.Value = patch.Value
	}
}

func mergeStruct(orig, patch *ast.StructLit) {
	for _, pDecl := range patch.Elts {
		if pField, ok := pDecl.(*ast.Field); ok {
			pLabel := fmt.Sprint(pField.Label)
			found := false
			for _, oDecl := range orig.Elts {
				if oField, ok := oDecl.(*ast.Field); ok {
					if fmt.Sprint(oField.Label) == pLabel {
						mergeField(oField, pField)
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
