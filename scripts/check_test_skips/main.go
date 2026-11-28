// check_test_skips requires an individually justified entry for every test skip.
// A newly missing fixture must fail its test, never silently remove coverage.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type skip struct {
	Key      string `json:"key"`
	Path     string `json:"path"`
	Function string `json:"function"`
	Call     string `json:"call"`
	Line     int    `json:"line"`
}
type allowance struct {
	Key      string `json:"key"`
	Category string `json:"category"`
	Reason   string `json:"reason"`
}

func scan(root string) ([]skip, error) {
	var out []skip
	err := filepath.WalkDir(root, func(path string, ent fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ent.IsDir() {
			if strings.HasPrefix(ent.Name(), ".") && path != root || ent.Name() == "vendor" || ent.Name() == "testdata" || ent.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, path, nil, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			counts := map[string]int{}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Skip" && selector.Sel.Name != "Skipf" && selector.Sel.Name != "SkipNow" {
					return true
				}
				var buffer bytes.Buffer
				if err := format.Node(&buffer, set, call); err != nil {
					return true
				}
				// The key is the file, the function and the skip call itself: a new skip or a
				// reworded one needs a new entry; an unrelated edit of the function does not.
				hash := sha256.Sum256(buffer.Bytes())
				key := filepath.ToSlash(rel) + ":" + fn.Name.Name + ":" + hex.EncodeToString(hash[:])[:16]
				counts[key]++
				key += fmt.Sprintf(":%d", counts[key])
				out = append(out, skip{key, filepath.ToSlash(rel), fn.Name.Name, buffer.String(), set.Position(call.Pos()).Line})
				return true
			})
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, err
}

func validate(rows []skip, allowed []allowance) []string {
	entries := map[string]allowance{}
	var problems []string
	for _, row := range allowed {
		if _, exists := entries[row.Key]; exists {
			problems = append(problems, "duplicate allowance: "+row.Key)
		}
		if strings.TrimSpace(row.Reason) == "" || strings.TrimSpace(row.Category) == "" || row.Category == "missing-input" {
			problems = append(problems, "unjustified allowance: "+row.Key)
		}
		entries[row.Key] = row
	}
	for _, row := range rows {
		if _, ok := entries[row.Key]; !ok {
			problems = append(problems, fmt.Sprintf("%s:%d %s: unlisted %s; missing fixtures must fail", row.Path, row.Line, row.Function, row.Call))
		}
		delete(entries, row.Key)
	}
	for key := range entries {
		problems = append(problems, "stale allowance: "+key)
	}
	sort.Strings(problems)
	return problems
}

func main() {
	root := flag.String("root", ".", "test source root")
	allow := flag.String("allowlist", "scripts/test-skips.json", "explicit allowed skips and reasons")
	inventory := flag.Bool("inventory", false, "print inventory without changing the allowlist")
	flag.Parse()
	rows, err := scan(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *inventory {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(rows); err != nil {
			panic(err)
		}
		return
	}
	raw, err := os.ReadFile(*allow)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var allowed []allowance
	if err := json.Unmarshal(raw, &allowed); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	problems := validate(rows, allowed)
	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, strings.Join(problems, "\n"))
		os.Exit(1)
	}
	var debt []string
	for _, entry := range allowed {
		if entry.Category == "legacy-precondition" {
			debt = append(debt, entry.Key)
		}
	}
	sort.Strings(debt)
	fmt.Printf("test skip lint: %d explicit exceptions; %d listed scenario preconditions to fix\n", len(rows), len(debt))
	for _, key := range debt {
		fmt.Println("  to fix: " + key)
	}
}
