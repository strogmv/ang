package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func runFix(args []string) {
	fs := flag.NewFlagSet("fix", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	check := fs.Bool("check", false, "check only; do not write changes")
	jsonOut := fs.Bool("json", false, "emit machine-readable summary")
	path := fs.String("path", "", "path to CUE directory (default: ./cue if present)")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Fix FAILED: %v\n", err)
		os.Exit(1)
	}

	targetRoot, err := resolveCueRoot(*path)
	if err != nil {
		fmt.Printf("Fix FAILED: %v\n", err)
		os.Exit(1)
	}

	res, err := canonicalizeCueAliases(targetRoot, *check)
	if err != nil {
		fmt.Printf("Fix FAILED: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"schema": "ang/fix/v1",
			"check":  *check,
			"path":   filepath.ToSlash(targetRoot),
			"result": res,
		})
	} else {
		if *check {
			fmt.Printf("Fix check: files_scanned=%d files_changed=%d replacements=%d\n", res.FilesScanned, res.FilesChanged, res.Replacements)
		} else {
			fmt.Printf("Fix applied: files_changed=%d replacements=%d\n", res.FilesChanged, res.Replacements)
		}
		for _, f := range res.ChangedFiles {
			fmt.Printf("  - %s\n", f)
		}
	}

	if *check && res.FilesChanged > 0 {
		os.Exit(1)
	}
}

func resolveCueRoot(flagPath string) (string, error) {
	if flagPath != "" {
		return filepath.Clean(flagPath), nil
	}
	if st, err := os.Stat("cue"); err == nil && st.IsDir() {
		return "cue", nil
	}
	return ".", nil
}
