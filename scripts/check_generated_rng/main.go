// check_generated_rng lints rendered Go and Go runtime templates without running Unity.
package main

import (
	"bytes"
	"fmt"
	"github.com/strogmv/ang/compiler/determinism"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	roots := os.Args[1:]
	if len(roots) == 0 {
		roots = []string{"compiler", "examples"}
	}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == "Library" {
					return filepath.SkipDir
				}
				return nil
			}
			template := strings.HasSuffix(path, ".go.tmpl")
			if !template && !strings.HasSuffix(path, ".go") {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !template && !bytes.Contains(source, []byte("Code generated")) {
				return nil
			}
			// Some general-purpose emitter templates use a templated package identifier.
			// Replace just that declaration so the import parser sees valid Go.
			if template {
				lines := strings.Split(string(source), "\n")
				for i, line := range lines {
					if strings.HasPrefix(line, "package ") {
						lines[i] = "package generated"
						break
					}
				}
				source = []byte(strings.Join(lines, "\n"))
			}
			return determinism.CheckImports(path, source)
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
