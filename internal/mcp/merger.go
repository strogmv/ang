package mcp

import (
	"fmt"
	"os"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
)

// MergeCUEFiles reads the original file, parses the patch, merges them at AST level, and writes back.
// If selector is provided (e.g. "#Impls.CreateTender"), it merges the patch specifically into that node.
func MergeCUEFiles(path string, selector string, patchContent string) error {
	// 1. Parse Original
	origContent, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read original: %w", err)
	}

	var origAST *ast.File
	if os.IsNotExist(err) {
		origAST = &ast.File{}
	} else {
		origAST, err = parser.ParseFile(path, origContent, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse original: %w", err)
		}
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

	// 4. Format and Save
	res, err := format.Node(origAST)
	if err != nil {
		return fmt.Errorf("format result: %w", err)
	}

	return os.WriteFile(path, res, 0644)
}

func mergeAtSelector(orig *ast.File, selector string, patchDecls []ast.Decl) error {
	parts := strings.Split(selector, ".")
	var currentDecls *[]ast.Decl = &orig.Decls

	// Navigate to the parent of the last selector part
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		found := false
		
		for _, decl := range *currentDecls {
			if f, ok := decl.(*ast.Field); ok && fmt.Sprint(f.Label) == part {
				if i == len(parts)-1 {
					// Last part - merge patch here
					mergeField(f, &ast.Field{Value: &ast.StructLit{Elts: patchDecls}})
					return nil
				}
				
				// Not last part - go deeper
				if s, ok := f.Value.(*ast.StructLit); ok {
					currentDecls = &s.Elts
					found = true
					break
				}
				return fmt.Errorf("selector path %s is not a struct", strings.Join(parts[:i+1], "."))
			}
		}

		if !found {
			// Path not found - we could auto-create it, but for safety let's return error for now
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