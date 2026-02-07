package mcp

import (
	"fmt"
	"os"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
)

// MergeCUEFiles reads the original file, parses the patch, merges them at AST level, and writes back.
func MergeCUEFiles(path string, patchContent string) error {
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

	// 3. Recursive Merge
	mergeDecls(origAST, patchAST.Decls)

	// 4. Format and Save
	res, err := format.Node(origAST)
	if err != nil {
		return fmt.Errorf("format result: %w", err)
	}

	return os.WriteFile(path, res, 0644)
}

func mergeDecls(orig *ast.File, patchDecls []ast.Decl) {
	for _, patchDecl := range patchDecls {
		found := false
		
		// We only merge Fields (x: y) or Structs
		if pField, ok := patchDecl.(*ast.Field); ok {
			pLabel := fmt.Sprint(pField.Label)
			
			for _, oDecl := range orig.Decls {
				if oField, ok := oDecl.(*ast.Field); ok {
					if fmt.Sprint(oField.Label) == pLabel {
						// Found matching label!
						mergeField(oField, pField)
						found = true
						break
					}
				}
			}
		}

		if !found {
			// If not found, just append to declarations
			orig.Decls = append(orig.Decls, patchDecl)
		}
	}
}

func mergeField(orig, patch *ast.Field) {
	oStruct, oOk := orig.Value.(*ast.StructLit)
	pStruct, pOk := patch.Value.(*ast.StructLit)

	if oOk && pOk {
		// Both are structs - recurse!
		mergeStruct(oStruct, pStruct)
	} else {
		// One is not a struct - overwrite the value
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
