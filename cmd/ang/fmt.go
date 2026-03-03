package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type fmtResult struct {
	FilesScanned int      `json:"files_scanned"`
	FilesChanged int      `json:"files_changed"`
	ChangedFiles []string `json:"changed_files,omitempty"`
}

func runFmt(args []string) {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	check := fs.Bool("check", false, "check formatting/canonicalization; do not write changes")
	jsonOut := fs.Bool("json", false, "emit machine-readable summary")
	path := fs.String("path", "", "path to CUE directory (default: ./cue if present)")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Fmt FAILED: %v\n", err)
		os.Exit(1)
	}

	targetRoot, err := resolveCueRoot(*path)
	if err != nil {
		fmt.Printf("Fmt FAILED: %v\n", err)
		os.Exit(1)
	}

	res, err := formatCueTree(targetRoot, *check)
	if err != nil {
		fmt.Printf("Fmt FAILED: %v\n", err)
		os.Exit(1)
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"schema": "ang/fmt/v1",
			"check":  *check,
			"path":   filepath.ToSlash(targetRoot),
			"result": res,
		})
	} else {
		if *check {
			fmt.Printf("Fmt check: files_scanned=%d files_changed=%d\n", res.FilesScanned, res.FilesChanged)
		} else {
			fmt.Printf("Fmt applied: files_changed=%d\n", res.FilesChanged)
		}
		for _, f := range res.ChangedFiles {
			fmt.Printf("  - %s\n", f)
		}
	}

	if *check && res.FilesChanged > 0 {
		os.Exit(1)
	}
}

func formatCueTree(root string, checkOnly bool) (fmtResult, error) {
	files, err := collectCueFiles(root)
	if err != nil {
		return fmtResult{}, err
	}
	res := fmtResult{
		FilesScanned: len(files),
	}
	rules := aliasRuleMap()

	for _, file := range files {
		raw, readErr := os.ReadFile(file)
		if readErr != nil {
			return fmtResult{}, readErr
		}
		src := string(raw)
		aliased, _ := rewriteCueActionAliases(src, rules)
		formatted, fmtErr := cueFmtBuffer([]byte(aliased))
		if fmtErr != nil {
			return fmtResult{}, fmt.Errorf("cue fmt %s: %w", file, fmtErr)
		}
		next := string(formatted)
		if next == src {
			continue
		}
		res.FilesChanged++
		res.ChangedFiles = append(res.ChangedFiles, filepath.ToSlash(file))
		if !checkOnly {
			if writeErr := os.WriteFile(file, formatted, 0o644); writeErr != nil {
				return fmtResult{}, writeErr
			}
		}
	}
	return res, nil
}

func cueFmtBuffer(src []byte) ([]byte, error) {
	cmd := exec.Command("cue", "fmt", "-")
	cmd.Stdin = bytes.NewReader(src)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%v: %s", err, stderr.String())
		}
		return nil, err
	}
	return out.Bytes(), nil
}
