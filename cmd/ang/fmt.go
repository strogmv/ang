package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	cueformat "cuelang.org/go/cue/format"
	cueparser "cuelang.org/go/cue/parser"
)

// maxFmtPasses bounds the repeat-until-stable loop in formatCueTreeWith; on
// dealingi-back two passes were needed and the third changed nothing.
const maxFmtPasses = 4

type fmtResult struct {
	FilesScanned        int              `json:"files_scanned"`
	FilesChanged        int              `json:"files_changed"`
	ChangedFiles        []string         `json:"changed_files,omitempty"`
	EmbeddedGoFormatted int              `json:"embedded_go_formatted"`
	EmbeddedGoSkipped   []embeddedGoSkip `json:"embedded_go_skipped,omitempty"`
}

func runFmt(args []string) {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	check := fs.Bool("check", false, "check formatting/canonicalization; do not write changes")
	jsonOut := fs.Bool("json", false, "emit machine-readable summary")
	path := fs.String("path", "", "path to CUE directory (default: ./cue if present)")
	goOnly := fs.Bool("go-only", false, "format only Go embedded in CUE strings; keep CUE layout and action names as they are")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Fmt FAILED: %v\n", err)
		os.Exit(1)
	}

	targetRoot, err := resolveCueRoot(*path)
	if err != nil {
		fmt.Printf("Fmt FAILED: %v\n", err)
		os.Exit(1)
	}

	res, err := formatCueTreeWith(targetRoot, *check, *goOnly)
	if err != nil {
		fmt.Printf("Fmt FAILED: %v\n", err)
		os.Exit(1)
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"schema":  "ang/fmt/v1",
			"check":   *check,
			"go_only": *goOnly,
			"path":    filepath.ToSlash(targetRoot),
			"result":  res,
		})
	} else {
		if *check {
			fmt.Printf("Fmt check: files_scanned=%d files_changed=%d embedded_go_formatted=%d\n", res.FilesScanned, res.FilesChanged, res.EmbeddedGoFormatted)
		} else {
			fmt.Printf("Fmt applied: files_changed=%d embedded_go_formatted=%d\n", res.FilesChanged, res.EmbeddedGoFormatted)
		}
		for _, f := range res.ChangedFiles {
			fmt.Printf("  - %s\n", f)
		}
		printEmbeddedGoSkips(res.EmbeddedGoSkipped)
	}

	if *check && res.FilesChanged > 0 {
		os.Exit(1)
	}
}

// printEmbeddedGoSkips reports fragments left unformatted. They are warnings,
// not failures: the Go is used as written. Fragments kept for their line count
// are usually many and all alike, so they are summed up in one line; --json
// lists each.
func printEmbeddedGoSkips(skips []embeddedGoSkip) {
	lineCount := 0
	for _, skip := range skips {
		if skip.Reason == embeddedGoLineCountReason {
			lineCount++
			continue
		}
		fmt.Fprintf(os.Stderr, "warning: %s:%d %s: Go left unformatted: %s\n", filepath.ToSlash(skip.File), skip.Line, skip.Field, skip.Reason)
	}
	if lineCount > 0 {
		fmt.Fprintf(os.Stderr, "note: %d Go fragments left unformatted: gofmt would change their number of lines, which generated // Source: comments refer to (see --json)\n", lineCount)
	}
}

func formatCueTree(root string, checkOnly bool) (fmtResult, error) {
	return formatCueTreeWith(root, checkOnly, false)
}

// formatCueTreeWith formats every CUE file under root. goOnly limits it to Go
// embedded in strings, for projects whose CUE is not kept in cue fmt layout:
// there a full pass would rewrite nearly every file.
func formatCueTreeWith(root string, checkOnly, goOnly bool) (fmtResult, error) {
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
		// One pass is not always enough: cue fmt can re-indent a string so
		// that Go it skipped for its line count becomes formattable. Repeat
		// until nothing changes, so ang fmt --check passes right after ang fmt.
		formatted := []byte(src)
		var goChanged int
		var goSkipped []embeddedGoSkip
		for pass := 0; pass < maxFmtPasses; pass++ {
			input := string(formatted)
			if !goOnly {
				input, _ = rewriteCueActionAliases(input, rules)
			}
			passResult, changed, skipped := formatEmbeddedGo(file, []byte(input))
			goChanged += changed
			goSkipped = skipped
			if !goOnly {
				var fmtErr error
				passResult, fmtErr = cueFmtBuffer(passResult)
				if fmtErr != nil {
					return fmtResult{}, fmt.Errorf("cue fmt %s: %w", file, fmtErr)
				}
			}
			if bytes.Equal(passResult, formatted) {
				break
			}
			formatted = passResult
		}
		res.EmbeddedGoFormatted += goChanged
		res.EmbeddedGoSkipped = append(res.EmbeddedGoSkipped, goSkipped...)
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

// cueFmtBuffer runs `cue fmt` when the CLI is installed. Without it, the same
// formatter runs in process (cue fmt parses with comments and calls
// format.Node), so ang fmt no longer fails on machines without cue.
func cueFmtBuffer(src []byte) ([]byte, error) {
	if _, err := exec.LookPath("cue"); err != nil {
		file, parseErr := cueparser.ParseFile("-", src, cueparser.ParseComments)
		if parseErr != nil {
			return nil, parseErr
		}
		return cueformat.Node(file)
	}
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
