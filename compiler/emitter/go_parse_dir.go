package emitter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// parseGoPackageFiles parses the .go files of dir (not its subdirectories and
// not *_test.go) that belong to package pkgName, in file name order. It stands
// in for parser.ParseDir, deprecated since Go 1.25; like ParseDir it ignores
// build tags. The first file that fails to parse fails the call.
func parseGoPackageFiles(fset *token.FileSet, dir, pkgName string, mode parser.Mode) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, mode)
		if err != nil {
			return nil, err
		}
		if file.Name.Name == pkgName {
			files = append(files, file)
		}
	}
	return files, nil
}
