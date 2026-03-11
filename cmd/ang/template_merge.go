package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	cueast "cuelang.org/go/cue/ast"
	cueformat "cuelang.org/go/cue/format"
	cueparser "cuelang.org/go/cue/parser"
	"github.com/strogmv/ang/compiler/emitter"
	pyemitter "github.com/strogmv/ang/compiler/emitter/python"
)

func tryTemplatePreserveMerge(relPath, desired, current string) (string, string, bool) {
	normalized := filepath.ToSlash(strings.TrimSpace(relPath))
	if strings.HasSuffix(normalized, ".cue") {
		if merged, ok := mergeCueTopLevelDecls(desired, current); ok {
			return merged, "cue_ast_top_level_merge", true
		}
	}
	if strings.HasSuffix(normalized, ".py") && pyemitter.ShouldPreserveCustomBlocks(normalized) && strings.Contains(current, "ANG:BEGIN_CUSTOM") {
		return pyemitter.MergeCustomBlocks(desired, current), "python_custom_blocks", true
	}
	if strings.HasSuffix(normalized, ".go") && strings.Contains(current, "ANG:BEGIN_CUSTOM") {
		return emitter.MergeGoCustomBlocksCompat(desired, current, normalized)
	}
	return "", "", false
}

func mergeCueTopLevelDecls(desired, current string) (string, bool) {
	df, err := cueparser.ParseFile("desired.cue", desired, cueparser.ParseComments)
	if err != nil {
		return "", false
	}
	cf, err := cueparser.ParseFile("current.cue", current, cueparser.ParseComments)
	if err != nil {
		return "", false
	}
	if pkgA, pkgB := strings.TrimSpace(df.PackageName()), strings.TrimSpace(cf.PackageName()); pkgA != pkgB {
		return "", false
	}
	importSpecs := make([]*cueast.ImportSpec, 0)
	seenImports := map[string]bool{}
	appendImport := func(spec *cueast.ImportSpec) {
		if spec == nil || spec.Path == nil {
			return
		}
		key := cueImportSpecKey(spec)
		if key == "" || seenImports[key] {
			return
		}
		seenImports[key] = true
		importSpecs = append(importSpecs, spec)
	}
	for _, decl := range df.Decls {
		if imp, ok := decl.(*cueast.ImportDecl); ok {
			for _, spec := range imp.Specs {
				appendImport(spec)
			}
		}
	}
	for _, decl := range cf.Decls {
		if imp, ok := decl.(*cueast.ImportDecl); ok {
			for _, spec := range imp.Specs {
				appendImport(spec)
			}
		}
	}
	desiredKeys := map[string]struct{}{}
	for _, d := range df.Decls {
		key, ok := cueDeclKey(d)
		if !ok {
			continue
		}
		desiredKeys[key] = struct{}{}
	}
	preserve := make([]cueast.Decl, 0)
	for _, d := range cf.Decls {
		key, ok := cueDeclKey(d)
		if !ok {
			continue
		}
		if _, exists := desiredKeys[key]; exists {
			continue
		}
		preserve = append(preserve, d)
	}
	if len(preserve) == 0 {
		return "", false
	}
	mergedDecls := append([]cueast.Decl{}, df.Decls...)
	sort.SliceStable(preserve, func(i, j int) bool {
		ki, _ := cueDeclKey(preserve[i])
		kj, _ := cueDeclKey(preserve[j])
		return ki < kj
	})
	mergedDecls = append(mergedDecls, preserve...)
	finalDecls := make([]cueast.Decl, 0, len(mergedDecls)+2)
	if pkgName := strings.TrimSpace(df.PackageName()); pkgName != "" {
		finalDecls = append(finalDecls, &cueast.Package{Name: cueast.NewIdent(pkgName)})
	}
	if len(importSpecs) > 0 {
		sort.SliceStable(importSpecs, func(i, j int) bool {
			return cueImportSpecKey(importSpecs[i]) < cueImportSpecKey(importSpecs[j])
		})
		finalDecls = append(finalDecls, &cueast.ImportDecl{Specs: importSpecs})
	}
	for _, decl := range mergedDecls {
		switch decl.(type) {
		case *cueast.Package, *cueast.ImportDecl:
			continue
		default:
			finalDecls = append(finalDecls, decl)
		}
	}
	mergedFile := &cueast.File{Filename: df.Filename, Decls: finalDecls}
	formatted, err := cueformat.Node(mergedFile)
	if err != nil {
		return "", false
	}
	return string(formatted), true
}

func cueImportSpecKey(spec *cueast.ImportSpec) string {
	if spec == nil || spec.Path == nil {
		return ""
	}
	alias := ""
	if spec.Name != nil {
		alias = strings.TrimSpace(spec.Name.Name)
	}
	return alias + ":" + strings.TrimSpace(spec.Path.Value)
}

func cueDeclKey(d cueast.Decl) (string, bool) {
	switch x := d.(type) {
	case *cueast.Package:
		return "", false
	case *cueast.ImportDecl:
		return "", false
	case *cueast.Field:
		name, _, err := cueast.LabelName(x.Label)
		if err != nil || strings.TrimSpace(name) == "" {
			return "", false
		}
		return "field:" + name, true
	case *cueast.Attribute:
		return "", false
	case *cueast.CommentGroup:
		return "", false
		// fall through unsupported decls below
	default:
		return fmt.Sprintf("decl:%T", d), true
	}
}
