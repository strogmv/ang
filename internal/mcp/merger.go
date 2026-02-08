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
func GetMergedContent(path string, selector string, patchContent string, force bool) ([]byte, error) {
	// 1. Parse Original
	origContent, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read original: %w", err)
	}

	var origAST *ast.File
	origLines := 0
	origDecls := 0
	if os.IsNotExist(err) {
		origAST = &ast.File{}
	} else {
		origAST, err = parser.ParseFile(path, origContent, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse original: %w", err)
		}
		origLines = bytes.Count(origContent, []byte("\n"))
		origDecls = len(origAST.Decls)
	}

	// 2. Measure Target Node Size
	origTargetSize := 0
	if selector != "" {
		origTargetSize = countNodeLines(findNodeBySelector(origAST, selector))
	}

	// 3. Parse Patch (Clean)
	// We parse patch into a temporary file to extract clean values
	patchAST, err := parser.ParseFile("patch.cue", patchContent, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse patch: %w", err)
	}

	// 4. Apply Merge
	if selector != "" {
		if err := mergeAtSelector(origAST, selector, patchAST.Decls, force); err != nil {
			return nil, err
		}
	} else {
		mergeDecls(origAST, patchAST.Decls, force)
	}

	// 5. Measure Results
	res, err := format.Node(origAST)
	if err != nil {
		return nil, fmt.Errorf("format result: %w", err)
	}

	newLines := bytes.Count(res, []byte("\n"))
	newDecls := len(origAST.Decls)

	// 6. Refined Data Loss Guard
	// 6a. Global Guard (Entity count)
	if selector == "" && origDecls > 3 && newDecls < origDecls {
		// If we lost root level declarations in a global merge, it's suspicious
		// return nil, fmt.Errorf("GUARD: unexpected loss of root declarations (%d -> %d). Use selector for precise updates", origDecls, newDecls)
	}

	// 6b. Node Guard
	if selector != "" && origTargetSize > 5 {
		newTargetSize := countNodeLines(findNodeBySelector(origAST, selector))
		if float64(newTargetSize) < float64(origTargetSize)*ReductionThreshold {
			return nil, fmt.Errorf("CRITICAL_REDUCTION_DETECTED: node [%s] reduced from %d to %d lines. Patch aborted for safety", selector, origTargetSize, newTargetSize)
		}
	}

	// 6c. File Guard (Legacy)
	if selector == "" && origLines > 20 && float64(newLines) < float64(origLines)*ReductionThreshold {
		return nil, fmt.Errorf("CRITICAL_REDUCTION_DETECTED: file reduced from %d to %d lines", origLines, newLines)
	}

	return res, nil
}

func findNodeBySelector(orig *ast.File, selector string) ast.Node {
	if selector == "" { return nil }
	parts := strings.Split(selector, ".")
	var currentDecls []ast.Decl = orig.Decls

	for i, part := range parts {
		found := false
		for _, decl := range currentDecls {
			if f, ok := decl.(*ast.Field); ok && fmt.Sprint(f.Label) == part {
				if i == len(parts)-1 {
					return f.Value
				}
				if s, ok := f.Value.(*ast.StructLit); ok {
					currentDecls = s.Elts
					found = true
					break
				}
			}
		}
		if !found { return nil }
	}
	return nil
}

func countNodeLines(n ast.Node) int {
	if n == nil { return 0 }
	b, err := format.Node(n)
	if err != nil { return 0 }
	return bytes.Count(b, []byte("\n")) + 1
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
					// Found the target field
					if force {
						// Overwrite value. Wrap declarations into a struct if multiple.
						f.Value = declsToValue(patchDecls)
					} else {
						// Deep merge into field value
						mergeField(f, &ast.Field{Value: declsToValue(patchDecls)}, false)
					}
					return nil
				}
				// Go deeper
				if s, ok := f.Value.(*ast.StructLit); ok {
					currentDecls = &s.Elts
					found = true
					break
				}
				// Auto-create struct
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
				newField.Value = declsToValue(patchDecls)
				return nil
			}
			newStruct := &ast.StructLit{}
			newField.Value = newStruct
			currentDecls = &newStruct.Elts
		}
	}
	return nil
}

func declsToValue(decls []ast.Decl) ast.Expr {
	if len(decls) == 0 { return &ast.StructLit{} }
	
	// If the patch is a single field or expression, extract its value
	if len(decls) == 1 {
		if _, ok := decls[0].(*ast.Field); ok {
			return &ast.StructLit{Elts: decls}
		}
	}
	return &ast.StructLit{Elts: decls}
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
		} else if _, ok := patchDecl.(*ast.Package); ok {
			// Ignore package declarations in patches
			continue
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