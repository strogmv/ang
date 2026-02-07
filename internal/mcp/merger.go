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

const ReductionThreshold = 0.7 // Block if new file is < 70% of original size

// MergeCUEFiles reads the original file, parses the patch, merges them at AST level, and writes back.
func MergeCUEFiles(path string, selector string, patchContent string) error {
	// 1. Parse Original
	origContent, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read original: %w", err)
	}

	var origAST *ast.File
	origLines := 0
	if os.IsNotExist(err) {
		origAST = &ast.File{}
	} else {
		origAST, err = parser.ParseFile(path, origContent, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse original: %w", err)
		}
		origLines = bytes.Count(origContent, []byte("\n"))
	}

	// 2. Parse Patch
	patchAST, err := parser.ParseFile("patch.cue", patchContent, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse patch: %w", err)
	}

	// 3. Selective or Global Merge
	if selector != "" {
		if err := mergeAtSelector(origAST, selector, patchAST.Decls); err != nil {
			return err
		}
	} else {
		mergeDecls(origAST, patchAST.Decls)
	}

	// 4. Format and Validate Size
	res, err := format.Node(origAST)
	if err != nil {
		return fmt.Errorf("format result: %w", err)
	}

	newLines := bytes.Count(res, []byte("\n"))
	
	// Data Loss Guard: Check for critical reduction
	if origLines > 10 && float64(newLines) < float64(origLines)*ReductionThreshold {
		return fmt.Errorf("CRITICAL_REDUCTION_DETECTED: new file size (%d lines) is significantly smaller than original (%d lines). Patch rejected to prevent data loss", newLines, origLines)
	}

	return os.WriteFile(path, res, 0644)
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
